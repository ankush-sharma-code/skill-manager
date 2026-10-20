package ports_test

import (
	"path/filepath"
	"testing"

	"github.com/ankush-sharma-code/skill-manager/internal/ports"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/fake"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/osfs"
)

func TestAuditorParity(t *testing.T) {
	type res struct{ emptyDir, fileOntoDir, mkdirThroughLink bool }
	run := func(f ports.FS, root string) res {
		var r res
		_ = f.MkdirAll(filepath.Join(root, "a"), 0o755)
		_ = f.MkdirAll(filepath.Join(root, "b"), 0o755)
		r.emptyDir = f.Rename(filepath.Join(root, "a"), filepath.Join(root, "b")) == nil
		_ = f.WriteFileAtomic(filepath.Join(root, "f"), []byte("x"), 0o644)
		_ = f.MkdirAll(filepath.Join(root, "d"), 0o755)
		r.fileOntoDir = f.Rename(filepath.Join(root, "f"), filepath.Join(root, "d")) == nil
		_ = f.MkdirAll(filepath.Join(root, "real"), 0o755)
		_ = f.Symlink(filepath.Join(root, "real"), filepath.Join(root, "ln"))
		r.mkdirThroughLink = f.MkdirAll(filepath.Join(root, "ln", "x"), 0o755) == nil
		return r
	}
	o := run(osfs.New(), t.TempDir())
	k := run(fake.New(), "/p")
	if o != k {
		t.Errorf("parity: os=%+v fake=%+v", o, k)
	}
}
