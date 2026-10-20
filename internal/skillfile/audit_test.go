package skillfile_test

import (
	"bytes"
	"errors"
	"io/fs"
	"testing"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/skillfile"
	"gopkg.in/yaml.v3"
)

func TestAuditNames(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in string
		ok bool
	}{
		{"", false}, {"..", false}, {"a/b", false}, {`a\b`, false}, {"Abc", false},
		{"аbc", false},
		{"ａbc", false}, {"a\u200bb", false}, {"abc\n", false}, {"-abc", false},
		{"abc-", true},
		{string(bytes.Repeat([]byte("a"), 64)), true},
		{string(bytes.Repeat([]byte("a"), 65)), false},
		{"a b", false}, {"a.b", false}, {"a_b", false},
	}
	for _, c := range cases {
		_, err := domain.ParseSkillName(c.in)
		if (err == nil) != c.ok {
			t.Errorf("%q: ok=%v err=%v", c.in, c.ok, err)
		}
	}
}

func TestAuditDuplicateKeysRejected(t *testing.T) {
	t.Parallel()

	cases := []string{
		"---\nname: good\ndescription: d\nname: evil\n---\n",
		"---\nname: good\ndescription: d\ndescription: other\n---\n",
	}
	for _, in := range cases {
		_, _, _, _, err := skillfile.Parse([]byte(in))
		if !errors.Is(err, domain.ErrInvalidFrontmatter) {
			t.Fatalf("%q error = %v, want %v", in, err, domain.ErrInvalidFrontmatter)
		}
	}
}

func TestAuditBOMCRLF(t *testing.T) {
	t.Parallel()

	for _, in := range []string{
		"\ufeff---\r\nname: a\r\ndescription: d\r\n---\r\nbody\r\n",
		"---\r\nname: a\r\ndescription: d\r\n---",
	} {
		n, d, _, _, err := skillfile.Parse([]byte(in))
		if err != nil || n != "a" || d != "d" {
			t.Errorf("%q: %v %v %v", in, n, d, err)
		}
	}
}

func TestAuditMetaRoundTrip(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"timestamp":     "created: 2024-01-01\n",
		"nonstring key": "x:\n  1: one\n",
		"big int":       "n: 18446744073709551616\n",
		"nested list":   "tags: [a, {b: 1}]\n",
	}
	for label, extra := range cases {
		src := "---\nname: a\ndescription: d\n" + extra + "---\nbody\n"
		n, d, meta, body, err := skillfile.Parse([]byte(src))
		if err != nil {
			t.Errorf("%s: parse: %v", label, err)
			continue
		}
		out, err := skillfile.Serialize(n, d, meta, body)
		if err != nil {
			t.Errorf("%s: serialize failed after successful parse: %v", label, err)
			continue
		}
		if _, _, _, _, err := skillfile.Parse(out); err != nil {
			t.Errorf("%s: reparse: %v", label, err)
		}
	}
}

func TestUnknownKeysKeepOrderQuotesAndComments(t *testing.T) {
	t.Parallel()

	src := []byte("---\nname: a\ndescription: d\n# keep\nzed: 1\nauthor: \"Ada\"\n---\nbody\n")
	name, desc, meta, body, err := skillfile.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := skillfile.Serialize(name, desc, meta, body)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	if !bytes.Contains(out, []byte("# keep")) {
		t.Fatalf("comment lost:\n%s", out)
	}
	if !bytes.Contains(out, []byte(`author: "Ada"`)) {
		t.Fatalf("quoting lost:\n%s", out)
	}
	zed := bytes.Index(out, []byte("zed:"))
	author := bytes.Index(out, []byte("author:"))
	if zed < 0 || author < 0 || zed > author {
		t.Fatalf("key order lost:\n%s", out)
	}
	if !bytes.Equal(body, []byte("body\n")) {
		t.Fatalf("body = %q", body)
	}
}

func TestAuditSymlinkVsFileDigest(t *testing.T) {
	t.Parallel()

	md := []byte("---\nname: a\ndescription: d\n---\n")
	a, err := skillfile.TreeDigest([]skillfile.File{
		{RelPath: "SKILL.md", Mode: 0o644, Content: md},
		{RelPath: "x", Mode: 0o644, Content: []byte("t")},
	})
	if err != nil {
		t.Fatalf("file digest: %v", err)
	}
	b, err := skillfile.TreeDigest([]skillfile.File{
		{RelPath: "SKILL.md", Mode: 0o644, Content: md},
		{RelPath: "x", Mode: fs.ModeSymlink | 0o644, Content: []byte("t")},
	})
	if err != nil {
		t.Fatalf("symlink digest: %v", err)
	}
	if a == b {
		t.Fatal("symlink and regular file with same bytes produce identical digest")
	}
}

func TestAuditPaths(t *testing.T) {
	t.Parallel()

	for _, p := range []string{"../x", "/x", "a/../../x", `a\b`, "a\x00b", "."} {
		_, err := skillfile.TreeDigest([]skillfile.File{{RelPath: p, Mode: 0o644}})
		if !errors.Is(err, skillfile.ErrBadPath) {
			t.Errorf("%q accepted: %v", p, err)
		}
	}
	_, err := skillfile.TreeDigest([]skillfile.File{
		{RelPath: "a/b", Mode: 0o644},
		{RelPath: "a/./b", Mode: 0o644},
	})
	if !errors.Is(err, skillfile.ErrDuplicatePath) {
		t.Errorf("aliased path accepted: %v", err)
	}
}

func TestAuditDescriptionWhitespace(t *testing.T) {
	t.Parallel()

	_, _, _, _, err := skillfile.Parse([]byte("---\nname: a\ndescription: \"   \"\n---\n"))
	if !errors.Is(err, domain.ErrDescriptionLength) {
		t.Fatalf("whitespace description error = %v", err)
	}
}

func TestSerializeRejectsBadInputs(t *testing.T) {
	t.Parallel()

	_, err := skillfile.Serialize("NOPE", "ok", nil, nil)
	if !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("bad name error = %v", err)
	}
	_, err = skillfile.Serialize("ok", "   ", nil, nil)
	if !errors.Is(err, domain.ErrDescriptionLength) {
		t.Fatalf("whitespace description error = %v", err)
	}
	_, err = skillfile.Serialize("ok", "d", &yaml.Node{Kind: yaml.ScalarNode, Value: "nope"}, nil)
	if !errors.Is(err, domain.ErrInvalidFrontmatter) {
		t.Fatalf("bad meta error = %v", err)
	}
	out, err := skillfile.Serialize("ok", "d", nil, []byte("body\n"))
	if err != nil {
		t.Fatalf("nil meta: %v", err)
	}
	if !bytes.Contains(out, []byte("name: ok\n")) || !bytes.Contains(out, []byte("body\n")) {
		t.Fatalf("nil meta output:\n%s", out)
	}
}
