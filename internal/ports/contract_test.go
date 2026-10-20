package ports_test

import (
	"bytes"
	"errors"
	"io/fs"
	"syscall"
	"testing"

	"github.com/ankush-sharma-code/skill-manager/internal/ports"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/fake"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/osfs"
)

func TestFSContract(t *testing.T) {
	t.Parallel()
	backends := []struct {
		name string
		open func(t *testing.T) (ports.FS, string)
	}{
		{name: "os", open: func(t *testing.T) (ports.FS, string) { return osfs.New(), t.TempDir() }},
		{name: "fake", open: func(t *testing.T) (ports.FS, string) { return fake.New(), "/contract" }},
	}
	for _, backend := range backends {
		backend := backend
		t.Run(backend.name, func(t *testing.T) {
			t.Parallel()
			runFSContract(t, backend.open)
		})
	}
}

func runFSContract(t *testing.T, open func(t *testing.T) (ports.FS, string)) {
	t.Helper()
	steps := []struct {
		name string
		fn   func(*testing.T, ports.FS, string)
	}{
		{"write read and mode", contractWriteRead},
		{"rename file over file", contractRenameFile},
		{"rename directory", contractRenameDir},
		{"rename refuses a directory destination", contractRenameOverDir},
		{"symlink readlink and rename", contractSymlink},
		{"symlink parent and dotdot", contractSymlinkDotDot},
		{"removeall leaves a symlink target", contractRemoveAllSymlink},
	}
	for _, step := range steps {
		step := step
		t.Run(step.name, func(t *testing.T) {
			fsys, root := open(t)
			if err := fsys.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			step.fn(t, fsys, root)
		})
	}
}

