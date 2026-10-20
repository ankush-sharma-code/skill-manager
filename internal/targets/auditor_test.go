package targets_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/fake"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/osfs"
	"github.com/ankush-sharma-code/skill-manager/internal/targets"
)

func mk(t *testing.T, p ...string) string {
	d := filepath.Join(p...)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

// D1: no skill registered twice across overlapping roots, incl. parent-dir symlinks.
func TestAuditorDedupeParentSymlink(t *testing.T) {
	home := t.TempDir()
	mk(t, home, ".cursor", "skills")
	if err := os.Symlink(filepath.Join(home, ".cursor"), filepath.Join(home, ".claude")); err != nil {
		t.Fatal(err)
	}
	fsys := osfs.New()
	ts, _, _ := targets.Propose(fsys, home)
	paths, _ := targets.SlotPaths(fsys, []domain.Target{
		{ID: "claude-user", Root: filepath.Join(home, ".claude", "skills")},
		{ID: "cursor-user", Root: filepath.Join(home, ".cursor", "skills")},
	}, "a")
	if len(paths) != 1 {
		t.Errorf("same physical dir registered %d times via parent symlink: %v (proposed %d targets)", len(paths), paths, len(ts))
	}
}

func TestAuditorObserveFIFODoesNotHang(t *testing.T) {
	root := t.TempDir()
	skill := mk(t, root, "a")
	_ = os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("x"), 0o644)
	if err := syscall.Mkfifo(filepath.Join(skill, "pipe"), 0o644); err != nil {
		t.Skip(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := targets.NewCodex("/r").Observe(osfs.New(), domain.Target{ID: "t", Root: root})
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Observe hangs on a FIFO inside a target skill dir (ReadFile on non-regular file)")
	}
}

func TestAuditorObserveCaseAlias(t *testing.T) {
	fsys := fake.NewCaseFolding()
	_ = fsys.MkdirAll("/t/Foo", 0o755)
	_ = fsys.WriteFileAtomic("/t/Foo/SKILL.md", []byte("user"), 0o644)
	obs, err := targets.NewCursor("/r").Observe(fsys, domain.Target{ID: "t", Root: "/t"})
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) == 0 {
		t.Error("Observe drops user dir 'Foo' that aliases skill 'foo' on a case-insensitive FS; planner would see foo as Missing and may overwrite it")
	}
}

func TestAuditorCustomRootGuards(t *testing.T) {
	for _, root := range []string{"/", "/home/u/.skills", "/home/u/.skills/sub", "/x/../home/u/.skills"} {
		if _, err := targets.OptIn("pi", root, domain.SyncCopy); err == nil {
			t.Errorf("OptIn accepted dangerous root %q (filesystem root or inside the repo)", root)
		}
	}
	for _, root := range []string{"~/x", "$HOME/x", "rel/x", ""} {
		if _, err := targets.OptIn("pi", root, domain.SyncCopy); err == nil {
			t.Errorf("OptIn accepted non-absolute %q", root)
		}
	}
}

func TestAuditorCustomSymlinkConsistency(t *testing.T) {
	c := targets.NewCustom("/r")
	if c.Supports(domain.SyncSymlink) != c.SupportsSymlink() {
		t.Errorf("Custom: Supports(symlink)=%v but SupportsSymlink()=%v", c.Supports(domain.SyncSymlink), c.SupportsSymlink())
	}
}

func TestAuditorObserveRootIsRepo(t *testing.T) {
	home := t.TempDir()
	repo := mk(t, home, ".skills")
	mk(t, repo, "a")
	_ = os.Symlink(repo, filepath.Join(home, "tgt"))
	_, err := targets.NewCursor(repo).Observe(osfs.New(), domain.Target{ID: "t", Root: filepath.Join(home, "tgt")})
	if err == nil {
		t.Error("Observe accepted a target root that resolves to the repo root (later sync would write into ~/.skills)")
	}
}

func TestAuditorLinkStates(t *testing.T) {
	home := t.TempDir()
	repo := mk(t, home, ".skills")
	mk(t, repo, "a")
	root := mk(t, home, "t")
	evil := mk(t, home, "evil", "a")
	_ = os.WriteFile(filepath.Join(evil, "SKILL.md"), []byte("x"), 0o644)
	_ = os.Symlink(filepath.Join(repo, "a"), filepath.Join(root, "a"))
	_ = os.Symlink(evil, filepath.Join(root, "b"))
	_ = os.Symlink(filepath.Join(root, "c"), filepath.Join(root, "c"))
	_ = os.Symlink(filepath.Join(home, "nope"), filepath.Join(root, "d"))
	_ = os.Symlink(filepath.Join(repo, "..", ".skills", "a"), filepath.Join(root, "e"))
	got := map[domain.SkillName]domain.SlotState{}
	obs, err := targets.NewCodex(repo).Observe(osfs.New(), domain.Target{ID: "t", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range obs {
		got[o.Name] = o.State
	}
	want := map[domain.SkillName]domain.SlotState{"a": domain.SlotOurSymlink, "b": domain.SlotForeignSymlink, "c": domain.SlotBroken, "d": domain.SlotBroken, "e": domain.SlotForeignSymlink}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %s want %s", k, got[k], v)
		}
	}
}
