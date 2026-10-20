package skillfile

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"gopkg.in/yaml.v3"
)

// Parse reads SKILL.md bytes into a name, description, remaining frontmatter, and body.
// The returned node is the mapping of every key except name and description, in file order.
func Parse(skillMD []byte) (domain.SkillName, string, *yaml.Node, []byte, error) {
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

// Serialize writes name, description, then the unknown-key node, then the body.
// The unknown-key node is emitted as stored, including order, comments, and scalar style.
func Serialize(name domain.SkillName, description string, meta *yaml.Node, body []byte) ([]byte, error) {
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

func decodeFrontmatter(raw []byte) (string, string, *yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return "", "", nil, fmt.Errorf("%w: %v", domain.ErrInvalidFrontmatter, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return "", "", nil, domain.ErrInvalidFrontmatter
	}
	root := doc.Content[0]
	if err := validateTree(root, true); err != nil {
		return "", "", nil, err
	}

	var (
		name        string
		description string
		nameSet     bool
		descSet     bool
	)
	meta := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for i := 0; i < len(root.Content); i += 2 {
		key := root.Content[i]
		val := root.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			meta.Content = append(meta.Content, key, val)
			continue
		}
		switch key.Value {
		case "name":
			if nameSet {
				return "", "", nil, domain.ErrInvalidFrontmatter
			}
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
			if descSet {
				return "", "", nil, domain.ErrInvalidFrontmatter
			}
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
			meta.Content = append(meta.Content, key, val)
		}
	}
	if !nameSet {
		return "", "", nil, domain.ErrMissingName
	}
	if !descSet {
		return "", "", nil, domain.ErrMissingDescription
	}
	if len(meta.Content) == 0 {
		return name, description, nil, nil
	}
	return name, description, meta, nil
}

func yamlString(v *yaml.Node) (string, bool, error) {
	if v.Anchor != "" || v.Alias != nil || v.Kind == yaml.AliasNode || v.Style&yaml.TaggedStyle != 0 {
		return "", false, domain.ErrInvalidFrontmatter
	}
	if v.Tag == "!!null" {
		return "", false, nil
	}
	if v.Kind != yaml.ScalarNode || v.Tag != "!!str" {
		return "", false, domain.ErrInvalidFrontmatter
	}
	return v.Value, true, nil
}

func validateTree(n *yaml.Node, top bool) error {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range n.Content {
			if err := validateTree(child, false); err != nil {
				return err
			}
		}
		return nil
	case yaml.MappingNode:
		if len(n.Content)%2 != 0 {
			return domain.ErrInvalidFrontmatter
		}
		seen := make(map[string]struct{}, len(n.Content)/2)
		for i := 0; i < len(n.Content); i += 2 {
			key := n.Content[i]
			if isMergeKey(key) {
				return fmt.Errorf("%w: merge key is not allowed", domain.ErrInvalidFrontmatter)
			}
			if err := checkMappingKey(key, top); err != nil {
				return err
			}
			id := key.Tag + "\x00" + key.Value
			if _, ok := seen[id]; ok {
				return domain.ErrInvalidFrontmatter
			}
			seen[id] = struct{}{}
			if err := validateTree(n.Content[i+1], false); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

func isMergeKey(key *yaml.Node) bool {
	return key.Tag == "!!merge" || key.Value == "<<"
}

func checkMappingKey(key *yaml.Node, top bool) error {
	if key.Kind != yaml.ScalarNode || key.Alias != nil || key.Anchor != "" || key.Style&yaml.TaggedStyle != 0 {
		return domain.ErrInvalidFrontmatter
	}
	if key.Tag != "" && !strings.HasPrefix(key.Tag, "!!") {
		return domain.ErrInvalidFrontmatter
	}
	if top && key.Tag != "!!str" {
		return domain.ErrInvalidFrontmatter
	}
	return nil
}

func marshalFrontmatter(name, description string, meta *yaml.Node) ([]byte, error) {
	if meta != nil && (meta.Kind != yaml.MappingNode || len(meta.Content)%2 != 0) {
		return nil, domain.ErrInvalidFrontmatter
	}
	root := &yaml.Node{Kind: yaml.MappingNode}
	root.Content = append(root.Content,
		plainString("name"), plainString(name),
		plainString("description"), plainString(description),
	)
	if meta != nil {
		root.Content = append(root.Content, meta.Content...)
	}
	out, err := yaml.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidFrontmatter, err)
	}
	return out, nil
}

func plainString(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}
