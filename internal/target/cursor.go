package target

import (
	"path/filepath"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
)

// Cursor adapts Cursor user skills.
// The managed root is a copy. Drift is CopyDrift on the observed digest.
// Symlink following is unverified.
type Cursor struct{ common }

// NewCursor binds classification of "our" symlinks to repoRoot, usually ~/.skills.
func NewCursor(repoRoot string) *Cursor {
	return &Cursor{common: common{repoRoot: repoRoot}}
}

func (*Cursor) Kind() domain.TargetKind { return domain.TargetCursorUser }

func (*Cursor) SupportsSymlink() bool { return false }

func (*Cursor) Supports(mode domain.SyncMode) bool { return mode == domain.SyncCopy }

// AlsoReads lists roots Cursor is verified to read besides its own.
// The list stays empty until docs/targets.md records a credentialed check.
func (*Cursor) AlsoReads() []TargetID { return nil }

func (*Cursor) DefaultTargets(home string) []domain.Target {
	return []domain.Target{cursorTarget(home)}
}

// Skip reports that Cursor should not get its own root.
// ~/.claude/skills present would register the same skill on a second root.
// The macOS probe did not verify that Cursor reads that directory.
func (c *Cursor) Skip(fsys ports.FS, home string) bool {
	if !cursorInstalled(fsys, home) {
		return false
	}
	return existsDir(fsys, filepath.Join(home, ".claude", "skills"))
}

func (c *Cursor) Detect(fsys ports.FS, home string) DetectResult {
	if !cursorInstalled(fsys, home) {
		return DetectResult{}
	}
	if c.Skip(fsys, home) {
		claude := filepath.Join(home, ".claude", "skills")
		return DetectResult{
			Installed: true,
			Skip:      true,
			Roots:     []string{claude},
			Message:   "cursor already reads " + claude,
		}
	}
	root := filepath.Join(home, ".cursor", "skills")
	return DetectResult{Installed: true, Roots: []string{root}, Message: "cursor skills root " + root}
}

func cursorInstalled(fsys ports.FS, home string) bool {
	return existsDir(fsys, filepath.Join(home, ".cursor"))
}

func cursorTarget(home string) domain.Target {
	return domain.Target{
		ID:      "cursor-user",
		Kind:    domain.TargetCursorUser,
		Root:    filepath.Join(home, ".cursor", "skills"),
		Mode:    domain.SyncCopy,
		Enabled: true,
	}
}
