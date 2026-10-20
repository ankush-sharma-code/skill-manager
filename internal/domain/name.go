package domain

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidName        = errors.New("invalid skill name")
	ErrDescriptionLength  = errors.New("description must be 1 to 1024 non-whitespace characters")
	ErrMissingFrontmatter = errors.New("missing frontmatter delimiters")
	ErrInvalidFrontmatter = errors.New("invalid frontmatter")
	ErrMissingName        = errors.New("frontmatter missing name")
	ErrMissingDescription = errors.New("frontmatter missing description")
	ErrNameMismatch       = errors.New("frontmatter name does not match directory")
	ErrMissingSkillMD     = errors.New("SKILL.md is required")
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

const descriptionMax = 1024

// ParseSkillName accepts a lowercase name of 1 to 64 characters.
// The first character is a letter or digit. The rest are letters, digits, or hyphens.
func ParseSkillName(s string) (SkillName, error) {
	if !skillNamePattern.MatchString(s) {
		return "", ErrInvalidName
	}
	return SkillName(s), nil
}

// ValidateDescription accepts 1 to 1024 Unicode characters that are not only whitespace.
func ValidateDescription(s string) error {
	if strings.TrimSpace(s) == "" || utf8.RuneCountInString(s) > descriptionMax {
		return ErrDescriptionLength
	}
	return nil
}
