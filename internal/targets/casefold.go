package targets

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
)

func slotNames(ents []fs.DirEntry) []string {
	names := make([]string, 0, len(ents))
	for _, ent := range ents {
		if includeSlot(ent) {
			names = append(names, ent.Name())
		}
	}
	return names
}

func includeSlot(ent fs.DirEntry) bool {
	name := ent.Name()
	if strings.HasPrefix(name, ".") || !slotElement(name) {
		return false
	}
	if _, err := domain.ParseSkillName(name); err == nil {
		return true
	}
	if !foldsToSkill(name) {
		return false
	}
	return ent.IsDir() || ent.Type()&fs.ModeSymlink != 0
}

func foldsToSkill(name string) bool {
	canon := strings.ToLower(name)
	if _, err := domain.ParseSkillName(canon); err != nil {
		return false
	}
	return strings.EqualFold(name, canon)
}

func caseProblems(id, root string, names []string) []Problem {
	var out []Problem
	for _, group := range foldGroups(names) {
		if !collides(group) {
			continue
		}
		spellings := blockedSpellings(group)
		out = append(out, Problem{
			Level:   LevelConflict,
			Target:  id,
			Path:    filepath.Join(root, group[0]),
			Message: "case-fold collision: " + strings.Join(spellings, ", "),
			Names:   spellings,
		})
	}
	return out
}

func foldGroups(names []string) [][]string {
	var groups [][]string
	for _, name := range names {
		if i := foldIndex(groups, name); i >= 0 {
			groups[i] = append(groups[i], name)
			continue
		}
		groups = append(groups, []string{name})
	}
	return groups
}

func foldIndex(groups [][]string, name string) int {
	for i, group := range groups {
		if strings.EqualFold(group[0], name) {
			return i
		}
	}
	return -1
}

func collides(group []string) bool {
	if len(group) > 1 {
		return true
	}
	_, err := domain.ParseSkillName(group[0])
	return err != nil
}

func blockedSpellings(group []string) []string {
	out := append([]string{}, group...)
	canon := strings.ToLower(group[0])
	if _, err := domain.ParseSkillName(canon); err != nil || containsName(out, canon) {
		return out
	}
	return append(out, canon)
}

func containsName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
