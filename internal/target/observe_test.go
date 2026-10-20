package target_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
	"github.com/ankush-sharma-code/skill-manager/internal/skillfile"
	"github.com/ankush-sharma-code/skill-manager/internal/target"
)

func TestObserveClassifiesEverySlotState(t *testing.T) {
	t.Parallel()
	for _, be := range backends() {
		be := be
		t.Run(be.name, func(t *testing.T) {
			t.Parallel()
			fsys, home := be.open(t)
			repoRoot := filepath.Join(home, ".skills")
			root := buildSlotTree(t, fsys, home, repoRoot)
			ad := target.NewCursor(repoRoot)
			tgt := domain.Target{ID: "cursor-user", Kind: domain.TargetCursorUser, Root: root, Mode: domain.SyncCopy, Enabled: true}

			var got []domain.Observed
			callWithin(t, 2*time.Second, func() error {
				var err error
				got, err = ad.Observe(fsys, tgt)
				return err
			})
			byName := indexObserved(t, got)

			alpha := byName["alpha"]
			if alpha.State != domain.SlotOurSymlink || alpha.LinkTo != filepath.Join(repoRoot, "alpha") || alpha.Digest != (domain.Digest{}) {
				t.Fatalf("alpha = %+v", alpha)
			}
			rel := byName["rel"]
			relTo, err := filepath.Rel(root, filepath.Join(repoRoot, "rel"))
			if err != nil {
				t.Fatal(err)
			}
			if rel.State != domain.SlotOurSymlink || rel.LinkTo != relTo {
				t.Fatalf("rel = %+v, want link %s", rel, relTo)
			}
			foreign := byName["foreign"]
			if foreign.State != domain.SlotForeignSymlink || foreign.LinkTo != filepath.Join(home, "elsewhere", "beta") {
				t.Fatalf("foreign = %+v", foreign)
			}
			demo := byName["demo"]
			if demo.State != domain.SlotDir || demo.LinkTo != "" {
				t.Fatalf("demo = %+v", demo)
			}
			wantDigest := treeDigest(t, fsys, filepath.Join(root, "demo"), "SKILL.md", "notes.txt")
			if demo.Digest != wantDigest {
				t.Fatalf("demo digest = %x, want %x", demo.Digest, wantDigest)
			}
			plain := byName["plain"]
			if plain.State != domain.SlotFile || plain.Digest != (domain.Digest{}) || plain.LinkTo != "" {
				t.Fatalf("plain = %+v", plain)
			}
			if byName["broken"].State != domain.SlotBroken || byName["broken"].LinkTo != filepath.Join(home, "missing", "gone") {
				t.Fatalf("broken = %+v", byName["broken"])
			}
			if byName["loopa"].State != domain.SlotBroken || byName["loopb"].State != domain.SlotBroken {
				t.Fatalf("loops = %s %s", byName["loopa"].State, byName["loopb"].State)
			}
			if _, ok := byName[".system"]; ok {
				t.Fatal("hidden .system was registered")
			}
			if _, ok := byName["README"]; ok {
				t.Fatal("README is not a skill name")
			}

			miss, err := ad.Classify(fsys, tgt, "absent")
			if err != nil {
				t.Fatal(err)
			}
			if miss.State != domain.SlotMissing || miss.Name != "absent" || miss.Target != tgt.ID {
				t.Fatalf("absent = %+v", miss)
			}

			seen := map[domain.SlotState]bool{domain.SlotMissing: true}
			for _, ob := range got {
				seen[ob.State] = true
				if ob.Target != tgt.ID {
					t.Fatalf("target id = %s", ob.Target)
				}
			}
			for _, st := range []domain.SlotState{
				domain.SlotMissing, domain.SlotOurSymlink, domain.SlotForeignSymlink,
				domain.SlotDir, domain.SlotFile, domain.SlotBroken,
			} {
				if !seen[st] {
					t.Errorf("slot state %s was not classified", st)
				}
			}
		})
	}
}

