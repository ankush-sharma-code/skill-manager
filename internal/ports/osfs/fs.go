package osfs

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ankush-sharma-code/skill-manager/internal/ports"
)

// FS is the process filesystem.
type FS struct{}

func New() FS { return FS{} }

var _ ports.FS = FS{}

func (FS) Lstat(path string) (fs.FileInfo, error) { return os.Lstat(path) }

func (FS) ReadDir(path string) ([]fs.DirEntry, error) { return os.ReadDir(path) }

func (FS) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (FS) Readlink(path string) (string, error) { return os.Readlink(path) }

func (FS) Symlink(old, new string) error { return os.Symlink(old, new) }

func (FS) Rename(old, new string) error { return os.Rename(old, new) }

func (FS) RemoveAll(path string) error { return os.RemoveAll(path) }

func (FS) MkdirAll(path string, mode fs.FileMode) error { return os.MkdirAll(path, mode) }

// atomicFile is the temp file WriteFileAtomic fills and renames into place.
type atomicFile interface {
	Name() string
	Write([]byte) (int, error)
	Chmod(fs.FileMode) error
	Sync() error
	Close() error
}

var createAtomicFile = func(dir string) (atomicFile, error) {
	return os.CreateTemp(dir, ".tmp-*")
}

func (FS) WriteFileAtomic(path string, data []byte, mode fs.FileMode) error {
	dir := filepath.Dir(path)
	file, err := createAtomicFile(dir)
	if err != nil {
		return err
	}
	tmp := file.Name()
	closed := false
	committed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
		if !committed {
			_ = os.Remove(tmp)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Chmod(mode.Perm()); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	closed = true
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	committed = true
	_ = syncDir(dir)
	return nil
}

func syncDir(dir string) error {
	file, err := os.Open(dir)
	if err != nil {
		return err
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
