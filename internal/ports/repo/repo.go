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
// The comparison is ASCII A-Z folding. It does not use Unicode case folding or NFC/NFD.
var ErrCaseCollision = errors.New("case-insensitive name collision")

// ErrSymlink is returned when a path inside the repo root is a symlink.
var ErrSymlink = errors.New("symlink under skill root")

// Repo is the skill directory at root, usually ~/.skills.
type Repo struct {
	fs       ports.FS
	clock    ports.Clock
	rng      io.Reader
	root     string
	problems []ports.SkillProblem
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
	if err := r.rejectSymlink(r.root); err != nil {
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
	problems := make([]ports.SkillProblem, 0)
	for _, name := range names {
		skill, err := r.Get(ctx, domain.SkillName(name))
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			problems = append(problems, ports.SkillProblem{Name: domain.SkillName(name), Err: err})
			continue
		}
		out = append(out, skill)
	}
	r.problems = problems
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

// Put writes the new tree under .stage/<id>.new, moves an existing skill to
// .stage/<id>.old, then renames the new tree into place.
// A crash between those renames is finished by Recover.
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
	if err := r.ensureRealDir(r.root); err != nil {
		return err
	}
	exact, folded, err := r.classify(string(parsed))
	if err != nil {
		return err
	}
	if folded {
		return ErrCaseCollision
	}
	if err := r.ensureRealDir(filepath.Join(r.root, ".stage")); err != nil {
		return err
	}
	id, err := r.stageID(string(parsed))
	if err != nil {
		return err
	}
	newDir := filepath.Join(r.root, ".stage", id+".new")
	oldDir := filepath.Join(r.root, ".stage", id+".old")
	if err := r.writeTree(newDir, staged); err != nil {
		r.removeQuiet(newDir)
		return err
	}
	dest := filepath.Join(r.root, string(parsed))
	if exact {
		if err := r.fs.Rename(dest, oldDir); err != nil {
			r.removeQuiet(newDir)
			return err
		}
	}
	if err := r.fs.Rename(newDir, dest); err != nil {
		if exact {
			if backErr := r.fs.Rename(oldDir, dest); backErr != nil {
				return fmt.Errorf("publish %s: %w (rollback: %v)", parsed, err, backErr)
			}
		}
		r.removeQuiet(newDir)
		return err
	}
	if exact {
		if err := r.fs.RemoveAll(oldDir); err != nil {
			return err
		}
	}
	return nil
}

// Delete moves <root>/<name> to <root>/.trash/<name>-<unix nano>-<rand>.
// .trash grows without bound.
// TODO: a later change should purge old trash entries.
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
	if err := r.rejectSymlink(r.root); err != nil {
		return err
	}
	trashDir := filepath.Join(r.root, ".trash")
	if err := r.ensureRealDir(trashDir); err != nil {
		return err
	}
	stamp := strconv.FormatInt(r.clock.Now().UTC().UnixNano(), 10)
	var last string
	for attempt := 0; attempt < 8; attempt++ {
		suffix, err := r.randomHex(8)
		if err != nil {
			return err
		}
		dest := filepath.Join(trashDir, string(parsed)+"-"+stamp+"-"+suffix)
		last = dest
		_, err = r.fs.Lstat(dest)
		if err == nil {
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		err = r.fs.Rename(src, dest)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return err
	}
	return fmt.Errorf("trash name still exists: %s", last)
}

func (r *Repo) rejectSymlink(path string) error {
	info, err := r.fs.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s", ErrSymlink, path)
	}
	return nil
}

// ensureRealDir creates path when it is missing. An existing symlink is refused
// before any create, so the call does not follow it out of the root.
func (r *Repo) ensureRealDir(path string) error {
	if err := r.rejectSymlink(path); err != nil {
		return err
	}
	info, err := r.fs.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := r.fs.MkdirAll(path, 0o755); err != nil {
			return err
		}
		info, err = r.fs.Lstat(path)
	}
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s", ErrSymlink, path)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
}

