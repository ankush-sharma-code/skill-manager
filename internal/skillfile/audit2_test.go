package skillfile_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/skillfile"
)

func TestAudit2RejectsUnsafeFrontmatter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
	}{
		{name: "anchor on name", in: "---\nname: &n a\ndescription: d\nalias: *n\n---\n"},
		{name: "anchor on description", in: "---\nname: a\ndescription: &d hello\n---\n"},
		{name: "explicit tag on name", in: "---\nname: !!str a\ndescription: d\n---\n"},
		{name: "custom tag on description", in: "---\nname: a\ndescription: !custom hello\n---\n"},
		{name: "anchor on key", in: "---\n&k name: a\ndescription: d\n---\n"},
		{name: "alias key", in: "---\nx: &k 1\n*k: 2\nname: a\ndescription: d\n---\n"},
		{name: "custom tagged key", in: "---\nname: a\ndescription: d\n!custom name: evil\n---\n"},
		{name: "quoted duplicate name", in: "---\nname: a\ndescription: d\n\"name\": evil\n---\n"},
		{name: "duplicate unknown", in: "---\nname: a\ndescription: d\nx: 1\nx: 2\n---\n"},
		{name: "nested duplicate", in: "---\nname: a\ndescription: d\nx:\n  a: 1\n  a: 2\n---\n"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, _, _, err := skillfile.Parse([]byte(tt.in))
			if !errors.Is(err, domain.ErrInvalidFrontmatter) {
				t.Fatalf("Parse error = %v, want %v", err, domain.ErrInvalidFrontmatter)
			}
		})
	}
}

func TestAudit2ScalarRoundTrip(t *testing.T) {
	t.Parallel()

	for _, desc := range []string{"yes", "123", "a: b", "line1\nline2", "#x", "null", "- x", "'q'", " lead"} {
		name, _, _, _, err := skillfile.Parse([]byte("---\nname: a\ndescription: x\n---\n"))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		out, err := skillfile.Serialize(name, desc, nil, nil)
		if err != nil {
			t.Errorf("%q: %v", desc, err)
			continue
		}
		_, got, _, _, err := skillfile.Parse(out)
		if err != nil || got != desc {
			t.Errorf("%q -> %q, %v", desc, got, err)
		}
	}
}

func TestAudit2SafeFormsReparse(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"quoting":   "---\nname: a\ndescription: d\ns: 'single'\nq: \"dbl\"\n---\nbody\n",
		"order":     "---\nname: a\ndescription: d\nz: 1\na: 2\nm: 3\n---\nbody\n",
		"timestamp": "---\nname: a\ndescription: d\ncreated: 2024-01-01\n---\nbody\n",
		"nested":    "---\nname: a\ndescription: d\nx:\n    1: one\n    y: [a, {b: 1}]\n---\nbody\n",
		"anchor":    "---\nname: a\ndescription: d\nbase: &b {k: 1}\nuse: *b\n---\nbody\n",
		"flow":      "---\nname: a\ndescription: d\ntags: [a,b]\n---\nbody\n",
	}
	for label, src := range cases {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			out, err := roundTrip(t, src)
			if err != nil {
				t.Fatalf("round trip: %v", err)
			}
			if _, err := roundTrip(t, out); err != nil {
				t.Fatalf("reparse: %v\n%s", err, out)
			}
		})
	}
}

func TestSerializeDropsNameLineComments(t *testing.T) {
	t.Parallel()

	src := "---\n# top\nname: a # keep\ndescription: d\ntags: [a,b]\n---\nbody\n"
	out, err := roundTrip(t, src)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if bytes.Contains([]byte(out), []byte("# top")) || bytes.Contains([]byte(out), []byte("# keep")) {
		t.Fatalf("comments on the name line were kept:\n%s", out)
	}
	if _, err := roundTrip(t, out); err != nil {
		t.Fatalf("reparse: %v\n%s", err, out)
	}
}

func roundTrip(t *testing.T, src string) (string, error) {
	t.Helper()
	name, desc, meta, body, err := skillfile.Parse([]byte(src))
	if err != nil {
		return "", err
	}
	out, err := skillfile.Serialize(name, desc, meta, body)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
