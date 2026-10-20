#!/usr/bin/env python3
"""Probe Claude Code, Cursor CLI, and Codex CLI skill paths without credentials.

Installs each CLI, runs version/help/doctor (or the closest config command),
lists ~/.claude, ~/.cursor, ~/.codex, and ~/.agents, then symlinks
~/.skills/probe into each documented user skills directory and checks whether
the tool can see that skill without an API key.

Authenticated prompts are out of scope. This script never reads
ANTHROPIC_API_KEY, OPENAI_API_KEY, or CURSOR_API_KEY.
"""

from __future__ import annotations

import json
import os
import platform
import re
import shutil
import signal
import subprocess
import sys
import time
from pathlib import Path

MARKER = "PROBE_SKILL_MARKER"
SKILL_BODY = """---
name: probe
description: Discoverable macOS probe skill. Marker PROBE_SKILL_MARKER.
---

# probe

PROBE_SKILL_MARKER

Fixture skill for the macOS path probe. It has no side effects.
"""

OUTPUT_LIMIT = 24_000
TREE_LIMIT = 250
TREE_DEPTH = 3

HOME_DIRS = (".claude", ".cursor", ".codex", ".agents")

CREDENTIAL_ENV = (
    "ANTHROPIC_API_KEY",
    "ANTHROPIC_AUTH_TOKEN",
    "CLAUDE_API_KEY",
    "OPENAI_API_KEY",
    "CODEX_API_KEY",
    "CURSOR_API_KEY",
)

SECRET_RES = (
    re.compile(r"sk-ant-[A-Za-z0-9_\-]{8,}"),
    re.compile(r"sk-[A-Za-z0-9_\-]{20,}"),
    re.compile(r"cursor_[A-Za-z0-9_\-]{16,}"),
    re.compile(r"(?i)(api[_-]?key|authorization|bearer)\s*[:=]\s*\S+"),
)


def redact(text: str) -> str:
    if not text:
        return ""
    text = text.replace("\x00", "")
    for pattern in SECRET_RES:
        if pattern.groups:
            text = pattern.sub(lambda m: m.group(1) + "=[redacted]", text)
        else:
            text = pattern.sub("[redacted]", text)
    return text


def clip(text: str, limit: int = OUTPUT_LIMIT) -> dict:
    text = redact(text)
    if len(text) <= limit:
        return {"text": text, "truncated": False, "original_chars": len(text)}
    return {
        "text": text[:limit] + "\n…[truncated]",
        "truncated": True,
        "original_chars": len(text),
    }


def drop_credentials() -> None:
    for key in CREDENTIAL_ENV:
        os.environ.pop(key, None)


def prepend_path(*dirs: str) -> None:
    current = [part for part in os.environ.get("PATH", "").split(":") if part]
    for directory in reversed([d for d in dirs if d]):
        if directory not in current:
            current.insert(0, directory)
    os.environ["PATH"] = ":".join(current)


def run_cmd(argv: list[str], timeout: int, cwd: Path | None = None) -> dict:
    try:
        proc = subprocess.Popen(
            argv,
            cwd=str(cwd) if cwd else None,
            env=os.environ.copy(),
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            start_new_session=True,
        )
    except FileNotFoundError as exc:
        return {
            "argv": argv,
            "exit_code": None,
            "timed_out": False,
            "not_found": True,
            "stdout": clip(""),
            "stderr": clip(str(exc)),
        }
    try:
        out, err = proc.communicate(timeout=timeout)
        return _finished(argv, proc.returncode, False, out, err)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except OSError:
            proc.kill()
        out, err = proc.communicate()
        return _finished(argv, None, True, out, err)


def _finished(argv: list[str], exit_code: int | None, timed_out: bool, out: bytes | None, err: bytes | None) -> dict:
    stdout = (out or b"").decode("utf-8", "replace")
    stderr = (err or b"").decode("utf-8", "replace")
    full = stdout + "\n" + stderr
    return {
        "argv": argv,
        "exit_code": exit_code,
        "timed_out": timed_out,
        "not_found": False,
        "stdout": clip(stdout),
        "stderr": clip(stderr),
        "classification": classify_listing(full),
        "excerpt": excerpt(full, MARKER)
        or excerpt(full, "Discoverable macOS probe skill.")
        or excerpt(full, "probe/SKILL.md"),
    }


def which(name: str) -> str | None:
    return shutil.which(name)


def npm_global_bin() -> str | None:
    npm = which("npm")
    if not npm:
        return None
    result = run_cmd([npm, "prefix", "-g"], timeout=30)
    prefix = result["stdout"]["text"].strip().splitlines()
    if result["exit_code"] == 0 and prefix:
        return str(Path(prefix[-1]) / "bin")
    return None


