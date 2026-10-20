package repo_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/ports"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/fake"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/osfs"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/repo"
)

func TestPutRefusesSymlinkedRoot(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "skills")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	r, err := repo.New(osfs.New(), fixedClock{time.Unix(1, 0)}, link)
	if err != nil {
		t.Fatal(err)
	}
	if err := put(t, r, "a", nil, nil); err == nil {
		t.Fatal("Put followed a symlinked root")
	}
	if _, err := os.Stat(filepath.Join(real, "a")); err == nil {
		t.Fatal("skill was written outside the lexical root")
	}
}

func TestDeleteRefusesSymlinkedTrash(t *testing.T) {
	r, root := newOS(t)
	if err := put(t, r, "a", nil, nil); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".trash")); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(context.Background(), "a"); err == nil {
		t.Fatal("Delete followed a symlinked .trash")
	}
	if _, err := os.Stat(filepath.Join(root, "a", "SKILL.md")); err != nil {
		t.Fatalf("skill left the root: %v", err)
	}
	ents, _ := os.ReadDir(outside)
	if len(ents) > 0 {
		t.Fatalf("moved outside: %s", ents[0].Name())
	}
}

func TestListReportsInvalidSkill(t *testing.T) {
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
	got, err := store.List(ctx)
	if err != nil || len(got) != 1 || got[0].Name != "good" {
		t.Fatalf("List = %v %v", got, err)
	}
	probs := store.Problems()
	if len(probs) != 1 || probs[0].Name != "stray" || probs[0].Err == nil {
		t.Fatalf("problems = %+v", probs)
	}
}

func TestTrashSuffixUsesInjectedRand(t *testing.T) {
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	when := time.Unix(1, 0)
	store, _ := openRepo(t, fsys, root, when)
	store.SetRand(&countingRand{})
	name := mustName(t, "a")
	files := map[string][]byte{"SKILL.md": skillMD(t, "a", "Says hello", "ok\n")}
	for i := 0; i < 3; i++ {
		if err := store.Put(ctx, skillWith(name, "Says hello", nil), files); err != nil {
			t.Fatal(err)
		}
		if err := store.Delete(ctx, name); err != nil {
			t.Fatalf("delete %d: %v", i, err)
		}
	}
	ents, err := fsys.ReadDir(root + "/.trash")
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 3 {
		t.Fatalf("trash entries = %d", len(ents))
	}
	prefix := "a-" + fmt.Sprint(when.UTC().UnixNano()) + "-"
	seen := map[string]bool{}
	for _, ent := range ents {
		if !strings.HasPrefix(ent.Name(), prefix) || seen[ent.Name()] {
			t.Fatalf("trash name %q", ent.Name())
		}
		seen[ent.Name()] = true
	}
}

func TestRecoverAfterTornPublish(t *testing.T) {
	ctx := context.Background()
	const root = "/skills"
	name := mustName(t, "demo")
	oldFiles := map[string][]byte{"SKILL.md": skillMD(t, "demo", "Says hello", "old\n")}
	newFiles := map[string][]byte{
		"SKILL.md":  skillMD(t, "demo", "Says hello", "new\n"),
		"notes.txt": []byte("n\n"),
	}
	inner := fake.New()
	plain, _ := openRepo(t, inner, root, time.Unix(10, 0))
	if err := plain.Put(ctx, skillWith(name, "Says hello", nil), oldFiles); err != nil {
		t.Fatal(err)
	}
	broken := &failNRenames{FS: inner, base: "demo", left: 2}
	store, _ := openRepo(t, broken, root, time.Unix(10, 0))
	if err := store.Put(ctx, skillWith(name, "Says hello", nil), newFiles); err == nil {
		t.Fatal("torn publish returned nil")
	}
	if err := store.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	assertWholeSkill(t, inner, root+"/demo", oldFiles, newFiles)
	if leftovers := stageLeftovers(t, inner, root); len(leftovers) != 0 {
		t.Fatalf("stage leftovers = %v", leftovers)
	}
}

