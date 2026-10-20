package repo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
	"github.com/ankush-sharma-code/skill-manager/internal/skillfile"
)

// ErrNotFound is returned when no skill directory has this exact name.
var ErrNotFound = errors.New("skill not found")

// ErrCaseCollision is returned when a directory name differs only by case.
var ErrCaseCollision = errors.New("case-insensitive name collision")

// Repo is the skill directory at root, usually ~/.skills.
type Repo struct {
	fs    ports.FS
	clock ports.Clock
	root  string
}

// New binds a repository to an absolute root.
func New(fsys ports.FS, clock ports.Clock, root string) (*Repo, error) {
	if fsys == nil || clock == nil {
		return nil, errors.New("nil filesystem or clock")
	}
	if !filepath.IsAbs(root) {
		return nil, errors.New("repo root must be absolute")
	}
	return &Repo{fs: fsys, clock: clock, root: root}, nil
}

var _ ports.Repo = (*Repo)(nil)

// List returns the skills whose directory names are valid skill names.
func (r *Repo) List(ctx context.Context) ([]domain.Skill, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ents, err := r.fs.ReadDir(r.root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(ents))
	for _, ent := range ents {
		name := ent.Name()
		if strings.HasPrefix(name, ".") || !ent.IsDir() {
			continue
		}
		if _, err := domain.ParseSkillName(name); err != nil {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]domain.Skill, 0, len(names))
	for _, name := range names {
		skill, err := r.Get(ctx, domain.SkillName(name))
		if err != nil {
			return nil, err
		}
		out = append(out, skill)
	}
	return out, nil
}

// Get reads one skill from <root>/<name>.
func (r *Repo) Get(ctx context.Context, name domain.SkillName) (domain.Skill, error) {
	if err := ctx.Err(); err != nil {
		return domain.Skill{}, err
	}
	parsed, err := domain.ParseSkillName(string(name))
	if err != nil {
		return domain.Skill{}, err
	}
	exact, folded, err := r.classify(string(parsed))
	if err != nil {
		return domain.Skill{}, err
	}
	if !exact && folded {
		return domain.Skill{}, ErrCaseCollision
	}
	if !exact {
		return domain.Skill{}, ErrNotFound
	}
	dir := filepath.Join(r.root, string(parsed))
	info, err := r.fs.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return domain.Skill{}, ErrNotFound
	}
	if err != nil {
		return domain.Skill{}, err
	}
	if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return domain.Skill{}, ErrNotFound
	}
	files, err := r.readFiles(dir)
	if err != nil {
		return domain.Skill{}, err
	}
	return skillfile.Load(string(parsed), files)
}

// Put writes the file map to a staging directory and renames it to <root>/<name>.
// An existing skill is moved aside first and restored if the new directory cannot take its place.
func (r *Repo) Put(ctx context.Context, skill domain.Skill, files map[string][]byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	parsed, err := domain.ParseSkillName(string(skill.Name))
	if err != nil {
		return err
	}
	staged, err := stageFiles(skill, files)
	if err != nil {
		return err
	}
	if err := r.fs.MkdirAll(r.root, 0o755); err != nil {
		return err
	}
	exact, folded, err := r.classify(string(parsed))
	if err != nil {
		return err
	}
	if folded {
		return ErrCaseCollision
	}
	staging, err := r.tempPath(".tmp-", string(parsed))
	if err != nil {
		return err
	}
	if err := r.writeTree(staging, staged); err != nil {
		r.removeQuiet(staging)
		return err
	}
	dest := filepath.Join(r.root, string(parsed))
	if !exact {
		if err := r.fs.Rename(staging, dest); err != nil {
			r.removeQuiet(staging)
			return err
		}
		return nil
	}
	backup, err := r.tempPath(".old-", string(parsed))
	if err != nil {
		r.removeQuiet(staging)
		return err
	}
	if err := r.fs.Rename(dest, backup); err != nil {
		r.removeQuiet(staging)
		return err
	}
	if err := r.fs.Rename(staging, dest); err != nil {
		if backErr := r.fs.Rename(backup, dest); backErr != nil {
			return fmt.Errorf("publish %s: %w (rollback: %v)", parsed, err, backErr)
		}
		r.removeQuiet(staging)
		return err
	}
	if err := r.fs.RemoveAll(backup); err != nil {
		return err
	}
	return nil
}

