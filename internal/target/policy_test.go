package target_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/target"
)

func TestSupportsSymlinkFollowsTargetsDoc(t *testing.T) {
	t.Parallel()
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "targets.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(doc)
	verified := section(t, text, "## Verified\n", "## Unverified, needs credentials\n")
	unverified := section(t, text, "## Unverified, needs credentials\n", "## Not exercised by this run\n")
	if !strings.Contains(verified, "Codex follows directory symlinks at both user skill roots.") {
		t.Fatal("verified section no longer records that Codex follows directory symlinks")
	}
	for _, phrase := range []string{
		"Claude follows skill symlinks",
		"Cursor follows skill symlinks",
		"Cursor also reading `~/.claude/skills` was not tested.",
	} {
		if !strings.Contains(unverified, phrase) {
			t.Fatalf("unverified section dropped %q", phrase)
		}
	}
	rows := parsePolicy(t, text)
	codexVerified := strings.Contains(verified, "Codex follows directory symlinks at both user skill roots.")
	if rows["codex-user"].symlink != codexVerified {
		t.Fatal("codex SupportsSymlink must follow the verified section")
	}
	for _, kind := range []string{"claude-user", "claude-project", "cursor-user", "agents"} {
		if rows[kind].symlink {
			t.Fatalf("%s symlink support is unverified", kind)
		}
	}

	home := "/Users/ada"
	repo := filepath.Join(home, ".skills")
	checkUser(t, rows["claude-user"], target.NewClaude(repo), home)
	checkUser(t, rows["cursor-user"], target.NewCursor(repo), home)
	checkUser(t, rows["codex-user"], target.NewCodex(repo), home)

	custom := target.NewCustom(repo)
	if custom.SupportsSymlink() || custom.Kind() != domain.TargetCustom || len(custom.DefaultTargets(home)) != 0 {
		t.Fatal("custom has no verified symlink support and no default root")
	}
	if !custom.Supports(domain.SyncCopy) || custom.Supports(domain.SyncSymlink) {
		t.Fatal("custom symlink support is unverified")
	}

	project, err := target.ClaudeProject(filepath.Join(home, "repo"))
	if err != nil {
		t.Fatal(err)
	}
	if project.Kind != domain.TargetClaudeProject || project.Enabled || project.Mode != domain.SyncMode(rows["claude-project"].mode) {
		t.Fatalf("project = %+v", project)
	}
	if project.Root != filepath.Join(home, "repo", ".claude", "skills") {
		t.Fatalf("project root = %s", project.Root)
	}
	if rows["agents"].mode != "opt-in" || rows["agents"].root != "~/.agents/skills" {
		t.Fatalf("agents policy = %+v", rows["agents"])
	}
}

func TestValidateSkill(t *testing.T) {
	t.Parallel()
	repo := "/Users/ada/.skills"
	name, err := domain.ParseSkillName("demo")
	if err != nil {
		t.Fatal(err)
	}
	good := domain.Skill{
		Name:        name,
		Description: "Says hello",
		Files:       []domain.FileEntry{{RelPath: "SKILL.md"}},
	}
	bad := domain.Skill{Name: "NOPE"}
	adapters := []interface {
		Validate(domain.Skill) []target.Issue
	}{
		target.NewClaude(repo),
		target.NewCursor(repo),
		target.NewCodex(repo),
		target.NewCustom(repo),
	}
	for _, ad := range adapters {
		if issues := ad.Validate(good); len(issues) != 0 {
			t.Fatalf("valid skill issues = %+v", issues)
		}
		issues := ad.Validate(bad)
		fields := map[string]bool{}
		for _, issue := range issues {
			fields[issue.Field] = true
		}
		for _, field := range []string{"name", "description", "files"} {
			if !fields[field] {
				t.Fatalf("issues = %+v", issues)
			}
		}
	}
}

type userAdapter interface {
	Kind() domain.TargetKind
	SupportsSymlink() bool
	Supports(domain.SyncMode) bool
	DefaultTargets(string) []domain.Target
}

type policyRow struct {
	symlink bool
	mode    string
	root    string
}

func checkUser(t *testing.T, row policyRow, ad userAdapter, home string) {
	t.Helper()
	if ad.SupportsSymlink() != row.symlink {
		t.Fatalf("%s SupportsSymlink = %v, doc says %v", ad.Kind(), ad.SupportsSymlink(), row.symlink)
	}
	got := ad.DefaultTargets(home)
	if len(got) != 1 || got[0].Kind != ad.Kind() || string(got[0].Mode) != row.mode {
		t.Fatalf("%s defaults = %+v", ad.Kind(), got)
	}
	if got[0].Root != expandHome(home, row.root) {
		t.Fatalf("%s root = %s, want %s", ad.Kind(), got[0].Root, expandHome(home, row.root))
	}
	if row.symlink {
		if !ad.Supports(domain.SyncSymlink) {
			t.Fatalf("%s rejects verified symlink mode", ad.Kind())
		}
		return
	}
	if ad.Supports(domain.SyncSymlink) || !ad.Supports(domain.SyncCopy) {
		t.Fatalf("%s offers unverified symlink mode", ad.Kind())
	}
}

func parsePolicy(t *testing.T, doc string) map[string]policyRow {
	t.Helper()
	body := section(t, doc, "## Adapter policy\n", "")
	rows := map[string]policyRow{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || strings.Contains(line, "---") || strings.Contains(line, "SupportsSymlink") {
			continue
		}
		cells := splitCells(line)
		if len(cells) != 4 {
			t.Fatalf("policy row %q", line)
		}
		rows[cells[0]] = policyRow{symlink: cells[1] == "yes", mode: cells[2], root: cells[3]}
	}
	for _, kind := range []string{"claude-user", "claude-project", "cursor-user", "codex-user", "agents"} {
		if _, ok := rows[kind]; !ok {
			t.Fatalf("policy missing %s", kind)
		}
	}
	return rows
}

func section(t *testing.T, doc, start, end string) string {
	t.Helper()
	i := strings.Index(doc, start)
	if i < 0 {
		t.Fatalf("missing %q", start)
	}
	body := doc[i+len(start):]
	if end == "" {
		return body
	}
	j := strings.Index(body, end)
	if j < 0 {
		t.Fatalf("missing %q", end)
	}
	return body[:j]
}

func splitCells(line string) []string {
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func expandHome(home, root string) string {
	if strings.HasPrefix(root, "~/") {
		return filepath.Join(home, root[2:])
	}
	return root
}
