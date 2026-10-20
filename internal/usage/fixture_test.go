package usage_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
)

var errLineTooLong = errors.New("line too long")

const maxLine = 1 << 20

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestNoLiveProjectSkills(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	_, err := os.Stat(filepath.Join(root, ".cursor", "skills"))
	if err == nil {
		t.Fatal(".cursor/skills is present and Cursor would auto-load it")
	}
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	skill := filepath.Join(root, "testdata", "targets", "cursor", "skills", "probe-repo", "SKILL.md")
	body, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("name: probe-repo")) {
		t.Fatalf("probe-repo fixture missing name: %s", skill)
	}
}

func TestForbiddenIdentifiersAbsent(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	// Split so this test source does not itself contain the identifiers.
	forbidden := []string{
		"225a" + "04c0",
		"edc3" + "cff3",
		"toolu_01" + "Dvju",
		"together-chat-" + "completions",
		"ac0e" + "d6aa",
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, needle := range forbidden {
			if bytes.Contains(body, []byte(needle)) {
				t.Errorf("%s contains %s", rel, needle)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCloudTranscriptShape(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	f, err := os.Open(filepath.Join(root, "testdata", "usage", "cursor", "transcript.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("close transcript: %v", err)
		}
	}()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var doc struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Messages) == 0 {
		t.Fatal("messages is empty")
	}
	metaRaw, err := os.ReadFile(filepath.Join(root, "testdata", "usage", "cursor", "transcript.meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		Format        string `json:"format"`
		FormatVersion string `json:"format_version"`
	}
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Format != "cursor-cloud-export" || meta.FormatVersion == "" {
		t.Fatalf("transcript meta = %#v", meta)
	}
	hits := cloudHits(t, doc.Messages, "cloud-export")
	raw := skills(hits)
	if count(raw, "probe-alpha") != 2 {
		t.Fatalf("probe-alpha reads = %d, hits %#v", count(raw, "probe-alpha"), raw)
	}
	if count(raw, "probe-beta") != 1 || count(raw, "poteto-mode") != 1 || count(raw, "probe-repo") != 1 {
		t.Fatalf("skill hits %#v", raw)
	}
	deduped := dedupe(hits)
	if count(skills(deduped), "probe-alpha") != 1 {
		t.Fatal("probe-alpha reads in the same minute did not dedupe")
	}
	for _, h := range hits {
		if h.At.IsZero() {
			t.Fatalf("export read %s missing started_at_ms", h.Skill)
		}
		if h.At.UnixMilli() >= 1_790_000_000_000 {
			t.Fatalf("export timestamp was not synthesized: %d", h.At.UnixMilli())
		}
	}
}

func TestFormatHeaders(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	files := map[string]string{
		"testdata/usage/cursor/desktop-agent-transcript.unverified.jsonl": "cursor-desktop-jsonl",
		"testdata/usage/claude/skill-tool-use.unverified.jsonl":           "claude-code-jsonl",
		"testdata/usage/codex/rollout-sample.unverified.jsonl":            "codex-rollout-jsonl",
	}
	for rel, format := range files {
		row := firstJSON(t, filepath.Join(root, rel))
		if row["fixture_header"] != true {
			t.Fatalf("%s missing fixture_header", rel)
		}
		if row["format"] != format || row["format_version"] == "" || row["provenance"] != "synthetic" {
			t.Fatalf("%s header = %#v", rel, row)
		}
		if row["unverified"] != true {
			t.Fatalf("%s header is not marked unverified", rel)
		}
	}
}

func TestEdgeFixtures(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	usage := filepath.Join(root, "testdata", "usage")

	t.Run("malformed", func(t *testing.T) {
		t.Parallel()
		hits, skipped := scanJSONL(t, filepath.Join(usage, "claude", "edge", "malformed.jsonl"), "claude")
		if skipped < 1 {
			t.Fatal("malformed line was not skipped")
		}
		if count(skills(hits), "kept-skill") != 1 {
			t.Fatalf("hits %#v", skills(hits))
		}
	})
	t.Run("truncated", func(t *testing.T) {
		t.Parallel()
		hits, skipped := scanJSONL(t, filepath.Join(usage, "claude", "edge", "truncated.jsonl"), "claude")
		if skipped < 1 {
			t.Fatal("truncated line was not skipped")
		}
		if count(skills(hits), "kept-skill") != 1 {
			t.Fatalf("hits %#v", skills(hits))
		}
	})
	t.Run("crlf", func(t *testing.T) {
		t.Parallel()
		body, err := os.ReadFile(filepath.Join(usage, "claude", "edge", "crlf.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(body, []byte("\r\n")) {
			t.Fatal("crlf fixture has no CR")
		}
		hits, skipped := scanJSONL(t, filepath.Join(usage, "claude", "edge", "crlf.jsonl"), "claude")
		if skipped != 0 || count(skills(hits), "example-skill") != 1 {
			t.Fatalf("skipped %d hits %#v", skipped, skills(hits))
		}
	})
	t.Run("unicode", func(t *testing.T) {
		t.Parallel()
		hits, skipped := scanJSONL(t, filepath.Join(usage, "claude", "edge", "unicode.jsonl"), "claude")
		if skipped != 0 {
			t.Fatalf("skipped %d", skipped)
		}
		got := skills(hits)
		if count(got, "example-skill") != 1 || len(got) != 1 {
			t.Fatalf("hits %#v", got)
		}
	})
	t.Run("oversize", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(usage, "claude", "edge", "oversize.jsonl")
		if !hasLongLine(t, path, maxLine) {
			t.Fatal("oversize fixture has no line over 1 MiB")
		}
		hits, skipped := scanJSONL(t, path, "claude")
		if skipped < 1 {
			t.Fatal("oversize line was not skipped")
		}
		got := skills(hits)
		if count(got, "kept-skill") != 1 || count(got, "skipped-skill") != 0 {
			t.Fatalf("hits %#v", got)
		}
	})
	t.Run("nested sessions", func(t *testing.T) {
		t.Parallel()
		parent := filepath.Join(usage, "claude", "projects", "-tmp-demo", "11111111-1111-4111-8111-111111111111.jsonl")
		child := filepath.Join(usage, "claude", "projects", "-tmp-demo", "11111111-1111-4111-8111-111111111111", "subagents", "agent-aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa.jsonl")
		other := filepath.Join(usage, "claude", "projects", "-tmp-other", "22222222-2222-4222-8222-222222222222.jsonl")
		ph, _ := scanJSONL(t, parent, "claude")
		ch, _ := scanJSONL(t, child, "claude")
		oh, _ := scanJSONL(t, other, "claude")
		if count(skills(ph), "parent-skill") != 1 || ph[0].Session == ch[0].Session {
			t.Fatalf("parent %#v child %#v", ph, ch)
		}
		if count(skills(ch), "sub-skill") != 1 {
			t.Fatalf("child %#v", ch)
		}
		if count(skills(oh), "other-skill") != 1 || oh[0].Session == ph[0].Session {
			t.Fatalf("other %#v", oh)
		}
	})
	t.Run("non skill path", func(t *testing.T) {
		t.Parallel()
		hits := cloudFile(t, filepath.Join(usage, "cursor", "edge", "non-skill-path.json"))
		if len(hits) != 0 {
			t.Fatalf("hits %#v", hits)
		}
	})
	t.Run("invalid name", func(t *testing.T) {
		t.Parallel()
		hits := cloudFile(t, filepath.Join(usage, "cursor", "edge", "invalid-name.json"))
		if len(hits) != 0 {
			t.Fatalf("hits %#v", hits)
		}
	})
	t.Run("symlink and resolved", func(t *testing.T) {
		t.Parallel()
		hits := cloudFile(t, filepath.Join(usage, "cursor", "edge", "symlink-and-resolved.json"))
		if count(skills(hits), "probe-alpha") != 2 {
			t.Fatalf("hits %#v", hits)
		}
		if count(skills(dedupe(hits)), "probe-alpha") != 1 {
			t.Fatal("symlink and resolved path in one minute did not dedupe")
		}
	})
	t.Run("slash name", func(t *testing.T) {
		t.Parallel()
		hits := cloudFile(t, filepath.Join(usage, "cursor", "edge", "slash-name.json"))
		if len(hits) != 1 || hits[0].Skill != "probe-beta" || hits[0].Kind != "slash" {
			t.Fatalf("hits %#v", hits)
		}
	})
	t.Run("missing timestamps", func(t *testing.T) {
		t.Parallel()
		hits := cloudFile(t, filepath.Join(usage, "cursor", "edge", "missing-timestamps.json"))
		if len(hits) != 1 || hits[0].Skill != "probe-beta" || !hits[0].At.IsZero() {
			t.Fatalf("hits %#v", hits)
		}
	})
	t.Run("unmatched call id", func(t *testing.T) {
		t.Parallel()
		hits, skipped := scanJSONL(t, filepath.Join(usage, "codex", "edge", "unmatched-call-id.jsonl"), "codex")
		if skipped != 0 {
			t.Fatalf("skipped %d", skipped)
		}
		got := skills(hits)
		if count(got, "example-skill") != 1 || count(got, "orphan-skill") != 0 {
			t.Fatalf("hits %#v", got)
		}
	})
	t.Run("desktop sample", func(t *testing.T) {
		t.Parallel()
		hits, skipped := scanJSONL(t, filepath.Join(usage, "cursor", "desktop-agent-transcript.unverified.jsonl"), "cursor-desktop")
		if skipped != 0 {
			t.Fatalf("skipped %d", skipped)
		}
		got := skills(hits)
		if count(got, "probe-alpha") != 1 || count(got, "probe-beta") != 1 {
			t.Fatalf("hits %#v", got)
		}
	})
	t.Run("synthetic claude and codex", func(t *testing.T) {
		t.Parallel()
		ch, _ := scanJSONL(t, filepath.Join(usage, "claude", "skill-tool-use.unverified.jsonl"), "claude")
		xh, _ := scanJSONL(t, filepath.Join(usage, "codex", "rollout-sample.unverified.jsonl"), "codex")
		if count(skills(ch), "example-skill") < 1 {
			t.Fatalf("claude %#v", ch)
		}
		if count(skills(xh), "example-skill") != 1 {
			t.Fatalf("codex %#v", xh)
		}
		layout, skipped := scanJSONL(t, filepath.Join(usage, "codex", "sessions", "2026", "01", "02", "rollout-2026-01-02T03-04-05-66666666.jsonl"), "codex")
		if skipped != 0 || count(skills(layout), "layout-skill") != 1 {
			t.Fatalf("layout skipped %d hits %#v", skipped, layout)
		}
	})
}

type hit struct {
	Skill   string
	Tool    string
	At      time.Time
	Session string
	Kind    string
}

func skills(hits []hit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Skill
	}
	return out
}

func count(names []string, want string) int {
	n := 0
	for _, name := range names {
		if name == want {
			n++
		}
	}
	return n
}

func dedupe(hits []hit) []hit {
	seen := map[string]struct{}{}
	var out []hit
	for _, h := range hits {
		minute := int64(0)
		if !h.At.IsZero() {
			minute = h.At.UTC().Truncate(time.Minute).Unix()
		}
		key := h.Tool + "|" + h.Session + "|" + h.Skill + "|" + itoa(minute)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, h)
	}
	return out
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

func cloudFile(t *testing.T, path string) []hit {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var doc struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	return cloudHits(t, doc.Messages, filepath.Base(path))
}

func cloudHits(t *testing.T, raw []json.RawMessage, session string) []hit {
	t.Helper()
	type call struct {
		name    string
		args    map[string]any
		started *int64
	}
	type msg struct {
		Role      string          `json:"role"`
		Text      string          `json:"text"`
		ToolName  string          `json:"tool_name"`
		ToolID    string          `json:"tool_call_id"`
		ToolArgs  map[string]any  `json:"tool_args"`
		Started   *int64          `json:"started_at_ms"`
		ToolCalls []cloudToolCall `json:"tool_calls"`
	}
	calls := map[string]*call{}
	var order []string
	add := func(id, name string, args map[string]any, started *int64) {
		if id == "" {
			id = "anon-" + itoa(int64(len(order)))
		}
		c, ok := calls[id]
		if !ok {
			c = &call{}
			calls[id] = c
			order = append(order, id)
		}
		if name != "" {
			c.name = name
		}
		if len(args) > 0 {
			c.args = args
		}
		if started != nil {
			c.started = started
		}
	}
	var hits []hit
	for _, row := range raw {
		var m msg
		if err := json.Unmarshal(row, &m); err != nil {
			t.Fatal(err)
		}
		if m.Role == "user" {
			hits = append(hits, slashHits(m.Text, "cursor", session, time.Time{})...)
		}
		for _, tc := range m.ToolCalls {
			add(tc.ID, tc.Name, tc.Args, tc.Started)
		}
		if m.Role == "tool" {
			add(m.ToolID, m.ToolName, m.ToolArgs, m.Started)
		}
	}
	for _, id := range order {
		c := calls[id]
		at := time.Time{}
		if c.started != nil {
			at = time.UnixMilli(*c.started).UTC()
		}
		if name, ok := skillFromPath(stringArg(c.args, "target_file")); ok {
			hits = append(hits, hit{Skill: name, Tool: "cursor", At: at, Session: session, Kind: "read"})
		}
		hits = append(hits, pathHits(stringArg(c.args, "command"), "cursor", session, at)...)
	}
	return hits
}

type cloudToolCall struct {
	ID      string         `json:"tool_call_id"`
	Name    string         `json:"tool_name"`
	Args    map[string]any `json:"tool_args"`
	Started *int64         `json:"started_at_ms"`
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	s, _ := args[key].(string)
	return s
}

func scanJSONL(t *testing.T, path, format string) ([]hit, int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()
	hits, skipped, err := scanReader(f, format, filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	return hits, skipped
}

func scanReader(r io.Reader, format, session string) ([]hit, int, error) {
	br := bufio.NewReader(r)
	var hits []hit
	skipped := 0
	for {
		line, err := readLine(br, maxLine)
		if errors.Is(err, errLineTooLong) {
			skipped++
			if !bytes.Contains(line, []byte("\n")) {
				if serr := skipRestOfLine(br); serr != nil && serr != io.EOF {
					return hits, skipped, serr
				}
			}
			continue
		}
		if err == io.EOF {
			if len(bytes.TrimSpace(line)) == 0 {
				break
			}
		}
		if len(bytes.TrimSpace(line)) == 0 {
			if err == io.EOF {
				break
			}
			continue
		}
		line = bytes.TrimSuffix(line, []byte("\r"))
		rowHits, ok := parseLine(line, format, session)
		if !ok {
			skipped++
		} else {
			hits = append(hits, rowHits...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return hits, skipped, err
		}
	}
	return hits, skipped, nil
}

func readLine(br *bufio.Reader, max int) ([]byte, error) {
	var buf []byte
	for {
		frag, err := br.ReadSlice('\n')
		if len(buf)+len(frag) > max {
			return frag, errLineTooLong
		}
		buf = append(buf, frag...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return buf, err
		}
		return bytes.TrimSuffix(buf, []byte("\n")), nil
	}
}

func skipRestOfLine(br *bufio.Reader) error {
	for {
		frag, err := br.ReadSlice('\n')
		_ = frag
		if err == bufio.ErrBufferFull {
			continue
		}
		return err
	}
}

func parseLine(line []byte, format, fallbackSession string) ([]hit, bool) {
	var head struct {
		FixtureHeader bool   `json:"fixture_header"`
		Type          string `json:"type"`
		Role          string `json:"role"`
	}
	if err := json.Unmarshal(line, &head); err != nil {
		return nil, false
	}
	if head.FixtureHeader {
		return nil, true
	}
	switch format {
	case "claude":
		return parseClaude(line, fallbackSession)
	case "codex":
		return parseCodex(line, fallbackSession)
	case "cursor-desktop":
		return parseDesktop(line, fallbackSession)
	default:
		return nil, false
	}
}

func parseClaude(line []byte, fallback string) ([]hit, bool) {
	var row struct {
		Type      string `json:"type"`
		SessionID string `json:"sessionId"`
		Timestamp string `json:"timestamp"`
		Content   string `json:"content"`
		Message   struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &row); err != nil {
		return nil, false
	}
	session := row.SessionID
	if session == "" {
		session = fallback
	}
	at := parseTime(row.Timestamp)
	var hits []hit
	if row.Content != "" {
		hits = append(hits, commandNameHits(row.Content, "claude", session, at)...)
	}
	if len(row.Message.Content) == 0 {
		return hits, true
	}
	var text string
	if err := json.Unmarshal(row.Message.Content, &text); err == nil {
		hits = append(hits, commandNameHits(text, "claude", session, at)...)
		if row.Type == "user" {
			hits = append(hits, slashHits(text, "claude", session, at)...)
		}
		return hits, true
	}
	var blocks []struct {
		Type  string `json:"type"`
		Name  string `json:"name"`
		Input struct {
			Skill string `json:"skill"`
			Path  string `json:"file_path"`
		} `json:"input"`
	}
	if err := json.Unmarshal(row.Message.Content, &blocks); err != nil {
		return hits, true
	}
	for _, b := range blocks {
		if b.Type == "tool_use" && b.Name == "Skill" {
			if name, err := domain.ParseSkillName(b.Input.Skill); err == nil {
				hits = append(hits, hit{Skill: string(name), Tool: "claude", At: at, Session: session, Kind: "skill-tool"})
			}
		}
		if name, ok := skillFromPath(b.Input.Path); ok {
			hits = append(hits, hit{Skill: name, Tool: "claude", At: at, Session: session, Kind: "read"})
		}
	}
	return hits, true
}

func parseCodex(line []byte, fallback string) ([]hit, bool) {
	var row struct {
		Timestamp string `json:"timestamp"`
		Type      string `json:"type"`
		Payload   struct {
			Type      string `json:"type"`
			ID        string `json:"id"`
			Name      string `json:"name"`
			CallID    string `json:"call_id"`
			Arguments string `json:"arguments"`
			CWD       string `json:"cwd"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(line, &row); err != nil {
		return nil, false
	}
	session := fallback
	if row.Type == "session_meta" && row.Payload.ID != "" {
		session = row.Payload.ID
	}
	at := parseTime(row.Timestamp)
	if row.Payload.Type != "function_call" {
		return nil, true
	}
	return pathHits(row.Payload.Arguments, "codex", session, at), true
}

func parseDesktop(line []byte, session string) ([]hit, bool) {
	var row struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Message struct {
			Content []struct {
				Type  string `json:"type"`
				Text  string `json:"text"`
				Name  string `json:"name"`
				Input struct {
					Path string `json:"path"`
				} `json:"input"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &row); err != nil {
		return nil, false
	}
	if row.Type == "turn_ended" {
		return nil, true
	}
	var hits []hit
	for _, b := range row.Message.Content {
		if row.Role == "user" && b.Type == "text" {
			hits = append(hits, slashHits(b.Text, "cursor", session, time.Time{})...)
		}
		if b.Type == "tool_use" && b.Name == "Read" {
			if name, ok := skillFromPath(b.Input.Path); ok {
				hits = append(hits, hit{Skill: name, Tool: "cursor", At: time.Time{}, Session: session, Kind: "read"})
			}
		}
	}
	return hits, true
}

var (
	commandNameRe = regexp.MustCompile(`<command-name>/([a-z0-9][a-z0-9-]{0,63})</command-name>`)
	slashRe       = regexp.MustCompile(`(?:^|\s)/([a-z0-9][a-z0-9-]{0,63})(?:\s|$)`)
	skillPathRe   = regexp.MustCompile(`(?:^|[\s"'=])(\S*/(?:\.skills|skills)/[A-Za-z0-9._-]{1,64}/SKILL\.md)`)
)

func commandNameHits(s, tool, session string, at time.Time) []hit {
	var hits []hit
	for _, m := range commandNameRe.FindAllStringSubmatch(s, -1) {
		name, err := domain.ParseSkillName(m[1])
		if err != nil {
			continue
		}
		hits = append(hits, hit{Skill: string(name), Tool: tool, At: at, Session: session, Kind: "slash"})
	}
	return hits
}

func slashHits(s, tool, session string, at time.Time) []hit {
	var hits []hit
	for _, m := range slashRe.FindAllStringSubmatch(s, -1) {
		name, err := domain.ParseSkillName(m[1])
		if err != nil {
			continue
		}
		hits = append(hits, hit{Skill: string(name), Tool: tool, At: at, Session: session, Kind: "slash"})
	}
	return hits
}

func pathHits(s, tool, session string, at time.Time) []hit {
	var hits []hit
	for _, m := range skillPathRe.FindAllStringSubmatch(s, -1) {
		if name, ok := skillFromPath(m[1]); ok {
			hits = append(hits, hit{Skill: name, Tool: tool, At: at, Session: session, Kind: "read"})
		}
	}
	return hits
}

func skillFromPath(p string) (string, bool) {
	if p == "" || filepath.Base(p) != "SKILL.md" {
		return "", false
	}
	clean := filepath.Clean(p)
	if filepath.Base(clean) != "SKILL.md" {
		return "", false
	}
	name := filepath.Base(filepath.Dir(clean))
	parent := filepath.Base(filepath.Dir(filepath.Dir(clean)))
	if parent != "skills" && parent != ".skills" {
		return "", false
	}
	parsed, err := domain.ParseSkillName(name)
	if err != nil {
		return "", false
	}
	return string(parsed), true
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func firstJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()
	br := bufio.NewReader(f)
	line, err := br.ReadBytes('\n')
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(line), &row); err != nil {
		t.Fatal(err)
	}
	return row
}

func hasLongLine(t *testing.T, path string, min int) bool {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()
	br := bufio.NewReader(f)
	n := 0
	for {
		frag, err := br.ReadSlice('\n')
		n += len(frag)
		if n > min {
			return true
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err == io.EOF {
			return false
		}
		if err != nil {
			t.Fatal(err)
		}
		n = 0
	}
}
