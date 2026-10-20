package repo_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/ports/fake"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/osfs"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/repo"
)

func TestRecoverCanceledAndMissingStage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store, _ := openRepo(t, fake.New(), "/skills", time.Unix(1, 0))
	if err := store.Recover(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Recover err = %v", err)
	}
	store, _ = openRepo(t, fake.New(), "/skills", time.Unix(1, 0))
	if err := store.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverRefusesSymlinkedRoot(t *testing.T) {
	real := t.TempDir()
	link := t.TempDir() + "/skills"
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	store, err := repo.New(osfs.New(), fixedClock{time.Unix(1, 0)}, link)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(context.Background()); !errors.Is(err, repo.ErrSymlink) {
		t.Fatalf("Recover err = %v", err)
	}
}

func TestRecoverStageIOErrors(t *testing.T) {
	ctx := context.Background()
	const root = "/skills"
	const stage = root + "/.stage"
	inner := fake.New()
	if err := inner.MkdirAll(stage, 0o700); err != nil {
		t.Fatal(err)
	}
	store, _ := openRepo(t, lstatingFS{FS: inner, failPath: stage}, root, time.Unix(1, 0))
	if err := store.Recover(ctx); !errors.Is(err, errBlocked) {
		t.Fatalf("lstat err = %v", err)
	}
	store, _ = openRepo(t, readingFS{FS: inner, failPath: stage}, root, time.Unix(1, 0))
	if err := store.Recover(ctx); !errors.Is(err, errBlocked) {
		t.Fatalf("readdir err = %v", err)
	}
}

func TestRecoverRestoresOldWhenNewIsPartial(t *testing.T) {
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	oldBody := skillMD(t, "demo", "Says hello", "old\n")
	writeSkill(t, fsys, root+"/.stage/demo-0123456789abcdef.old", oldBody)
	if err := fsys.MkdirAll(root+"/.stage/demo-0123456789abcdef.new", 0o755); err != nil {
		t.Fatal(err)
	}
	store, _ := openRepo(t, fsys, root, time.Unix(1, 0))
	if err := store.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	assertSkillBody(t, fsys, root+"/demo/SKILL.md", oldBody)
	if _, err := fsys.Lstat(root + "/.stage/demo-0123456789abcdef.new"); err == nil {
		t.Fatal("partial .new kept")
	}
}

func TestRecoverDropsStageWhenLiveExists(t *testing.T) {
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	live := skillMD(t, "demo", "Says hello", "live\n")
	writeSkill(t, fsys, root+"/demo", live)
	writeSkill(t, fsys, root+"/.stage/demo-0123456789abcdef.new", skillMD(t, "demo", "Says hello", "new\n"))
	writeSkill(t, fsys, root+"/.stage/demo-0123456789abcdef.old", skillMD(t, "demo", "Says hello", "old\n"))
	store, _ := openRepo(t, fsys, root, time.Unix(1, 0))
	if err := store.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	assertSkillBody(t, fsys, root+"/demo/SKILL.md", live)
	if leftovers := stageLeftovers(t, fsys, root); len(leftovers) != 0 {
		t.Fatalf("leftovers = %v", leftovers)
	}
}

func TestRecoverDoesNotPromoteSymlinkedSkillMD(t *testing.T) {
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	oldBody := skillMD(t, "demo", "Says hello", "old\n")
	writeSkill(t, fsys, root+"/.stage/demo-0123456789abcdef.old", oldBody)
	newDir := root + "/.stage/demo-0123456789abcdef.new"
	if err := fsys.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/outside.md", []byte("evil"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Symlink(root+"/outside.md", newDir+"/SKILL.md"); err != nil {
		t.Fatal(err)
	}
	store, _ := openRepo(t, fsys, root, time.Unix(1, 0))
	if err := store.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	assertSkillBody(t, fsys, root+"/demo/SKILL.md", oldBody)
	if _, err := fsys.Lstat(root + "/outside.md"); err != nil {
		t.Fatalf("symlink target removed: %v", err)
	}
}

func TestRecoverQuarantinesPlainJunkAndRetriesName(t *testing.T) {
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	if err := fsys.MkdirAll(root+"/.stage/rejected", 0o700); err != nil {
		t.Fatal(err)
	}
	const base = "demo-0123456789abcdef.old"
	if err := fsys.WriteFileAtomic(root+"/.stage/rejected/"+base, []byte("taken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/.stage/"+base, []byte("bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/.stage/notes", []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	var logged string
	store, _ := openRepo(t, fsys, root, time.Unix(1, 0))
	store.SetLog(func(format string, args ...any) {
		logged = format
	})
	if err := store.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := fsys.Lstat(root + "/.stage/rejected/" + base + "-2"); err != nil {
		t.Fatalf("collision suffix missing: %v", err)
	}
	if _, err := fsys.Lstat(root + "/.stage/rejected/notes"); err != nil {
		t.Fatalf("plain junk missing: %v", err)
	}
	if _, err := fsys.Lstat(root + "/demo"); err == nil {
		t.Fatal("junk promoted")
	}
	if logged == "" {
		t.Fatal("quarantine was not logged")
	}
	if err := store.Recover(ctx); err != nil {
		t.Fatalf("second Recover: %v", err)
	}
}
