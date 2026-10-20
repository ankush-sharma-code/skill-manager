package target

import (
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
	"github.com/ankush-sharma-code/skill-manager/internal/ports"
	"github.com/ankush-sharma-code/skill-manager/internal/skillfile"
)

const maxTreeDepth = 32

func inspect(fsys ports.FS, t domain.Target, repoRoot string) (Result, error) {
	root, err := openRoot(fsys, t, repoRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Result{}, nil
		}
		return Result{}, err
	}
	ents, err := fsys.ReadDir(root)
	if err != nil {
		return Result{}, err
	}
	names := slotNames(ents)
	out := make([]domain.Observed, 0, len(names))
	problems := caseProblems(t.ID, root, names)
	for _, name := range names {
		ob, found, err := classifyAt(fsys, t.ID, root, repoRoot, name)
		if err != nil {
			return Result{}, err
		}
		out = append(out, ob)
		problems = append(problems, found...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return Result{Observed: out, Problems: problems}, nil
}

func classify(fsys ports.FS, t domain.Target, repoRoot, name string) (domain.Observed, error) {
	if !slotElement(name) {
		return domain.Observed{}, ErrOutside
	}
	parsed, err := domain.ParseSkillName(name)
	if err != nil {
		return domain.Observed{}, err
	}
	root, err := openRoot(fsys, t, repoRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return missing(t.ID, parsed), nil
	}
	if err != nil {
		return domain.Observed{}, err
	}
	ob, _, err := classifyAt(fsys, t.ID, root, repoRoot, string(parsed))
	return ob, err
}

func classifyAt(fsys ports.FS, id string, root, repoRoot, name string) (domain.Observed, []Problem, error) {
	parsed := domain.SkillName(name)
	slot := filepath.Join(root, name)
	info, err := fsys.Lstat(slot)
	if errors.Is(err, fs.ErrNotExist) {
		return missing(id, parsed), nil, nil
	}
	if err != nil {
		return domain.Observed{}, nil, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		ob, err := classifyLink(fsys, id, repoRoot, name, slot)
		return ob, nil, err
	}
	if info.IsDir() {
		return classifyDir(fsys, id, parsed, slot)
	}
	if kind, ok := specialKind(info.Mode()); ok {
		return domain.Observed{Target: id, Name: parsed, State: domain.SlotFile},
			[]Problem{irregularProblem(id, name, slot, kind)}, nil
	}
	return domain.Observed{Target: id, Name: parsed, State: domain.SlotFile}, nil, nil
}

func classifyLink(fsys ports.FS, id, repoRoot, name, slot string) (domain.Observed, error) {
	parsed := domain.SkillName(name)
	raw, err := fsys.Readlink(slot)
	if err != nil {
		return domain.Observed{}, err
	}
	final, err := linkTarget(fsys, filepath.Dir(slot), raw)
	if err != nil {
		if errors.Is(err, ErrLoop) || errors.Is(err, fs.ErrNotExist) {
			return domain.Observed{Target: id, Name: parsed, State: domain.SlotBroken, LinkTo: raw}, nil
		}
		return domain.Observed{}, err
	}
	if sameSkill(fsys, final, repoRoot, name) {
		return domain.Observed{Target: id, Name: parsed, State: domain.SlotOurSymlink, LinkTo: raw}, nil
	}
	return domain.Observed{Target: id, Name: parsed, State: domain.SlotForeignSymlink, LinkTo: raw}, nil
}

func linkTarget(fsys ports.FS, dir, raw string) (string, error) {
	final, _, err := resolveFinal(fsys, joinLink(dir, raw))
	if err != nil {
		return "", err
	}
	return final, nil
}

func sameSkill(fsys ports.FS, final, repoRoot, name string) bool {
	want := filepath.Clean(filepath.Join(repoRoot, name))
	if filepath.Clean(final) == want {
		return true
	}
	resolved, _, err := resolveFinal(fsys, want)
	return err == nil && resolved == filepath.Clean(final)
}

func classifyDir(fsys ports.FS, id string, name domain.SkillName, slot string) (domain.Observed, []Problem, error) {
	sum, problems, err := digestDir(fsys, slot, id, string(name))
	if err != nil {
		return domain.Observed{}, nil, err
	}
	return domain.Observed{Target: id, Name: name, State: domain.SlotDir, Digest: sum}, problems, nil
}

func missing(id string, name domain.SkillName) domain.Observed {
	return domain.Observed{Target: id, Name: name, State: domain.SlotMissing}
}

func slotElement(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsRune(name, 0) {
		return false
	}
	return !strings.ContainsAny(name, `/\`)
}

func digestDir(fsys ports.FS, dir, target, skill string) (domain.Digest, []Problem, error) {
	g := &gather{target: target, skill: skill}
	if err := collect(fsys, dir, "", 0, g); err != nil {
		return domain.Digest{}, nil, err
	}
	sum, err := skillfile.TreeDigest(g.files)
	if err != nil {
		return domain.Digest{}, nil, err
	}
	return sum, g.problems, nil
}

type gather struct {
	files    []skillfile.File
	problems []Problem
	target   string
	skill    string
}

func collect(fsys ports.FS, dir, relDir string, depth int, g *gather) error {
	if depth > maxTreeDepth {
		return errors.New("skill tree too deep")
	}
	ents, err := fsys.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, ent := range ents {
		if err := collectEntry(fsys, dir, relDir, depth, ent.Name(), g); err != nil {
			return err
		}
	}
	return nil
}

func collectEntry(fsys ports.FS, dir, relDir string, depth int, name string, g *gather) error {
	abs := filepath.Join(dir, name)
	rel := childRel(relDir, name)
	info, err := fsys.Lstat(abs)
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return appendLink(fsys, abs, rel, info.Mode(), &g.files)
	}
	if info.IsDir() {
		return collect(fsys, abs, rel, depth+1, g)
	}
	if kind, ok := specialKind(info.Mode()); ok {
		g.problems = append(g.problems, irregularProblem(g.target, g.skill, abs, kind))
		return nil
	}
	return appendFile(fsys, abs, rel, info.Mode(), &g.files)
}

func specialKind(mode fs.FileMode) (string, bool) {
	switch {
	case mode&fs.ModeNamedPipe != 0:
		return "fifo", true
	case mode&fs.ModeSocket != 0:
		return "socket", true
	case mode&fs.ModeDevice != 0:
		return "device", true
	default:
		return "", !mode.IsRegular()
	}
}

func irregularProblem(target, skill, path, kind string) Problem {
	if kind == "" {
		kind = "non-regular file"
	}
	return Problem{
		Level:   LevelConflict,
		Target:  target,
		Path:    path,
		Message: kind + " is not a regular file",
		Names:   []string{skill},
	}
}

func appendLink(fsys ports.FS, abs, rel string, mode fs.FileMode, files *[]skillfile.File) error {
	target, err := fsys.Readlink(abs)
	if err != nil {
		return err
	}
	*files = append(*files, skillfile.File{RelPath: rel, Mode: mode, Content: []byte(target)})
	return nil
}

func appendFile(fsys ports.FS, abs, rel string, mode fs.FileMode, files *[]skillfile.File) error {
	body, err := fsys.ReadFile(abs)
	if err != nil {
		return err
	}
	*files = append(*files, skillfile.File{RelPath: rel, Mode: mode, Content: body})
	return nil
}

func childRel(relDir, name string) string {
	if relDir == "" {
		return name
	}
	return path.Join(relDir, name)
}
