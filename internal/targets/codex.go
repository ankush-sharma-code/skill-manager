package targets

import (
	"path/filepath"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
)

// Codex adapts Codex user skills.
// ~/.codex/skills is the managed root and follows symlinks.
// ~/.agents/skills is the newer catalog root. It stays opt-in.
type Codex struct{ common }

// NewCodex binds classification of "our" symlinks to repoRoot, usually ~/.skills.
func NewCodex(repoRoot string) *Codex {
	return &Codex{common: common{repoRoot: repoRoot}}
}

func (*Codex) Kind() domain.TargetKind { return domain.TargetCodexUser }

func (*Codex) SupportsSymlink() bool { return true }

func (*Codex) Supports(mode domain.SyncMode) bool {
	return mode == domain.SyncSymlink || mode == domain.SyncCopy
}

func (*Codex) DefaultTargets(home string) []domain.Target {
	return []domain.Target{codexTarget(home)}
}

func (*Codex) Detect(fsys ports.FS, home string) DetectResult {
	if !codexInstalled(fsys, home) {
		return DetectResult{}
	}
	oldPath := filepath.Join(home, ".codex", "skills")
	out := DetectResult{
		Installed: true,
		Roots:     []string{oldPath},
		Message:   "codex skills root " + oldPath,
	}
	newPath := filepath.Join(home, ".agents", "skills")
	if existsDir(fsys, newPath) {
		out.Problems = append(out.Problems, codexAgentsProblem(newPath))
	}
	return out
}

func codexAgentsProblem(path string) Problem {
	return Problem{
		Level:   LevelInfo,
		Target:  "codex-user",
		Path:    path,
		Message: "enable " + path + " as a custom target",
	}
}

func codexInstalled(fsys ports.FS, home string) bool {
	return existsDir(fsys, filepath.Join(home, ".codex"))
}

func codexTarget(home string) domain.Target {
	return domain.Target{
		ID:      "codex-user",
		Kind:    domain.TargetCodexUser,
		Root:    filepath.Join(home, ".codex", "skills"),
		Mode:    domain.SyncSymlink,
		Enabled: true,
	}
}