func TestObserveSymlinkLoopDoesNotHang(t *testing.T) {
	t.Parallel()
	for _, be := range backends() {
		be := be
		t.Run(be.name, func(t *testing.T) {
			t.Parallel()
			fsys, home := be.open(t)
			root := filepath.Join(home, ".cursor", "skills")
			mustMkdir(t, fsys, root)
			mustSymlink(t, fsys, filepath.Join(root, "loopb"), filepath.Join(root, "loopa"))
			mustSymlink(t, fsys, filepath.Join(root, "loopa"), filepath.Join(root, "loopb"))
			ad := target.NewCursor(filepath.Join(home, ".skills"))
			tgt := domain.Target{ID: "cursor-user", Root: root}
			var got []domain.Observed
			callWithin(t, 2*time.Second, func() error {
				var err error
				got, err = ad.Observe(fsys, tgt)
				return err
			})
			if len(got) != 2 || got[0].State != domain.SlotBroken || got[1].State != domain.SlotBroken {
				t.Fatalf("loop slots = %+v", got)
			}
		})
	}
}

func TestObserveRootSymlinkLoop(t *testing.T) {
	t.Parallel()
	for _, be := range backends() {
		be := be
		t.Run(be.name, func(t *testing.T) {
			t.Parallel()
			fsys, home := be.open(t)
			mustMkdir(t, fsys, home)
			root := filepath.Join(home, "looproot")
			mustSymlink(t, fsys, root, root)
			ad := target.NewClaude(filepath.Join(home, ".skills"))
			var err error
			callWithin(t, 2*time.Second, func() error {
				_, err = ad.Observe(fsys, domain.Target{ID: "claude-user", Root: root})
				return nil
			})
			if !errors.Is(err, target.ErrLoop) {
				t.Fatalf("root loop err = %v", err)
			}
		})
	}
}

func TestObserveOutsideRootIsNotFollowed(t *testing.T) {
	t.Parallel()
	for _, be := range backends() {
		be := be
		t.Run(be.name, func(t *testing.T) {
			t.Parallel()
			fsys, home := be.open(t)
			root := filepath.Join(home, ".cursor", "skills")
			outside := filepath.Join(home, "outside")
			mustMkdir(t, fsys, filepath.Join(outside, "other"))
			mustWrite(t, fsys, filepath.Join(outside, "secret.txt"), []byte("SECRET"), 0o644)
			mustWrite(t, fsys, filepath.Join(outside, "other", "SKILL.md"), []byte("other"), 0o644)

			demo := filepath.Join(root, "demo")
			mustMkdir(t, fsys, filepath.Join(demo, "extra"))
			mustWrite(t, fsys, filepath.Join(demo, "SKILL.md"), fixture(t, "SKILL.md"), 0o644)
			mustWrite(t, fsys, filepath.Join(demo, "extra", "a.txt"), []byte("nested"), 0o644)
			mustSymlink(t, fsys, filepath.Join(outside, "secret.txt"), filepath.Join(demo, "leak"))
			mustSymlink(t, fsys, outside, filepath.Join(root, "foreign"))

			guard := boundaryFS{FS: fsys, root: root}
			ad := target.NewCursor(filepath.Join(home, ".skills"))
			got, err := ad.Observe(guard, domain.Target{ID: "cursor-user", Root: root})
			if err != nil {
				t.Fatal(err)
			}
			byName := indexObserved(t, got)
			if _, ok := byName["other"]; ok {
				t.Fatal("skill outside the root was registered")
			}
			if byName["foreign"].State != domain.SlotForeignSymlink {
				t.Fatalf("foreign = %+v", byName["foreign"])
			}
			want := treeDigest(t, fsys, demo, "SKILL.md", "extra/a.txt")
			leakInfo, err := fsys.Lstat(filepath.Join(demo, "leak"))
			if err != nil {
				t.Fatal(err)
			}
			leakTo, err := fsys.Readlink(filepath.Join(demo, "leak"))
			if err != nil {
				t.Fatal(err)
			}
			extra, err := skillfile.TreeDigest([]skillfile.File{{
				RelPath: "leak",
				Mode:    leakInfo.Mode(),
				Content: []byte(leakTo),
			}})
			if err != nil {
				t.Fatal(err)
			}
			withLeak := combineDigest(t, fsys, demo, []string{"SKILL.md", "extra/a.txt"}, skillfile.File{
				RelPath: "leak",
				Mode:    leakInfo.Mode(),
				Content: []byte(leakTo),
			})
			if byName["demo"].Digest != withLeak {
				t.Fatalf("digest followed a symlink out of the root: got %x want %x (files only %x leak %x)", byName["demo"].Digest, withLeak, want, extra)
			}
			secret, err := fsys.ReadFile(filepath.Join(outside, "secret.txt"))
			if err != nil || string(secret) != "SECRET" {
				t.Fatalf("outside file = %q %v", secret, err)
			}
		})
	}
}

