package repo_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/fake"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/osfs"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/repo"
	"github.com/ankush-sharma-code/skill-manager/internal/skillfile"
)

func TestRepoRoundTrip(t *testing.T) {
	t.Parallel()
	for _, backend := range []struct {
		name string
		open func(t *testing.T) (ports.FS, string)
	}{
		{name: "os", open: func(t *testing.T) (ports.FS, string) { return osfs.New(), t.TempDir() }},
		{name: "fake", open: func(t *testing.T) (ports.FS, string) { return fake.New(), "/skills" }},
	} {
		backend := backend
		t.Run(backend.name, func(t *testing.T) {
			t.Parallel()
			fsys, root := backend.open(t)
			when := time.Date(2026, 10, 20, 15, 4, 5, 123, time.UTC)
			store, clock := openRepo(t, fsys, root, when)
			ctx := context.Background()

			gotList, err := store.List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(gotList) != 0 {
				t.Fatalf("List empty = %d", len(gotList))
			}

			demo := mustName(t, "demo")
			first := map[string][]byte{
				"SKILL.md":       skillMD(t, "demo", "Says hello", "one\n"),
				"scripts/run.sh": []byte("echo hi\n"),
			}
			modes := map[string]fs.FileMode{"scripts/run.sh": 0o755}
			if err := store.Put(ctx, skillWith(demo, "Says hello", modes), first); err != nil {
				t.Fatal(err)
			}
			assertSkill(t, fsys, store, root, demo, "Says hello", first, modes)

			other := mustName(t, "other")
			otherFiles := map[string][]byte{
				"SKILL.md": skillMD(t, "other", "Second skill", "two\n"),
			}
			if err := store.Put(ctx, skillWith(other, "Second skill", nil), otherFiles); err != nil {
				t.Fatal(err)
			}
			listed, err := store.List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, skill := range listed {
				names = append(names, string(skill.Name))
			}
			sort.Strings(names)
			if strings.Join(names, ",") != "demo,other" {
				t.Fatalf("List = %v", names)
			}

			second := map[string][]byte{
				"SKILL.md":  skillMD(t, "demo", "Says hello", "replaced\n"),
				"notes.txt": []byte("note\n"),
			}
			if err := store.Put(ctx, skillWith(demo, "Says hello", nil), second); err != nil {
				t.Fatal(err)
			}
			assertSkill(t, fsys, store, root, demo, "Says hello", second, nil)
			if _, err := fsys.Lstat(filepath.Join(root, "demo", "scripts", "run.sh")); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("old file err = %v", err)
			}

			if err := store.Delete(ctx, demo); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Get(ctx, demo); !errors.Is(err, repo.ErrNotFound) {
				t.Fatalf("Get after delete = %v", err)
			}
			if _, err := fsys.Lstat(filepath.Join(root, "demo")); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("skill dir after delete = %v", err)
			}
			trash := filepath.Join(root, ".trash", "demo-"+strconv.FormatInt(clock.Now().UnixNano(), 10))
			body, err := fsys.ReadFile(filepath.Join(trash, "notes.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != "note\n" {
				t.Fatalf("trash notes = %q", body)
			}
			if _, err := store.Get(ctx, other); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPutFailureLeavesPreviousSkill(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := mustName(t, "demo")
	oldFiles := map[string][]byte{
		"SKILL.md":  skillMD(t, "demo", "Says hello", "old\n"),
		"notes.txt": []byte("old-notes\n"),
	}
	newFiles := map[string][]byte{
		"SKILL.md":  skillMD(t, "demo", "Says hello", "new\n"),
		"other.txt": []byte("new-other\n"),
	}

	t.Run("write fails before rename", func(t *testing.T) {
		fsys := fake.New()
		store, _ := openRepo(t, fsys, "/skills", time.Unix(10, 0))
		if err := store.Put(ctx, skillWith(name, "Says hello", nil), oldFiles); err != nil {
			t.Fatal(err)
		}
		fsys.FailWriteFileAtomicBeforeRename(2)
		err := store.Put(ctx, skillWith(name, "Says hello", nil), newFiles)
		if !errors.Is(err, fake.ErrInjected) {
			t.Fatalf("Put err = %v", err)
		}
		assertFiles(t, fsys, "/skills/demo", oldFiles)
	})

	t.Run("publish rename fails", func(t *testing.T) {
		fsys := fake.New()
		store, _ := openRepo(t, fsys, "/skills", time.Unix(10, 0))
		if err := store.Put(ctx, skillWith(name, "Says hello", nil), oldFiles); err != nil {
			t.Fatal(err)
		}
		fsys.FailNextRenameTo("demo")
		err := store.Put(ctx, skillWith(name, "Says hello", nil), newFiles)
		if !errors.Is(err, fake.ErrInjected) {
			t.Fatalf("Put err = %v", err)
		}
		assertFiles(t, fsys, "/skills/demo", oldFiles)
		if err := store.Put(ctx, skillWith(name, "Says hello", nil), newFiles); err != nil {
			t.Fatal(err)
		}
		assertFiles(t, fsys, "/skills/demo", newFiles)
	})
}

func TestCaseCollisionRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := mustName(t, "foo")
	files := map[string][]byte{
		"SKILL.md": skillMD(t, "foo", "Says hello", "body\n"),
	}
	foreign := skillMD(t, "foo", "Foreign", "keep\n")

	t.Run("case-sensitive sibling", func(t *testing.T) {
		fsys := fake.New()
		const root = "/skills"
		seedDir(t, fsys, root+"/Foo", foreign)
		before := dirNames(t, fsys, root)
		store, _ := openRepo(t, fsys, root, time.Unix(10, 0))
		err := store.Put(ctx, skillWith(name, "Says hello", nil), files)
		if !errors.Is(err, repo.ErrCaseCollision) {
			t.Fatalf("Put err = %v", err)
		}
		if names := dirNames(t, fsys, root); !sameNames(before, names) {
			t.Fatalf("root names = %v, want %v", names, before)
		}
		got, err := fsys.ReadFile(root + "/Foo/SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, foreign) {
			t.Fatalf("Foo changed to %q", got)
		}
	})

	t.Run("case-folding fake", func(t *testing.T) {
		fsys := fake.NewCaseFolding()
		const root = "/skills"
		seedDir(t, fsys, root+"/Foo", foreign)
		store, _ := openRepo(t, fsys, root, time.Unix(10, 0))
		err := store.Put(ctx, skillWith(name, "Says hello", nil), files)
		if !errors.Is(err, repo.ErrCaseCollision) {
			t.Fatalf("Put err = %v", err)
		}
		names := dirNames(t, fsys, root)
		if len(names) != 1 || names[0] != "Foo" {
			t.Fatalf("root names = %v", names)
		}
		got, err := fsys.ReadFile(root + "/Foo/SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, foreign) {
			t.Fatalf("Foo changed to %q", got)
		}
		if _, err := store.Get(ctx, name); !errors.Is(err, repo.ErrCaseCollision) {
			t.Fatalf("Get err = %v", err)
		}
		if err := store.Delete(ctx, name); !errors.Is(err, repo.ErrCaseCollision) {
			t.Fatalf("Delete err = %v", err)
		}
		if _, err := fsys.Lstat(filepath.Join(root, ".trash")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("trash created: %v", err)
		}
	})
}

func TestPutRejectsBadInput(t *testing.T) {
	t.Parallel()
	fsys := fake.New()
	store, _ := openRepo(t, fsys, "/skills", time.Unix(10, 0))
	ctx := context.Background()
	err := store.Put(ctx, domain.Skill{Name: "Foo"}, map[string][]byte{
		"SKILL.md": []byte("x"),
	})
	if !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("invalid name err = %v", err)
	}
	err = store.Put(ctx, skillWith(mustName(t, "demo"), "Says hello", nil), map[string][]byte{
		"SKILL.md": skillMD(t, "demo", "Says hello", "ok\n"),
		"../evil":  []byte("nope"),
	})
	if !errors.Is(err, skillfile.ErrBadPath) {
		t.Fatalf("escape err = %v", err)
	}
	err = store.Put(ctx, skillWith(mustName(t, "demo"), "Says hello", nil), map[string][]byte{
		"readme.md": []byte("no skill"),
	})
	if !errors.Is(err, domain.ErrMissingSkillMD) {
		t.Fatalf("missing SKILL.md err = %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.List(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled List err = %v", err)
	}
}

func TestRepoConstructorAndContext(t *testing.T) {
	t.Parallel()
	fsys := fake.New()
	clock := fake.NewClock(time.Unix(10, 0))
	if _, err := repo.New(nil, clock, "/skills"); err == nil {
		t.Fatal("nil fs accepted")
	}
	if _, err := repo.New(fsys, nil, "/skills"); err == nil {
		t.Fatal("nil clock accepted")
	}
	if _, err := repo.New(fsys, clock, "skills"); err == nil {
		t.Fatal("relative root accepted")
	}

	store, _ := openRepo(t, fsys, "/skills", time.Unix(10, 0))
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	name := mustName(t, "demo")
	if _, err := store.Get(canceled, name); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get err = %v", err)
	}
	if err := store.Put(canceled, skillWith(name, "Says hello", nil), map[string][]byte{
		"SKILL.md": skillMD(t, "demo", "Says hello", "x\n"),
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Put err = %v", err)
	}
	if err := store.Delete(canceled, name); !errors.Is(err, context.Canceled) {
		t.Fatalf("Delete err = %v", err)
	}
	if _, err := store.Get(context.Background(), "Foo"); !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("Get name err = %v", err)
	}
	if err := store.Delete(context.Background(), "Foo"); !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("Delete name err = %v", err)
	}
	if _, err := store.Get(context.Background(), name); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("Get missing err = %v", err)
	}
	if err := store.Delete(context.Background(), name); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("Delete missing err = %v", err)
	}
}