def refresh_path() -> None:
    extra = [
        str(Path.home() / ".local" / "bin"),
        str(Path.home() / ".claude" / "local"),
        str(Path.home() / ".claude" / "bin"),
        "/usr/local/bin",
        "/opt/homebrew/bin",
    ]
    npm_bin = npm_global_bin()
    if npm_bin:
        extra.append(npm_bin)
    prepend_path(*extra)


def combined_text(result: dict) -> str:
    return result["stdout"]["text"] + "\n" + result["stderr"]["text"]


def command_record(result: dict) -> dict:
    return {
        "argv": result["argv"],
        "exit_code": result["exit_code"],
        "timed_out": result["timed_out"],
        "not_found": result.get("not_found", False),
        "stdout": result["stdout"]["text"],
        "stderr": result["stderr"]["text"],
        "stdout_truncated": result["stdout"]["truncated"],
        "stderr_truncated": result["stderr"]["truncated"],
        "classification": result.get("classification") or classify_listing(""),
        "excerpt": result.get("excerpt") or "",
    }


def list_tree(path: Path) -> dict:
    home = Path.home()
    if not path.exists() and not path.is_symlink():
        return {"exists": False, "entries": [], "truncated": False}

    entries: list[str] = []

    def add(line: str) -> None:
        if len(entries) < TREE_LIMIT:
            entries.append(line)

    def rel(p: Path) -> str:
        try:
            return str(p.relative_to(home))
        except ValueError:
            return str(p)

    def walk(current: Path, depth: int) -> None:
        if len(entries) >= TREE_LIMIT:
            return
        try:
            if current.is_symlink():
                add(f"{rel(current)} -> {os.readlink(current)}")
                return
        except OSError as exc:
            add(f"{rel(current)}: {exc}")
            return
        if depth > 0:
            kind = "/" if current.is_dir() else ""
            add(f"{rel(current)}{kind}")
        if not current.is_dir() or depth >= TREE_DEPTH:
            return
        try:
            children = sorted(current.iterdir(), key=lambda p: p.name)
        except OSError as exc:
            add(f"{rel(current)}: {exc}")
            return
        for child in children:
            walk(child, depth + 1)

    walk(path, 0)
    return {
        "exists": True,
        "entries": entries,
        "truncated": len(entries) >= TREE_LIMIT,
    }


def snapshot_homes() -> dict:
    home = Path.home()
    return {name: list_tree(home / name) for name in HOME_DIRS}


def new_entries(before: dict, after: dict) -> dict:
    created = {}
    for name in HOME_DIRS:
        old = set(before.get(name, {}).get("entries") or [])
        new = [entry for entry in after.get(name, {}).get("entries") or [] if entry not in old]
        created[name] = {
            "exists_after": bool(after.get(name, {}).get("exists")),
            "created": new,
        }
    return created


def skill_roots() -> list[dict]:
    home = Path.home()
    return [
        {
            "id": "claude-user",
            "spec": "§5",
            "tool": "claude",
            "root": home / ".claude" / "skills",
            "expected": "~/.claude/skills",
            "mode": "symlink",
        },
        {
            "id": "cursor-user",
            "spec": "§5",
            "tool": "cursor",
            "root": home / ".cursor" / "skills",
            "expected": "~/.cursor/skills",
            "mode": "symlink",
        },
        {
            "id": "codex-user",
            "spec": "§5",
            "tool": "codex",
            "root": home / ".codex" / "skills",
            "expected": "~/.codex/skills",
            "mode": "symlink",
        },
        {
            "id": "codex-user-agents",
            "spec": "§5",
            "tool": "codex",
            "root": home / ".agents" / "skills",
            "expected": "~/.agents/skills",
            "mode": "symlink",
        },
    ]


def log_paths() -> list[dict]:
    home = Path.home()
    return [
        {
            "tool": "claude",
            "spec": "§7",
            "expected": "~/.claude/projects/<slugified-cwd>/<session-uuid>.jsonl",
            "parent": home / ".claude" / "projects",
        },
        {
            "tool": "cursor",
            "spec": "§7",
            "expected": "~/.cursor/projects/*/agent-transcripts/",
            "parent": home / ".cursor" / "projects",
        },
        {
            "tool": "cursor",
            "spec": "§7",
            "expected": "~/Library/Application Support/Cursor/User/workspaceStorage/*/state.vscdb",
            "parent": home / "Library" / "Application Support" / "Cursor" / "User" / "workspaceStorage",
        },
        {
            "tool": "codex",
            "spec": "§7",
            "expected": "~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl",
            "parent": home / ".codex" / "sessions",
        },
    ]


