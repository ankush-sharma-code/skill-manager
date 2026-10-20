package targets_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/fake"
	"github.com/ankush-sharma-code/skill-manager/internal/ports/osfs"
	"github.com/ankush-sharma-code/skill-manager/internal/targets"
)

func TestAlsoReadsDocumentedEmpty(t *testing.T) {
	t.Parallel()
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "targets.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(doc)
	if !strings.Contains(text, "AlsoReads") {
		t.Fatal("docs/targets.md does not document AlsoReads")
	}
	if !strings.Contains(text, "The list is empty") {
		t.Fatal("AlsoReads must be documented as empty")
	}
	if len(targets.NewCursor("/r").AlsoReads()) != 0 {
		t.Fatal("AlsoReads is not empty")
	}
}

func TestDedupeReadsUsesOnlyTheCapability(t *testing.T) {
	t.Parallel()
	claude := domain.Target{ID: "claude-user", Root: "/home/u/.claude/skills"}
	cursor := domain.Target{ID: "cursor-user", Root: "/home/u/.cursor/skills"}
	both := []domain.Target{claude, cursor}
	dropped := targets.DedupeReads(both, map[targets.TargetID][]targets.TargetID{
		"cursor-user": {"claude-user"},
	})
	if len(dropped) != 1 || dropped[0].ID != "claude-user" {
		t.Fatalf("non-empty AlsoReads did not drop cursor: %+v", dropped)
	}
	kept := targets.DedupeReads(both, map[targets.TargetID][]targets.TargetID{
		"cursor-user": targets.NewCursor("/r").AlsoReads(),
	})
	if len(kept) != 2 {
		t.Fatalf("empty AlsoReads dropped a target: %+v", kept)
	}
}

func TestParentSymlinkAliasIsInfo(t *testing.T) {
	t.Parallel()
	fsys := fake.New()
	home := "/Users/ada"
	cursorSkills := filepath.Join(home, ".cursor", "skills")
	mustMkdir(t, fsys, cursorSkills)
	mustSymlink(t, fsys, filepath.Join(home, ".cursor"), filepath.Join(home, ".claude"))
	got, problems := targets.DedupeRoots(fsys, []domain.Target{
		{ID: "claude-user", Root: filepath.Join(home, ".claude", "skills")},
		{ID: "cursor-user", Root: cursorSkills},
	})
	if len(got) != 1 || got[0].ID != "claude-user" {
		t.Fatalf("kept = %+v", got)
	}
	if !problemHas(problems, targets.LevelInfo, "alias") || !problemTextHas(problems, "cursor-user") {
		t.Fatalf("problems = %+v", problems)
	}
}

