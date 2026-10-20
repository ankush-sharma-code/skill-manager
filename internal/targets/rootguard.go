package targets

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
)

// ErrRoot means a target root must not be managed.
var ErrRoot = errors.New("target root is not allowed")

// ValidateRoot rejects the filesystem root, the home directory, the skills
// repository and anything inside it, and (unless custom) anything outside home.
func ValidateRoot(resolved, home, repo string, custom bool) error {
	resolved = filepath.Clean(resolved)
	if resolved == string(filepath.Separator) {
		return fmt.Errorf("%w: filesystem root", ErrRoot)
	}
	home = cleanOrEmpty(home)
	repo = cleanOrEmpty(repo)
	if home != "" && resolved == home {
		return fmt.Errorf("%w: home directory", ErrRoot)
	}
	if repo != "" && (resolved == repo || pathInside(repo, resolved)) {
		return fmt.Errorf("%w: skills repository", ErrRoot)
	}
	if !custom && home != "" && !pathInside(home, resolved) {
		return fmt.Errorf("%w: outside home", ErrRoot)
	}
	return nil
}

func cleanOrEmpty(path string) string {
	if path == "" {
		return ""
	}
	clean := filepath.Clean(path)
	if clean == "." {
		return ""
	}
	return clean
}

func pathInside(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil || rel == "." {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func skillHome(repoRoot string) (string, string) {
	repo := filepath.Clean(repoRoot)
	if filepath.Base(repo) != ".skills" {
		return "", repo
	}
	return filepath.Dir(repo), repo
}

func openRoot(fsys ports.FS, t domain.Target, repoRoot string) (string, error) {
	root, err := usableRoot(fsys, t.Root)
	if err != nil {
		return "", err
	}
	home, repo := skillHome(repoRoot)
	if err := ValidateRoot(root, home, repo, t.Kind == domain.TargetCustom); err != nil {
		return "", err
	}
	return root, nil
}

func guardOptInRoot(clean string) error {
	if clean == string(filepath.Separator) {
		return fmt.Errorf("%w: filesystem root", ErrRoot)
	}
	if hasSkillsComponent(clean) {
		return fmt.Errorf("%w: skills repository", ErrRoot)
	}
	return nil
}

func hasSkillsComponent(clean string) bool {
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		if part == ".skills" {
			return true
		}
	}
	return false
}
