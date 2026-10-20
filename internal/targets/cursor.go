package targets

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

func (*Cursor) Detect(fsys ports.FS, home string) DetectResult {
	if !cursorInstalled(fsys, home) {
		return DetectResult{}
	}
	root := filepath.Join(home, ".cursor", "skills")
	out := DetectResult{Installed: true, Roots: []string{root}, Message: "cursor skills root " + root}
	if existsDir(fsys, filepath.Join(home, ".claude", "skills")) {
		out.Problems = append(out.Problems, cursorReadProblem(home))
	}
	return out
}

func cursorReadProblem(home string) Problem {
	return Problem{
		Level:   LevelInfo,
		Target:  "cursor-user",
		Path:    filepath.Join(home, ".claude", "skills"),
		Message: "may also be read via ~/.claude/skills",
	}
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