func contractWriteRead(t *testing.T, fsys ports.FS, root string) {
	t.Helper()
	path := root + "/hello.txt"
	if err := fsys.WriteFileAtomic(path, []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := fsys.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "alpha" {
		t.Fatalf("ReadFile = %q", got)
	}
	info, err := fsys.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name() != "hello.txt" || info.Mode().Perm() != 0o644 || info.IsDir() {
		t.Fatalf("Lstat name=%s mode=%o dir=%v", info.Name(), info.Mode().Perm(), info.IsDir())
	}
	if err := fsys.WriteFileAtomic(path, []byte("alphabet"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err = fsys.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "alphabet" {
		t.Fatalf("replaced content = %q", got)
	}
	info, err = fsys.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}

func contractRenameFile(t *testing.T, fsys ports.FS, root string) {
	t.Helper()
	if err := fsys.WriteFileAtomic(root+"/a", []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/b", []byte("B"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Rename(root+"/a", root+"/b"); err != nil {
		t.Fatal(err)
	}
	if _, err := fsys.Lstat(root + "/a"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("old path err = %v", err)
	}
	got, err := fsys.ReadFile(root + "/b")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "A" {
		t.Fatalf("renamed content = %q", got)
	}
}

func contractRenameDir(t *testing.T, fsys ports.FS, root string) {
	t.Helper()
	if err := fsys.MkdirAll(root+"/dir/sub", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/dir/sub/f", []byte("Z"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Rename(root+"/dir", root+"/moved"); err != nil {
		t.Fatal(err)
	}
	got, err := fsys.ReadFile(root + "/moved/sub/f")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "Z" {
		t.Fatalf("moved content = %q", got)
	}
	if _, err := fsys.Lstat(root + "/dir"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("source dir err = %v", err)
	}
}

func contractRenameOverDir(t *testing.T, fsys ports.FS, root string) {
	t.Helper()
	if err := fsys.MkdirAll(root+"/left", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.MkdirAll(root+"/right", 0o755); err != nil {
		t.Fatal(err)
	}
	err := fsys.Rename(root+"/left", root+"/right")
	if !errors.Is(err, syscall.EEXIST) {
		t.Fatalf("rename onto dir err = %v", err)
	}
	if _, err := fsys.Lstat(root + "/left"); err != nil {
		t.Fatalf("source removed: %v", err)
	}
	if err := fsys.WriteFileAtomic(root+"/file", []byte("q"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.MkdirAll(root+"/emptydir", 0o755); err != nil {
		t.Fatal(err)
	}
	err = fsys.Rename(root+"/emptydir", root+"/file")
	if !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("rename dir onto file err = %v", err)
	}
}

func contractSymlink(t *testing.T, fsys ports.FS, root string) {
	t.Helper()
	if err := fsys.WriteFileAtomic(root+"/target", []byte("TGT"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Symlink("target", root+"/link"); err != nil {
		t.Fatal(err)
	}
	info, err := fsys.Lstat(root + "/link")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&fs.ModeSymlink == 0 || info.IsDir() {
		t.Fatalf("Lstat link mode = %v", info.Mode())
	}
	if info.Size() != int64(len("target")) {
		t.Fatalf("symlink size = %d", info.Size())
	}
	gotLink, err := fsys.Readlink(root + "/link")
	if err != nil {
		t.Fatal(err)
	}
	if gotLink != "target" {
		t.Fatalf("Readlink = %q", gotLink)
	}
	body, err := fsys.ReadFile(root + "/link")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "TGT" {
		t.Fatalf("ReadFile link = %q", body)
	}
	ents, err := fsys.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var linkEntry fs.DirEntry
	for _, ent := range ents {
		if ent.Name() == "link" {
			linkEntry = ent
		}
	}
	if linkEntry == nil || linkEntry.IsDir() || linkEntry.Type()&fs.ModeSymlink == 0 {
		t.Fatalf("ReadDir link entry = %v", linkEntry)
	}
	if err := fsys.Symlink("missing", root+"/dangle"); err != nil {
		t.Fatal(err)
	}
	if _, err := fsys.Lstat(root + "/dangle"); err != nil {
		t.Fatal(err)
	}
	if _, err := fsys.ReadFile(root + "/dangle"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("dangling ReadFile err = %v", err)
	}
	if err := fsys.Symlink("loop2", root+"/loop1"); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Symlink("loop1", root+"/loop2"); err != nil {
		t.Fatal(err)
	}
	if _, err := fsys.Lstat(root + "/loop1"); err != nil {
		t.Fatal(err)
	}
	if _, err := fsys.ReadFile(root + "/loop1"); !errors.Is(err, syscall.ELOOP) {
		t.Fatalf("loop ReadFile err = %v", err)
	}
	if _, err := fsys.Readlink(root + "/target"); !errors.Is(err, syscall.EINVAL) {
		t.Fatalf("Readlink file err = %v", err)
	}
	if err := fsys.Rename(root+"/link", root+"/link2"); err != nil {
		t.Fatal(err)
	}
	gotLink, err = fsys.Readlink(root + "/link2")
	if err != nil {
		t.Fatal(err)
	}
	if gotLink != "target" {
		t.Fatalf("renamed Readlink = %q", gotLink)
	}
	body, err = fsys.ReadFile(root + "/target")
	if err != nil || string(body) != "TGT" {
		t.Fatalf("target after rename = %q, %v", body, err)
	}
	if err := fsys.WriteFileAtomic(root+"/link2", []byte("NEW"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err = fsys.Lstat(root + "/link2")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		t.Fatal("WriteFileAtomic followed the symlink")
	}
	body, err = fsys.ReadFile(root + "/link2")
	if err != nil || string(body) != "NEW" {
		t.Fatalf("replaced link = %q, %v", body, err)
	}
	body, err = fsys.ReadFile(root + "/target")
	if err != nil || string(body) != "TGT" {
		t.Fatalf("target after replace = %q, %v", body, err)
	}
}

func contractSymlinkDotDot(t *testing.T, fsys ports.FS, root string) {
	t.Helper()
	if err := fsys.MkdirAll(root+"/elsewhere", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.MkdirAll(root+"/sub", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/file", []byte("TOP"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/sub/file", []byte("SUB"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/elsewhere/file", []byte("OUT"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Symlink("../elsewhere", root+"/sub/link"); err != nil {
		t.Fatal(err)
	}
	got, err := fsys.ReadFile(root + "/sub/link/../file")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "TOP" {
		t.Fatalf("ReadFile through symlink .. = %q, want TOP", got)
	}
	if err := fsys.MkdirAll(root+"/via", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Symlink("via", root+"/vialink"); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/vialink/child", []byte("C"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = fsys.ReadFile(root + "/via/child")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "C" {
		t.Fatalf("write through parent symlink = %q", got)
	}
	if err := fsys.Rename(root+"/vialink/child", root+"/vialink/renamed"); err != nil {
		t.Fatal(err)
	}
	got, err = fsys.ReadFile(root + "/via/renamed")
	if err != nil || string(got) != "C" {
		t.Fatalf("rename through parent symlink = %q, %v", got, err)
	}
}

func contractRemoveAllSymlink(t *testing.T, fsys ports.FS, root string) {
	t.Helper()
	if err := fsys.MkdirAll(root+"/real", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/real/keep", []byte("K"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Symlink(root+"/real", root+"/dirlink"); err != nil {
		t.Fatal(err)
	}
	if err := fsys.RemoveAll(root + "/dirlink"); err != nil {
		t.Fatal(err)
	}
	got, err := fsys.ReadFile(root + "/real/keep")
	if err != nil || string(got) != "K" {
		t.Fatalf("target after RemoveAll = %q, %v", got, err)
	}
	if _, err := fsys.Lstat(root + "/dirlink"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("link after RemoveAll err = %v", err)
	}
	if err := fsys.RemoveAll(root + "/real"); err != nil {
		t.Fatal(err)
	}
	if _, err := fsys.Lstat(root + "/real/keep"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("tree after RemoveAll err = %v", err)
	}
	if err := fsys.RemoveAll(root + "/missing"); err != nil {
		t.Fatalf("RemoveAll missing = %v", err)
	}
}

func TestWriteFileAtomicFailureLeavesOldOrNew(t *testing.T) {
	t.Parallel()
	fsys := fake.New()
	const root = "/contract"
	if err := fsys.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	path := root + "/hello.txt"
	oldBody := []byte("alpha")
	newBody := []byte("alphabet-and-more")
	if err := fsys.WriteFileAtomic(path, oldBody, 0o644); err != nil {
		t.Fatal(err)
	}
	fsys.FailWriteFileAtomicBeforeRename(1)
	err := fsys.WriteFileAtomic(path, newBody, 0o644)
	if !errors.Is(err, fake.ErrInjected) {
		t.Fatalf("WriteFileAtomic err = %v", err)
	}
	got, err := fsys.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, oldBody) && !bytes.Equal(got, newBody) {
		t.Fatalf("torn content %q", got)
	}

	missing := root + "/missing.txt"
	fsys.FailWriteFileAtomicBeforeRename(1)
	err = fsys.WriteFileAtomic(missing, newBody, 0o644)
	if !errors.Is(err, fake.ErrInjected) {
		t.Fatalf("create err = %v", err)
	}
	got, err = fsys.ReadFile(missing)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, newBody) {
		t.Fatalf("torn create %q", got)
	}
}
