package repo_test

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/ports"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/fake"
)

// existOnceFS returns ErrExist for the first rename into .trash, then delegates.
type existOnceFS struct {
	ports.FS
	left int
}

func (f *existOnceFS) Rename(oldpath, newpath string) error {
	if f.left > 0 && strings.Contains(newpath, "/.trash/") {
		f.left--
		return fs.ErrExist
	}
	return f.FS.Rename(oldpath, newpath)
}

func TestTrashRetriesRenameErrExist(t *testing.T) {
	ctx := context.Background()
	const root = "/skills"
	inner := fake.New()
	store, _ := openRepo(t, inner, root, time.Unix(1, 0))
	name := mustName(t, "demo")
	files := map[string][]byte{"SKILL.md": skillMD(t, "demo", "Says hello", "ok\n")}
	if err := store.Put(ctx, skillWith(name, "Says hello", nil), files); err != nil {
		t.Fatal(err)
	}
	wrapped := &existOnceFS{FS: inner, left: 1}
	store, _ = openRepo(t, wrapped, root, time.Unix(1, 0))
	if err := store.Delete(ctx, name); err != nil {
		t.Fatalf("Delete err = %v", err)
	}
	if wrapped.left != 0 {
		t.Fatal("trash rename did not retry ErrExist")
	}
	if _, err := inner.Lstat(root + "/demo"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("skill remains: %v", err)
	}
	ents, err := inner.ReadDir(root + "/.trash")
	if err != nil || len(ents) != 1 {
		t.Fatalf("trash = %v %v", ents, err)
	}
}

func TestRecoverRejectsFileAtRejected(t *testing.T) {
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	if err := fsys.MkdirAll(root+"/.stage", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/.stage/rejected", []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/.stage/notes", []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, _ := openRepo(t, fsys, root, time.Unix(1, 0))
	err := store.Recover(ctx)
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("Recover err = %v", err)
	}
	body, readErr := fsys.ReadFile(root + "/.stage/notes")
	if readErr != nil || string(body) != "junk" {
		t.Fatalf("junk = %q %v", body, readErr)
	}
	if _, statErr := fsys.Lstat(root + "/.stage/rejected"); statErr != nil {
		t.Fatalf("rejected file replaced: %v", statErr)
	}
}
