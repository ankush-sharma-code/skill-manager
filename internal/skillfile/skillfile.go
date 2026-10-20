package skillfile

import (
	"errors"
	"io/fs"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
)

var (
	ErrDuplicatePath = errors.New("duplicate skill file path")
	ErrBadPath       = errors.New("skill file path escapes the skill directory")
)

// File is one in-memory skill file. ModTime is accepted and ignored by the digest.
type File struct {
	RelPath string
	Mode    fs.FileMode
	Content []byte
	ModTime time.Time
}

func Parse(skillMD []byte) (domain.SkillName, string, map[string]any, []byte, error) {
	return "", "", nil, nil, errors.New("not implemented")
}

func Serialize(name domain.SkillName, description string, meta map[string]any, body []byte) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func TreeDigest(files []File) (domain.Digest, error) {
	return domain.Digest{}, errors.New("not implemented")
}

func Load(dirName string, files []File) (domain.Skill, error) {
	return domain.Skill{}, errors.New("not implemented")
}