def classify_listing(text: str) -> dict:
    """Decide whether tool output shows the probe skill was read through a symlink."""
    marker = MARKER in text
    description = "Discoverable macOS probe skill." in text
    skill_path = bool(re.search(r"probe[/\\]SKILL\.md", text))
    catalog_name = bool(
        re.search(r"(?m)^[\s\-*]*probe\s*[:\-]", text)
        or re.search(r"\bname\s*[:=]\s*['\"]?probe['\"]?", text)
    )
    read_through = marker or description
    return {
        "marker_present": marker,
        "description_present": description,
        "skill_path_present": skill_path,
        "catalog_name_present": catalog_name,
        "read_through_symlink": read_through,
    }


def conclusion_for(classification: dict, discovery_ran: bool, command_failed: bool) -> str:
    if classification["read_through_symlink"]:
        return "symlink_followed"
    if not discovery_ran:
        return "not_observable_without_credentials"
    if command_failed:
        return "check_failed"
    if classification["catalog_name_present"] or classification["skill_path_present"]:
        return "listed_without_frontmatter"
    return "not_listed"


def excerpt(text: str, needle: str, radius: int = 240) -> str:
    idx = text.find(needle)
    if idx < 0:
        return ""
    start = max(0, idx - radius)
    end = min(len(text), idx + len(needle) + radius)
    return text[start:end]


def install_claude() -> dict:
    refresh_path()
    attempts = []
    script = run_cmd(
        ["bash", "-lc", "curl -fsSL https://claude.ai/install.sh | bash"],
        timeout=240,
    )
    attempts.append({"method": "curl -fsSL https://claude.ai/install.sh | bash", **command_record(script)})
    refresh_path()
    binary = which("claude")
    if not binary:
        npm = which("npm")
        if npm:
            npm_result = run_cmd([npm, "i", "-g", "@anthropic-ai/claude-code"], timeout=300)
            if npm_result["exit_code"] != 0:
                sudo = which("sudo")
                if sudo:
                    npm_result = run_cmd(
                        [sudo, npm, "i", "-g", "@anthropic-ai/claude-code"],
                        timeout=300,
                    )
                    method = "sudo npm i -g @anthropic-ai/claude-code"
                else:
                    method = "npm i -g @anthropic-ai/claude-code"
            else:
                method = "npm i -g @anthropic-ai/claude-code"
            attempts.append({"method": method, **command_record(npm_result)})
            refresh_path()
            binary = which("claude")
        else:
            attempts.append({"method": "npm", "error": "npm not found"})
    return {"binary": binary, "attempts": attempts}


def install_cursor() -> dict:
    refresh_path()
    result = run_cmd(
        ["bash", "-lc", "curl https://cursor.com/install -fsS | bash"],
        timeout=240,
    )
    refresh_path()
    binary = which("agent") or which("cursor-agent")
    return {
        "binary": binary,
        "attempts": [
            {
                "method": "curl https://cursor.com/install -fsS | bash",
                **command_record(result),
            }
        ],
    }


def install_codex() -> dict:
    refresh_path()
    npm = which("npm")
    attempts = []
    if not npm:
        return {"binary": None, "attempts": [{"method": "npm", "error": "npm not found"}]}
    result = run_cmd([npm, "i", "-g", "@openai/codex"], timeout=360)
    method = "npm i -g @openai/codex"
    if result["exit_code"] != 0 and which("sudo"):
        result = run_cmd(["sudo", npm, "i", "-g", "@openai/codex"], timeout=360)
        method = "sudo npm i -g @openai/codex"
    attempts.append({"method": method, **command_record(result)})
    refresh_path()
    return {"binary": which("codex"), "attempts": attempts}


def version_and_help(binary: str) -> dict:
    version = run_cmd([binary, "--version"], timeout=30)
    if version["exit_code"] not in (0, None) or version["timed_out"]:
        short = run_cmd([binary, "-v"], timeout=30)
        if short["exit_code"] == 0:
            version = short
    help_result = run_cmd([binary, "--help"], timeout=30)
    if help_result["exit_code"] not in (0, None) and not help_result["timed_out"]:
        short_help = run_cmd([binary, "-h"], timeout=30)
        if short_help["exit_code"] == 0:
            help_result = short_help
    return {
        "version": command_record(version),
        "help": command_record(help_result),
        "version_text": first_line(version["stdout"]["text"]) or first_line(version["stderr"]["text"]),
    }


