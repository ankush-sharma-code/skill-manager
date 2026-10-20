package domain

import "errors"

var (
	ErrInvalidName        = errors.New("invalid skill name")
	ErrDescriptionLength  = errors.New("description must be 1 to 1024 characters")
	ErrMissingFrontmatter = errors.New("missing frontmatter delimiters")
	ErrInvalidFrontmatter = errors.New("invalid frontmatter")
	ErrMissingName        = errors.New("frontmatter missing name")
	ErrMissingDescription = errors.New("frontmatter missing description")
	ErrNameMismatch       = errors.New("frontmatter name does not match directory")
	ErrMissingSkillMD     = errors.New("SKILL.md is required")
)

func ParseSkillName(s string) (SkillName, error) {
	return "", errors.New("not implemented")
}

func ValidateDescription(s string) error {
	return errors.New("not implemented")
}
