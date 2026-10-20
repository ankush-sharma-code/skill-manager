# Cursor transcripts and usage fixtures

Captured 2026-10-20 on a Linux cloud agent. Paths observed on this VM are marked **verified on Linux cloud agent; macOS pending**.

The committed Cursor fixture is `testdata/usage/cursor/transcript.json`. Provenance for the synthetic files is `testdata/usage/README.md`. Fixture tests live in `internal/usage/fixture_test.go`.

## Where this session's transcript lived

The process environment sets:

```
AGENT_TRANSCRIPTS=/home/ubuntu/.cursor/projects/workspace/agent-transcripts
```

That directory was absent for the whole session. A search of `~/.cursor`, other home dot-directories, `/tmp`, `/opt`, and `/cursor` after a marker file at `2026-10-20T15:55:31Z` found no `agent-transcripts/*.jsonl` and no `state.vscdb`. The exec-daemon on this VM names the folder `agent-transcripts` and exports it for the agent to read. It does not append the session JSONL.

The session record that exists was fetched from the cloud agent API. The fetch wrote one JSON object, key `messages`, 126 records, about 376 KB before redaction. `events.json` beside that fetch had `count: 0`. The fetch is a point-in-time export, not a log the runtime appends.

`~/.cursor/projects/workspace/` is the project directory for `/workspace`. The daemon's `slugifyPath` replaces every character outside `[A-Za-z0-9]` with `-`, collapses repeats, and strips leading and trailing dashes, so `/workspace` becomes `workspace`. The same function would turn `/Users/you/proj` into `Users-you-proj`. **Verified on Linux cloud agent; macOS pending.**

Public desktop reports, not observed here, use:

```
~/.cursor/projects/<slug>/agent-transcripts/<conversation-uuid>/<conversation-uuid>.jsonl
```

SPEC.md also names `~/Library/Application Support/Cursor/User/workspaceStorage/*/state.vscdb` for older desktop data. That file was absent here. **macOS pending.** No `~/.config/Cursor` tree existed on this VM.

## Cloud export shape

Verified on the export. Format version: the object has no version field. `testdata/usage/cursor/transcript.meta.json` labels it `cursor-cloud-export` / `unversioned-2026-10-20`.

Top-level object, only key `messages`. An assistant turn is several records. There is no `system` role, no `message` wrapper, and no `content[]` array.

| Kind | Keys |
|---|---|
| user | `role`, `text` |
| assistant thinking | `role`, `thinking` |
| assistant text | `role`, `text` |
| assistant tool call | `role`, `tool_calls` (length 1) |
| tool result | `role`, `tool_call_id`, `tool_name`, optional `tool_args`, optional `tool_result`, `started_at_ms`, `completed_at_ms`, `duration_ms` |

A `tool_calls[]` item carries `tool_call_id`, `tool_name`, optional `tool_args`, and the same three millisecond fields. Ids in this session look like `turn-0:step:47:tool`. The following record is `role: "tool"` with the same id.

A skill read is `tool_name: "read_file"` and `tool_args.target_file` set to the absolute path. The agent-facing tool in this chat is named Read. The export stores `read_file`. The result type is `readFileResult`. `value.contents` in the fixture is `<redacted>`. `value.totalLines` stays a number.

The millisecond fields in the committed file are shifted onto synthetic epoch `1700000000000`. Order and `duration_ms` match the export. The original epoch values are not in the file.

Shell clock times for the reads, from this session:

| Step | UTC | `target_file` |
|---|---|---|
| poteto-mode | 2026-10-20T15:57:08Z | `/home/ubuntu/.agents/skills/poteto-mode/SKILL.md` |
| probe-alpha, first | 2026-10-20T15:57:19Z | `/home/ubuntu/.cursor/skills/probe-alpha/SKILL.md` |
| probe-alpha, second | 2026-10-20T15:57:23Z | same symlink path |
| probe-beta | 2026-10-20T15:57:28Z | `/home/ubuntu/.cursor/skills/probe-beta/SKILL.md` |
| probe-repo | 2026-10-20T15:57:33Z | `/workspace/.cursor/skills/probe-repo/SKILL.md` |

The two probe-alpha reads are 4 seconds apart, so the minute dedupe in SPEC.md §7 counts one use. The committed export still has both reads.

## Desktop JSONL

`testdata/usage/cursor/desktop-agent-transcript.unverified.jsonl` is synthetic and **UNVERIFIED** on this VM. The header format is `cursor-desktop-jsonl`, format version `unverified-2026-07`. It uses the publicly described desktop key set: `role` plus `message.content[]` of `text` or `tool_use` (`type`, `name`, `input`), and a `turn_ended` row. A Read is `name: "Read"` and `input.path`. That sample has no timestamps and no tool-result blocks.

