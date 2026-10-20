# Usage fixtures

`cursor/transcript.json` is a redacted cloud-agent export from the capture run. Its shape is that export: one JSON object whose only key is `messages`. Prompt text, file bodies, shell output, and thinking are `<redacted>`. `started_at_ms` and `completed_at_ms` are shifted onto a synthetic epoch so the original millisecond values are not in the file. `duration_ms` is unchanged. The export itself has no version field. `cursor/transcript.meta.json` records that as format `cursor-cloud-export`, format version `unversioned-2026-10-20`.

Every other file under this directory is synthetic. Field names follow public descriptions of each tool's transcript format. They are UNVERIFIED against a local Claude Code, Codex, or Cursor desktop install. No third-party transcript text is included.

Shape references, not copied samples:

- Claude Code JSONL: public session-format notes for `~/.claude/projects/<slug>/<session>.jsonl`, including a `Skill` tool_use (`input.skill`) and a `<command-name>/name</command-name>` slash record. Subagent files use `<session>/subagents/`.
- Codex rollout JSONL: public notes for `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`. Lines are `{timestamp, type, payload}`. A shell call is `payload.type` `function_call` with `name` `exec_command` and `arguments` as a JSON string. Join results on `call_id` when both sides have it.
- Cursor desktop JSONL: public notes for `~/.cursor/projects/<slug>/agent-transcripts/<id>/<id>.jsonl`. Lines are `{role, message:{content:[...]}}` with `tool_use` blocks (`name`, `input`) and an optional `{type: turn_ended}` row. This VM did not write that directory.

Each synthetic JSONL file starts with a `fixture_header` row: `format`, `format_version`, `provenance: synthetic`, `unverified: true`.

Edge fixtures live in `*/edge/`, `claude/projects/`, and `codex/sessions/`. They cover a malformed line, a truncated last line, CRLF, unicode text, a line over 1 MiB, nested sessions and `subagents/`, a `SKILL.md` outside a `skills` directory, an invalid skill name, a symlink path and the resolved `~/.skills` path, a `/name` mention, a cloud record with no `*_at_ms`, and a Codex `call_id` whose output has no matching call.
