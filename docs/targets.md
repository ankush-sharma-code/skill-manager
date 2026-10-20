# Target paths

Skill directories (§5) and transcript locations (§7) for the adapters.
The macOS probe (`.github/workflows/macos-probe.yml`) checks install locations, these skill directories, and whether a symlinked skill is visible without credentials. It uploads `probe-report.md` and `probe-report.json`. It does not edit this file.

## Probe

Recorded from [run 39454060253](https://github.com/ankush-sharma-code/skill-manager/actions/runs/39454060253).

- OS: macOS 26.6.2, arm64
- Claude Code 2.1.304 (`~/.local/bin/claude`)
- Cursor CLI 2026.10.19-6bbf2c4 (`~/.local/bin/agent`)
- Codex CLI 0.163.0 (`/opt/homebrew/bin/codex`)

`claude auth status` was logged out and still printed the Claude paths. `codex doctor` printed `CODEX_HOME` and the config path. Codex skill discovery used `codex debug prompt-input` (skills are already enabled; `--enable skills` returned `Unknown feature flag: skills`).

## Verified

| What | Path | Evidence |
| --- | --- | --- |
| Claude config | `~/.claude` | `claude auth status` field `configDirectory` |
| Claude projects directory | `~/.claude/projects` | `claude auth status` field `projectsDirectory`. The directory was not created. |
| Cursor CLI config | `~/.cursor/cli-config.json` | Created by `agent about` / `agent status` while logged out |
| Codex home | `~/.codex` | `codex doctor` field `CODEX_HOME` |
| Codex config | `~/.codex/config.toml` | `codex doctor` |
| Codex user skill root | `~/.codex/skills` | Skill catalog root `r0` when `~/.agents/skills` was absent. A directory symlink `probe` → `~/.skills/probe` was listed as `r0/probe/SKILL.md` and the fixture frontmatter was read. |
| Codex user skill root | `~/.agents/skills` | When this directory existed, `r0` switched here and the same symlink was read. Bundled skills stayed at `~/.codex/skills/.system`. `~/.agents` was not created until the probe made it. |

Codex follows directory symlinks at both user skill roots.

## Unverified, needs credentials

| What | Path | Why it is still open |
| --- | --- | --- |
| Claude follows skill symlinks | `~/.claude/skills/<name>` | The probe symlinked `probe` → `~/.skills/probe`. `claude doctor` does not list skills. |
| Cursor follows skill symlinks | `~/.cursor/skills/<name>` | The probe symlinked `probe` → `~/.skills/probe`. `agent about` and `agent status` do not list skills. Cursor also reading `~/.claude/skills` was not tested. |
| Claude session layout | `~/.claude/projects/<slugified-cwd>/<session-uuid>.jsonl` | The projects directory path is verified above. No session file was written. |
| Cursor agent transcripts | `~/.cursor/projects/*/agent-transcripts/` | `~/.cursor/projects` was not created. |
| Cursor workspace storage | `~/Library/Application Support/Cursor/User/workspaceStorage/*/state.vscdb` | That directory was not created. |
| Codex session layout | `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl` | `~/.codex/sessions` was not created. Doctor reported log dir `~/.codex/log` (missing), which is a different path. |

## Not exercised by this run

| Kind | Expected root | Default mode | Notes |
| --- | --- | --- | --- |
| claude-project | `<repo>/.claude/skills` | copy | Opt-in per project. The probe does not copy a skill into a repo. |
| custom | user-given | symlink or copy | No default root. |

An adapter whose tool does not follow symlinks defaults to copy. On this run that fallback is still required for Claude Code and Cursor until a credentialed check shows they follow directory symlinks. Codex user roots stay on symlink.
