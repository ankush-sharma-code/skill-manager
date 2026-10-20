package target

import (
	"errors"
	"path/filepath"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
)

// Propose returns the targets to enable on first run.
// A skill is registered once when roots resolve to the same directory.
// ~/.agents/skills is omitted. Cursor is always managed when it is installed.
// Overlapping tool reads are removed only through AlsoReads.
func Propose(fsys ports.FS, home string) ([]domain.Target, []Problem, error) {
	if !filepath.IsAbs(home) {
		return nil, nil, errors.New("home must be absolute")
	}
	home = filepath.Clean(home)
	var out []domain.Target
	var problems []Problem
	if claudeInstalled(fsys, home) {
		out = append(out, claudeTarget(home))
	}
	if cursorInstalled(fsys, home) {
		out = append(out, cursorTarget(home))
		if existsDir(fsys, filepath.Join(home, ".claude", "skills")) {
			problems = append(problems, cursorReadProblem(home))
		}
	}
	if codexInstalled(fsys, home) {
		out = append(out, codexTarget(home))
		agents := filepath.Join(home, ".agents", "skills")
		if existsDir(fsys, agents) {
			problems = append(problems, codexAgentsProblem(agents))
		}
	}
	kept, aliases := DedupeRoots(fsys, out)
	problems = append(problems, aliases...)
	kept = DedupeReads(kept, map[TargetID][]TargetID{
		"cursor-user": NewCursor("").AlsoReads(),
	})
	return kept, problems, nil
}

// DedupeRoots keeps the first target for each physical root.
// A later target that resolves to the same directory is an alias.
func DedupeRoots(fsys ports.FS, targets []domain.Target) ([]domain.Target, []Problem) {
	seen := make(map[string]domain.Target, len(targets))
	out := make([]domain.Target, 0, len(targets))
	var problems []Problem
	for _, t := range targets {
		key := rootKey(fsys, t.Root)
		if first, ok := seen[key]; ok {
			problems = append(problems, Problem{
				Level:   LevelInfo,
				Target:  t.ID,
				Path:    t.Root,
				Message: t.ID + " is an alias of " + first.ID,
			})
			continue
		}
		seen[key] = t
		out = append(out, t)
	}
	return out, problems
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