func TestRepoReadsRejectOddTrees(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const root = "/skills"
	fsys := fake.New()
	store, _ := openRepo(t, fsys, root, time.Unix(10, 0))
	name := mustName(t, "demo")
	files := map[string][]byte{"SKILL.md": skillMD(t, "demo", "Says hello", "ok\n")}
	if err := store.Put(ctx, skillWith(name, "Says hello", nil), files); err != nil {
		t.Fatal(err)
	}
	if err := fsys.MkdirAll(root+"/Foo", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.MkdirAll(root+"/.hidden", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/notes.txt", []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Symlink(root+"/demo", root+"/alias"); err != nil {
		t.Fatal(err)
	}
	listed, err := store.List(ctx)
	if err != nil || len(listed) != 1 || listed[0].Name != name {
		t.Fatalf("List = %v %v", listed, err)
	}
	if _, err := store.Get(ctx, mustName(t, "alias")); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("Get symlink err = %v", err)
	}
	if err := store.Delete(ctx, mustName(t, "alias")); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("Delete symlink err = %v", err)
	}
	if _, err := fsys.Lstat(root + "/alias"); err != nil {
		t.Fatalf("symlink removed: %v", err)
	}
	if err := fsys.WriteFileAtomic(root+"/plain", []byte("p"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, mustName(t, "plain")); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("Get file err = %v", err)
	}
	if err := store.Delete(ctx, mustName(t, "plain")); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("Delete file err = %v", err)
	}

	if err := fsys.Symlink("elsewhere", root+"/demo/scripts"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, name); err == nil || !strings.Contains(err.Error(), "symlink in skill tree") {
		t.Fatalf("Get symlink tree err = %v", err)
	}
}