def first_line(text: str) -> str:
    for line in text.splitlines():
        stripped = line.strip()
        if stripped:
            return stripped
    return ""


def extra_unauth_commands(tool: str, binary: str, help_text: str) -> list[dict]:
    argv_list: list[list[str]] = []
    if tool == "claude":
        argv_list = [
            [binary, "doctor"],
            [binary, "auth", "status"],
        ]
    elif tool == "cursor":
        argv_list = [
            [binary, "about"],
            [binary, "status"],
        ]
        if "--format" in help_text:
            argv_list.extend(
                [
                    [binary, "about", "--format", "json"],
                    [binary, "status", "--format", "json"],
                ]
            )
    elif tool == "codex":
        argv_list = [
            [binary, "doctor", "--summary"],
            [binary, "doctor"],
            [binary, "debug", "prompt-input", "--help"],
        ]
    records = []
    for argv in argv_list:
        records.append(command_record(run_cmd(argv, timeout=90)))
    return records


def help_mentions(help_text: str, word: str) -> bool:
    return re.search(rf"(?m)^\s*{re.escape(word)}\b", help_text) is not None or (
        f" {word}" in help_text or f"\n{word}" in help_text
    )


def probe_skill_commands(tool: str, binary: str, help_text: str) -> list[list[str]]:
    """Commands that might list skills without an API key. Codex prompt-input is the known one."""
    commands: list[list[str]] = []
    if tool == "codex":
        commands.append([binary, "--enable", "skills", "debug", "prompt-input", "probe"])
    if help_mentions(help_text, "skills"):
        commands.append([binary, "skills", "--help"])
    return commands


def remove_probe_links(roots: list[dict]) -> None:
    for item in roots:
        dest = item["root"] / "probe"
        if dest.is_symlink():
            dest.unlink()


def place_symlink(root: Path, source: Path) -> dict:
    root.mkdir(parents=True, exist_ok=True)
    dest = root / "probe"
    if dest.is_symlink():
        dest.unlink()
    elif dest.exists():
        return {
            "ok": False,
            "path": str(dest),
            "error": "a real probe entry already exists; refusing to replace it",
        }
    dest.symlink_to(source, target_is_directory=True)
    resolved = None
    try:
        resolved = str(dest.resolve())
    except OSError as exc:
        resolved = f"resolve failed: {exc}"
    return {
        "ok": True,
        "path": str(dest),
        "target": str(source),
        "readlink": os.readlink(dest),
        "resolved": resolved,
        "is_symlink": dest.is_symlink(),
    }


def search_tool_files(tool_homes: list[Path], since: float) -> list[dict]:
    """Find tool-written files that contain the marker, skipping the fixture skill."""
    hits = []
    source = Path.home() / ".skills"
    for home_dir in tool_homes:
        if not home_dir.exists():
            continue
        for dirpath, dirnames, filenames in os.walk(home_dir, followlinks=False):
            current = Path(dirpath)
            if current.is_symlink():
                dirnames[:] = []
                continue
            kept = []
            for name in dirnames:
                child = current / name
                if child.is_symlink():
                    continue
                try:
                    if source in child.resolve().parents or child.resolve() == source:
                        continue
                except OSError:
                    pass
                kept.append(name)
            dirnames[:] = kept
            for name in filenames:
                path = current / name
                try:
                    if path.is_symlink():
                        continue
                    stat = path.stat()
                    if stat.st_mtime < since or stat.st_size > 2_000_000:
                        continue
                    text = path.read_text(encoding="utf-8", errors="replace")
                except OSError:
                    continue
                if MARKER in text or "Discoverable macOS probe skill." in text:
                    hits.append(
                        {
                            "path": str(path),
                            "classification": classify_listing(text),
                            "excerpt": excerpt(text, MARKER) or excerpt(text, "Discoverable macOS probe skill."),
                        }
                    )
                    if len(hits) >= 10:
                        return hits
    return hits


def ensure_codex_trust(repo: Path) -> dict:
    home = Path.home() / ".codex"
    home.mkdir(parents=True, exist_ok=True)
    cfg = home / "config.toml"
    existing = cfg.read_text(encoding="utf-8") if cfg.exists() else ""
    key = str(repo)
    if f'[projects."{key}"]' in existing and "trust_level" in existing:
        return {"wrote": False, "path": str(cfg)}
    snippet = f'\n[projects."{key}"]\ntrust_level = "trusted"\n'
    cfg.write_text(existing + snippet, encoding="utf-8")
    return {"wrote": True, "path": str(cfg)}


