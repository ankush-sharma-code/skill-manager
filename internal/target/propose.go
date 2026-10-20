package target

import (
	"errors"
	"path/filepath"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
)

// Propose returns the targets to enable on first run.
// A skill is registered once when roots resolve to the same directory.
// ~/.agents/skills is omitted. Cursor is omitted when it would overlap
// ~/.claude/skills.
func Propose(fsys ports.FS, home string) ([]domain.Target, []Problem, error) {
	if !filepath.IsAbs(home) {
		return nil, nil, errors.New("home must be absolute")
	}
	home = filepath.Clean(home)
	var out []domain.Target
	if claudeInstalled(fsys, home) {
		out = append(out, claudeTarget(home))
	}
	cursor := NewCursor("")
	if cursorInstalled(fsys, home) && !cursor.Skip(fsys, home) {
		out = append(out, cursorTarget(home))
	}
	if codexInstalled(fsys, home) {
		out = append(out, codexTarget(home))
	}
	kept, problems := DedupeRoots(fsys, out)
	return kept, problems, nil
}

// DedupeRoots keeps the first target for each physical root.
// Alias problems are recorded by the canonical-root fix.
func DedupeRoots(fsys ports.FS, targets []domain.Target) ([]domain.Target, []Problem) {
	return dedupe(fsys, targets), nil
}

func dedupe(fsys ports.FS, targets []domain.Target) []domain.Target {
	seen := make(map[string]struct{}, len(targets))
	out := make([]domain.Target, 0, len(targets))
	for _, t := range targets {
		key := rootKey(fsys, t.Root)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	return out
}

// SlotPaths lists one managed path per distinct root for skill.
// Roots that resolve to the same directory contribute one path.
func SlotPaths(fsys ports.FS, targets []domain.Target, skill string) ([]string, error) {
	name, err := domain.ParseSkillName(skill)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(targets))
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		key := rootKey(fsys, t.Root)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, filepath.Join(t.Root, string(name)))
	}
	return out, nil
}
