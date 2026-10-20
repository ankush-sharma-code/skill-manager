package targets_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/targets"
)

func TestDetectCodexAgentsSuggestsCustomTarget(t *testing.T) {
	t.Parallel()
	for _, be := range backends() {
		be := be
		t.Run(be.name, func(t *testing.T) {
			t.Parallel()
			fsys, home := be.open(t)
			oldPath := filepath.Join(home, ".codex", "skills")
			newPath := filepath.Join(home, ".agents", "skills")
			mustMkdir(t, fsys, oldPath)
			mustMkdir(t, fsys, newPath)
			det := targets.NewCodex(filepath.Join(home, ".skills")).Detect(fsys, home)
			if !det.Installed || det.Moved || det.From != "" || det.To != "" || strings.Contains(det.Message, "moved") {
				t.Fatalf("detect = %+v", det)
			}
			if len(det.Roots) != 1 || det.Roots[0] != oldPath {
				t.Fatalf("roots = %v", det.Roots)
			}
			if !problemHas(det.Problems, targets.LevelInfo, newPath) || !problemTextHas(det.Problems, "custom") {
				t.Fatalf("problems = %+v", det.Problems)
			}
			got, problems, err := targets.Propose(fsys, home)
			if err != nil {
				t.Fatal(err)
			}
			for _, tg := range got {
				if strings.Contains(tg.Root, filepath.Join(".agents", "skills")) {
					t.Fatalf("agents root registered: %+v", tg)
				}
			}
			var codex domain.Target
			for _, tg := range got {
				if tg.Kind == domain.TargetCodexUser {
					codex = tg
				}
			}
			if codex.Root != oldPath || codex.Mode != domain.SyncSymlink {
				t.Fatalf("codex = %+v", codex)
			}
			if !problemHas(problems, targets.LevelInfo, newPath) || !problemTextHas(problems, "custom") {
				t.Fatalf("propose problems = %+v", problems)
			}
		})
	}
}

func TestDetectCodexLegacyAndNewOnly(t *testing.T) {
	t.Parallel()
	fsys, home := backends()[0].open(t)
	ad := targets.NewCodex(filepath.Join(home, ".skills"))

	mustMkdir(t, fsys, filepath.Join(home, ".codex", "skills"))
	legacy := ad.Detect(fsys, home)
	if !legacy.Installed || legacy.Moved || strings.Contains(legacy.Message, "moved") {
		t.Fatalf("legacy = %+v", legacy)
	}
	if len(legacy.Roots) != 1 || legacy.Roots[0] != filepath.Join(home, ".codex", "skills") {
		t.Fatalf("legacy roots = %v", legacy.Roots)
	}

	onlyNew, homeNew := backends()[0].open(t)
	mustMkdir(t, onlyNew, filepath.Join(homeNew, ".codex"))
	mustMkdir(t, onlyNew, filepath.Join(homeNew, ".agents", "skills"))
	moved := targets.NewCodex(filepath.Join(homeNew, ".skills")).Detect(onlyNew, homeNew)
	if !moved.Installed || moved.Moved || strings.Contains(moved.Message, "moved") {
		t.Fatalf("new only = %+v", moved)
	}
	agents := filepath.Join(homeNew, ".agents", "skills")
	if len(moved.Roots) != 1 || moved.Roots[0] != filepath.Join(homeNew, ".codex", "skills") {
		t.Fatalf("new only roots = %v", moved.Roots)
	}
	if !problemHas(moved.Problems, targets.LevelInfo, agents) || !problemTextHas(moved.Problems, "custom") {
		t.Fatalf("new only problems = %+v", moved.Problems)
	}

	agentsOnly, homeAgents := backends()[0].open(t)
	mustMkdir(t, agentsOnly, filepath.Join(homeAgents, ".agents", "skills"))
	absent := targets.NewCodex(filepath.Join(homeAgents, ".skills")).Detect(agentsOnly, homeAgents)
	if absent.Installed || absent.Moved {
		t.Fatalf("agents without codex = %+v", absent)
	}
}

func TestCursorStaysWhenItMayReadClaudeSkills(t *testing.T) {
	t.Parallel()
	for _, be := range backends() {
		be := be
		t.Run(be.name, func(t *testing.T) {
			t.Parallel()
			fsys, home := be.open(t)
			mustMkdir(t, fsys, filepath.Join(home, ".cursor"))
			mustMkdir(t, fsys, filepath.Join(home, ".claude", "skills"))
			mustWrite(t, fsys, filepath.Join(home, ".cursor", "cli-config.json"), []byte("{}\n"), 0o644)
			cur := targets.NewCursor(filepath.Join(home, ".skills"))
			if len(cur.AlsoReads()) != 0 {
				t.Fatalf("AlsoReads = %v", cur.AlsoReads())
			}
			det := cur.Detect(fsys, home)
			if !det.Installed || det.Skip || det.Moved {
				t.Fatalf("detect = %+v", det)
			}
			if len(det.Roots) != 1 || det.Roots[0] != filepath.Join(home, ".cursor", "skills") {
				t.Fatalf("roots = %v", det.Roots)
			}
			const phrase = "may also be read via ~/.claude/skills"
			if !problemHas(det.Problems, targets.LevelInfo, phrase) {
				t.Fatalf("detect problems = %+v", det.Problems)
			}
			got, problems, err := targets.Propose(fsys, home)
			if err != nil {
				t.Fatal(err)
			}
			var sawCursor, sawClaude bool
			for _, tg := range got {
				if tg.Kind == domain.TargetCursorUser {
					sawCursor = true
					if tg.Root != filepath.Join(home, ".cursor", "skills") || tg.Mode != domain.SyncCopy || !tg.Enabled {
						t.Fatalf("cursor = %+v", tg)
					}
				}
				if tg.Kind == domain.TargetClaudeUser {
					sawClaude = true
				}
			}
			if !sawCursor || !sawClaude {
				t.Fatalf("propose = %+v", got)
			}
			if !problemHas(problems, targets.LevelInfo, phrase) {
				t.Fatalf("propose problems = %+v", problems)
			}
		})
	}
}