def run_discovery(tool: str, binary: str, repo: Path, help_text: str) -> dict:
    commands = []
    trust = None
    if tool == "claude":
        commands.append(command_record(run_cmd([binary, "doctor"], timeout=90, cwd=repo)))
    elif tool == "cursor":
        commands.append(command_record(run_cmd([binary, "about"], timeout=45, cwd=repo)))
        commands.append(command_record(run_cmd([binary, "status"], timeout=45, cwd=repo)))
    elif tool == "codex":
        variants = [
            [binary, "--enable", "skills", "debug", "prompt-input", "probe"],
            [binary, "debug", "prompt-input", "probe"],
        ]
        for argv in variants:
            result = run_cmd(argv, timeout=90, cwd=repo)
            commands.append(command_record(result))
            blob = combined_text(result).lower()
            if result["timed_out"]:
                break
            if result["exit_code"] != 0 and "trust" in blob and trust is None:
                trust = ensure_codex_trust(repo)
                result = run_cmd(argv, timeout=90, cwd=repo)
                commands.append(command_record(result))
                blob = combined_text(result).lower()
            succeeded = result["exit_code"] == 0 and not result["timed_out"]
            flag_rejected = any(
                phrase in blob
                for phrase in ("unexpected argument", "unrecognized", "unknown flag", "invalid option")
            )
            if succeeded and not flag_rejected:
                break
            if result["timed_out"]:
                break
        for extra in probe_skill_commands(tool, binary, help_text):
            if extra in [item["argv"] for item in commands]:
                continue
            if extra[1:3] == ["skills", "--help"]:
                commands.append(command_record(run_cmd(extra, timeout=20, cwd=repo)))
    discovery_commands = {
        "claude": "doctor",
        "cursor": "about/status",
        "codex": "debug prompt-input",
    }
    known_lister = tool == "codex"
    return {
        "commands": commands,
        "trust": trust,
        "known_unauthenticated_lister": known_lister,
        "lister": discovery_commands.get(tool),
    }


def texts_of(commands: list[dict]) -> str:
    return "\n".join(item.get("stdout", "") + "\n" + item.get("stderr", "") for item in commands)


def merge_classification(commands: list[dict], file_hits: list[dict]) -> dict:
    merged = classify_listing("")
    keys = (
        "marker_present",
        "description_present",
        "skill_path_present",
        "catalog_name_present",
        "read_through_symlink",
    )
    sources = [item.get("classification") or {} for item in commands]
    sources.extend(hit.get("classification") or {} for hit in file_hits)
    for source in sources:
        for key in keys:
            merged[key] = bool(merged[key] or source.get(key))
    merged["read_from_tool_file"] = bool(file_hits)
    return merged


def command_failed(commands: list[dict], known_lister: bool) -> bool:
    if not known_lister:
        return False
    lister = [
        item
        for item in commands
        if "prompt-input" in item.get("argv", []) and "--help" not in item.get("argv", [])
    ]
    if not lister:
        return True
    return not any(item["exit_code"] == 0 and not item["timed_out"] for item in lister)


def probe_one_root(item: dict, binary: str | None, repo: Path, help_text: str, source: Path) -> dict:
    link = place_symlink(item["root"], source)
    record = {
        "id": item["id"],
        "spec": item["spec"],
        "tool": item["tool"],
        "expected": item["expected"],
        "mode": item["mode"],
        "root": str(item["root"]),
        "symlink": link,
    }
    if not binary or not link.get("ok"):
        record["conclusion"] = "check_failed" if binary else "tool_not_installed"
        record["classification"] = classify_listing("")
        record["commands"] = []
        record["file_hits"] = []
        return record
    started = time.time() - 1
    discovery = run_discovery(item["tool"], binary, repo, help_text)
    homes = {
        "claude": [Path.home() / ".claude"],
        "cursor": [Path.home() / ".cursor"],
        "codex": [Path.home() / ".codex", Path.home() / ".agents"],
    }[item["tool"]]
    file_hits = search_tool_files(homes, started)
    classification = merge_classification(discovery["commands"], file_hits)
    failed = command_failed(discovery["commands"], discovery["known_unauthenticated_lister"])
    record.update(
        {
            "commands": discovery["commands"],
            "trust_config": discovery["trust"],
            "known_unauthenticated_lister": discovery["known_unauthenticated_lister"],
            "lister": discovery["lister"],
            "classification": classification,
            "file_hits": file_hits,
            "conclusion": conclusion_for(
                classification,
                discovery["known_unauthenticated_lister"],
                failed,
            ),
        }
    )
    return record


