package target

import (
	"sort"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
)

// Level is how serious a Problem is.
type Level string

const (
	// LevelInfo is a hint. It does not change which roots are managed.
	LevelInfo Level = "info"
	// LevelConflict means both paths must be left untouched.
	LevelConflict Level = "conflict"
)

// Problem is something Observe or registration found and did not fix.
type Problem struct {
	Level   Level
	Target  string
	Path    string
	Message string
	// Names are spellings a later planner must not write.
	Names []string
}

// TargetID is a managed target id such as "cursor-user".
type TargetID string

// Result is an observation plus the problems found along the way.
type Result struct {
	Observed []domain.Observed
	Problems []Problem
}

// BlockedNames lists conflict spellings. Callers must not write those paths.
func BlockedNames(problems []Problem) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, problem := range problems {
		if problem.Level != LevelConflict {
			continue
		}
		for _, name := range problem.Names {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// DedupeReads drops a target only when its AlsoReads list names another
// target that is also present. An empty list drops nothing.
func DedupeReads(targets []domain.Target, reads map[TargetID][]TargetID) []domain.Target {
	present := make(map[TargetID]bool, len(targets))
	for _, t := range targets {
		present[TargetID(t.ID)] = true
	}
	out := make([]domain.Target, 0, len(targets))
	for _, t := range targets {
		if readsPresent(reads[TargetID(t.ID)], present) {
			continue
		}
		out = append(out, t)
	}
	return out
}

func readsPresent(ids []TargetID, present map[TargetID]bool) bool {
	for _, id := range ids {
		if present[id] {
			return true
		}
	}
	return false
}

// ValidateRoot reports whether a resolved root is safe to manage.
// The stub accepts every root; checks land with the root-guard fix.
func ValidateRoot(resolved, home, repo string, custom bool) error {
	if resolved == "" && home == "" && repo == "" && !custom {
		return nil
	}
	return nil
}

// Inspect observes a target and returns problems found in the tree.
func Inspect(fsys ports.FS, t domain.Target, repoRoot string) (Result, error) {
	return inspect(fsys, t, repoRoot)
}
