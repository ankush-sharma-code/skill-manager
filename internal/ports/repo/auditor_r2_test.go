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

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/fake"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/osfs"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/repo"
)

var errCrash = errors.New("crash")

// crashFS lets `budget` mutating ops through, then fails every op (process death).
type crashFS struct {
	ports.FS
	budget int
	dead   bool
}

func (c *crashFS) tick() error {
	if c.dead {
		return errCrash
	}
	if c.budget == 0 {
		c.dead = true
		return errCrash
	}
	c.budget--
	return nil
}
func (c *crashFS) Lstat(p string) (fs.FileInfo, error) {
	if c.dead {
		return nil, errCrash
	}
	return c.FS.Lstat(p)
}
func (c *crashFS) ReadDir(p string) ([]fs.DirEntry, error) {
	if c.dead {
		return nil, errCrash
	}
	return c.FS.ReadDir(p)
}
func (c *crashFS) WriteFileAtomic(p string, b []byte, m fs.FileMode) error {
	if err := c.tick(); err != nil {
		return err
	}
	return c.FS.WriteFileAtomic(p, b, m)
}
func (c *crashFS) Rename(a, b string) error {
	if err := c.tick(); err != nil {
		return err
	}
	return c.FS.Rename(a, b)
}
func (c *crashFS) RemoveAll(p string) error {
	if err := c.tick(); err != nil {
		return err
	}
	return c.FS.RemoveAll(p)
}
func (c *crashFS) MkdirAll(p string, m fs.FileMode) error {
	if err := c.tick(); err != nil {
		return err
	}
	return c.FS.MkdirAll(p, m)
}

func aMD(desc string) []byte { return []byte("---\nname: a\ndescription: " + desc + "\n---\n") }

func TestAuditorCrashEveryStep(t *testing.T) {
	ctx := context.Background()
	for _, existing := range []bool{false, true} {
		for n := 0; n < 40; n++ {
			base := fake.New()
			clk := fixedClock{time.Unix(1, 0)}
			r0, _ := repo.New(base, clk, "/s")
			if existing {
				if err := r0.Put(ctx, domain.Skill{Name: "a"}, map[string][]byte{"SKILL.md": aMD("old"), "x/y": []byte("old")}); err != nil {
					t.Fatal(err)
				}
			}
			cf := &crashFS{FS: base, budget: n}
			rc, _ := repo.New(cf, clk, "/s")
			perr := rc.Put(ctx, domain.Skill{Name: "a"}, map[string][]byte{"SKILL.md": aMD("new"), "x/y": []byte("new")})
			r1, _ := repo.New(base, clk, "/s")
			if err := r1.Recover(ctx); err != nil {
				t.Errorf("existing=%v n=%d: Recover: %v", existing, n, err)
				continue
			}
			if err := r1.Recover(ctx); err != nil {
				t.Errorf("existing=%v n=%d: Recover not idempotent: %v", existing, n, err)
			}
			s, err := r1.Get(ctx, "a")
			switch {
			case err != nil && (existing || perr == nil):
				t.Errorf("existing=%v n=%d: skill missing after recover: %v (put err %v)", existing, n, err, perr)
			case err == nil:
				body, _ := base.ReadFile("/s/a/x/y")
				if s.Description != string(body) {
					t.Errorf("existing=%v n=%d: mixed tree desc=%q x/y=%q", existing, n, s.Description, body)
				}
				if perr == nil && s.Description != "new" {
					t.Errorf("existing=%v n=%d: Put ok but old content", existing, n)
				}
			}
			ents, _ := base.ReadDir("/s/.stage")
			for _, e := range ents {
				t.Errorf("existing=%v n=%d: leftover .stage/%s (put err %v)", existing, n, e.Name(), perr)
			}
			ents, _ = base.ReadDir("/s")
			for _, e := range ents {
				if strings.HasPrefix(e.Name(), ".tmp") {
					t.Errorf("existing=%v n=%d: leftover %s", existing, n, e.Name())
				}
			}
			if cf.budget > 0 && !cf.dead {
				break
			}
		}
	}
}

func TestAuditorRecoverStageSymlinks(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "s")
	outside := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".stage"), 0o755)
	_ = os.WriteFile(filepath.Join(outside, "SKILL.md"), aMD("evil"), 0o644)
	id := "a-0123456789abcdef"
	_ = os.Symlink(outside, filepath.Join(root, ".stage", id+".old"))
	r, _ := repo.New(osfs.New(), fixedClock{time.Unix(1, 0)}, root)
	_ = r.Recover(ctx)
	if fi, err := os.Lstat(filepath.Join(root, "a")); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
		t.Error("Recover promoted a symlinked .stage entry to a live skill (points outside root)")
	}
	// symlinked .stage itself
	root2 := filepath.Join(t.TempDir(), "s")
	_ = os.MkdirAll(root2, 0o755)
	_ = os.Symlink(outside, filepath.Join(root2, ".stage"))
	r2, _ := repo.New(osfs.New(), fixedClock{time.Unix(1, 0)}, root2)
	if err := r2.Recover(ctx); !errors.Is(err, repo.ErrSymlink) {
		t.Errorf("symlinked .stage: %v", err)
	}
}

func TestAuditorRecoverCorruptJournal(t *testing.T) {
	ctx := context.Background()
	f := fake.New()
	r, _ := repo.New(f, fixedClock{time.Unix(1, 0)}, "/s")
	_ = f.MkdirAll("/s/.stage/a-0123456789abcdef.new", 0o755) // partial: no SKILL.md, no live skill
	_ = f.WriteFileAtomic("/s/.stage/a-0123456789abcdef.old", []byte("x"), 0o644) // file, not dir
	_ = f.MkdirAll("/s/.stage/junk.new", 0o755)
	_ = f.MkdirAll("/s/.stage/A-0123456789abcdef.new", 0o755)
	err := r.Recover(ctx)
	if fi, e := f.Lstat("/s/a"); e == nil && !fi.IsDir() {
		t.Errorf("Recover promoted a regular file as skill dir (err=%v)", err)
	}
	if _, e := r.Get(ctx, "a"); e == nil {
		t.Error("partial .new promoted")
	}
	ents, _ := f.ReadDir("/s/.stage")
	var left []string
	for _, e := range ents {
		left = append(left, e.Name())
	}
	t.Logf("leftovers after Recover: %v (err=%v)", left, err)
}

func TestAuditorConcurrent(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "s")
	r, _ := repo.New(osfs.New(), fixedClock{time.Unix(1, 0)}, root)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			switch i % 4 {
			case 0, 1:
				_ = r.Put(ctx, domain.Skill{Name: "a"}, map[string][]byte{"SKILL.md": aMD("v")})
			case 2:
				_ = r.Delete(ctx, "a")
			case 3:
				_, _ = r.List(ctx)
				_ = r.Problems()
			}
		}(i)
	}
	wg.Wait()
	_ = r.Recover(ctx)
	ents, _ := os.ReadDir(filepath.Join(root, ".stage"))
	if len(ents) != 0 {
		t.Errorf("leftovers after concurrent ops + Recover: %d", len(ents))
	}
	if _, err := r.Get(ctx, "a"); err != nil && !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("bad state: %v", err)
	}
}
