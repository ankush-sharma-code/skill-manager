package repo_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/fake"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/osfs"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/repo"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func md(name string) []byte { return []byte("---\nname: " + name + "\ndescription: d\n---\nbody\n") }

func newOS(t *testing.T) (*repo.Repo, string) {
	root := filepath.Join(t.TempDir(), "skills")
	r, err := repo.New(osfs.New(), fixedClock{time.Unix(1, 0)}, root)
	if err != nil {
		t.Fatal(err)
	}
	return r, root
}

func put(t *testing.T, r *repo.Repo, name string, extra map[string][]byte, files []domain.FileEntry) error {
	m := map[string][]byte{"SKILL.md": md(name)}
	for k, v := range extra {
		m[k] = v
	}
	return r.Put(context.Background(), domain.Skill{Name: domain.SkillName(name), Files: files}, m)
}

func TestAuditorEscapes(t *testing.T) {
	for _, p := range []string{"../x", "/etc/x", "a/../../x", `a\..\x`, "a/\x00"} {
		r, root := newOS(t)
		if err := put(t, r, "a", map[string][]byte{p: []byte("x")}, nil); err == nil {
			t.Errorf("%q accepted", p)
		}
		if _, err := os.Stat(filepath.Join(root, "..", "x")); err == nil {
			t.Errorf("%q escaped", p)
		}
	}
}

func TestAuditorTrashSymlinkEscape(t *testing.T) {
	r, root := newOS(t)
	if err := put(t, r, "a", nil, nil); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".trash")); err != nil {
		t.Fatal(err)
	}
	_ = r.Delete(context.Background(), "a")
	ents, _ := os.ReadDir(outside)
	if len(ents) > 0 {
		t.Errorf("Delete followed .trash symlink and moved skill outside root: %v", ents[0].Name())
	}
}

func TestAuditorSkillDirSymlinkOutside(t *testing.T) {
	r, root := newOS(t)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	_ = os.WriteFile(filepath.Join(outside, "SKILL.md"), md("a"), 0o644)
	_ = os.Symlink(outside, filepath.Join(root, "a"))
	if _, err := r.Get(context.Background(), "a"); err == nil {
		t.Error("Get followed symlinked skill dir")
	}
	if err := r.Delete(context.Background(), "a"); err == nil {
		t.Log("Delete on symlinked skill dir succeeded")
	}
	if _, err := os.Stat(filepath.Join(outside, "SKILL.md")); err != nil {
		t.Error("outside content touched")
	}
}

func TestAuditorTrashCollisionTwice(t *testing.T) {
	r, root := newOS(t)
	for i := 0; i < 3; i++ {
		if err := put(t, r, "a", nil, nil); err != nil {
			t.Fatal(err)
		}
		if err := r.Delete(context.Background(), "a"); err != nil {
			t.Errorf("delete %d with same clock tick: %v", i, err)
		}
	}
	ents, _ := os.ReadDir(filepath.Join(root, ".trash"))
	if len(ents) != 3 {
		t.Errorf("trash has %d entries, want 3 (older trash overwritten or lost)", len(ents))
	}
}

func TestAuditorPermsIgnoreUmask(t *testing.T) {
	old := syscall.Umask(0o022)
	defer syscall.Umask(old)
	r, root := newOS(t)
	if err := put(t, r, "a", map[string][]byte{"run.sh": []byte("x")}, []domain.FileEntry{{RelPath: "run.sh", Mode: 0o777}}); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(filepath.Join(root, "a", "run.sh"))
	if fi.Mode().Perm()&0o022 != 0 {
		t.Errorf("file is group/world-writable (%v) despite umask 022", fi.Mode().Perm())
	}
}

func TestAuditorListOneBadDir(t *testing.T) {
	r, root := newOS(t)
	if err := put(t, r, "good", nil, nil); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(root, "stray"), 0o755) // valid name, no SKILL.md
	got, err := r.List(context.Background())
	if err != nil || len(got) != 1 {
		t.Errorf("one invalid dir breaks List: n=%d err=%v", len(got), err)
	}
}

func TestAuditorPutNameMismatch(t *testing.T) {
	r, _ := newOS(t)
	err := r.Put(context.Background(), domain.Skill{Name: "a"}, map[string][]byte{"SKILL.md": md("b")})
	if err == nil {
		t.Log("NOTE: Put stores SKILL.md whose frontmatter name != dir (tested as intended by TestPutStoresUnreadableSkill?)")
	}
}

func TestAuditorStagingLeftoverOnFailure(t *testing.T) {
	fsys := fake.New()
	r, _ := repo.New(fsys, fixedClock{time.Unix(1, 0)}, "/s")
	fsys.FailWriteFileAtomicBeforeRename(1)
	_ = put(t, r, "a", nil, nil)
	ents, _ := fsys.ReadDir("/s")
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("leftover staging %s", e.Name())
		}
	}
}

var _ ports.FS = osfs.New()
