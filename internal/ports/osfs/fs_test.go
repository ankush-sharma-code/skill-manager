package osfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestSyncDirErrors(t *testing.T) {
	err := syncDir(filepath.Join(t.TempDir(), "missing"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing dir = %v", err)
	}
	err = syncDir("/dev/full")
	if !errors.Is(err, syscall.EINVAL) {
		t.Fatalf("sync /dev/full = %v", err)
	}
	if err := syncDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

func TestWriteFileAtomicInjectedFailures(t *testing.T) {
	orig := createAtomicFile
	t.Cleanup(func() { createAtomicFile = orig })
	dir := t.TempDir()
	for _, op := range []string{"write", "chmod", "sync", "close"} {
		op := op
		t.Run(op, func(t *testing.T) {
			tmp := filepath.Join(dir, op+".tmp")
			if err := os.WriteFile(tmp, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			createAtomicFile = func(string) (atomicFile, error) {
				return &errFile{name: tmp, op: op}, nil
			}
			dest := filepath.Join(dir, op+".dest")
			err := FS{}.WriteFileAtomic(dest, []byte("data"), 0o640)
			if err == nil || err.Error() != op+" failed" {
				t.Fatalf("err = %v", err)
			}
			if _, statErr := os.Lstat(tmp); !errors.Is(statErr, fs.ErrNotExist) {
				t.Fatalf("temp remains: %v", statErr)
			}
			if _, statErr := os.Lstat(dest); !errors.Is(statErr, fs.ErrNotExist) {
				t.Fatalf("dest created: %v", statErr)
			}
		})
	}
}

type errFile struct {
	name string
	op   string
}

func (e *errFile) Name() string { return e.name }

func (e *errFile) Write([]byte) (int, error) {
	if e.op == "write" {
		return 0, errors.New("write failed")
	}
	return 1, nil
}

func (e *errFile) Chmod(fs.FileMode) error {
	if e.op == "chmod" {
		return errors.New("chmod failed")
	}
	return nil
}

func (e *errFile) Sync() error {
	if e.op == "sync" {
		return errors.New("sync failed")
	}
	return nil
}

func (e *errFile) Close() error {
	if e.op == "close" {
		return errors.New("close failed")
	}
	return nil
}