## Detection rule

Count a Cursor skill use when either of these is present:

1. A read of a path whose directory is `skills` or `.skills` and whose file is `<name>/SKILL.md`, with `<name>` matching `^[a-z0-9][a-z0-9-]{0,63}$`.
2. An explicit `/name` mention in user text, or a `<command-name>/name</command-name>` record for Claude Code.

On the cloud export, (1) is `tool_name == "read_file"` and `tool_args.target_file`. On the desktop shape, (1) is `tool_use` with `name == "Read"` and `input.path`. This cloud session had no Skill tool. poteto-mode, probe-alpha, and probe-beta were used by reading `SKILL.md`.

Treat (1) and (2) as `confidence: inferred`. Count once per `(session, skill, minute)`. Skip unknown shapes, a truncated last line, and lines over the size cap (SPEC.md names 16 MB; the oversize fixture is just over 1 MiB so the suite stays small).

A parser that accepts only `Read` / `input.path` misses this cloud export. A parser that accepts only `read_file` / `target_file` misses the desktop shape.

## Symlinked skill

`~/.cursor/skills/probe-alpha` was a symlink to `/home/ubuntu/.skills/probe-alpha`. `~/.cursor/skills/probe-beta` was a real directory. The project skill read during the session was `.cursor/skills/probe-repo/SKILL.md`. That copy now lives at `testdata/targets/cursor/skills/probe-repo/SKILL.md` so this repo does not auto-load it. The export still records the path that was read.

The symlinked skill was not picked up by skill discovery.

- The session prompt's skill list was the image's `~/.cursor/skills-cursor/` set. probe-alpha, probe-beta, probe-repo, and poteto-mode were absent.
- Those files were created after the session started.
- The export has no `available_skills` catalog and no `system` message.
- Cursor's skill docs say user skills under `~/.cursor/skills/` and `~/.agents/skills/` stay on the local machine until they are synced, and that Cursor does not copy them onto a cloud agent. **Verified on Linux cloud agent** for this session's catalog. **macOS pending.**

The symlink was readable. Both `read_file` calls recorded the link path. Neither recorded `/home/ubuntu/.skills/probe-alpha/SKILL.md`. A parser that keys off the path the tool was given still sees `probe-alpha`. The edge fixture `cursor/edge/symlink-and-resolved.json` has both the link path and the resolved `~/.skills` path. Both match, because the parent directory is `skills` or `.skills`. A `SKILL.md` whose parent directory has any other name does not match.

## pstack install

Command run, with non-interactive flags so the CLI would finish:

```
npx skills add backnotprop/pstack -g -a cursor --skill '*' -y
```

The CLI treats Cursor as a universal agent: project dir `.agents/skills`, global dir `~/.cursor/skills`. On a global install it writes `~/.agents/skills/<name>/` and returns before linking that copy into `~/.cursor/skills`. poteto-mode was at `/home/ubuntu/.agents/skills/poteto-mode/SKILL.md` only. Its frontmatter sets `disable-model-invocation: true`.

## Claude Code and Codex

Synthetic, UNVERIFIED. See `testdata/usage/README.md`.

Claude: `testdata/usage/claude/skill-tool-use.unverified.jsonl`. An assistant `tool_use` with `name: "Skill"` and `input.skill`, plus a user record whose content holds `<command-name>/example-skill</command-name>`. Nested files under `claude/projects/` cover two sessions and a `subagents/` transcript.

Codex: `testdata/usage/codex/rollout-sample.unverified.jsonl` and `codex/sessions/2026/01/02/rollout-*.jsonl`. A skill use is an `exec_command` `function_call` whose `arguments` open `…/skills/<name>/SKILL.md`. `codex/edge/unmatched-call-id.jsonl` has an output `call_id` with no matching call. The skill path in that output is not a call, so it is not counted.

## Caveats

- This VM is a Linux cloud agent. Desktop JSONL under `agent-transcripts/` was not written here. macOS paths and the macOS SQLite store are pending.
- Cloud export tool names (`read_file`, `run_terminal_cmd`) differ from the names shown to the model (`Read`, `Shell`).
- The export can omit `tool_result` on a call that did succeed. poteto-mode's read is the example in the captured file.
- Skill discovery and skill use are different events. A symlink under `~/.cursor/skills` was readable by path and absent from the catalog.
- `npx skills add` for Cursor global installs into `~/.agents/skills` and, in this CLI version, does not also link `~/.cursor/skills`.
- Heuristic reads stay tagged inferred. Repeated reads inside one minute are one use.