func TestRecoverJournalStates(t *testing.T) {
	ctx := context.Background()
	const root = "/skills"
	oldBody := skillMD(t, "demo", "Says hello", "old\n")
	newBody := skillMD(t, "demo", "Says hello", "new\n")

	t.Run("new only", func(t *testing.T) {
		fsys := fake.New()
		writeSkill(t, fsys, root+"/demo", oldBody)
		writeSkill(t, fsys, root+"/.stage/demo-0123456789abcdef.new", newBody)
		store, _ := openRepo(t, fsys, root, time.Unix(10, 0))
		if err := store.Recover(ctx); err != nil {
			t.Fatal(err)
		}
		assertSkillBody(t, fsys, root+"/demo/SKILL.md", oldBody)
		if _, err := fsys.Lstat(root + "/.stage/demo-0123456789abcdef.new"); err == nil {
			t.Fatal("uncommitted new tree kept")
		}
	})

	t.Run("both and no live", func(t *testing.T) {
		fsys := fake.New()
		writeSkill(t, fsys, root+"/.stage/demo-0123456789abcdef.old", oldBody)
		writeSkill(t, fsys, root+"/.stage/demo-0123456789abcdef.new", newBody)
		store, _ := openRepo(t, fsys, root, time.Unix(10, 0))
		if err := store.Recover(ctx); err != nil {
			t.Fatal(err)
		}
		assertSkillBody(t, fsys, root+"/demo/SKILL.md", newBody)
		if leftovers := stageLeftovers(t, fsys, root); len(leftovers) != 0 {
			t.Fatalf("leftovers = %v", leftovers)
		}
	})

	t.Run("old only and live new", func(t *testing.T) {
		fsys := fake.New()
		writeSkill(t, fsys, root+"/demo", newBody)
		writeSkill(t, fsys, root+"/.stage/demo-0123456789abcdef.old", oldBody)
		store, _ := openRepo(t, fsys, root, time.Unix(10, 0))
		if err := store.Recover(ctx); err != nil {
			t.Fatal(err)
		}
		assertSkillBody(t, fsys, root+"/demo/SKILL.md", newBody)
		if leftovers := stageLeftovers(t, fsys, root); len(leftovers) != 0 {
			t.Fatalf("leftovers = %v", leftovers)
		}
	})

	t.Run("old only and no live", func(t *testing.T) {
		fsys := fake.New()
		writeSkill(t, fsys, root+"/.stage/demo-0123456789abcdef.old", oldBody)
		store, _ := openRepo(t, fsys, root, time.Unix(10, 0))
		if err := store.Recover(ctx); err != nil {
			t.Fatal(err)
		}
		assertSkillBody(t, fsys, root+"/demo/SKILL.md", oldBody)
	})
}

func TestPutSucceedsWhenBackupRemoveFails(t *testing.T) {
	ctx := context.Background()
	const root = "/skills"
	name := mustName(t, "demo")
	oldFiles := map[string][]byte{"SKILL.md": skillMD(t, "demo", "Says hello", "old\n")}
	newFiles := map[string][]byte{"SKILL.md": skillMD(t, "demo", "Says hello", "new\n")}
	inner := fake.New()
	plain, _ := openRepo(t, inner, root, time.Unix(10, 0))
	if err := plain.Put(ctx, skillWith(name, "Says hello", nil), oldFiles); err != nil {
		t.Fatal(err)
	}
	var logged string
	store, _ := openRepo(t, backupFailFS{FS: inner}, root, time.Unix(10, 0))
	store.SetLog(func(format string, args ...any) {
		logged = fmt.Sprintf(format, args...)
	})
	if err := store.Put(ctx, skillWith(name, "Says hello", nil), newFiles); err != nil {
		t.Fatalf("backup removal failed the publish: %v", err)
	}
	assertFiles(t, inner, root+"/demo", newFiles)
	if !strings.Contains(logged, "backup") && !strings.Contains(logged, ".stage") {
		t.Fatalf("log = %q", logged)
	}
}

