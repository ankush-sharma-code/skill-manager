package skillfile_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io/fs"
	"reflect"
	"testing"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/skillfile"
)

func TestParseFrontmatter(t *testing.T) {
	t.Parallel()

	valid := []byte("---\nname: demo\ndescription: Says hello\n---\n# Hello\n")
	tests := []struct {
		name    string
		in      []byte
		wantErr error
		want    string
		desc    string
		body    string
	}{
		{name: "basic", in: valid, want: "demo", desc: "Says hello", body: "# Hello\n"},
		{
			name: "crlf",
			in:   []byte("---\r\nname: demo\r\ndescription: Says hello\r\n---\r\n# Hello\r\n"),
			want: "demo",
			desc: "Says hello",
			body: "# Hello\r\n",
		},
		{
			name: "bom",
			in:   append([]byte{0xEF, 0xBB, 0xBF}, valid...),
			want: "demo",
			desc: "Says hello",
			body: "# Hello\n",
		},
		{
			name: "bom and crlf",
			in:   append([]byte{0xEF, 0xBB, 0xBF}, []byte("---\r\nname: demo\r\ndescription: Says hello\r\n---\r\nbody\r\n")...),
			want: "demo",
			desc: "Says hello",
			body: "body\r\n",
		},
		{name: "missing opening", in: []byte("name: demo\ndescription: Says hello\n"), wantErr: domain.ErrMissingFrontmatter},
		{name: "missing closing", in: []byte("---\nname: demo\ndescription: Says hello\n"), wantErr: domain.ErrMissingFrontmatter},
		{name: "missing name", in: []byte("---\ndescription: Says hello\n---\n"), wantErr: domain.ErrMissingName},
		{name: "missing description", in: []byte("---\nname: demo\n---\n"), wantErr: domain.ErrMissingDescription},
		{name: "empty description", in: []byte("---\nname: demo\ndescription:\n---\n"), wantErr: domain.ErrMissingDescription},
		{name: "null description", in: []byte("---\nname: demo\ndescription: null\n---\n"), wantErr: domain.ErrMissingDescription},
		{name: "uppercase name", in: []byte("---\nname: Demo\ndescription: Says hello\n---\n"), wantErr: domain.ErrInvalidName},
		{name: "bad yaml", in: []byte("---\nname: [\n---\n"), wantErr: domain.ErrInvalidFrontmatter},
		{name: "description too long", in: tooLongDescription(), wantErr: domain.ErrDescriptionLength},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			name, desc, _, body, err := skillfile.Parse(tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Parse error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse unexpected error: %v", err)
			}
			if string(name) != tt.want || desc != tt.desc || string(body) != tt.body {
				t.Fatalf("Parse = (%q, %q, %q), want (%q, %q, %q)", name, desc, body, tt.want, tt.desc, tt.body)
			}
		})
	}
}