func TestObserveLongSymlinkChainIsBroken(t *testing.T) {
	t.Parallel()
	fsys, home := backends()[0].open(t)
	root := filepath.Join(home, ".codex", "skills")
	mustMkdir(t, fsys, root)
	const links = 16
	for i := 0; i < links-1; i++ {
		mustSymlink(t, fsys, filepath.Join(root, chainName(i+1)), filepath.Join(root, chainName(i)))
	}
	mustMkdir(t, fsys, filepath.Join(home, "real"))
	mustSymlink(t, fsys, filepath.Join(home, "real"), filepath.Join(root, chainName(links-1)))
	ad := target.NewCodex(filepath.Join(home, ".skills"))
	got, err := ad.Classify(fsys, domain.Target{ID: "codex-user", Root: root}, chainName(0))
	if err != nil {
		t.Fatal(err)
	}
	if got.State != domain.SlotBroken {
		t.Fatalf("long chain = %s", got.State)
	}
}

func TestClassifyRejectsOutsideName(t *testing.T) {
	t.Parallel()
	fsys, home := backends()[0].open(t)
	root := filepath.Join(home, ".claude", "skills")
	mustMkdir(t, fsys, root)
	ad := target.NewClaude(filepath.Join(home, ".skills"))
	tgt := domain.Target{ID: "claude-user", Root: root}
	for _, name := range []string{"../secret", `..\secret`, "a/b", "..", "."} {
		_, err := ad.Classify(fsys, tgt, name)
		if !errors.Is(err, target.ErrOutside) {
			t.Fatalf("Classify %q err = %v", name, err)
		}
	}
	_, err := ad.Classify(fsys, tgt, "Bad")
	if !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("Classify Bad err = %v", err)
	}
	miss, err := ad.Classify(fsys, domain.Target{ID: "claude-user", Root: filepath.Join(home, "missing-root")}, "alone")
	if err != nil {
		t.Fatal(err)
	}
	if miss.State != domain.SlotMissing {
		t.Fatalf("missing root slot = %+v", miss)
	}
}

func TestObserveMissingRootAndFileRoot(t *testing.T) {
	t.Parallel()
	fsys, home := backends()[0].open(t)
	ad := target.NewCustom(filepath.Join(home, ".skills"))
	got, err := ad.Observe(fsys, domain.Target{ID: "custom", Root: filepath.Join(home, "absent")})
	if err != nil || len(got) != 0 {
		t.Fatalf("missing root = %v %+v", err, got)
	}
	file := filepath.Join(home, "notdir")
	mustMkdir(t, fsys, home)
	mustWrite(t, fsys, file, []byte("x"), 0o644)
	_, err = ad.Observe(fsys, domain.Target{ID: "custom", Root: file})
	if !errors.Is(err, target.ErrNotDir) {
		t.Fatalf("file root err = %v", err)
	}
}

func TestObserveSymlinkedRoot(t *testing.T) {
	t.Parallel()
	fsys, home := backends()[0].open(t)
	real := filepath.Join(home, "real-skills")
	mustMkdir(t, fsys, filepath.Join(real, "demo"))
	mustWrite(t, fsys, filepath.Join(real, "demo", "SKILL.md"), fixture(t, "SKILL.md"), 0o644)
	link := filepath.Join(home, ".cursor", "skills")
	mustMkdir(t, fsys, filepath.Dir(link))
	mustSymlink(t, fsys, real, link)
	ad := target.NewCursor(filepath.Join(home, ".skills"))
	got, err := ad.Observe(fsys, domain.Target{ID: "cursor-user", Root: link})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "demo" || got[0].State != domain.SlotDir {
		t.Fatalf("symlinked root = %+v", got)
	}
}