def observed_strings(commands: list[dict]) -> str:
    return texts_of(commands)


def log_observations(tool_outputs: dict) -> list[dict]:
    observations = []
    for item in log_paths():
        parent: Path = item["parent"]
        exists = parent.exists()
        jsonl = []
        vscdb = []
        if exists and parent.is_dir() and not parent.is_symlink():
            for dirpath, dirnames, filenames in os.walk(parent, followlinks=False):
                dirnames[:] = [name for name in dirnames if not (Path(dirpath) / name).is_symlink()]
                for name in filenames:
                    if name.endswith(".jsonl"):
                        jsonl.append(str(Path(dirpath) / name))
                    if name.endswith(".vscdb"):
                        vscdb.append(str(Path(dirpath) / name))
                if len(jsonl) + len(vscdb) >= 20:
                    break
        tool_text = tool_outputs.get(item["tool"], "")
        parent_mentioned = str(parent) in tool_text or item["expected"] in tool_text
        if jsonl or vscdb:
            status = "files_present_layout_not_validated"
        elif parent_mentioned:
            status = "path_mentioned_by_tool"
        elif exists:
            status = "parent_exists_layout_unverified"
        else:
            status = "not_created_unverified"
        observations.append(
            {
                "tool": item["tool"],
                "spec": item["spec"],
                "expected": item["expected"],
                "parent": str(parent),
                "parent_exists": exists,
                "parent_mentioned_by_tool": parent_mentioned,
                "jsonl_sample": jsonl[:10],
                "vscdb_sample": vscdb[:10],
                "status": status,
                "note": "Authenticated transcript capture is not part of this job.",
            }
        )
    return observations


def render_markdown(report: dict) -> str:
    lines = [
        "# macOS target probe",
        "",
        "Unauthenticated check of install locations, user skill directories, and symlink visibility.",
        "Authenticated prompts are not part of this job. No repository secrets were read, and no step was skipped for a missing secret.",
        "",
        "## Runner",
        "",
        f"- OS: {report.get('runner', {}).get('platform', '')}",
        f"- Machine: {report.get('runner', {}).get('machine', '')}",
        f"- Home: {report.get('runner', {}).get('home', '')}",
        f"- Python: {report.get('runner', {}).get('python', '')}",
        "",
        "## Installations",
        "",
    ]
    for name, tool in (report.get("tools") or {}).items():
        version = tool.get("version_text") or "(no version)"
        binary = tool.get("binary") or "(not found)"
        lines.append(f"### {name}")
        lines.append("")
        lines.append(f"- Binary: `{binary}`")
        lines.append(f"- Version: `{version}`")
        for attempt in tool.get("install_attempts", []):
            method = attempt.get("method", "install")
            code = attempt.get("exit_code")
            timed = attempt.get("timed_out")
            lines.append(f"- Install `{method}` exit={code} timed_out={timed}")
        lines.append("")
        lines.append("Version output:")
        lines.append("")
        lines.append("```")
        lines.append((tool.get("version") or {}).get("stdout") or "(empty)")
        lines.append("```")
        lines.append("")
    lines.extend(["## Home directories after unauthenticated commands", ""])
    for name, tree in (report.get("home_after_commands") or {}).items():
        lines.append(f"### ~/{name}")
        lines.append("")
        if not tree.get("exists"):
            lines.append("Absent.")
            lines.append("")
            continue
        lines.append("```")
        lines.extend(tree.get("entries") or ["(empty)"])
        if tree.get("truncated"):
            lines.append("…[listing truncated]")
        lines.append("```")
        lines.append("")
    lines.extend(["## Home directories after the skill probe", ""])
    lines.append("These listings include the fixture symlink while it is still in place.")
    lines.append("")
    for name, tree in (report.get("home_after_probe") or {}).items():
        lines.append(f"### ~/{name}")
        lines.append("")
        if not tree.get("exists"):
            lines.append("Absent.")
            lines.append("")
            continue
        lines.append("```")
        lines.extend(tree.get("entries") or ["(empty)"])
        if tree.get("truncated"):
            lines.append("…[listing truncated]")
        lines.append("```")
        lines.append("")
    lines.extend(["## Directories created by each tool", ""])
    for name, created in (report.get("created_by_tool") or {}).items():
        lines.append(f"### {name}")
        lines.append("")
        any_created = False
        for directory, info in created.items():
            made = info.get("created") or []
            if not made and not info.get("exists_after"):
                continue
            any_created = True
            lines.append(f"`~/{directory}` exists={info.get('exists_after')}")
            lines.append("")
            if made:
                lines.append("```")
                lines.extend(made)
                lines.append("```")
                lines.append("")
        if not any_created:
            lines.append("No watched home directory entries appeared during this tool's commands.")
            lines.append("")
    lines.extend(["## Skill symlink probe", ""])
    lines.append(f"Fixture: `~/.skills/probe/SKILL.md` (marker `{MARKER}`).")
    lines.append("Each documented user root was symlinked alone, then the tool's unauthenticated commands were run.")
    lines.append("")
    for item in report.get("skill_probes") or []:
        lines.append(f"### {item['tool']} {item['expected']}")
        lines.append("")
        lines.append(f"- Conclusion: `{item['conclusion']}`")
        link = item.get("symlink") or {}
        lines.append(f"- Symlink ok: {link.get('ok')}")
        if link.get("readlink"):
            lines.append(f"- Readlink: `{link.get('path')}` -> `{link.get('readlink')}`")
        classification = item.get("classification") or {}
        lines.append(
            "- Evidence: "
            f"marker={classification.get('marker_present')} "
            f"description={classification.get('description_present')} "
            f"skill_path={classification.get('skill_path_present')}"
        )
        if item.get("known_unauthenticated_lister") is False:
            lines.append("- This tool has no unauthenticated skill-list command in this probe. `about`, `status`, and `doctor` were still run after the symlink was in place.")
        for hit in item.get("file_hits") or []:
            lines.append(f"- Marker also found in `{hit['path']}`")
        interesting = ""
        for command in item.get("commands") or []:
            interesting = command.get("excerpt") or ""
            if interesting:
                break
        if interesting:
            lines.append("")
            lines.append("```")
            lines.append(interesting)
            lines.append("```")
        lines.append("")
    lines.extend(["## §7 transcript locations", ""])
    lines.append("These paths were not filled by a session. The job only records whether the parent directory exists or a tool printed the path.")
    lines.append("")
    for item in report.get("log_paths") or []:
        lines.append(
            f"- `{item['expected']}` — {item['status']} "
            f"(parent exists={item['parent_exists']}, mentioned={item['parent_mentioned_by_tool']})"
        )
    lines.extend(
        [
            "",
            "## Credentials",
            "",
            "Authenticated prompts were removed from this job. No API key was read. No step was skipped because a secret was missing.",
            "",
        ]
    )
    return "\n".join(lines) + "\n"


