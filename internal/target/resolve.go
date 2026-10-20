package target

import (
	"errors"
	"io/fs"
	"path/filepath"

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
	current := filepath.Clean(start)
	seen := make(map[string]struct{}, maxLinkHops)
	for hop := 0; ; hop++ {
		if hop > maxLinkHops {
			return "", nil, ErrLoop
		}
		if _, ok := seen[current]; ok {
			return "", nil, ErrLoop
		}
		seen[current] = struct{}{}
		info, err := fsys.Lstat(current)
		if err != nil {
			return "", nil, err
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			return current, info, nil
		}
		next, err := fsys.Readlink(current)
		if err != nil {
			return "", nil, err
		}
		current = joinLink(filepath.Dir(current), next)
	}
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