func buildSlotTree(t *testing.T, fsys ports.FS, home, repoRoot string) string {
	t.Helper()
	publish(t, fsys, repoRoot, "alpha", map[string][]byte{"SKILL.md": []byte("---\nname: alpha\ndescription: Alpha skill\n---\n")})
	publish(t, fsys, repoRoot, "rel", map[string][]byte{"SKILL.md": []byte("---\nname: rel\ndescription: Relative skill\n---\n")})
	root := filepath.Join(home, ".cursor", "skills")
	mustMkdir(t, fsys, root)
	mustSymlink(t, fsys, filepath.Join(repoRoot, "alpha"), filepath.Join(root, "alpha"))
	relTo, err := filepath.Rel(root, filepath.Join(repoRoot, "rel"))
	if err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, fsys, relTo, filepath.Join(root, "rel"))
	elsewhere := filepath.Join(home, "elsewhere", "beta")
	mustMkdir(t, fsys, elsewhere)
	mustWrite(t, fsys, filepath.Join(elsewhere, "SKILL.md"), []byte("beta"), 0o644)
	mustSymlink(t, fsys, elsewhere, filepath.Join(root, "foreign"))
	demo := filepath.Join(root, "demo")
	mustMkdir(t, fsys, demo)
	mustWrite(t, fsys, filepath.Join(demo, "SKILL.md"), fixture(t, "SKILL.md"), 0o644)
	mustWrite(t, fsys, filepath.Join(demo, "notes.txt"), fixture(t, "notes.txt"), 0o644)
	mustWrite(t, fsys, filepath.Join(root, "plain"), []byte("x"), 0o644)
	mustSymlink(t, fsys, filepath.Join(home, "missing", "gone"), filepath.Join(root, "broken"))
	mustSymlink(t, fsys, filepath.Join(root, "loopb"), filepath.Join(root, "loopa"))
	mustSymlink(t, fsys, filepath.Join(root, "loopa"), filepath.Join(root, "loopb"))
	mustMkdir(t, fsys, filepath.Join(root, ".system"))
	mustWrite(t, fsys, filepath.Join(root, ".system", "SKILL.md"), []byte("system"), 0o644)
	mustWrite(t, fsys, filepath.Join(root, "README"), []byte("readme"), 0o644)
	return root
}

func indexObserved(t *testing.T, got []domain.Observed) map[string]domain.Observed {
	t.Helper()
	out := make(map[string]domain.Observed, len(got))
	for _, ob := range got {
		if _, ok := out[string(ob.Name)]; ok {
			t.Fatalf("skill %s registered twice", ob.Name)
		}
		out[string(ob.Name)] = ob
	}
	return out
}

func treeDigest(t *testing.T, fsys ports.FS, dir string, rels ...string) domain.Digest {
	t.Helper()
	files := make([]skillfile.File, 0, len(rels))
	for _, rel := range rels {
		files = append(files, fileEntry(t, fsys, dir, rel))
	}
	sum, err := skillfile.TreeDigest(files)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func combineDigest(t *testing.T, fsys ports.FS, dir string, rels []string, extra skillfile.File) domain.Digest {
	t.Helper()
	files := make([]skillfile.File, 0, len(rels)+1)
	for _, rel := range rels {
		files = append(files, fileEntry(t, fsys, dir, rel))
	}
	files = append(files, extra)
	sum, err := skillfile.TreeDigest(files)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func fileEntry(t *testing.T, fsys ports.FS, dir, rel string) skillfile.File {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	info, err := fsys.Lstat(abs)
	if err != nil {
		t.Fatal(err)
	}
	body, err := fsys.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	return skillfile.File{RelPath: rel, Mode: info.Mode(), Content: body}
}

func chainName(i int) string {
	return "c" + string(rune('a'+i))
}

type boundaryFS struct {
	ports.FS
	root string
}

func (b boundaryFS) ReadFile(p string) ([]byte, error) {
	info, err := b.FS.Lstat(p)
	if err != nil {
		return nil, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return nil, errors.New("read follows symlink: " + p)
	}
	if !pathInside(b.root, p) {
		return nil, errors.New("read outside root: " + p)
	}
	return b.FS.ReadFile(p)
}

func (b boundaryFS) ReadDir(p string) ([]fs.DirEntry, error) {
	info, err := b.FS.Lstat(p)
	if err != nil {
		return nil, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return nil, errors.New("readdir follows symlink: " + p)
	}
	if !pathInside(b.root, p) {
		return nil, errors.New("readdir outside root: " + p)
	}
	return b.FS.ReadDir(p)
}

func pathInside(root, full string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(full))
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
