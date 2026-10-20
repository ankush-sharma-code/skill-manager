package skillfile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/ankush-sharma-code/skill-manager/internal/domain"
)

// TreeDigest is the SHA-256 of a canonical listing.
// Records are sorted by cleaned relative path. Each record is
// relpath, a NUL, the 8-digit hex file type, a NUL, the four-digit permission, a NUL, the lowercase content SHA-256, and a newline.
// Directory entries and modification times are omitted. The file type is mode.Type(), so a symlink and a regular file differ.
func TreeDigest(files []File) (domain.Digest, error) {
	entries, err := normalize(files)
	if err != nil {
		return domain.Digest{}, err
	}
	return hashTree(entries), nil
}

// Load parses SKILL.md from files and fills the skill digest.
// dirName must be a valid skill name and must equal the frontmatter name.
func Load(dirName string, files []File) (domain.Skill, error) {
	dir, err := domain.ParseSkillName(dirName)
	if err != nil {
		return domain.Skill{}, err
	}
	entries, err := normalize(files)
	if err != nil {
		return domain.Skill{}, err
	}
	var skillMD []byte
	found := false
	for _, e := range entries {
		if e.rel == "SKILL.md" && !e.isDir {
			skillMD = e.content
			found = true
			break
		}
	}
	if !found {
		return domain.Skill{}, domain.ErrMissingSkillMD
	}
	name, description, meta, _, err := Parse(skillMD)
	if err != nil {
		return domain.Skill{}, err
	}
	if name != dir {
		return domain.Skill{}, domain.ErrNameMismatch
	}
	return domain.Skill{
		Name:        name,
		Description: description,
		Meta:        meta,
		Files:       fileEntries(entries),
		Digest:      hashTree(entries),
	}, nil
}

type entry struct {
	rel     string
	mode    fs.FileMode
	content []byte
	isDir   bool
}

func normalize(files []File) ([]entry, error) {
	out := make([]entry, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	for _, f := range files {
		rel, err := cleanRel(f.RelPath)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[rel]; ok {
			return nil, ErrDuplicatePath
		}
		seen[rel] = struct{}{}
		out = append(out, entry{
			rel:     rel,
			mode:    f.Mode,
			content: f.Content,
			isDir:   f.Mode.IsDir(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return out, nil
}

func cleanRel(p string) (string, error) {
	if p == "" || strings.Contains(p, `\`) || strings.ContainsRune(p, 0) {
		return "", ErrBadPath
	}
	if path.IsAbs(p) {
		return "", ErrBadPath
	}
	cleaned := path.Clean(p)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", ErrBadPath
	}
	return cleaned, nil
}

func hashTree(entries []entry) domain.Digest {
	var buf bytes.Buffer
	for _, e := range entries {
		if e.isDir {
			continue
		}
		sum := sha256.Sum256(e.content)
		buf.WriteString(e.rel)
		buf.WriteByte(0)
		buf.WriteString(typeTag(e.mode))
		buf.WriteByte(0)
		buf.WriteString(permOctal(e.mode))
		buf.WriteByte(0)
		buf.WriteString(hex.EncodeToString(sum[:]))
		buf.WriteByte('\n')
	}
	return sha256.Sum256(buf.Bytes())
}

func typeTag(mode fs.FileMode) string {
	const digits = "0123456789abcdef"
	v := uint32(mode.Type())
	var out [8]byte
	for i := 7; i >= 0; i-- {
		out[i] = digits[v&0xf]
		v >>= 4
	}
	return string(out[:])
}

func permOctal(mode fs.FileMode) string {
	const digits = "01234567"
	perm := uint32(mode.Perm())
	var out [4]byte
	for i := 3; i >= 0; i-- {
		out[i] = digits[perm&7]
		perm >>= 3
	}
	return string(out[:])
}

func fileEntries(entries []entry) []domain.FileEntry {
	out := make([]domain.FileEntry, 0, len(entries))
	for _, e := range entries {
		if e.isDir {
			continue
		}
		out = append(out, domain.FileEntry{
			RelPath: e.rel,
			Mode:    e.mode.Type() | e.mode.Perm(),
			Size:    int64(len(e.content)),
			Digest:  sha256.Sum256(e.content),
		})
	}
	return out
}
