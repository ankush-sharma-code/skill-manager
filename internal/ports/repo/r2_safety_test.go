package repo_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/ports/fake"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/osfs"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/repo"
)

func TestRecoverQuarantinesOutsideSymlink(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "skills")
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".stage"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "SKILL.md"), aMD("evil"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, ".stage", "a-0123456789abcdef.old")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	store, err := repo.New(osfs.New(), fixedClock{time.Unix(1, 0)}, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(filepath.Join(root, "a")); err == nil {
		t.Fatalf("live skill = %s mode %v", fi.Name(), fi.Mode())
	}
	if _, err := os.Stat(filepath.Join(outside, "SKILL.md")); err != nil {
		t.Fatalf("outside target touched: %v", err)
	}
	ents, err := os.ReadDir(filepath.Join(root, ".stage", "rejected"))
	if err != nil {
		t.Fatal(err)
	}
	var quarantined string
	for _, ent := range ents {
		if strings.HasPrefix(ent.Name(), "a-0123456789abcdef.old-") {
			quarantined = filepath.Join(root, ".stage", "rejected", ent.Name())
		}
	}
	fi, err := os.Lstat(quarantined)
	if quarantined == "" || err != nil || fi.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("quarantine = %q %v %v", quarantined, fi, err)
	}
	probs := store.Problems()
	if len(probs) != 1 || probs[0].Name != "a" || probs[0].Err == nil {
		t.Fatalf("problems = %+v", probs)
	}
}

func TestRecoverDropsOrphanNew(t *testing.T) {
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	writeSkill(t, fsys, root+"/.stage/demo-0123456789abcdef.new", skillMD(t, "demo", "Says hello", "new\n"))
	store, _ := openRepo(t, fsys, root, time.Unix(1, 0))
	if err := store.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, mustName(t, "demo")); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("orphan .new promoted: %v", err)
	}
	ents, err := fsys.ReadDir(root + "/.stage")
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range ents {
		if ent.Name() != "rejected" {
			t.Fatalf("leftover .stage/%s", ent.Name())
		}
	}
}

func TestListAndProblemsAreRaceFree(t *testing.T) {
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	store, _ := openRepo(t, fsys, root, time.Unix(1, 0))
	if err := store.Put(ctx, skillWith(mustName(t, "good"), "Says hello", nil), map[string][]byte{
		"SKILL.md": skillMD(t, "good", "Says hello", "ok\n"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := fsys.MkdirAll(root+"/stray", 0o755); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.List(ctx)
			_ = store.Problems()
		}()
	}
	wg.Wait()
}

func TestPutRefusesNonDirStage(t *testing.T) {
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	if err := fsys.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/.stage", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, _ := openRepo(t, fsys, root, time.Unix(1, 0))
	err := store.Put(ctx, skillWith(mustName(t, "demo"), "Says hello", nil), map[string][]byte{
		"SKILL.md": skillMD(t, "demo", "Says hello", "ok\n"),
	})
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("Put err = %v", err)
	}
	if _, statErr := fsys.Lstat(root + "/demo"); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("skill created: %v", statErr)
	}
}

func TestPutRefusesSymlinkedStage(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	outside := t.TempDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".stage")); err != nil {
		t.Fatal(err)
	}
	store, err := repo.New(osfs.New(), fixedClock{time.Unix(1, 0)}, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := put(t, store, "a", nil, nil); err == nil {
		t.Fatal("Put followed a symlinked .stage")
	}
	ents, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("wrote outside: %s", ents[0].Name())
	}
}

func TestEnsureRealDirRejectsFileRoot(t *testing.T) {
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	if err := fsys.WriteFileAtomic(root, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, _ := openRepo(t, fsys, root, time.Unix(1, 0))
	err := store.Put(ctx, skillWith(mustName(t, "demo"), "Says hello", nil), map[string][]byte{
		"SKILL.md": skillMD(t, "demo", "Says hello", "ok\n"),
	})
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("Put err = %v", err)
	}
}

func TestTrashRetryKillsSingleSuffix(t *testing.T) {
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	when := time.Unix(1, 0)
	store, clock := openRepo(t, fsys, root, when)
	name := mustName(t, "a")
	files := map[string][]byte{"SKILL.md": skillMD(t, "a", "Says hello", "ok\n")}
	if err := store.Put(ctx, skillWith(name, "Says hello", nil), files); err != nil {
		t.Fatal(err)
	}
	store.SetRand(&countingRand{})
	stamp := clock.Now().UTC().UnixNano()
	blocked := filepath.Join(root, ".trash", "a-1000000000-0001020304050607")
	if stamp != 1000000000 {
		t.Fatalf("stamp = %d", stamp)
	}
	if err := fsys.MkdirAll(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, name); err != nil {
		t.Fatal(err)
	}
	if _, err := fsys.Lstat(blocked); err != nil {
		t.Fatalf("blocked name missing: %v", err)
	}
	if _, err := fsys.Lstat(root + "/.trash/a-1000000000-08090a0b0c0d0e0f"); err != nil {
		t.Fatalf("retry name missing: %v", err)
	}
}

func TestRecoverRejectsBadStageIDs(t *testing.T) {
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	body := skillMD(t, "demo", "Says hello", "x\n")
	for _, dir := range []string{
		root + "/.stage/junk.new",
		root + "/.stage/A-0123456789abcdef.new",
		root + "/.stage/a-0123456789ABCDEF.new",
		root + "/.stage/a-short.new",
	} {
		writeSkill(t, fsys, dir, body)
	}
	store, _ := openRepo(t, fsys, root, time.Unix(1, 0))
	if err := store.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"junk", "A", "a", "a-short"} {
		if _, err := fsys.Lstat(root + "/" + name); err == nil {
			t.Fatalf("bad stage id promoted %s", name)
		}
	}
	if _, err := fsys.Lstat(root); err != nil {
		t.Fatalf("root damaged: %v", err)
	}
}

func TestRootAndStageAreOwnerOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	store, err := repo.New(osfs.New(), fixedClock{time.Unix(1, 0)}, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := put(t, store, "a", nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{root, filepath.Join(root, ".stage")} {
		fi, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o700 {
			t.Fatalf("%s mode = %o", path, fi.Mode().Perm())
		}
	}
}
