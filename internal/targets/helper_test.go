package targets_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/fake"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/osfs"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/repo"
)

type backend struct {
	name string
	open func(t *testing.T) (ports.FS, string)
}

func backends() []backend {
	return []backend{
		{name: "fake", open: func(t *testing.T) (ports.FS, string) { return fake.New(), "/Users/ada" }},
		{name: "os", open: func(t *testing.T) (ports.FS, string) { return osfs.New(), t.TempDir() }},
	}
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "targets", "demo", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func publish(t *testing.T, fsys ports.FS, root, name string, files map[string][]byte) {
	t.Helper()
	clk := fake.NewClock(time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC))
	store, err := repo.New(fsys, clk, root)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := domain.ParseSkillName(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), domain.Skill{Name: parsed, Description: "Fixture skill"}, files); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, fsys ports.FS, path string) {
	t.Helper()
	if err := fsys.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, fsys ports.FS, path string, body []byte, mode os.FileMode) {
	t.Helper()
	if err := fsys.WriteFileAtomic(path, body, mode); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, fsys ports.FS, old, new string) {
	t.Helper()
	if err := fsys.Symlink(old, new); err != nil {
		t.Fatal(err)
	}
}

func callWithin(t *testing.T, d time.Duration, fn func() error) {
	t.Helper()
	errc := make(chan error, 1)
	go func() {
		errc <- fn()
	}()
	select {
	case err := <-errc:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(d):
		t.Fatal("timed out")
	}
}
