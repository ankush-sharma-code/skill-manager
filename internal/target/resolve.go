package target

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/ankush-sharma-code/skill-manager/internal/ports"
)

const maxLinkHops = 8

// ErrLoop means a symlink chain repeats or exceeds maxLinkHops.
var ErrLoop = errors.New("symlink loop")

// ErrOutside means a slot name leaves the target root.
var ErrOutside = errors.New("path is outside the target root")

// ErrNotDir means the target root exists and is not a directory.
var ErrNotDir = errors.New("target root is not a directory")

func resolveFinal(fsys ports.FS, start string) (string, fs.FileInfo, error) {
	return canonical(fsys, start)
}

// canonical resolves every path component, the same way EvalSymlinks does.
// Child walks do not use it, so a link inside a skill is not followed for content.
func canonical(fsys ports.FS, start string) (string, fs.FileInfo, error) {
	clean := filepath.Clean(start)
	if !filepath.IsAbs(clean) {
		return "", nil, errors.New("path must be absolute")
	}
	return walkCanonical(fsys, clean, 0, map[string]struct{}{})
}

func walkCanonical(fsys ports.FS, start string, hops int, seen map[string]struct{}) (string, fs.FileInfo, error) {
	if hops > maxLinkHops {
		return "", nil, ErrLoop
	}
	parts := pathParts(start)
	current := string(filepath.Separator)
	if len(parts) == 0 {
		info, err := fsys.Lstat(current)
		return current, info, err
	}
	var info fs.FileInfo
	for i, part := range parts {
		next := filepath.Join(current, part)
		if _, ok := seen[next]; ok {
			return "", nil, ErrLoop
		}
		got, err := fsys.Lstat(next)
		if err != nil {
			return "", nil, err
		}
		if got.Mode()&fs.ModeSymlink == 0 {
			current = next
			info = got
			continue
		}
		seen[next] = struct{}{}
		raw, err := fsys.Readlink(next)
		if err != nil {
			return "", nil, err
		}
		return walkCanonical(fsys, joinRest(joinLink(current, raw), parts[i+1:]), hops+1, seen)
	}
	return current, info, nil
}

func pathParts(clean string) []string {
	if clean == string(filepath.Separator) {
		return nil
	}
	return strings.Split(strings.Trim(clean, string(filepath.Separator)), string(filepath.Separator))
}

func joinRest(base string, rest []string) string {
	if len(rest) == 0 {
		return filepath.Clean(base)
	}
	parts := make([]string, 0, len(rest)+1)
	parts = append(parts, base)
	parts = append(parts, rest...)
	return filepath.Clean(filepath.Join(parts...))
}

func joinLink(dir, link string) string {
	if filepath.IsAbs(link) {
		return filepath.Clean(link)
	}
	return filepath.Clean(filepath.Join(dir, link))
}

func existsDir(fsys ports.FS, path string) bool {
	_, info, err := resolveFinal(fsys, path)
	return err == nil && info != nil && info.IsDir()
}

func usableRoot(fsys ports.FS, root string) (string, error) {
	final, info, err := resolveFinal(fsys, root)
	if err != nil {
		return "", err
	}
	if info == nil || !info.IsDir() {
		return "", ErrNotDir
	}
	return final, nil
}

func rootKey(fsys ports.FS, root string) string {
	final, info, err := resolveFinal(fsys, root)
	if err != nil || info == nil || !info.IsDir() {
		return filepath.Clean(root)
	}
	return final
}
