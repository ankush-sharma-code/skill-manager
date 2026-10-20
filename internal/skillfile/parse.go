package skillfile

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"gopkg.in/yaml.v3"
)

// Parse reads SKILL.md bytes into a name, description, remaining frontmatter, and body.
func Parse(skillMD []byte) (domain.SkillName, string, map[string]any, []byte, error) {
	raw, body, err := splitFrontmatter(skillMD)
	if err != nil {
		return "", "", nil, nil, err
	}
	name, description, meta, err := decodeFrontmatter(raw)
	if err != nil {
		return "", "", nil, nil, err
	}
	skillName, err := domain.ParseSkillName(name)
	if err != nil {
		return "", "", nil, nil, err
	}
	if err := domain.ValidateDescription(description); err != nil {
		return "", "", nil, nil, err
	}
	return skillName, description, meta, body, nil
}

// Serialize writes canonical frontmatter and the body.
// Keys are name, description, then remaining keys in byte order.
func Serialize(name domain.SkillName, description string, meta map[string]any, body []byte) ([]byte, error) {
	if _, err := domain.ParseSkillName(string(name)); err != nil {
		return nil, err
	}
	if err := domain.ValidateDescription(description); err != nil {
		return nil, err
	}
	front, err := marshalFrontmatter(string(name), description, meta)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(front)
	if len(front) == 0 || front[len(front)-1] != '\n' {
		buf.WriteByte('\n')
	}
	buf.WriteString("---\n")
	buf.Write(body)
	return buf.Bytes(), nil
}

func splitFrontmatter(raw []byte) (front, body []byte, err error) {
	b := bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	line, rest, ok := nextLine(b)
	if !ok || string(line) != "---" {
		return nil, nil, domain.ErrMissingFrontmatter
	}
	var yamlBytes bytes.Buffer
	for {
		line, next, ok := nextLine(rest)
		if !ok {
			return nil, nil, domain.ErrMissingFrontmatter
		}
		if string(line) == "---" {
			return yamlBytes.Bytes(), next, nil
		}
		yamlBytes.Write(line)
		yamlBytes.WriteByte('\n')
		rest = next
	}
}

func nextLine(b []byte) (line, rest []byte, ok bool) {
	if len(b) == 0 {
		return nil, nil, false
	}
	i := bytes.IndexByte(b, '\n')
	if i < 0 {
		return bytes.TrimSuffix(b, []byte{'\r'}), nil, true
	}
	return bytes.TrimSuffix(b[:i], []byte{'\r'}), b[i+1:], true
}

func decodeFrontmatter(raw []byte) (string, string, map[string]any, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return "", "", nil, fmt.Errorf("%w: %v", domain.ErrInvalidFrontmatter, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return "", "", nil, domain.ErrInvalidFrontmatter
	}
	root := doc.Content[0]
	if len(root.Content)%2 != 0 {
		return "", "", nil, domain.ErrInvalidFrontmatter
	}

	var (
		name        string
		description string
		nameSet     bool
		descSet     bool
	)
	metaNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for i := 0; i < len(root.Content); i += 2 {
		key := root.Content[i]
		val := root.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			return "", "", nil, domain.ErrInvalidFrontmatter
		}
		switch key.Value {
		case "name":
			s, present, err := yamlString(val)
			if err != nil {
				return "", "", nil, err
			}
			if !present {
				return "", "", nil, domain.ErrMissingName
			}
			name = s
			nameSet = true
		case "description":
			s, present, err := yamlString(val)
			if err != nil {
				return "", "", nil, err
			}
			if !present {
				return "", "", nil, domain.ErrMissingDescription
			}
			description = s
			descSet = true
		default:
			metaNode.Content = append(metaNode.Content, key, val)
		}
	}
	if !nameSet {
		return "", "", nil, domain.ErrMissingName
	}
	if !descSet {
		return "", "", nil, domain.ErrMissingDescription
	}
	if len(metaNode.Content) == 0 {
		return name, description, nil, nil
	}
	var meta map[string]any
	if err := metaNode.Decode(&meta); err != nil {
		return "", "", nil, fmt.Errorf("%w: %v", domain.ErrInvalidFrontmatter, err)
	}
	return name, description, meta, nil
}

func yamlString(v *yaml.Node) (string, bool, error) {
	if v.Tag == "!!null" {
		return "", false, nil
	}
	if v.Kind != yaml.ScalarNode || v.Tag != "!!str" {
		return "", false, domain.ErrInvalidFrontmatter
	}
	return v.Value, true, nil
}

func marshalFrontmatter(name, description string, meta map[string]any) ([]byte, error) {
	root := &yaml.Node{Kind: yaml.MappingNode}
	if err := appendPair(root, "name", name); err != nil {
		return nil, err
	}
	if err := appendPair(root, "description", description); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(meta))
	for k := range meta {
		if k == "name" || k == "description" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := appendPair(root, k, meta[k]); err != nil {
			return nil, err
		}
	}
	out, err := yaml.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidFrontmatter, err)
	}
	return out, nil
}

func appendPair(m *yaml.Node, key string, value any) error {
	kn := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	vn, err := encodeValue(value)
	if err != nil {
		return err
	}
	m.Content = append(m.Content, kn, vn)
	return nil
}

func encodeValue(v any) (*yaml.Node, error) {
	switch t := v.(type) {
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: t, Style: yaml.DoubleQuotedStyle}, nil
	case bool:
		val := "false"
		if t {
			val = "true"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: val}, nil
	case int:
		return intNode(int64(t)), nil
	case int64:
		return intNode(t), nil
	case uint64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.FormatUint(t, 10)}, nil
	case float64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: strconv.FormatFloat(t, 'f', -1, 64)}, nil
	case map[string]any:
		n := &yaml.Node{Kind: yaml.MappingNode}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if err := appendPair(n, k, t[k]); err != nil {
				return nil, err
			}
		}
		return n, nil
	case []any:
		n := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range t {
			en, err := encodeValue(item)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, en)
		}
		return n, nil
	default:
		return nil, fmt.Errorf("%w: unsupported frontmatter value %T", domain.ErrInvalidFrontmatter, v)
	}
}

func intNode(n int64) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.FormatInt(n, 10)}
}
