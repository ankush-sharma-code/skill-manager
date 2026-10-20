package targets_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
	"github.com/ankush-sharma-code/skill-manager/internal/targets"
)

func TestNoSkillRegisteredTwiceAcrossOverlappingRoots(t *testing.T) {
	t.Parallel()
	for _, be := range backends() {
		be := be
		t.Run(be.name, func(t *testing.T) {
			t.Parallel()
			fsys, home := be.open(t)
			claudeSkills := filepath.Join(home, ".claude", "skills")
			mustMkdir(t, fsys, claudeSkills)
			mustMkdir(t, fsys, filepath.Join(home, ".cursor"))
			mustSymlink(t, fsys, claudeSkills, filepath.Join(home, ".cursor", "skills"))
			mustMkdir(t, fsys, filepath.Join(home, ".codex"))
			mustSymlink(t, fsys, claudeSkills, filepath.Join(home, ".codex", "skills"))
			mustMkdir(t, fsys, filepath.Join(home, ".agents"))
			mustSymlink(t, fsys, claudeSkills, filepath.Join(home, ".agents", "skills"))

			got, _, err := targets.Propose(fsys, home)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 {
				t.Fatalf("propose = %+v", got)
			}
			for _, tg := range got {
				if tg.Kind == domain.TargetCursorUser {
					t.Fatalf("cursor overlaps %s", claudeSkills)
				}
				if strings.Contains(tg.Root, filepath.Join(".agents", "skills")) {
					t.Fatalf("agents root registered: %+v", tg)
				}
			}
			if followPath(t, fsys, got[0].Root) != claudeSkills {
				t.Fatalf("registered root = %s", got[0].Root)
			}
			paths, err := targets.SlotPaths(fsys, got, "demo")
			if err != nil {
				t.Fatal(err)
			}
			if len(paths) != 1 {
				t.Fatalf("skill demo registered %d times: %v", len(paths), paths)
			}
			if followPath(t, fsys, filepath.Dir(paths[0])) != claudeSkills {
				t.Fatalf("slot = %s", paths[0])
			}
		})
	}
}

func TestAgentsSkillsIsOptInOnly(t *testing.T) {
	t.Parallel()
	fsys, home := backends()[0].open(t)
	mustMkdir(t, fsys, filepath.Join(home, ".codex", "skills"))
	mustMkdir(t, fsys, filepath.Join(home, ".agents", "skills"))
	mustMkdir(t, fsys, filepath.Join(home, ".claude"))
	got, _, err := targets.Propose(fsys, home)
	if err != nil {
		t.Fatal(err)
	}
	var codex domain.Target
	for _, tg := range got {
		if strings.Contains(tg.Root, filepath.Join(".agents", "skills")) {
			t.Fatalf("default target uses agents skills: %+v", tg)
		}
		if tg.Kind == domain.TargetCodexUser {
			codex = tg
		}
	}
	if codex.Mode != domain.SyncSymlink || codex.Root != filepath.Join(home, ".codex", "skills") {
		t.Fatalf("codex = %+v", codex)
	}
	opt, err := targets.OptIn("agents", filepath.Join(home, ".agents", "skills"), domain.SyncCopy)
	if err != nil {
		t.Fatal(err)
	}
	if opt.Kind != domain.TargetCustom || !opt.Enabled || opt.Root != filepath.Join(home, ".agents", "skills") {
		t.Fatalf("opt-in = %+v", opt)
	}
	again, _, err := targets.Propose(fsys, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != len(got) {
		t.Fatal("opt-in changed default registration")
	}
}

func TestProposeAndOptInRejectBadPaths(t *testing.T) {
	t.Parallel()
	fsys, _ := backends()[0].open(t)
	if _, _, err := targets.Propose(fsys, "relative"); err == nil {
		t.Fatal("relative home was accepted")
	}
	if _, err := targets.OptIn("agents", "relative/skills", domain.SyncCopy); err == nil {
		t.Fatal("relative opt-in root was accepted")
	}
	if _, err := targets.OptIn("../x", "/tmp/skills", domain.SyncCopy); err == nil {
		t.Fatal("escaping opt-in id was accepted")
	}
	if _, err := targets.OptIn("agents", "/tmp/skills", domain.SyncMode("link")); err == nil {
		t.Fatal("bad mode was accepted")
	}
	if _, err := targets.ClaudeProject("relative"); err == nil {
		t.Fatal("relative project was accepted")
	}
	if _, err := targets.SlotPaths(fsys, nil, "BAD"); err == nil {
		t.Fatal("bad skill name was accepted")
	}
}

func followPath(t *testing.T, fsys ports.FS, path string) string {
	t.Helper()
	current := filepath.Clean(path)
	seen := map[string]struct{}{}
	for hop := 0; hop < 8; hop++ {
		if _, ok := seen[current]; ok {
			t.Fatalf("loop at %s", current)
		}
		seen[current] = struct{}{}
		info, err := fsys.Lstat(current)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			return current
		}
		next, err := fsys.Readlink(current)
		if err != nil {
			t.Fatal(err)
		}
		if !filepath.IsAbs(next) {
			next = filepath.Join(filepath.Dir(current), next)
		}
		current = filepath.Clean(next)
	}
	t.Fatal("too many links")
	return ""
}

func TestSlotPathsCollapseDistinctRoots(t *testing.T) {
	t.Parallel()
	fsys, home := backends()[0].open(t)
	shared := filepath.Join(home, "shared")
	mustMkdir(t, fsys, shared)
	link := filepath.Join(home, "link")
	mustMkdir(t, fsys, home)
	mustSymlink(t, fsys, shared, link)
	list := []domain.Target{
		{ID: "a", Root: shared},
		{ID: "b", Root: link},
	}
	paths, err := targets.SlotPaths(fsys, list, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("paths = %v", paths)
	}
	if _, err := targets.SlotPaths(fsys, list, "../demo"); !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("escape err = %v", err)
	}
}