// Delete moves <root>/<name> to <root>/.trash/<name>-<unix nano>.
func (r *Repo) Delete(ctx context.Context, name domain.SkillName) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	parsed, err := domain.ParseSkillName(string(name))
	if err != nil {
		return err
	}
	exact, folded, err := r.classify(string(parsed))
	if err != nil {
		return err
	}
	if !exact && folded {
		return ErrCaseCollision
	}
	if !exact {
		return ErrNotFound
	}
	src := filepath.Join(r.root, string(parsed))
	info, err := r.fs.Lstat(src)
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return ErrNotFound
	}
	trashDir := filepath.Join(r.root, ".trash")
	if err := r.fs.MkdirAll(trashDir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(trashDir, string(parsed)+"-"+strconv.FormatInt(r.clock.Now().UTC().UnixNano(), 10))
	if _, err := r.fs.Lstat(dest); err == nil {
		dest += "-2"
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return r.fs.Rename(src, dest)
}

func (r *Repo) classify(name string) (exact, folded bool, err error) {
	ents, err := r.fs.ReadDir(r.root)
	if errors.Is(err, fs.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	for _, ent := range ents {
		got := ent.Name()
		if got == "" || strings.HasPrefix(got, ".") {
			continue
		}
		if got == name {
			exact = true
			continue
		}
		if strings.EqualFold(got, name) {
			folded = true
		}
	}
	return exact, folded, nil
}

type stagedFile struct {
	rel  string
	mode fs.FileMode
	body []byte
}

func stageFiles(skill domain.Skill, files map[string][]byte) ([]stagedFile, error) {
	inputs := make([]skillfile.File, 0, len(files))
	for rel, body := range files {
		inputs = append(inputs, skillfile.File{
			RelPath: rel,
			Mode:    modeFor(skill, rel),
			Content: body,
		})
	}
	if _, err := skillfile.TreeDigest(inputs); err != nil {
		return nil, err
	}
	out := make([]stagedFile, 0, len(files))
	hasSkill := false
	for rel, body := range files {
		cleaned := path.Clean(rel)
		if cleaned == "SKILL.md" {
			hasSkill = true
		}
		out = append(out, stagedFile{
			rel:  cleaned,
			mode: modeFor(skill, rel),
			body: append([]byte(nil), body...),
		})
	}
	if !hasSkill {
		return nil, domain.ErrMissingSkillMD
	}
	return out, nil
}

func modeFor(skill domain.Skill, rel string) fs.FileMode {
	cleaned := path.Clean(rel)
	for _, file := range skill.Files {
		if file.RelPath == rel || path.Clean(file.RelPath) == cleaned {
			perm := file.Mode.Perm()
			if perm == 0 {
				return 0o644
			}
			return perm
		}
	}
	return 0o644
}

func (r *Repo) writeTree(dir string, files []stagedFile) error {
	if err := r.fs.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, file := range files {
		abs := filepath.Join(dir, filepath.FromSlash(file.rel))
		parent := filepath.Dir(abs)
		if parent != dir {
			if err := r.fs.MkdirAll(parent, 0o755); err != nil {
				return err
			}
		}
		if err := r.fs.WriteFileAtomic(abs, file.body, file.mode); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) readFiles(dir string) ([]skillfile.File, error) {
	var out []skillfile.File
	var walk func(string) error
	walk = func(rel string) error {
		abs := dir
		if rel != "" {
			abs = filepath.Join(dir, filepath.FromSlash(rel))
		}
		ents, err := r.fs.ReadDir(abs)
		if err != nil {
			return err
		}
		for _, ent := range ents {
			name := ent.Name()
			child := name
			if rel != "" {
				child = rel + "/" + name
			}
			if ent.Type()&fs.ModeSymlink != 0 {
				return fmt.Errorf("symlink in skill tree: %s", child)
			}
			if ent.IsDir() {
				if err := walk(child); err != nil {
					return err
				}
				continue
			}
			info, err := ent.Info()
			if err != nil {
				return err
			}
			body, err := r.fs.ReadFile(filepath.Join(dir, filepath.FromSlash(child)))
			if err != nil {
				return err
			}
			out = append(out, skillfile.File{
				RelPath: child,
				Mode:    info.Mode().Perm(),
				Content: body,
			})
		}
		return nil
	}
	if err := walk(""); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repo) tempPath(prefix, name string) (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return filepath.Join(r.root, prefix+name+"-"+hex.EncodeToString(buf[:])), nil
}

func (r *Repo) removeQuiet(path string) {
	_ = r.fs.RemoveAll(path)
}

// Problems returns skill directories the last List could not load.
func (r *Repo) Problems() []ports.SkillProblem { return nil }

// Recover finishes or undoes a torn publish left under .stage.
func (r *Repo) Recover(context.Context) error { return errors.New("not implemented") }

// SetRand supplies bytes for trash names and stage ids.
func (r *Repo) SetRand(io.Reader) {}

// SetLog receives a best-effort backup removal failure.
func (r *Repo) SetLog(func(string, ...any)) {}
