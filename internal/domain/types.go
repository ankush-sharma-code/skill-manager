package domain

import (
	"io/fs"
	"time"
)

// SkillName is a repository skill name. ParseSkillName is the only constructor.
type SkillName string

// Digest is a SHA-256 sum. The skill digest is a canonical tree digest.
// A file entry digest is the SHA-256 of that file's bytes.
type Digest [32]byte

// FileEntry is one file in a skill tree.
type FileEntry struct {
	RelPath string
	Mode    fs.FileMode
	Size    int64
	Digest  Digest
}

// Provenance records where a skill came from.
type Provenance struct {
	Kind     string
	TargetID string
	Path     string
}

const (
	ProvenanceCreated  = "created"
	ProvenanceImported = "imported"
)

// Skill is one validated skill. Meta holds frontmatter keys other than name and description.
type Skill struct {
	Name        SkillName
	Description string
	Meta        map[string]any
	Files       []FileEntry
	Digest      Digest
	Source      Provenance
}

type TargetKind string

const (
	TargetClaudeUser    TargetKind = "claude-user"
	TargetClaudeProject TargetKind = "claude-project"
	TargetCursorUser    TargetKind = "cursor-user"
	TargetCodexUser     TargetKind = "codex-user"
	TargetCustom        TargetKind = "custom"
)

type SyncMode string

const (
	SyncSymlink SyncMode = "symlink"
	SyncCopy    SyncMode = "copy"
)

// Target is a configured tool directory that receives skills.
type Target struct {
	ID      string
	Kind    TargetKind
	Root    string
	Mode    SyncMode
	Enabled bool
	Include []SkillName
	Exclude []SkillName
}

type SlotState string

const (
	SlotMissing        SlotState = "Missing"
	SlotOurSymlink     SlotState = "OurSymlink"
	SlotForeignSymlink SlotState = "ForeignSymlink"
	SlotDir            SlotState = "Dir"
	SlotFile           SlotState = "File"
	SlotBroken         SlotState = "Broken"
)

// Observed is what is on disk in one target slot.
type Observed struct {
	Target string
	Name   SkillName
	State  SlotState
	Digest Digest
	LinkTo string
}

// SlotKey identifies a ledger entry.
type SlotKey struct {
	Target string
	Name   SkillName
}

// LedgerEntry is the last state written for one slot.
type LedgerEntry struct {
	Digest Digest
	Mode   SyncMode
	Time   time.Time
}

// Ledger is the last state written, per target and skill name.
type Ledger struct {
	Entries map[SlotKey]LedgerEntry
}

type ActionKind string

const (
	ActionLink       ActionKind = "Link"
	ActionCopy       ActionKind = "Copy"
	ActionReplace    ActionKind = "Replace"
	ActionRemove     ActionKind = "Remove"
	ActionAdoptDrift ActionKind = "AdoptDrift"
	ActionConflict   ActionKind = "Conflict"
	ActionSkip       ActionKind = "Skip"
)

// Action is one step the planner emits.
type Action struct {
	Kind   ActionKind
	Target string
	Name   SkillName
	Reason string
}

// UsageEvent is one counted skill invocation.
type UsageEvent struct {
	Skill   SkillName
	Tool    string
	At      time.Time
	Session string
	Project string
	Source  string
}
