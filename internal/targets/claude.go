package targets

import (
	"errors"
	"path/filepath"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
)

// Claude adapts Claude Code user skills.
// Symlink following is unverified, so the default mode is copy.
type Claude struct{ common }

// NewClaude binds classification of "our" symlinks to repoRoot, usually ~/.skills.
func NewClaude(repoRoot string) *Claude {
	return &Claude{common: common{repoRoot: repoRoot}}
}

func (*Claude) Kind() domain.TargetKind { return domain.TargetClaudeUser }

func (*Claude) SupportsSymlink() bool { return false }

func (*Claude) Supports(mode domain.SyncMode) bool { return mode == domain.SyncCopy }

func (*Claude) DefaultTargets(home string) []domain.Target {
	return []domain.Target{claudeTarget(home)}
}

func (*Claude) Detect(fsys ports.FS, home string) DetectResult {
	if !claudeInstalled(fsys, home) {
		return DetectResult{}
	}
	root := filepath.Join(home, ".claude", "skills")
	return DetectResult{Installed: true, Roots: []string{root}, Message: "claude skills root " + root}
}

func claudeInstalled(fsys ports.FS, home string) bool {
	return existsDir(fsys, filepath.Join(home, ".claude"))
}

func claudeTarget(home string) domain.Target {
	return domain.Target{
		ID:      "claude-user",
		Kind:    domain.TargetClaudeUser,
		Root:    filepath.Join(home, ".claude", "skills"),
		Mode:    domain.SyncCopy,
		Enabled: true,
	}
}

// ClaudeProject is the opt-in copy target for one repository.
func ClaudeProject(repoDir string) (domain.Target, error) {
	if !filepath.IsAbs(repoDir) {
		return domain.Target{}, errors.New("project directory must be absolute")
	}
	return domain.Target{
		ID:      "claude-project",
		Kind:    domain.TargetClaudeProject,
		Root:    filepath.Join(repoDir, ".claude", "skills"),
		Mode:    domain.SyncCopy,
		Enabled: false,
	}, nil
}
