package skillfile

import (
	"errors"
	"strings"
	"testing"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
)

func rt3(src string) (string, error) {
	n, d, m, b, err := Parse([]byte(src))
	if err != nil {
		return "", err
	}
	out, err := Serialize(n, d, m, b)
	if err != nil {
		return "", err
	}
	_, _, _, _, err = Parse(out)
	return string(out), err
}

func TestAudit3Probes(t *testing.T) {
	h := "---\nname: a\ndescription: d\n"
	cases := []struct {
		label, src string
		wantOK     bool
	}{
		{"meta anchors", h + "base: &b {k: 1}\nuse: *b\n---\n", true},
		{"nested merge", h + "base: &b {k: 1}\nx:\n  <<: *b\n  j: 2\n---\n", false},
		{"top merge", h + "base: &b {k: 1}\n<<: *b\n---\n", false},
		{"merge smuggles name", "---\nbase: &b {name: evil}\n<<: *b\nname: a\ndescription: d\n---\n", false},
		{"quoted name key", "---\n\"name\": a\ndescription: d\n---\n", true},
		{"quoted dup name", h + "'name': b\n---\n", false},
		{"!!str name value", "---\nname: !!str a\ndescription: d\n---\n", false},
		{"!!str name key", "---\n!!str name: a\ndescription: d\n---\n", false},
		{"int vs str nested keys", h + "x:\n  1: a\n  \"1\": b\n---\n", true},
		{"anchor on name", "---\nname: &n a\ndescription: d\nz: *n\n---\n", false},
		{"custom tag key nested", h + "x:\n  !foo k: 1\n---\n", false},
		{"binary nested", h + "x: !!binary aGk=\n---\n", true},
		{"non-scalar key", h + "? [a]: 1\n---\n", false},
		{"anchor key", h + "&k z: 1\n---\n", false},
		{"alias key", h + "z: &k 1\n*k: 2\n---\n", false},
	}
	for _, c := range cases {
		_, err := rt3(c.src)
		if (err == nil) != c.wantOK {
			t.Errorf("%s: wantOK=%v err=%v", c.label, c.wantOK, err)
		}
		if strings.Contains(c.label, "merge") && (!errors.Is(err, domain.ErrInvalidFrontmatter) || !strings.Contains(err.Error(), "merge key")) {
			t.Errorf("%s: err=%v, want invalid frontmatter naming a merge key", c.label, err)
		}
	}
}