func TestPutStoresUnreadableSkill(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fsys := fake.New()
	store, _ := openRepo(t, fsys, "/skills", time.Unix(10, 0))
	name := mustName(t, "demo")
	err := store.Put(ctx, skillWith(name, "Says hello", nil), map[string][]byte{
		"SKILL.md": []byte("not frontmatter\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, name); !errors.Is(err, domain.ErrMissingFrontmatter) {
		t.Fatalf("Get err = %v", err)
	}
	if _, err := store.List(ctx); !errors.Is(err, domain.ErrMissingFrontmatter) {
		t.Fatalf("List err = %v", err)
	}

	mismatch := mustName(t, "other")
	if err := store.Put(ctx, skillWith(mismatch, "Says hello", nil), map[string][]byte{
		"SKILL.md": skillMD(t, "demo", "Says hello", "x\n"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, mismatch); !errors.Is(err, domain.ErrNameMismatch) {
		t.Fatalf("mismatch err = %v", err)
	}
}

func TestPutZeroModeAndDotPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fsys := fake.New()
	const root = "/skills"
	store, _ := openRepo(t, fsys, root, time.Unix(10, 0))
	name := mustName(t, "demo")
	skill := domain.Skill{
		Name:        name,
		Description: "Says hello",
		Files: []domain.FileEntry{
			{RelPath: "./SKILL.md", Mode: 0},
			{RelPath: "./scripts/run.sh", Mode: 0},
		},
	}
	files := map[string][]byte{
		"./SKILL.md":     skillMD(t, "demo", "Says hello", "ok\n"),
		"scripts/run.sh": []byte("echo\n"),
	}
	if err := store.Put(ctx, skill, files); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"SKILL.md", "scripts/run.sh"} {
		info, err := fsys.Lstat(filepath.Join(root, "demo", rel))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("%s mode = %o", rel, info.Mode().Perm())
		}
	}
}

func TestPutRollbackAndPublishFailures(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := mustName(t, "demo")
	oldFiles := map[string][]byte{
		"SKILL.md": skillMD(t, "demo", "Says hello", "old\n"),
	}
	newFiles := map[string][]byte{
		"SKILL.md":  skillMD(t, "demo", "Says hello", "new\n"),
		"notes.txt": []byte("n\n"),
	}
	const root = "/skills"

	t.Run("create rename fails", func(t *testing.T) {
		inner := fake.New()
		store, _ := openRepo(t, renamingFS{FS: inner, failBase: "demo"}, root, time.Unix(10, 0))
		err := store.Put(ctx, skillWith(name, "Says hello", nil), oldFiles)
		if !errors.Is(err, errBlocked) {
			t.Fatalf("Put err = %v", err)
		}
		if _, statErr := inner.Lstat(root + "/demo"); !errors.Is(statErr, fs.ErrNotExist) {
			t.Fatalf("skill created: %v", statErr)
		}
		assertNoPrefix(t, inner, root, ".tmp-")
	})

	t.Run("aside rename fails", func(t *testing.T) {
		inner := fake.New()
		plain, _ := openRepo(t, inner, root, time.Unix(10, 0))
		if err := plain.Put(ctx, skillWith(name, "Says hello", nil), oldFiles); err != nil {
			t.Fatal(err)
		}
		store, _ := openRepo(t, renamingFS{FS: inner, failSuffix: ".old"}, root, time.Unix(10, 0))
		err := store.Put(ctx, skillWith(name, "Says hello", nil), newFiles)
		if !errors.Is(err, errBlocked) {
			t.Fatalf("Put err = %v", err)
		}
		assertFiles(t, inner, root+"/demo", oldFiles)
		if leftovers := stageLeftovers(t, inner, root); len(leftovers) != 0 {
			t.Fatalf("stage leftovers = %v", leftovers)
		}
	})

	t.Run("rollback rename fails", func(t *testing.T) {
		inner := fake.New()
		plain, _ := openRepo(t, inner, root, time.Unix(10, 0))
		if err := plain.Put(ctx, skillWith(name, "Says hello", nil), oldFiles); err != nil {
			t.Fatal(err)
		}
		store, _ := openRepo(t, renamingFS{FS: inner, failBase: "demo"}, root, time.Unix(10, 0))
		err := store.Put(ctx, skillWith(name, "Says hello", nil), newFiles)
		if !errors.Is(err, errBlocked) || !strings.Contains(err.Error(), "rollback") {
			t.Fatalf("Put err = %v", err)
		}
		if _, statErr := inner.Lstat(root + "/demo"); !errors.Is(statErr, fs.ErrNotExist) {
			t.Fatalf("live skill = %v", statErr)
		}
		stage := filepath.Join(root, ".stage")
		backup := findSuffix(t, inner, stage, ".old")
		assertFiles(t, inner, backup, oldFiles)
		staged := findSuffix(t, inner, stage, ".new")
		body, err := inner.ReadFile(staged + "/SKILL.md")
		if err != nil || !bytes.Contains(body, []byte("new\n")) {
			t.Fatalf("staging = %q %v", body, err)
		}
	})

	t.Run("backup remove fails", func(t *testing.T) {
		inner := fake.New()
		plain, _ := openRepo(t, inner, root, time.Unix(10, 0))
		if err := plain.Put(ctx, skillWith(name, "Says hello", nil), oldFiles); err != nil {
			t.Fatal(err)
		}
		store, _ := openRepo(t, removingFS{FS: inner, failSuffix: ".old"}, root, time.Unix(10, 0))
		err := store.Put(ctx, skillWith(name, "Says hello", nil), newFiles)
		if !errors.Is(err, errBlocked) {
			t.Fatalf("Put err = %v", err)
		}
		assertFiles(t, inner, root+"/demo", newFiles)
		if findSuffix(t, inner, filepath.Join(root, ".stage"), ".old") == "" {
			t.Fatal("backup removed")
		}
	})
}

func TestRepoFilesystemErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const root = "/skills"
	name := mustName(t, "demo")
	files := map[string][]byte{"SKILL.md": skillMD(t, "demo", "Says hello", "ok\n")}
	nested := map[string][]byte{
		"SKILL.md":       skillMD(t, "demo", "Says hello", "ok\n"),
		"scripts/run.sh": []byte("echo\n"),
	}

	t.Run("root read fails", func(t *testing.T) {
		inner := fake.New()
		plain, _ := openRepo(t, inner, root, time.Unix(10, 0))
		if err := plain.Put(ctx, skillWith(name, "Says hello", nil), files); err != nil {
			t.Fatal(err)
		}
		store, _ := openRepo(t, readingFS{FS: inner, failPath: root}, root, time.Unix(10, 0))
		if _, err := store.List(ctx); !errors.Is(err, errBlocked) {
			t.Fatalf("List err = %v", err)
		}
		if _, err := store.Get(ctx, name); !errors.Is(err, errBlocked) {
			t.Fatalf("Get err = %v", err)
		}
		if err := store.Delete(ctx, name); !errors.Is(err, errBlocked) {
			t.Fatalf("Delete err = %v", err)
		}
		if err := store.Put(ctx, skillWith(name, "Says hello", nil), files); !errors.Is(err, errBlocked) {
			t.Fatalf("Put err = %v", err)
		}
		assertFiles(t, inner, root+"/demo", files)
	})

	t.Run("root mkdir fails", func(t *testing.T) {
		inner := fake.New()
		store, _ := openRepo(t, mkdirFS{FS: inner, failPath: root}, root, time.Unix(10, 0))
		err := store.Put(ctx, skillWith(name, "Says hello", nil), files)
		if !errors.Is(err, errBlocked) {
			t.Fatalf("Put err = %v", err)
		}
		if _, statErr := inner.Lstat(root + "/demo"); !errors.Is(statErr, fs.ErrNotExist) {
			t.Fatalf("skill created: %v", statErr)
		}
	})

	t.Run("staging mkdir fails", func(t *testing.T) {
		inner := fake.New()
		plain, _ := openRepo(t, inner, root, time.Unix(10, 0))
		if err := plain.Put(ctx, skillWith(name, "Says hello", nil), files); err != nil {
			t.Fatal(err)
		}
		store, _ := openRepo(t, mkdirFS{FS: inner, failSuffix: ".new"}, root, time.Unix(10, 0))
		err := store.Put(ctx, skillWith(name, "Says hello", nil), files)
		if !errors.Is(err, errBlocked) {
			t.Fatalf("Put err = %v", err)
		}
		assertFiles(t, inner, root+"/demo", files)
	})

	t.Run("nested mkdir fails", func(t *testing.T) {
		inner := fake.New()
		plain, _ := openRepo(t, inner, root, time.Unix(10, 0))
		if err := plain.Put(ctx, skillWith(name, "Says hello", nil), files); err != nil {
			t.Fatal(err)
		}
		store, _ := openRepo(t, mkdirFS{FS: inner, failSuffix: "/scripts"}, root, time.Unix(10, 0))
		err := store.Put(ctx, skillWith(name, "Says hello", nil), nested)
		if !errors.Is(err, errBlocked) {
			t.Fatalf("Put err = %v", err)
		}
		assertFiles(t, inner, root+"/demo", files)
	})

	t.Run("skill lstat fails", func(t *testing.T) {
		inner := fake.New()
		plain, _ := openRepo(t, inner, root, time.Unix(10, 0))
		if err := plain.Put(ctx, skillWith(name, "Says hello", nil), files); err != nil {
			t.Fatal(err)
		}
		store, _ := openRepo(t, lstatingFS{FS: inner, failPath: filepath.Join(root, "demo")}, root, time.Unix(10, 0))
		if _, err := store.Get(ctx, name); !errors.Is(err, errBlocked) {
			t.Fatalf("Get err = %v", err)
		}
		if err := store.Delete(ctx, name); !errors.Is(err, errBlocked) {
			t.Fatalf("Delete err = %v", err)
		}
		assertFiles(t, inner, root+"/demo", files)
	})

	t.Run("tree read fails", func(t *testing.T) {
		inner := fake.New()
		plain, _ := openRepo(t, inner, root, time.Unix(10, 0))
		if err := plain.Put(ctx, skillWith(name, "Says hello", nil), nested); err != nil {
			t.Fatal(err)
		}
		store, _ := openRepo(t, readingFS{FS: inner, failPath: filepath.Join(root, "demo", "scripts")}, root, time.Unix(10, 0))
		if _, err := store.Get(ctx, name); !errors.Is(err, errBlocked) {
			t.Fatalf("Get err = %v", err)
		}
	})

	t.Run("file read fails", func(t *testing.T) {
		inner := fake.New()
		plain, _ := openRepo(t, inner, root, time.Unix(10, 0))
		if err := plain.Put(ctx, skillWith(name, "Says hello", nil), files); err != nil {
			t.Fatal(err)
		}
		store, _ := openRepo(t, fileFS{FS: inner, failPath: filepath.Join(root, "demo", "SKILL.md")}, root, time.Unix(10, 0))
		if _, err := store.Get(ctx, name); !errors.Is(err, errBlocked) {
			t.Fatalf("Get err = %v", err)
		}
	})

	t.Run("dir entry info fails", func(t *testing.T) {
		inner := fake.New()
		plain, _ := openRepo(t, inner, root, time.Unix(10, 0))
		if err := plain.Put(ctx, skillWith(name, "Says hello", nil), files); err != nil {
			t.Fatal(err)
		}
		store, _ := openRepo(t, infoFS{FS: inner, failPath: filepath.Join(root, "demo")}, root, time.Unix(10, 0))
		if _, err := store.Get(ctx, name); !errors.Is(err, errBlocked) {
			t.Fatalf("Get err = %v", err)
		}
	})

	t.Run("empty directory name is ignored", func(t *testing.T) {
		inner := fake.New()
		plain, _ := openRepo(t, inner, root, time.Unix(10, 0))
		if err := plain.Put(ctx, skillWith(name, "Says hello", nil), files); err != nil {
			t.Fatal(err)
		}
		store, _ := openRepo(t, emptyNameFS{FS: inner, root: root}, root, time.Unix(10, 0))
		got, err := store.Get(ctx, name)
		if err != nil || got.Name != name {
			t.Fatalf("Get = %s %v", got.Name, err)
		}
	})
}

