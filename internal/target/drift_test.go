package target_test

import (
	"path/filepath"
	"testing"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/target"
)

func TestCopyDrift(t *testing.T) {
	t.Parallel()
	repoDigest := domain.Digest{1}
	ledger := domain.Digest{2}
	third := domain.Digest{3}
	if got := target.CopyDrift(repoDigest, repoDigest, &ledger); got != target.DriftNone {
		t.Fatalf("match = %s", got)
	}
	if got := target.CopyDrift(ledger, repoDigest, &ledger); got != target.DriftStale {
		t.Fatalf("stale = %s", got)
	}
	if got := target.CopyDrift(third, repoDigest, &ledger); got != target.DriftConflict {
		t.Fatalf("conflict = %s", got)
	}
	if got := target.CopyDrift(third, repoDigest, nil); got != target.DriftForeign {
		t.Fatalf("foreign = %s", got)
	}
}

func TestCursorObservedDigestDrifts(t *testing.T) {
	t.Parallel()
	fsys, home := backends()[0].open(t)
	root := filepath.Join(home, ".cursor", "skills")
	demo := filepath.Join(root, "demo")
	mustMkdir(t, fsys, demo)
	mustWrite(t, fsys, filepath.Join(demo, "SKILL.md"), fixture(t, "SKILL.md"), 0o644)
	mustWrite(t, fsys, filepath.Join(demo, "notes.txt"), fixture(t, "notes.txt"), 0o644)
	ad := target.NewCursor(filepath.Join(home, ".skills"))
	got, err := ad.Observe(fsys, domain.Target{ID: "cursor-user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].State != domain.SlotDir {
		t.Fatalf("observe = %+v", got)
	}
	first := got[0].Digest
	want := treeDigest(t, fsys, demo, "SKILL.md", "notes.txt")
	if first != want || first == (domain.Digest{}) {
		t.Fatalf("digest = %x, want %x", first, want)
	}
	if target.CopyDrift(first, first, nil) != target.DriftNone {
		t.Fatal("matching copy was drift")
	}
	mustWrite(t, fsys, filepath.Join(demo, "notes.txt"), []byte("edited\n"), 0o644)
	next, err := ad.Observe(fsys, domain.Target{ID: "cursor-user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	second := next[0].Digest
	if second == first {
		t.Fatal("digest ignored a content change")
	}
	if target.CopyDrift(first, second, &first) != target.DriftStale {
		t.Fatal("ledger match was not stale")
	}
	edited := domain.Digest{9}
	if target.CopyDrift(edited, second, &first) != target.DriftConflict {
		t.Fatal("diverged digests were not a conflict")
	}
}
