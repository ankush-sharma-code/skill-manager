package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
)

func TestParseSkillName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		wantErr error
	}{
		{name: "single letter", in: "a"},
		{name: "single digit", in: "0"},
		{name: "hyphenated", in: "skill-name"},
		{name: "trailing hyphen", in: "a-"},
		{name: "max length", in: strings.Repeat("a", 64)},

		{name: "empty", in: "", wantErr: domain.ErrInvalidName},
		{name: "uppercase", in: "Skill", wantErr: domain.ErrInvalidName},
		{name: "mixed case", in: "skillName", wantErr: domain.ErrInvalidName},
		{name: "space", in: "skill name", wantErr: domain.ErrInvalidName},
		{name: "leading space", in: " skill", wantErr: domain.ErrInvalidName},
		{name: "tab", in: "skill\tname", wantErr: domain.ErrInvalidName},
		{name: "dotdot", in: "..", wantErr: domain.ErrInvalidName},
		{name: "dotdot slash", in: "../etc", wantErr: domain.ErrInvalidName},
		{name: "embedded dotdot", in: "foo/../bar", wantErr: domain.ErrInvalidName},
		{name: "slash", in: "a/b", wantErr: domain.ErrInvalidName},
		{name: "leading hyphen", in: "-skill", wantErr: domain.ErrInvalidName},
		{name: "underscore", in: "skill_name", wantErr: domain.ErrInvalidName},
		{name: "dot", in: "skill.md", wantErr: domain.ErrInvalidName},
		{name: "65 chars", in: strings.Repeat("b", 65), wantErr: domain.ErrInvalidName},
		{name: "cyrillic a", in: "\u0430", wantErr: domain.ErrInvalidName},
		{name: "cyrillic i", in: "sk\u0456ll", wantErr: domain.ErrInvalidName},
		{name: "unicode hyphen", in: "skill\u2010name", wantErr: domain.ErrInvalidName},
		{name: "fullwidth digit", in: "skill\uff11", wantErr: domain.ErrInvalidName},
		{name: "zero width", in: "sk\u200dll", wantErr: domain.ErrInvalidName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.ParseSkillName(tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ParseSkillName(%q) error = %v, want %v", tt.in, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSkillName(%q) unexpected error: %v", tt.in, err)
			}
			if string(got) != tt.in {
				t.Fatalf("ParseSkillName(%q) = %q", tt.in, got)
			}
		})
	}
}

func TestValidateDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		wantErr error
	}{
		{name: "short", in: "Says hello"},
		{name: "one rune", in: "é"},
		{name: "max runes", in: strings.Repeat("é", 1024)},
		{name: "empty", in: "", wantErr: domain.ErrDescriptionLength},
		{name: "too long", in: strings.Repeat("é", 1025), wantErr: domain.ErrDescriptionLength},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := domain.ValidateDescription(tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ValidateDescription error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateDescription(%q) unexpected error: %v", tt.in, err)
			}
		})
	}
}