func TestUnknownKeysRoundTrip(t *testing.T) {
	t.Parallel()

	raw := []byte(`---
name: demo
description: Says hello
author: ada
note: "true"
tags:
  - a
  - b
config:
  retries: 2
  enabled: true
---
# Hello

do the thing
`)

	name, desc, meta, body, err := skillfile.Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if string(name) != "demo" || desc != "Says hello" {
		t.Fatalf("identity = %q %q", name, desc)
	}
	if _, ok := meta["name"]; ok {
		t.Fatal("meta contains name")
	}
	if _, ok := meta["description"]; ok {
		t.Fatal("meta contains description")
	}
	want := map[string]any{
		"author": "ada",
		"note":   "true",
		"tags":   []any{"a", "b"},
		"config": map[string]any{"retries": 2, "enabled": true},
	}
	if !reflect.DeepEqual(meta, want) {
		t.Fatalf("meta = %#v", meta)
	}

	out, err := skillfile.Serialize(name, desc, meta, body)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	name2, desc2, meta2, body2, err := skillfile.Parse(out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if name2 != name || desc2 != desc || string(body2) != string(body) || !reflect.DeepEqual(meta2, meta) {
		t.Fatalf("round trip = (%q, %q, %#v, %q)", name2, desc2, meta2, body2)
	}
}

func TestLoadNameMustMatchDirectory(t *testing.T) {
	t.Parallel()

	files := []skillfile.File{{
		RelPath: "SKILL.md",
		Mode:    0o644,
		Content: []byte("---\nname: beta\ndescription: Says hello\n---\n"),
	}}
	_, err := skillfile.Load("alpha", files)
	if !errors.Is(err, domain.ErrNameMismatch) {
		t.Fatalf("Load error = %v, want %v", err, domain.ErrNameMismatch)
	}

	missing := []skillfile.File{{
		RelPath: "README.md",
		Mode:    0o644,
		Content: []byte("nope"),
	}}
	_, err = skillfile.Load("alpha", missing)
	if !errors.Is(err, domain.ErrMissingSkillMD) {
		t.Fatalf("Load missing SKILL.md error = %v", err)
	}
}

func TestTreeDigestStable(t *testing.T) {
	t.Parallel()

	skill := skillfile.File{RelPath: "SKILL.md", Mode: 0o644, Content: []byte("a"), ModTime: time.Unix(1, 0)}
	run := skillfile.File{RelPath: "bin/run", Mode: 0o755, Content: []byte("#!/bin/sh\n"), ModTime: time.Unix(2, 0)}
	notes := skillfile.File{RelPath: "notes.txt", Mode: 0o644, Content: []byte("n"), ModTime: time.Unix(3, 0)}

	first, err := skillfile.TreeDigest([]skillfile.File{notes, skill, run})
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	second, err := skillfile.TreeDigest([]skillfile.File{run, notes, skill})
	if err != nil {
		t.Fatalf("digest reordered: %v", err)
	}
	if first != second {
		t.Fatalf("order changed digest: %x vs %x", first, second)
	}

	laterSkill := skill
	laterSkill.ModTime = time.Unix(9_000, 0)
	laterRun := run
	laterRun.ModTime = time.Unix(1, 0)
	laterNotes := notes
	laterNotes.ModTime = time.Time{}
	third, err := skillfile.TreeDigest([]skillfile.File{laterSkill, laterRun, laterNotes})
	if err != nil {
		t.Fatalf("digest mtime: %v", err)
	}
	if third != first {
		t.Fatalf("mtime changed digest: %x vs %x", third, first)
	}

	// Golden listing: sorted relpath, 4-digit perm, lowercase sha256 of content, NUL separators.
	const want = "cf792702494b5a16bb7f5adf66d0b6451f07e49750719b01182eda72de5dcbd3"
	if got := hexDigest(first); got != want {
		t.Fatalf("digest = %s, want %s", got, want)
	}

	changed := notes
	changed.Content = []byte("changed")
	contentDigest, err := skillfile.TreeDigest([]skillfile.File{skill, run, changed})
	if err != nil {
		t.Fatalf("digest content: %v", err)
	}
	if contentDigest == first {
		t.Fatal("content change left the digest unchanged")
	}

	plain := run
	plain.Mode = 0o644
	modeDigest, err := skillfile.TreeDigest([]skillfile.File{skill, plain, notes})
	if err != nil {
		t.Fatalf("digest mode: %v", err)
	}
	if modeDigest == first {
		t.Fatal("exec bit change left the digest unchanged")
	}
	if modeDigest == contentDigest {
		t.Fatal("mode change collided with the content change")
	}

	withDir := []skillfile.File{skill, run, notes, {
		RelPath: "bin",
		Mode:    fs.ModeDir | 0o755,
		ModTime: time.Unix(4, 0),
	}}
	dirDigest, err := skillfile.TreeDigest(withDir)
	if err != nil {
		t.Fatalf("digest dir: %v", err)
	}
	if dirDigest != first {
		t.Fatal("directory entry changed the digest")
	}
}

func TestLoadDigestMatchesTree(t *testing.T) {
	t.Parallel()

	files := []skillfile.File{
		{
			RelPath: "SKILL.md",
			Mode:    0o644,
			Content: []byte("---\nname: demo\ndescription: Says hello\n---\nbody\n"),
			ModTime: time.Unix(10, 0),
		},
		{
			RelPath: "bin/run",
			Mode:    0o755,
			Content: []byte("#!/bin/sh\n"),
			ModTime: time.Unix(11, 0),
		},
	}
	got, err := skillfile.Load("demo", files)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want, err := skillfile.TreeDigest(files)
	if err != nil {
		t.Fatalf("TreeDigest: %v", err)
	}
	if got.Digest != want {
		t.Fatalf("skill digest %x, tree %x", got.Digest, want)
	}
	if string(got.Name) != "demo" || got.Description != "Says hello" {
		t.Fatalf("skill identity = %q %q", got.Name, got.Description)
	}
	if len(got.Files) != 2 {
		t.Fatalf("files = %#v", got.Files)
	}
	if got.Files[0].RelPath != "SKILL.md" || got.Files[1].RelPath != "bin/run" {
		t.Fatalf("file order = %s %s", got.Files[0].RelPath, got.Files[1].RelPath)
	}
	if got.Files[1].Mode.Perm() != 0o755 {
		t.Fatalf("exec mode = %o", got.Files[1].Mode.Perm())
	}
	sum := sha256.Sum256(files[1].Content)
	if got.Files[1].Digest != sum {
		t.Fatalf("file digest = %x, want %x", got.Files[1].Digest, sum)
	}
	if got.Files[0].Size != int64(len(files[0].Content)) {
		t.Fatalf("size = %d", got.Files[0].Size)
	}
}

func TestTreeDigestRejectsBadPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want error
	}{
		{name: "dotdot", path: "../secret", want: skillfile.ErrBadPath},
		{name: "absolute", path: "/etc/passwd", want: skillfile.ErrBadPath},
		{name: "empty", path: "", want: skillfile.ErrBadPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := skillfile.TreeDigest([]skillfile.File{{
				RelPath: tt.path,
				Mode:    0o644,
				Content: []byte("x"),
			}})
			if !errors.Is(err, tt.want) {
				t.Fatalf("TreeDigest error = %v, want %v", err, tt.want)
			}
		})
	}

	_, err := skillfile.TreeDigest([]skillfile.File{
		{RelPath: "a.txt", Mode: 0o644, Content: []byte("1")},
		{RelPath: "a.txt", Mode: 0o644, Content: []byte("2")},
	})
	if !errors.Is(err, skillfile.ErrDuplicatePath) {
		t.Fatalf("duplicate error = %v", err)
	}
}

func tooLongDescription() []byte {
	var b bytes.Buffer
	b.WriteString("---\nname: demo\ndescription: ")
	for i := 0; i < 1025; i++ {
		b.WriteString("x")
	}
	b.WriteString("\n---\n")
	return b.Bytes()
}

func hexDigest(d domain.Digest) string {
	const hexdigits = "0123456789abcdef"
	buf := make([]byte, 64)
	for i, b := range d {
		buf[i*2] = hexdigits[b>>4]
		buf[i*2+1] = hexdigits[b&0x0f]
	}
	return string(buf)
}
