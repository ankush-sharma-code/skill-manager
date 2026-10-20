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
