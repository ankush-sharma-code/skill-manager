package skillfile

import (
	"errors"
	"io/fs"
	"time"
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