func (r *Repo) classify(name string) (exact, folded bool, err error) {
	if err := r.rejectSymlink(r.root); err != nil {
		return false, false, err
	}
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

func (r *Repo) stageID(name string) (string, error) {
	suffix, err := r.randomHex(8)
	if err != nil {
		return "", err
	}
	return name + "-" + suffix, nil
}

func (r *Repo) randomHex(n int) (string, error) {
	buf := make([]byte, n)
	reader := r.rng
	if reader == nil {
		reader = rand.Reader
	}
	if _, err := io.ReadFull(reader, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

type stagePair struct {
	newPath string
	oldPath string
}

func skillFromStageID(id string) (string, bool) {
	const hexLen = 16
	if len(id) < hexLen+2 || id[len(id)-hexLen-1] != '-' {
		return "", false
	}
	hexPart := id[len(id)-hexLen:]
	for _, c := range hexPart {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return "", false
		}
	}
	name := id[:len(id)-hexLen-1]
	if _, err := domain.ParseSkillName(name); err != nil {
		return "", false
	}
	return name, true
}

// Recover finishes or undoes a torn publish left under .stage.
// A pair with both the new tree and the old tree, and no live skill, is rolled forward.
// A new tree beside a live skill is discarded. An old tree with no live skill is restored.
func (r *Repo) Recover(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.rejectSymlink(r.root); err != nil {
		return err
	}
	stage := filepath.Join(r.root, ".stage")
	info, err := r.fs.Lstat(stage)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s", ErrSymlink, stage)
	}
	ents, err := r.fs.ReadDir(stage)
	if err != nil {
		return err
	}
	pairs := map[string]*stagePair{}
	var ids []string
	for _, ent := range ents {
		name := ent.Name()
		var id, kind string
		switch {
		case strings.HasSuffix(name, ".new"):
			id, kind = strings.TrimSuffix(name, ".new"), "new"
		case strings.HasSuffix(name, ".old"):
			id, kind = strings.TrimSuffix(name, ".old"), "old"
		default:
			continue
		}
		if _, ok := skillFromStageID(id); !ok {
			continue
		}
		pair := pairs[id]
		if pair == nil {
			pair = &stagePair{}
			pairs[id] = pair
			ids = append(ids, id)
		}
		full := filepath.Join(stage, name)
		if kind == "new" {
			pair.newPath = full
		} else {
			pair.oldPath = full
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		skillName, _ := skillFromStageID(id)
		if err := r.recoverOne(skillName, pairs[id]); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) recoverOne(name string, pair *stagePair) error {
	live := filepath.Join(r.root, name)
	info, err := r.fs.Lstat(live)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	liveOK := err == nil && info.IsDir() && info.Mode()&fs.ModeSymlink == 0
	switch {
	case pair.newPath != "" && pair.oldPath != "" && !liveOK:
		if err := r.fs.Rename(pair.newPath, live); err != nil {
			return err
		}
		return r.fs.RemoveAll(pair.oldPath)
	case pair.oldPath != "" && !liveOK:
		return r.fs.Rename(pair.oldPath, live)
	case pair.newPath != "" && liveOK:
		r.removeQuiet(pair.newPath)
		return nil
	case pair.oldPath != "" && liveOK:
		return r.fs.RemoveAll(pair.oldPath)
	default:
		return nil
	}
}

func (r *Repo) removeQuiet(path string) {
	_ = r.fs.RemoveAll(path)
}

// Problems returns skill directories the last List could not load.
// Dot names and names that are not skill names are skipped, not reported.
func (r *Repo) Problems() []ports.SkillProblem {
	if len(r.problems) == 0 {
		return nil
	}
	out := make([]ports.SkillProblem, len(r.problems))
	copy(out, r.problems)
	return out
}

// SetRand supplies bytes for trash names and stage ids.
func (r *Repo) SetRand(rng io.Reader) { r.rng = rng }

// SetLog receives a best-effort backup removal failure.
func (r *Repo) SetLog(func(string, ...any)) {}
