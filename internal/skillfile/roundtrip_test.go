package skillfile

import (
	"bytes"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDescriptionLeadingNewlineRoundTrip(t *testing.T) {
	n, d, m, b, err := Parse([]byte("---\nname: a\ndescription: |\n\n 0\n---"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := Serialize(n, d, m, b)
	if err != nil {
		t.Fatal(err)
	}
	_, d2, _, _, err := Parse(out)
	if err != nil || d2 != d {
		t.Fatalf("%q -> %q (%v)", d, d2, err)
	}
}

func FuzzRoundTrip(f *testing.F) {
	f.Add([]byte("---\nname: a\ndescription: |\n\n 0\n---"))
	f.Add([]byte("---\nname: a\ndescription: d\n---\nbody\n"))
	f.Add([]byte("---\nname: demo\ndescription: Says hello\nauthor: ada\nnote: \"true\"\n---\n# Hello\n"))
	f.Add([]byte("---\nname: a\ndescription: d\nx:\n  1: one\n  \"1\": two\n---\n"))
	f.Add([]byte("---\nname: a\ndescription: d\ncreated: 2024-01-01\nbase: &b {k: 1}\nuse: *b\n---\n"))
	f.Add([]byte("---\nname: a\ndescription: \"  hi  \"\n---\n"))
	f.Fuzz(func(t *testing.T, in []byte) {
		n, d, m, b, err := Parse(in)
		if err != nil {
			return
		}
		out, err := Serialize(n, d, m, b)
		if err != nil {
			t.Fatalf("serialize: %v", err)
		}
		n2, d2, m2, _, err := Parse(out)
		if err != nil {
			t.Fatalf("reparse: %v\n%s", err, out)
		}
		if n2 != n || d2 != d {
			t.Fatalf("name %q -> %q, description %q -> %q", n, n2, d, d2)
		}
		if err := sameMeta(m, m2); err != nil {
			t.Fatal(err)
		}
	})
}

func sameMeta(a, b *yaml.Node) error {
	ab, err := marshalMeta(a)
	if err != nil {
		return err
	}
	bb, err := marshalMeta(b)
	if err != nil {
		return err
	}
	if !bytes.Equal(ab, bb) {
		return &metaMismatch{left: ab, right: bb}
	}
	return nil
}

func marshalMeta(n *yaml.Node) ([]byte, error) {
	if n == nil {
		return nil, nil
	}
	return yaml.Marshal(n)
}

type metaMismatch struct {
	left, right []byte
}

func (e *metaMismatch) Error() string {
	return "unknown keys changed:\n" + string(e.left) + "\n---\n" + string(e.right)
}
