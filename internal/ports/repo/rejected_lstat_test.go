package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/ports/fake"
)

func TestRejectedPathSkipsCollidingName(t *testing.T) {
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	store, _ := openRepo(t, fsys, root, time.Unix(1, 0))
	store.SetRand(&countingRand{})
	store.SetLog(func(string, ...any) {})
	const (
		occupied = root + "/.stage/rejected/notes-1000000000-0001020304050607"
		next     = root + "/.stage/rejected/notes-1000000000-08090a0b0c0d0e0f"
	)
	if err := fsys.MkdirAll(root+"/.stage/rejected", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(occupied, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/.stage/notes", []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := fsys.ReadFile(occupied)
	if err != nil || string(got) != "keep" {
		t.Fatalf("occupied entry = %q %v", got, err)
	}
	body, err := fsys.ReadFile(next)
	if err != nil || string(body) != "new" {
		t.Fatalf("next entry = %q %v", body, err)
	}
}