func TestDeleteTrashEdges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const root = "/skills"
	name := mustName(t, "demo")
	files := map[string][]byte{"SKILL.md": skillMD(t, "demo", "Says hello", "ok\n")}
	when := time.Unix(10, 5).UTC()

	t.Run("trash mkdir fails", func(t *testing.T) {
		inner := fake.New()
		plain, _ := openRepo(t, inner, root, when)
		if err := plain.Put(ctx, skillWith(name, "Says hello", nil), files); err != nil {
			t.Fatal(err)
		}
		store, _ := openRepo(t, mkdirFS{FS: inner, failPath: filepath.Join(root, ".trash")}, root, when)
		if err := store.Delete(ctx, name); !errors.Is(err, errBlocked) {
			t.Fatalf("Delete err = %v", err)
		}
		assertFiles(t, inner, root+"/demo", files)
	})

	t.Run("trash stat fails", func(t *testing.T) {
		inner := fake.New()
		plain, _ := openRepo(t, inner, root, when)
		if err := plain.Put(ctx, skillWith(name, "Says hello", nil), files); err != nil {
			t.Fatal(err)
		}
		store, _ := openRepo(t, lstatingFS{FS: inner, failPrefix: "demo-"}, root, when)
		if err := store.Delete(ctx, name); !errors.Is(err, errBlocked) {
			t.Fatalf("Delete err = %v", err)
		}
		assertFiles(t, inner, root+"/demo", files)
	})

	t.Run("trash name already exists", func(t *testing.T) {
		fsys := fake.New()
		store, clock := openRepo(t, fsys, root, when)
		if err := store.Put(ctx, skillWith(name, "Says hello", nil), files); err != nil {
			t.Fatal(err)
		}
		blocked := filepath.Join(root, ".trash", "demo-"+strconv.FormatInt(clock.Now().UTC().UnixNano(), 10))
		if err := fsys.MkdirAll(blocked, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := store.Delete(ctx, name); err != nil {
			t.Fatal(err)
		}
		if _, err := fsys.Lstat(filepath.Join(root, "demo")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("skill remains: %v", err)
		}
		if _, err := fsys.Lstat(blocked); err != nil {
			t.Fatalf("blocked trash missing: %v", err)
		}
		body, err := fsys.ReadFile(blocked + "-2/SKILL.md")
		if err != nil || !bytes.Contains(body, []byte("ok\n")) {
			t.Fatalf("trash -2 = %q %v", body, err)
		}
	})
}

