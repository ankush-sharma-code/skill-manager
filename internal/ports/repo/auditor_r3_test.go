package repo_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/ports/osfs"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/repo"
)

func TestAuditorR3RejectedSymlink(t *testing.T) {
	root := filepath.Join(t.TempDir(), "s")
	outside := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".stage"), 0o700)
	_ = os.Symlink(outside, filepath.Join(root, ".stage", "rejected"))
	_ = os.Symlink(outside, filepath.Join(root, ".stage", "a-0123456789abcdef.old"))
	r, _ := repo.New(osfs.New(), fixedClock{time.Unix(1, 0)}, root)
	err := r.Recover(context.Background())
	ents, _ := os.ReadDir(outside)
	if len(ents) != 0 {
		t.Errorf("quarantine wrote through symlinked .stage/rejected: %v", ents[0].Name())
	}
	if !errors.Is(err, repo.ErrSymlink) {
		t.Errorf("want ErrSymlink, got %v", err)
	}
}

func TestAuditorR3RejectedCollisions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "s")
	outside := t.TempDir()
	r, _ := repo.New(osfs.New(), fixedClock{time.Unix(1, 0)}, root)
	var lastErr error
	for i := 0; i < 12; i++ {
		_ = os.MkdirAll(filepath.Join(root, ".stage"), 0o700)
		_ = os.Symlink(outside, filepath.Join(root, ".stage", "a-0123456789abcdef.old"))
		lastErr = r.Recover(context.Background())
		if lastErr != nil {
			t.Logf("Recover #%d: %v", i+1, lastErr)
			break
		}
	}
	ents, _ := os.ReadDir(filepath.Join(root, ".stage", "rejected"))
	t.Logf("rejected entries: %d", len(ents))
	if lastErr != nil || len(ents) < 12 {
		t.Errorf("quarantine collisions: entries=%d err=%v (evidence overwritten or Recover wedged)", len(ents), lastErr)
	}
	_ = fmt.Sprint()
}

func TestAuditorR3ModesAndIdempotence(t *testing.T) {
	root := filepath.Join(t.TempDir(), "s")
	r, _ := repo.New(osfs.New(), fixedClock{time.Unix(1, 0)}, root)
	_ = os.MkdirAll(filepath.Join(root, ".stage"), 0o700)
	_ = os.WriteFile(filepath.Join(root, ".stage", "junk"), []byte("x"), 0o644)
	for i := 0; i < 3; i++ {
		if err := r.Recover(context.Background()); err != nil {
			t.Fatalf("Recover #%d: %v", i, err)
		}
	}
	ents, _ := os.ReadDir(filepath.Join(root, ".stage", "rejected"))
	if len(ents) != 1 {
		t.Errorf("rejected re-quarantined itself or junk duplicated: %d", len(ents))
	}
	if err := put(t, r, "a", nil, nil); err != nil {
		t.Fatal(err)
	}
	_ = r.Delete(context.Background(), "a")
	for _, d := range []string{"", ".stage", ".stage/rejected", ".trash"} {
		fi, err := os.Lstat(filepath.Join(root, d))
		if err == nil {
			t.Logf("mode %-16s %v", d, fi.Mode().Perm())
		}
	}
}
