# Target paths

Expected skill directories (§5) and transcript locations (§7) for the adapters.
The macOS probe (`.github/workflows/macos-probe.yml`) checks install locations, these skill directories, and whether a symlinked skill is visible without credentials. It uploads `probe-report.md` and `probe-report.json`. It does not edit this file.

Every path below is **unverified, pending macos-probe**.

## §5 Skill targets

| Kind | Expected root | Default mode | Notes | Status |
| --- | --- | --- | --- | --- |
| claude-user | `~/.claude/skills` | symlink | Claude Code follows symlinked skill directories. Confirm on the installed version. | unverified, pending macos-probe |
| claude-project | `<repo>/.claude/skills` | copy | Copy so the skill can be committed. Opt-in per project. | unverified, pending macos-probe |
| cursor-user | `~/.cursor/skills` | symlink | Cursor also reads `~/.claude/skills` in recent versions. Detect that and skip it to avoid duplicates. | unverified, pending macos-probe |
| codex-user | `~/.codex/skills` | symlink | Path has moved between Codex versions. This is the main case for `Detect`. | unverified, pending macos-probe |
| codex-user | `~/.agents/skills` | symlink | Alternate Codex user root, used when that directory is present. | unverified, pending macos-probe |
| custom | user-given | symlink or copy | Any other tool that reads `<name>/SKILL.md`. No default root. | unverified, pending macos-probe |

An adapter whose tool does not follow symlinks defaults to copy.

## §7 Transcript paths

| Tool | Expected path | What counts as an invocation | Status |
| --- | --- | --- | --- |
| Claude Code | `~/.claude/projects/<slugified-cwd>/<session-uuid>.jsonl` | An assistant `tool_use` block with `name == "Skill"` and `input.skill == <name>`, or the `<command-name>/<name></command-name>` slash form. | unverified, pending macos-probe |
| Cursor | `~/.cursor/projects/*/agent-transcripts/` (JSONL) | A read of `…/skills/<name>/SKILL.md`, or an explicit `/name` mention. Prefer JSONL. | unverified, pending macos-probe |
| Cursor | `~/Library/Application Support/Cursor/User/workspaceStorage/*/state.vscdb` | Same Cursor signals, in older SQLite workspace storage, only when JSONL is absent. | unverified, pending macos-probe |
| Codex | `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl` | A shell or read tool call that opens `…/skills/<name>/SKILL.md`, or an explicit skill mention. | unverified, pending macos-probe |
