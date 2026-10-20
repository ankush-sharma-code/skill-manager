package target

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
	newPath := filepath.Join(home, ".agents", "skills")
	out := DetectResult{Installed: true}
	if existsDir(fsys, oldPath) {
		out.Roots = append(out.Roots, oldPath)
	}
	if !existsDir(fsys, newPath) {
		out.Message = "codex skills root " + oldPath
		return out
	}
	out.Roots = append(out.Roots, newPath)
	out.Moved = true
	out.From = oldPath
	out.To = newPath
	out.Message = "codex skills root moved from " + oldPath + " to " + newPath
	return out
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