func TestBothSpellingsRejectPut(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const root = "/skills"
	fsys := fake.New()
	if err := fsys.MkdirAll(root+"/Foo", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/Foo/SKILL.md", skillMD(t, "foo", "Foreign", "keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.MkdirAll(root+"/foo", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(root+"/foo/SKILL.md", skillMD(t, "foo", "Says hello", "mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, _ := openRepo(t, fsys, root, time.Unix(10, 0))
	name := mustName(t, "foo")
	err := store.Put(ctx, skillWith(name, "Says hello", nil), map[string][]byte{
		"SKILL.md": skillMD(t, "foo", "Says hello", "new\n"),
	})
	if !errors.Is(err, repo.ErrCaseCollision) {
		t.Fatalf("Put err = %v", err)
	}
	got, err := store.Get(ctx, name)
	if err != nil || got.Description != "Says hello" {
		t.Fatalf("Get = %q %v", got.Description, err)
	}
}

var errBlocked = errors.New("blocked")

type renamingFS struct {
	ports.FS
	failBase   string
	failPrefix string
	failSuffix string
}

func (f renamingFS) Rename(oldpath, newpath string) error {
	base := filepath.Base(newpath)
	if f.failBase != "" && base == f.failBase {
		return errBlocked
	}
	if f.failPrefix != "" && strings.HasPrefix(base, f.failPrefix) {
		return errBlocked
	}
	if f.failSuffix != "" && strings.HasSuffix(base, f.failSuffix) {
		return errBlocked
	}
	return f.FS.Rename(oldpath, newpath)
}

type removingFS struct {
	ports.FS
	failPrefix string
	failSuffix string
}

func (f removingFS) RemoveAll(path string) error {
	base := filepath.Base(path)
	if f.failPrefix != "" && strings.HasPrefix(base, f.failPrefix) {
		return errBlocked
	}
	if f.failSuffix != "" && strings.HasSuffix(base, f.failSuffix) {
		return errBlocked
	}
	return f.FS.RemoveAll(path)
}

type readingFS struct {
	ports.FS
	failPath string
}

func (f readingFS) ReadDir(path string) ([]fs.DirEntry, error) {
	if path == f.failPath {
		return nil, errBlocked
	}
	return f.FS.ReadDir(path)
}

type mkdirFS struct {
	ports.FS
	failPath   string
	failPrefix string
	failSuffix string
}

func (f mkdirFS) MkdirAll(path string, mode fs.FileMode) error {
	base := filepath.Base(path)
	if f.failPath != "" && path == f.failPath {
		return errBlocked
	}
	if f.failPrefix != "" && strings.HasPrefix(base, f.failPrefix) {
		return errBlocked
	}
	if f.failSuffix != "" && strings.HasSuffix(path, f.failSuffix) {
		return errBlocked
	}
	return f.FS.MkdirAll(path, mode)
}

type lstatingFS struct {
	ports.FS
	failPath   string
	failPrefix string
}

func (f lstatingFS) Lstat(path string) (fs.FileInfo, error) {
	if f.failPath != "" && path == f.failPath {
		return nil, errBlocked
	}
	if f.failPrefix != "" && strings.HasPrefix(filepath.Base(path), f.failPrefix) {
		return nil, errBlocked
	}
	return f.FS.Lstat(path)
}

type fileFS struct {
	ports.FS
	failPath string
}

func (f fileFS) ReadFile(path string) ([]byte, error) {
	if path == f.failPath {
		return nil, errBlocked
	}
	return f.FS.ReadFile(path)
}

type infoFS struct {
	ports.FS
	failPath string
}

func (f infoFS) ReadDir(path string) ([]fs.DirEntry, error) {
	ents, err := f.FS.ReadDir(path)
	if err != nil || path != f.failPath {
		return ents, err
	}
	out := make([]fs.DirEntry, len(ents))
	for i, ent := range ents {
		out[i] = infoErrEntry{ent}
	}
	return out, nil
}

type infoErrEntry struct{ fs.DirEntry }

func (infoErrEntry) Info() (fs.FileInfo, error) { return nil, errBlocked }

type emptyNameFS struct {
	ports.FS
	root string
}

func (f emptyNameFS) ReadDir(path string) ([]fs.DirEntry, error) {
	ents, err := f.FS.ReadDir(path)
	if err != nil {
		return nil, err
	}
	if path == f.root {
		ents = append(ents, staticEntry{name: "", dir: true})
	}
	return ents, nil
}

type staticEntry struct {
	name string
	dir  bool
}

func (s staticEntry) Name() string { return s.name }
func (s staticEntry) IsDir() bool  { return s.dir }
func (s staticEntry) Type() fs.FileMode {
	if s.dir {
		return fs.ModeDir
	}
	return 0
}
func (s staticEntry) Info() (fs.FileInfo, error) { return nil, errBlocked }

func assertNoPrefix(t *testing.T, fsys ports.FS, dir, prefix string) {
	t.Helper()
	for _, name := range dirNames(t, fsys, dir) {
		if strings.HasPrefix(name, prefix) {
			t.Fatalf("found %s", name)
		}
	}
}

func findSuffix(t *testing.T, fsys ports.FS, dir, suffix string) string {
	t.Helper()
	for _, name := range dirNames(t, fsys, dir) {
		if strings.HasSuffix(name, suffix) {
			return filepath.Join(dir, name)
		}
	}
	t.Fatalf("no %s under %s", suffix, dir)
	return ""
}

func openRepo(t *testing.T, fsys ports.FS, root string, now time.Time) (*repo.Repo, *fake.Clock) {
	t.Helper()
	clock := fake.NewClock(now)
	store, err := repo.New(fsys, clock, root)
	if err != nil {
		t.Fatal(err)
	}
	return store, clock
}

func mustName(t *testing.T, name string) domain.SkillName {
	t.Helper()
	parsed, err := domain.ParseSkillName(name)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func skillWith(name domain.SkillName, description string, modes map[string]fs.FileMode) domain.Skill {
	var files []domain.FileEntry
	for rel, mode := range modes {
		files = append(files, domain.FileEntry{RelPath: rel, Mode: mode})
	}
	return domain.Skill{Name: name, Description: description, Files: files}
}

func skillMD(t *testing.T, name, description, body string) []byte {
	t.Helper()
	raw, err := skillfile.Serialize(domain.SkillName(name), description, nil, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertSkill(t *testing.T, fsys ports.FS, store *repo.Repo, root string, name domain.SkillName, description string, files map[string][]byte, modes map[string]fs.FileMode) {
	t.Helper()
	got, err := store.Get(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != name || got.Description != description {
		t.Fatalf("Get = %s %q", got.Name, got.Description)
	}
	assertFiles(t, fsys, filepath.Join(root, string(name)), files)
	for rel, mode := range modes {
		info, err := fsys.Lstat(filepath.Join(root, string(name), rel))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != mode {
			t.Fatalf("%s mode = %o, want %o", rel, info.Mode().Perm(), mode)
		}
	}
	inputs := make([]skillfile.File, 0, len(files))
	for rel, body := range files {
		mode := fs.FileMode(0o644)
		if modes[rel] != 0 {
			mode = modes[rel]
		}
		inputs = append(inputs, skillfile.File{RelPath: rel, Mode: mode, Content: body})
	}
	want, err := skillfile.TreeDigest(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if got.Digest != want {
		t.Fatalf("digest = %x, want %x", got.Digest, want)
	}
}

func assertFiles(t *testing.T, fsys ports.FS, dir string, files map[string][]byte) {
	t.Helper()
	got := map[string][]byte{}
	var walk func(string)
	walk = func(rel string) {
		t.Helper()
		abs := dir
		if rel != "" {
			abs = filepath.Join(dir, filepath.FromSlash(rel))
		}
		ents, err := fsys.ReadDir(abs)
		if err != nil {
			t.Fatal(err)
		}
		for _, ent := range ents {
			child := ent.Name()
			if rel != "" {
				child = rel + "/" + ent.Name()
			}
			if ent.IsDir() {
				walk(child)
				continue
			}
			body, err := fsys.ReadFile(filepath.Join(dir, filepath.FromSlash(child)))
			if err != nil {
				t.Fatal(err)
			}
			got[child] = body
		}
	}
	walk("")
	if len(got) != len(files) {
		t.Fatalf("files = %#v, want %#v", got, files)
	}
	for rel, body := range files {
		if !bytes.Equal(got[rel], body) {
			t.Fatalf("%s = %q, want %q", rel, got[rel], body)
		}
	}
}

func seedDir(t *testing.T, fsys ports.FS, dir string, skill []byte) {
	t.Helper()
	if err := fsys.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic(dir+"/SKILL.md", skill, 0o644); err != nil {
		t.Fatal(err)
	}
}

func dirNames(t *testing.T, fsys ports.FS, dir string) []string {
	t.Helper()
	ents, err := fsys.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(ents))
	for _, ent := range ents {
		names = append(names, ent.Name())
	}
	sort.Strings(names)
	return names
}

func sameNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