def write_reports(report: dict, dest: Path) -> None:
    dest.mkdir(parents=True, exist_ok=True)
    markdown = render_markdown(report)
    (dest / "probe-report.md").write_text(markdown, encoding="utf-8")
    (dest / "probe-report.json").write_text(
        json.dumps(report, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(markdown)


def build_report(repo: Path) -> dict:
    drop_credentials()
    refresh_path()
    baseline = snapshot_homes()
    print("Installing Claude Code", flush=True)
    claude_install = install_claude()
    claude_bin = claude_install["binary"]
    claude_info = {"binary": claude_bin, "install_attempts": claude_install["attempts"]}
    if claude_bin:
        meta = version_and_help(claude_bin)
        claude_info.update(meta)
        claude_info["commands"] = extra_unauth_commands(
            "claude", claude_bin, (claude_info.get("help") or {}).get("stdout") or ""
        )
        print(f"claude version: {claude_info.get('version_text')}", flush=True)
    after_claude = snapshot_homes()

    print("Installing Cursor CLI", flush=True)
    cursor_install = install_cursor()
    cursor_bin = cursor_install["binary"]
    cursor_info = {"binary": cursor_bin, "install_attempts": cursor_install["attempts"]}
    if cursor_bin:
        meta = version_and_help(cursor_bin)
        cursor_info.update(meta)
        cursor_info["commands"] = extra_unauth_commands(
            "cursor", cursor_bin, (cursor_info.get("help") or {}).get("stdout") or ""
        )
        print(f"cursor version: {cursor_info.get('version_text')}", flush=True)
    after_cursor = snapshot_homes()

    print("Installing Codex CLI", flush=True)
    codex_install = install_codex()
    codex_bin = codex_install["binary"]
    codex_info = {"binary": codex_bin, "install_attempts": codex_install["attempts"]}
    if codex_bin:
        meta = version_and_help(codex_bin)
        codex_info.update(meta)
        codex_info["commands"] = extra_unauth_commands(
            "codex", codex_bin, (codex_info.get("help") or {}).get("stdout") or ""
        )
        print(f"codex version: {codex_info.get('version_text')}", flush=True)
    after_codex = snapshot_homes()

    source = Path.home() / ".skills" / "probe"
    source.mkdir(parents=True, exist_ok=True)
    (source / "SKILL.md").write_text(SKILL_BODY, encoding="utf-8")

    roots = skill_roots()
    binaries = {"claude": claude_bin, "cursor": cursor_bin, "codex": codex_bin}
    help_text = {
        "claude": ((claude_info.get("help") or {}).get("stdout") or ""),
        "cursor": ((cursor_info.get("help") or {}).get("stdout") or ""),
        "codex": ((codex_info.get("help") or {}).get("stdout") or "")
        + "\n"
        + texts_of(codex_info.get("commands") or []),
    }
    probes = []
    for item in roots:
        remove_probe_links(roots)
        print(f"Probing {item['tool']} at {item['expected']}", flush=True)
        probes.append(
            probe_one_root(item, binaries[item["tool"]], repo, help_text[item["tool"]], source)
        )
    for item in roots:
        place_symlink(item["root"], source)
    after_probe = snapshot_homes()
    remove_probe_links(roots)

    tool_outputs = {
        "claude": texts_of(claude_info.get("commands") or [])
        + "\n"
        + ((claude_info.get("version") or {}).get("stdout") or ""),
        "cursor": texts_of(cursor_info.get("commands") or []),
        "codex": texts_of(codex_info.get("commands") or []),
    }
    for probe in probes:
        tool_outputs[probe["tool"]] += "\n" + texts_of(probe.get("commands") or [])

    sw = run_cmd(["sw_vers"], timeout=10)
    uname = run_cmd(["uname", "-a"], timeout=10)
    report = {
        "scope": {
            "authenticated_prompts": "removed",
            "secrets_read": [],
            "skipped_for_missing_secrets": [],
            "note": "The job checks install locations, skill directories, and symlink visibility only.",
        },
        "runner": {
            "platform": platform.platform(),
            "machine": platform.machine(),
            "home": str(Path.home()),
            "python": sys.version.split()[0],
            "sw_vers": sw["stdout"]["text"].strip(),
            "uname": uname["stdout"]["text"].strip(),
            "repo": str(repo),
        },
        "tools": {
            "claude": claude_info,
            "cursor": cursor_info,
            "codex": codex_info,
        },
        "home_before": baseline,
        "home_after_commands": after_codex,
        "home_after_probe": after_probe,
        "created_by_tool": {
            "claude": new_entries(baseline, after_claude),
            "cursor": new_entries(after_claude, after_cursor),
            "codex": new_entries(after_cursor, after_codex),
        },
        "skill_source": str(source / "SKILL.md"),
        "skill_probes": probes,
        "log_paths": log_observations(tool_outputs),
        "expected_paths": {
            "section_5": [
                {
                    "kind": item["id"],
                    "path": item["expected"],
                    "mode": item["mode"],
                    "documented_status": "unverified, pending macos-probe",
                }
                for item in roots
            ]
            + [
                {
                    "kind": "claude-project",
                    "path": "<repo>/.claude/skills",
                    "mode": "copy",
                    "documented_status": "unverified, pending macos-probe",
                    "probe": "not symlinked; spec default is copy",
                },
                {
                    "kind": "custom",
                    "path": "user-given",
                    "mode": "symlink or copy",
                    "documented_status": "unverified, pending macos-probe",
                    "probe": "no default root",
                },
            ],
            "section_7": [item["expected"] for item in log_paths()],
        },
    }
    return report


def any_tool_installed(report: dict) -> bool:
    return any(report["tools"][name].get("binary") for name in report["tools"])


def main() -> int:
    repo = Path(os.environ.get("GITHUB_WORKSPACE", Path.cwd())).resolve()
    dest = repo
    try:
        report = build_report(repo)
    except Exception as exc:
        report = {
            "scope": {
                "authenticated_prompts": "removed",
                "secrets_read": [],
                "skipped_for_missing_secrets": [],
            },
            "runner": {
                "platform": platform.platform(),
                "machine": platform.machine(),
                "home": str(Path.home()),
                "python": sys.version.split()[0],
                "error": redact(str(exc)),
            },
            "tools": {},
            "home_after_commands": snapshot_homes(),
            "created_by_tool": {},
            "skill_probes": [],
            "log_paths": [],
            "fatal_error": redact(str(exc)),
        }
        write_reports(report, dest)
        print(f"probe failed: {exc}", file=sys.stderr)
        return 1
    write_reports(report, dest)
    if not any_tool_installed(report):
        print("no CLI installed", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