func TestFailedWriteLeavesNoStage(t *testing.T) {
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	store, _ := openRepo(t, fsys, root, time.Unix(10, 0))
	name := mustName(t, "demo")
	oldFiles := map[string][]byte{"SKILL.md": skillMD(t, "demo", "Says hello", "old\n")}
	if err := store.Put(ctx, skillWith(name, "Says hello", nil), oldFiles); err != nil {
		t.Fatal(err)
	}
	fsys.FailWriteFileAtomicBeforeRename(1)
	err := store.Put(ctx, skillWith(name, "Says hello", nil), map[string][]byte{
		"SKILL.md": skillMD(t, "demo", "Says hello", "new\n"),
	})
	if err == nil {
		t.Fatal("write failure returned nil")
	}
	assertFiles(t, fsys, root+"/demo", oldFiles)
	if leftovers := stageLeftovers(t, fsys, root); len(leftovers) != 0 {
		t.Fatalf("leftovers = %v", leftovers)
	}
}

func TestPutCaseCollisionMutant(t *testing.T) {
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	foreign := skillMD(t, "foo", "Foreign", "keep\n")
	seedDir(t, fsys, root+"/Foo", foreign)
	store, _ := openRepo(t, fsys, root, time.Unix(10, 0))
	err := store.Put(ctx, skillWith(mustName(t, "foo"), "Says hello", nil), map[string][]byte{
		"SKILL.md": skillMD(t, "foo", "Says hello", "new\n"),
	})
	if err == nil {
		t.Fatal("case collision was stored")
	}
	got, err := fsys.ReadFile(root + "/Foo/SKILL.md")
	if err != nil || !bytes.Equal(got, foreign) {
		t.Fatalf("Foo = %q %v", got, err)
	}
}

type countingRand struct{ n byte }

func (c *countingRand) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = c.n
		c.n++
	}
	return len(p), nil
}

type failNRenames struct {
	ports.FS
	base string
	left int
}

func (f *failNRenames) Rename(oldpath, newpath string) error {
	if filepath.Base(newpath) == f.base && f.left > 0 {
		f.left--
		return errBlocked
	}
	return f.FS.Rename(oldpath, newpath)
}

type backupFailFS struct{ ports.FS }

func (f backupFailFS) RemoveAll(path string) error {
	base := filepath.Base(path)
	if strings.Contains(base, "old") || strings.Contains(path, ".stage") {
		return errBlocked
	}
	return f.FS.RemoveAll(path)
}

func writeSkill(t *testing.T, fsys ports.FS, dir string, body []byte) {
	t.Helper()
	if err := fsys.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(filepath.Join(dir, "SKILL.md"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertSkillBody(t *testing.T, fsys ports.FS, path string, want []byte) {
	t.Helper()
	got, err := fsys.ReadFile(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("%s = %q %v", path, got, err)
	}
}

func assertWholeSkill(t *testing.T, fsys ports.FS, dir string, oldFiles, newFiles map[string][]byte) {
	t.Helper()
	got := map[string][]byte{}
	ents, err := fsys.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range ents {
		if ent.IsDir() {
			t.Fatalf("unexpected dir %s", ent.Name())
		}
		body, err := fsys.ReadFile(filepath.Join(dir, ent.Name()))
		if err != nil {
			t.Fatal(err)
		}
		got[ent.Name()] = body
	}
	if sameFileMap(got, oldFiles) || sameFileMap(got, newFiles) {
		return
	}
	t.Fatalf("torn skill %#v", got)
}

func sameFileMap(got, want map[string][]byte) bool {
	if len(got) != len(want) {
		return false
	}
	for name, body := range want {
		if !bytes.Equal(got[name], body) {
			return false
		}
	}
	return true
}

func stageLeftovers(t *testing.T, fsys ports.FS, root string) []string {
	t.Helper()
	var names []string
	ents, err := fsys.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range ents {
		name := ent.Name()
		if strings.HasPrefix(name, ".tmp-") || strings.HasPrefix(name, ".old-") {
			names = append(names, name)
		}
		if name != ".stage" {
			continue
		}
		kids, err := fsys.ReadDir(filepath.Join(root, ".stage"))
		if err != nil {
			t.Fatal(err)
		}
		for _, kid := range kids {
			names = append(names, ".stage/"+kid.Name())
		}
	}
	return names
}