func TestCaseFoldConflictBlocksBothNames(t *testing.T) {
	t.Parallel()
	fsys := fake.NewCaseFolding()
	if err := fsys.MkdirAll("/t/Foo", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFileAtomic("/t/Foo/SKILL.md", []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := targets.Inspect(fsys, domain.Target{ID: "t", Root: "/t"}, "/r")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Observed) == 0 {
		t.Fatal("Observe dropped Foo")
	}
	blocked := targets.BlockedNames(res.Problems)
	if !containsAll(blocked, "Foo", "foo") {
		t.Fatalf("blocked = %v problems = %+v", blocked, res.Problems)
	}
	if !problemHas(res.Problems, targets.LevelConflict, "Foo") {
		t.Fatalf("problems = %+v", res.Problems)
	}
}

func TestCaseFoldPairBlocksBoth(t *testing.T) {
	t.Parallel()
	fsys := fake.New()
	for _, name := range []string{"Foo", "foo"} {
		dir := filepath.Join("/t", name)
		if err := fsys.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := fsys.WriteFileAtomic(filepath.Join(dir, "SKILL.md"), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := targets.Inspect(fsys, domain.Target{ID: "t", Root: "/t"}, "/r")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, ob := range res.Observed {
		got[string(ob.Name)] = true
	}
	if !got["Foo"] || !got["foo"] {
		t.Fatalf("observed = %+v", res.Observed)
	}
	if !containsAll(targets.BlockedNames(res.Problems), "Foo", "foo") {
		t.Fatalf("problems = %+v", res.Problems)
	}
}

func TestIrregularFilesAreProblems(t *testing.T) {
	t.Parallel()
	base := fake.New()
	root := "/home/u/.cursor/skills"
	skill := filepath.Join(root, "demo")
	if err := base.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SKILL.md", "pipe", "sock", "dev"} {
		if err := base.WriteFileAtomic(filepath.Join(skill, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wrap := &modeFS{
		FS: base,
		modes: map[string]fs.FileMode{
			filepath.Join(skill, "pipe"): fs.ModeNamedPipe | 0o644,
			filepath.Join(skill, "sock"): fs.ModeSocket | 0o644,
			filepath.Join(skill, "dev"):  fs.ModeDevice | fs.ModeCharDevice | 0o644,
		},
	}
	res, err := targets.Inspect(wrap, domain.Target{ID: "t", Root: root}, "/home/u/.skills")
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{"fifo", "socket", "device"} {
		if !problemTextHas(res.Problems, word) {
			t.Fatalf("problems = %+v, missing %s", res.Problems, word)
		}
	}
	for _, path := range wrap.reads {
		baseName := filepath.Base(path)
		if baseName == "pipe" || baseName == "sock" || baseName == "dev" {
			t.Fatalf("opened non-regular file %s", path)
		}
	}
	if len(res.Observed) != 1 || res.Observed[0].Name != "demo" {
		t.Fatalf("observed = %+v", res.Observed)
	}
}

func TestValidateRootRejectsDangerous(t *testing.T) {
	t.Parallel()
	home := "/home/u"
	repo := "/home/u/.skills"
	cases := []struct {
		resolved string
		custom   bool
		ok       bool
	}{
		{resolved: "/", custom: false},
		{resolved: "/", custom: true},
		{resolved: "/home/u", custom: false},
		{resolved: "/home/u", custom: true},
		{resolved: "/home/u/.skills", custom: false},
		{resolved: "/home/u/.skills", custom: true},
		{resolved: "/home/u/.skills/sub", custom: true},
		{resolved: "/opt/tools", custom: false},
		{resolved: "/opt/tools", custom: true, ok: true},
		{resolved: "/home/u/.cursor/skills", custom: false, ok: true},
		{resolved: "/home/u/.agents/skills", custom: true, ok: true},
	}
	for _, tc := range cases {
		err := targets.ValidateRoot(tc.resolved, home, repo, tc.custom)
		if tc.ok && err != nil {
			t.Errorf("ValidateRoot(%q, custom=%v) = %v", tc.resolved, tc.custom, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("ValidateRoot(%q, custom=%v) accepted a dangerous root", tc.resolved, tc.custom)
		}
	}
}

func TestObserveRejectsDangerousRoots(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, ".skills")
	if err := os.MkdirAll(filepath.Join(repo, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	fsys := osfs.New()
	cursor := targets.NewCursor(repo)
	if _, err := cursor.Observe(fsys, domain.Target{ID: "t", Kind: domain.TargetCursorUser, Root: "/"}); err == nil {
		t.Error("Observe accepted /")
	}
	if _, err := cursor.Observe(fsys, domain.Target{ID: "t", Kind: domain.TargetCursorUser, Root: home}); err == nil {
		t.Error("Observe accepted $HOME")
	}
	if _, err := targets.NewCustom(repo).Observe(fsys, domain.Target{ID: "pi", Kind: domain.TargetCustom, Root: home}); err == nil {
		t.Error("custom Observe accepted $HOME")
	}
	inside := filepath.Join(repo, "nested")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := cursor.Observe(fsys, domain.Target{ID: "t", Kind: domain.TargetCursorUser, Root: inside}); err == nil {
		t.Error("Observe accepted a directory inside the skills repo")
	}
	other := t.TempDir()
	if err := os.MkdirAll(filepath.Join(other, "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "demo", "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := cursor.Observe(fsys, domain.Target{ID: "t", Kind: domain.TargetCursorUser, Root: other}); err == nil {
		t.Error("Observe accepted a root outside $HOME")
	}
	got, err := targets.NewCustom(repo).Observe(fsys, domain.Target{ID: "pi", Kind: domain.TargetCustom, Root: other})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "demo" {
		t.Fatalf("custom outside home = %+v", got)
	}
}

func TestOptInRejectsSymlinkMode(t *testing.T) {
	t.Parallel()
	if _, err := targets.OptIn("pi", "/opt/pi", domain.SyncSymlink); err == nil {
		t.Fatal("OptIn accepted symlink mode for an unverified custom target")
	}
}

func TestObserveDepthLimit(t *testing.T) {
	t.Parallel()
	fsys, home := backends()[0].open(t)
	dir := filepath.Join(home, ".cursor", "skills", "demo")
	for i := 0; i < 33; i++ {
		dir = filepath.Join(dir, "d")
	}
	mustMkdir(t, fsys, dir)
	mustWrite(t, fsys, filepath.Join(dir, "SKILL.md"), []byte("x"), 0o644)
	_, err := targets.NewCursor(filepath.Join(home, ".skills")).Observe(fsys, domain.Target{
		ID:   "cursor-user",
		Kind: domain.TargetCursorUser,
		Root: filepath.Join(home, ".cursor", "skills"),
	})
	if err == nil || !strings.Contains(err.Error(), "too deep") {
		t.Fatalf("err = %v", err)
	}
}

func TestObserveFakeAndOSAgree(t *testing.T) {
	var got [][]domain.SlotState
	var names [][]string
	for _, be := range backends() {
		be := be
		fsys, home := be.open(t)
		repoRoot := filepath.Join(home, ".skills")
		root := buildSlotTree(t, fsys, home, repoRoot)
		obs, err := targets.NewCursor(repoRoot).Observe(fsys, domain.Target{
			ID:   "cursor-user",
			Kind: domain.TargetCursorUser,
			Root: root,
		})
		if err != nil {
			t.Fatal(err)
		}
		states := make([]domain.SlotState, len(obs))
		ids := make([]string, len(obs))
		for i, ob := range obs {
			states[i] = ob.State
			ids[i] = string(ob.Name)
		}
		got = append(got, states)
		names = append(names, ids)
	}
	if strings.Join(names[0], ",") != strings.Join(names[1], ",") {
		t.Fatalf("names fake=%v os=%v", names[0], names[1])
	}
	if len(got[0]) != len(got[1]) {
		t.Fatalf("states fake=%v os=%v", got[0], got[1])
	}
	for i := range got[0] {
		if got[0][i] != got[1][i] {
			t.Fatalf("state %s fake=%s os=%s", names[0][i], got[0][i], got[1][i])
		}
	}
}

func problemHas(problems []targets.Problem, level targets.Level, phrase string) bool {
	for _, problem := range problems {
		if problem.Level == level && strings.Contains(problem.Message, phrase) {
			return true
		}
	}
	return false
}

func problemTextHas(problems []targets.Problem, phrase string) bool {
	for _, problem := range problems {
		if strings.Contains(problem.Message, phrase) || strings.Contains(problem.Path, phrase) {
			return true
		}
		for _, name := range problem.Names {
			if name == phrase {
				return true
			}
		}
	}
	return false
}

func containsAll(got []string, want ...string) bool {
	have := map[string]bool{}
	for _, name := range got {
		have[name] = true
	}
	for _, name := range want {
		if !have[name] {
			return false
		}
	}
	return true
}

type modeFS struct {
	ports.FS
	modes map[string]fs.FileMode
	reads []string
}

func (m *modeFS) Lstat(path string) (fs.FileInfo, error) {
	info, err := m.FS.Lstat(path)
	if err != nil {
		return nil, err
	}
	mode, ok := m.modes[path]
	if !ok {
		return info, nil
	}
	return staticInfo{name: info.Name(), mode: mode}, nil
}

func (m *modeFS) ReadFile(path string) ([]byte, error) {
	m.reads = append(m.reads, path)
	return m.FS.ReadFile(path)
}

type staticInfo struct {
	name string
	mode fs.FileMode
}

func (s staticInfo) Name() string       { return s.name }
func (s staticInfo) Size() int64        { return 0 }
func (s staticInfo) Mode() fs.FileMode  { return s.mode }
func (s staticInfo) ModTime() time.Time { return time.Time{} }
func (s staticInfo) IsDir() bool        { return s.mode.IsDir() }
func (s staticInfo) Sys() any           { return nil }
