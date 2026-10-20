package target

import (
	"errors"
	"path/filepath"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
)

// Custom is an opt-in root for a tool this package does not detect.
// Symlink support is not verified. Either mode is allowed when the user asks.
type Custom struct{ common }

// NewCustom binds classification of "our" symlinks to repoRoot, usually ~/.skills.
func NewCustom(repoRoot string) *Custom {
	return &Custom{common: common{repoRoot: repoRoot}}
}

func (*Custom) Kind() domain.TargetKind { return domain.TargetCustom }

func (*Custom) SupportsSymlink() bool { return false }

func (*Custom) Supports(mode domain.SyncMode) bool {
	return mode == domain.SyncSymlink || mode == domain.SyncCopy
}

func (*Custom) DefaultTargets(string) []domain.Target { return nil }

func (*Custom) Detect(ports.FS, string) DetectResult { return DetectResult{} }

// OptIn builds a custom target. ~/.agents/skills is one such root.
func OptIn(id, root string, mode domain.SyncMode) (domain.Target, error) {
	if _, err := domain.ParseSkillName(id); err != nil {
		return domain.Target{}, err
	}
	if !filepath.IsAbs(root) {
		return domain.Target{}, errors.New("target root must be absolute")
	}
	if !validMode(mode) {
		return domain.Target{}, errors.New("invalid sync mode")
	}
	return domain.Target{
		ID:      id,
		Kind:    domain.TargetCustom,
		Root:    filepath.Clean(root),
		Mode:    mode,
		Enabled: true,
	}, nil
}

func validMode(mode domain.SyncMode) bool {
	return mode == domain.SyncSymlink || mode == domain.SyncCopy
}
