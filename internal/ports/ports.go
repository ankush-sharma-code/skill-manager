package ports

import (
	"context"
	"io/fs"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
)

// Clock supplies the current time.
type Clock interface {
	Now() time.Time
}

// SystemClock reads the process clock.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

// FS is the filesystem boundary. Lstat, Readlink, Rename, and RemoveAll do not
// follow a final symlink. ReadFile and ReadDir do, and stop on a link loop.
// WriteFileAtomic puts bytes in a temp file in the destination directory, syncs
// that file, and renames it onto the destination.
type FS interface {
	Lstat(p string) (fs.FileInfo, error)
	ReadDir(p string) ([]fs.DirEntry, error)
	ReadFile(p string) ([]byte, error)
	Readlink(p string) (string, error)
	WriteFileAtomic(p string, b []byte, mode fs.FileMode) error
	Symlink(old, new string) error
	Rename(old, new string) error
	RemoveAll(p string) error
	MkdirAll(p string, mode fs.FileMode) error
}

// SkillProblem is one skill directory List could not load.
type SkillProblem struct {
	Name domain.SkillName
	Err  error
}

// Repo stores skills at <root>/<name>/.
// Delete moves a skill to <root>/.trash/<name>-<ts>-<rand>.
// .trash grows without bound.
// TODO: a later change should purge old trash entries.
type Repo interface {
	List(ctx context.Context) ([]domain.Skill, error)
	Problems() []SkillProblem
	Get(ctx context.Context, name domain.SkillName) (domain.Skill, error)
	Put(ctx context.Context, skill domain.Skill, files map[string][]byte) error
	Delete(ctx context.Context, name domain.SkillName) error
	Recover(ctx context.Context) error
}

// LogFile is one transcript file a tool left on disk.
type LogFile struct {
	Path  string
	Inode uint64
	Size  int64
}

// Cursor is the last byte a LogSource consumed from a LogFile.
// A new Inode, or a Size smaller than Offset, means the file was replaced or truncated.
type Cursor struct {
	Path   string
	Inode  uint64
	Size   int64
	Offset int64
}

// LogSource reads usage events from one tool's transcripts.
type LogSource interface {
	ID() string
	Discover(ctx context.Context) ([]LogFile, error)
	Parse(ctx context.Context, file LogFile, from Cursor) ([]domain.UsageEvent, Cursor, error)
}
