package fake

import (
	"bytes"
	"errors"
	"io/fs"
	"syscall"
	"testing"
)

func TestFakeFileRoundTrip(t *testing.T) {
	f := New()
	if err := f.MkdirAll("/r", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteFileAtomic("/r/a.txt", []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := f.ReadFile("/r/a.txt")
	if err != nil || string(got) != "alpha" {
		t.Fatalf("read = %q %v", got, err)
	}
	info, err := f.Lstat("/r/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 || !info.ModTime().IsZero() || info.Sys() != nil || info.Size() != 5 {
		t.Fatalf("info mode=%o size=%d sys=%v time=%s", info.Mode().Perm(), info.Size(), info.Sys(), info.ModTime())
	}
	ents, err := f.ReadDir("/r")
	if err != nil || len(ents) != 1 {
		t.Fatal(err)
	}
	if _, err := ents[0].Info(); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteFileAtomic("/r/a.txt", []byte("next"), 0o755); err != nil {
		t.Fatal(err)
	}
	info, err = f.Lstat("/r/a.txt")
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("replaced mode %v", info)
	}
}

func TestFakeRenameAndMkdir(t *testing.T) {
	f := New()
	if err := f.MkdirAll("/r/sub", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteFileAtomic("/r/sub/f", []byte("Z"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.MkdirAll("/r/left", 0o755); err != nil || f.MkdirAll("/r/right", 0o755) != nil {
		t.Fatal("mkdir")
	}
	if err := f.Rename("/r/left", "/r/right"); !errors.Is(err, syscall.EEXIST) {
		t.Fatalf("rename onto dir = %v", err)
	}
	if err := f.Rename("/r/sub", "/r/sub/f/inner"); !errors.Is(err, syscall.EINVAL) && !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("rename into child = %v", err)
	}
	if err := f.Rename("/r/missing", "/r/z"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("rename missing = %v", err)
	}
	if err := f.Rename("/r/sub", "/r/moved"); err != nil {
		t.Fatal(err)
	}
	got, err := f.ReadFile("/r/moved/f")
	if err != nil || string(got) != "Z" {
		t.Fatalf("moved = %q %v", got, err)
	}
	if err := f.MkdirAll("", 0o755); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("mkdir empty = %v", err)
	}
	if err := f.WriteFileAtomic("/r/file", []byte("p"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.MkdirAll("/r/file", 0o755); !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("mkdir on file = %v", err)
	}
	if err := f.MkdirAll("/r/file/child", 0o755); !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("mkdir through file = %v", err)
	}
	if err := f.Symlink("missing", "/r/dangle"); err != nil {
		t.Fatal(err)
	}
	if err := f.MkdirAll("/r/dangle", 0o755); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("mkdir dangle = %v", err)
	}
	if err := f.Symlink("/r/right", "/r/dirlink"); err != nil {
		t.Fatal(err)
	}
	if err := f.MkdirAll("/r/dirlink/nested", 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Lstat("/r/right/nested"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.ReadFile("/r/right"); !errors.Is(err, syscall.EISDIR) {
		t.Fatalf("read dir = %v", err)
	}
	if _, err := f.ReadDir("/r/file"); !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("readdir file = %v", err)
	}
	if err := f.RemoveAll("/"); !errors.Is(err, syscall.EPERM) {
		t.Fatalf("remove root = %v", err)
	}
	if err := f.Rename("/", "/nope"); !errors.Is(err, syscall.EBUSY) {
		t.Fatalf("rename root = %v", err)
	}
}

func TestFakeWalkFrom(t *testing.T) {
	f := New()
	if err := f.MkdirAll("/r/sub", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteFileAtomic("/r/sub/f", []byte("Z"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.Symlink("sub", "/r/link"); err != nil {
		t.Fatal(err)
	}
	loc, err := f.walkFrom(f.root, "/r/link/f", true, 0, true)
	if err != nil || string(loc.node.data) != "Z" {
		t.Fatalf("walk absolute = %v %v", loc, err)
	}
	loc, err = f.walkFrom(f.root, "r/sub/..", false, 0, false)
	if err != nil || loc.node.name != "r" {
		t.Fatalf("walk dotdot = %v %v", loc.node, err)
	}
	if _, err := f.walkFrom(f.root, "/missing", false, 0, true); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("walk missing = %v", err)
	}
	if _, err := f.walk("/r/sub/f/child", false); !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("walk through file = %v", err)
	}
}

func TestCaseFoldIsASCIIOnly(t *testing.T) {
	f := NewCaseFolding()
	if err := f.MkdirAll("/r", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteFileAtomic("/r/Foo", []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.Rename("/r/Foo", "/r/foo"); err != nil {
		t.Fatal(err)
	}
	got, err := f.ReadFile("/r/foo")
	if err != nil || string(got) != "A" {
		t.Fatalf("ascii rename = %q %v", got, err)
	}
	if err := f.WriteFileAtomic("/r/Ä", []byte("U"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteFileAtomic("/r/ä", []byte("L"), 0o644); err != nil {
		t.Fatal(err)
	}
	ents, err := f.ReadDir("/r")
	if err != nil {
		t.Fatal(err)
	}
	var unicode int
	for _, ent := range ents {
		if ent.Name() == "Ä" || ent.Name() == "ä" {
			unicode++
		}
	}
	if unicode != 2 {
		t.Fatalf("unicode names folded, entries=%v", namesOf(ents))
	}
	if err := f.WriteFileAtomic("/r/é", []byte("C"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteFileAtomic("/r/e\u0301", []byte("D"), 0o644); err != nil {
		t.Fatal(err)
	}
	ents, err = f.ReadDir("/r")
	if err != nil {
		t.Fatal(err)
	}
	var marks int
	for _, ent := range ents {
		if ent.Name() == "é" || ent.Name() == "e\u0301" {
			marks++
			body, err := f.ReadFile("/r/" + ent.Name())
			if err != nil {
				t.Fatal(err)
			}
			if ent.Name() == "é" && !bytes.Equal(body, []byte("C")) {
				t.Fatalf("composed = %q", body)
			}
		}
	}
	if marks != 2 {
		t.Fatalf("normalization folded, entries=%v", namesOf(ents))
	}
}

func namesOf(ents []fs.DirEntry) []string {
	out := make([]string, len(ents))
	for i, ent := range ents {
		out[i] = ent.Name()
	}
	return out
}
