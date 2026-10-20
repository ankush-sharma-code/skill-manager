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

func observe(fsys ports.FS, t domain.Target, repoRoot string) ([]domain.Observed, error) {
	root, err := usableRoot(fsys, t.Root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	ents, err := fsys.ReadDir(root)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Observed, 0, len(ents))
	for _, ent := range ents {
		if !keepSlot(ent.Name()) {
			continue
		}
		ob, err := classifyAt(fsys, t.ID, root, repoRoot, ent.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, ob)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func classify(fsys ports.FS, t domain.Target, repoRoot, name string) (domain.Observed, error) {
	if !slotElement(name) {
		return domain.Observed{}, ErrOutside
	}
	parsed, err := domain.ParseSkillName(name)
	if err != nil {
		return domain.Observed{}, err
	}
	root, err := usableRoot(fsys, t.Root)
	if errors.Is(err, fs.ErrNotExist) {
		return missing(t.ID, parsed), nil
	}
	if err != nil {
		return domain.Observed{}, err
	}
	return classifyAt(fsys, t.ID, root, repoRoot, string(parsed))
}

func classifyAt(fsys ports.FS, id string, root, repoRoot, name string) (domain.Observed, error) {
	parsed := domain.SkillName(name)
	slot := filepath.Join(root, name)
	info, err := fsys.Lstat(slot)
	if errors.Is(err, fs.ErrNotExist) {
		return missing(id, parsed), nil
	}
	if err != nil {
		return domain.Observed{}, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return classifyLink(fsys, id, repoRoot, name, slot)
	}
	if info.IsDir() {
		return classifyDir(fsys, id, parsed, slot)
	}
	return domain.Observed{Target: id, Name: parsed, State: domain.SlotFile}, nil
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

func classifyDir(fsys ports.FS, id string, name domain.SkillName, slot string) (domain.Observed, error) {
	sum, err := digestDir(fsys, slot)
	if err != nil {
		return domain.Observed{}, err
	}
	return domain.Observed{Target: id, Name: name, State: domain.SlotDir, Digest: sum}, nil
}

func missing(id string, name domain.SkillName) domain.Observed {
	return domain.Observed{Target: id, Name: name, State: domain.SlotMissing}
}

func keepSlot(name string) bool {
	if strings.HasPrefix(name, ".") || !slotElement(name) {
		return false
	}
	_, err := domain.ParseSkillName(name)
	return err == nil
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

func digestDir(fsys ports.FS, dir string) (domain.Digest, error) {
	var files []skillfile.File
	if err := collect(fsys, dir, "", 0, &files); err != nil {
		return domain.Digest{}, err
	}
	return skillfile.TreeDigest(files)
}

func collect(fsys ports.FS, dir, relDir string, depth int, files *[]skillfile.File) error {
	if depth > maxTreeDepth {
		return errors.New("skill tree too deep")
	}
	ents, err := fsys.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, ent := range ents {
		if err := collectEntry(fsys, dir, relDir, depth, ent.Name(), files); err != nil {
			return err
		}
	}
	return nil
}

func collectEntry(fsys ports.FS, dir, relDir string, depth int, name string, files *[]skillfile.File) error {
	abs := filepath.Join(dir, name)
	rel := childRel(relDir, name)
	info, err := fsys.Lstat(abs)
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return appendLink(fsys, abs, rel, info.Mode(), files)
	}
	if info.IsDir() {
		return collect(fsys, abs, rel, depth+1, files)
	}
	return appendFile(fsys, abs, rel, info.Mode(), files)
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