func TestCursorManagedCopyWhenClaudeSkillsAbsent(t *testing.T) {
	t.Parallel()
	fsys, home := backends()[0].open(t)
	mustMkdir(t, fsys, filepath.Join(home, ".cursor"))
	cur := targets.NewCursor(filepath.Join(home, ".skills"))
	if cur.SupportsSymlink() || len(cur.AlsoReads()) != 0 {
		t.Fatal("cursor should manage a copy and AlsoReads stays empty")
	}
	if cur.Supports(domain.SyncSymlink) || !cur.Supports(domain.SyncCopy) {
		t.Fatal("cursor symlink support is unverified")
	}
	det := cur.Detect(fsys, home)
	if !det.Installed || det.Skip {
		t.Fatalf("detect = %+v", det)
	}
	got, _, err := targets.Propose(fsys, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Kind != domain.TargetCursorUser || got[0].Mode != domain.SyncCopy {
		t.Fatalf("propose = %+v", got)
	}
	if got[0].Root != filepath.Join(home, ".cursor", "skills") || !got[0].Enabled {
		t.Fatalf("cursor target = %+v", got[0])
	}
}

func TestClaudeDefaultsToCopy(t *testing.T) {
	t.Parallel()
	fsys, home := backends()[0].open(t)
	mustMkdir(t, fsys, filepath.Join(home, ".claude"))
	ad := targets.NewClaude(filepath.Join(home, ".skills"))
	if ad.SupportsSymlink() || ad.Supports(domain.SyncSymlink) || !ad.Supports(domain.SyncCopy) {
		t.Fatal("claude symlink support is unverified")
	}
	homeTargets := ad.DefaultTargets(home)
	if len(homeTargets) != 1 || homeTargets[0].Mode != domain.SyncCopy || homeTargets[0].Kind != domain.TargetClaudeUser {
		t.Fatalf("defaults = %+v", homeTargets)
	}
	for _, tg := range homeTargets {
		if tg.Kind == domain.TargetClaudeProject {
			t.Fatal("claude project is opt-in")
		}
	}
	det := ad.Detect(fsys, home)
	if !det.Installed || det.Skip || det.Moved {
		t.Fatalf("detect = %+v", det)
	}
	project, err := targets.ClaudeProject(filepath.Join(home, "repo"))
	if err != nil {
		t.Fatal(err)
	}
	if project.Enabled || project.Mode != domain.SyncCopy || project.Kind != domain.TargetClaudeProject {
		t.Fatalf("project = %+v", project)
	}
	if project.Root != filepath.Join(home, "repo", ".claude", "skills") {
		t.Fatalf("project root = %s", project.Root)
	}
}

func TestCodexSymlinkDefault(t *testing.T) {
	t.Parallel()
	home := "/Users/ada"
	ad := targets.NewCodex(filepath.Join(home, ".skills"))
	if !ad.SupportsSymlink() || !ad.Supports(domain.SyncSymlink) {
		t.Fatal("codex symlink support is verified")
	}
	got := ad.DefaultTargets(home)
	if len(got) != 1 || got[0].Mode != domain.SyncSymlink || got[0].Root != filepath.Join(home, ".codex", "skills") {
		t.Fatalf("defaults = %+v", got)
	}
	if strings.Contains(got[0].Root, ".agents") {
		t.Fatalf("default root = %s", got[0].Root)
	}
}

func TestDetectNotInstalled(t *testing.T) {
	t.Parallel()
	fsys, home := backends()[0].open(t)
	if det := targets.NewClaude(home).Detect(fsys, home); det.Installed {
		t.Fatalf("claude = %+v", det)
	}
	if det := targets.NewCursor(home).Detect(fsys, home); det.Installed || det.Skip {
		t.Fatalf("cursor = %+v", det)
	}
	if det := targets.NewCodex(home).Detect(fsys, home); det.Installed {
		t.Fatalf("codex = %+v", det)
	}
	if det := targets.NewCustom(home).Detect(fsys, home); det.Installed || len(targets.NewCustom(home).DefaultTargets(home)) != 0 {
		t.Fatalf("custom = %+v", det)
	}
}
