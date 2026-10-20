package targets

import (
	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
)

var (
	_ Adapter = (*Claude)(nil)
	_ Adapter = (*Cursor)(nil)
	_ Adapter = (*Codex)(nil)
	_ Adapter = (*Custom)(nil)
)

// Adapter is one tool's skill root.
// SupportsSymlink reports the verified result in docs/targets.md.
type Adapter interface {
	Kind() domain.TargetKind
	DefaultTargets(home string) []domain.Target
	Detect(fsys ports.FS, home string) DetectResult
	Observe(fsys ports.FS, t domain.Target) ([]domain.Observed, error)
	Supports(mode domain.SyncMode) bool
	SupportsSymlink() bool
	Validate(s domain.Skill) []Issue
}

// DetectResult is what Detect found for one tool.
// Problems carries info that does not change the managed root.
type DetectResult struct {
	Installed bool
	Version   string
	Roots     []string
	Moved     bool
	From      string
	To        string
	Skip      bool
	Message   string
	Problems  []Problem
}

// Issue is one tool-specific validation failure.
type Issue struct {
	Field   string
	Message string
}

// Drift is the digest relationship for a managed copy.
type Drift string

const (
	// DriftNone means the copy matches the repository.
	DriftNone Drift = "none"
	// DriftStale means the copy still matches the ledger and the repository moved.
	DriftStale Drift = "stale"
	// DriftConflict means the copy matches neither the repository nor the ledger.
	DriftConflict Drift = "conflict"
	// DriftForeign means the copy differs from the repository and has no ledger entry.
	DriftForeign Drift = "foreign"
)

// CopyDrift compares a managed copy with the repository digest.
// Cursor copies use this instead of a symlink. ledger is nil when this
// process has not written the slot.
func CopyDrift(slot, repo domain.Digest, ledger *domain.Digest) Drift {
	if slot == repo {
		return DriftNone
	}
	if ledger != nil && *ledger == slot {
		return DriftStale
	}
	if ledger == nil {
		return DriftForeign
	}
	return DriftConflict
}

type common struct {
	repoRoot string
}

func (c common) Observe(fsys ports.FS, t domain.Target) ([]domain.Observed, error) {
	res, err := Inspect(fsys, t, c.repoRoot)
	if err != nil {
		return nil, err
	}
	return res.Observed, nil
}

func (c common) Classify(fsys ports.FS, t domain.Target, name string) (domain.Observed, error) {
	return classify(fsys, t, c.repoRoot, name)
}

func (c common) Validate(s domain.Skill) []Issue {
	return validateSkill(s)
}

func validateSkill(s domain.Skill) []Issue {
	var out []Issue
	if _, err := domain.ParseSkillName(string(s.Name)); err != nil {
		out = append(out, Issue{Field: "name", Message: "invalid skill name"})
	}
	if err := domain.ValidateDescription(s.Description); err != nil {
		out = append(out, Issue{Field: "description", Message: "invalid description"})
	}
	if !hasSkillMD(s.Files) {
		out = append(out, Issue{Field: "files", Message: "SKILL.md is required"})
	}
	return out
}

func hasSkillMD(files []domain.FileEntry) bool {
	for _, file := range files {
		if file.RelPath == "SKILL.md" {
			return true
		}
	}
	return false
}
