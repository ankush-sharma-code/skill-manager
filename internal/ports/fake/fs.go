package fake

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/ports"
)

const maxLinks = 40

// ErrInjected is returned when a test failpoint fires.
// The destination path is unchanged. A temp sibling from WriteFileAtomic may remain.
var ErrInjected = errors.New("injected failure")

type kind uint8

const (
	kindDir kind = iota + 1
	kindFile
	kindLink
)

type node struct {
	parent   *node
	name     string
	kind     kind
	mode     fs.FileMode
	data     []byte
	children map[string]*node
}

// FS is an in-memory filesystem with symlink and rename behavior matching the os package.
type FS struct {
	mu             sync.Mutex
	root           *node
	fold           bool
	seq            int
	atomicCalls    int
	failAtomicOn   int
	failRenameBase string
}

func New() *FS {
	return &FS{root: newDir("", 0o755)}
}

// NewCaseFolding returns an empty filesystem that treats names equal under case folding as one entry.
// ReadDir keeps the spelling of the first create.
func NewCaseFolding() *FS {
	fsys := New()
	fsys.fold = true
	return fsys
}

var _ ports.FS = (*FS)(nil)

// FailWriteFileAtomicBeforeRename makes the n-th later WriteFileAtomic return ErrInjected
// after the temp file is in place and before the destination path changes.
// n is 1 for the next call.
func (f *FS) FailWriteFileAtomicBeforeRename(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.atomicCalls = 0
	f.failAtomicOn = n
}

// FailNextRenameTo makes the next Rename whose destination base name is base return ErrInjected
// without changing either path.
func (f *FS) FailNextRenameTo(base string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failRenameBase = base
}

func (f *FS) Lstat(path string) (fs.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	loc, err := f.walk(path, false)
	if err != nil {
		return nil, pathErr("lstat", path, err)
	}
	return loc.node.info(baseName(path)), nil
}

func (f *FS) ReadDir(path string) ([]fs.DirEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	loc, err := f.walk(path, true)
	if err != nil {
		return nil, pathErr("open", path, err)
	}
	if loc.node.kind != kindDir {
		return nil, pathErr("readdirent", path, syscall.ENOTDIR)
	}
	kids := make([]*node, 0, len(loc.node.children))
	for _, child := range loc.node.children {
		kids = append(kids, child)
	}
	sort.Slice(kids, func(i, j int) bool { return kids[i].name < kids[j].name })
	out := make([]fs.DirEntry, 0, len(kids))
	for _, child := range kids {
		out = append(out, dirEntry{child.info(child.name)})
	}
	return out, nil
}

func (f *FS) ReadFile(path string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	loc, err := f.walk(path, true)
	if err != nil {
		return nil, pathErr("open", path, err)
	}
	if loc.node.kind == kindDir {
		return nil, pathErr("open", path, syscall.EISDIR)
	}
	if loc.node.kind != kindFile {
		return nil, pathErr("open", path, syscall.EINVAL)
	}
	return append([]byte(nil), loc.node.data...), nil
}

func (f *FS) Readlink(path string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	loc, err := f.walk(path, false)
	if err != nil {
		return "", pathErr("readlink", path, err)
	}
	if loc.node.kind != kindLink {
		return "", pathErr("readlink", path, syscall.EINVAL)
	}
	return string(loc.node.data), nil
}

func (f *FS) WriteFileAtomic(path string, data []byte, mode fs.FileMode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.atomicCalls++
	call := f.atomicCalls
	parent, base, err := f.parentOf(path)
	if err != nil {
		return pathErr("open", path, err)
	}
	f.seq++
	tmp := &node{
		name: fmt.Sprintf(".tmp-%d", f.seq),
		kind: kindFile,
		mode: mode.Perm(),
		data: append([]byte(nil), data...),
	}
	f.add(parent, tmp)
	if f.failAtomicOn > 0 && call == f.failAtomicOn {
		f.failAtomicOn = 0
		return ErrInjected
	}
	if existing := f.get(parent, base); existing != nil && existing.kind == kindDir {
		f.drop(parent, tmp)
		return &os.LinkError{Op: "rename", Old: filepath.Join(filepath.Dir(path), tmp.name), New: path, Err: syscall.EEXIST}
	} else if existing != nil {
		f.drop(parent, existing)
	}
	f.drop(parent, tmp)
	tmp.name = base
	f.add(parent, tmp)
	return nil
}

func (f *FS) Symlink(old, new string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	parent, base, err := f.parentOf(new)
	if err != nil {
		return pathErr("symlink", new, err)
	}
	if f.get(parent, base) != nil {
		return pathErr("symlink", new, syscall.EEXIST)
	}
	f.add(parent, &node{
		name: base,
		kind: kindLink,
		mode: fs.ModeSymlink | 0o777,
		data: []byte(old),
	})
	return nil
}

// Rename returns EEXIST when the destination is a different directory, matching os.Rename.
func (f *FS) Rename(oldpath, newpath string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failRenameBase != "" && baseName(newpath) == f.failRenameBase {
		f.failRenameBase = ""
		return ErrInjected
	}
	src, err := f.walk(oldpath, false)
	if err != nil {
		return linkErr(oldpath, newpath, err)
	}
	if src.parent == nil {
		return linkErr(oldpath, newpath, syscall.EBUSY)
	}
	dir, base := splitParent(newpath)
	if base == "" || base == "." || base == ".." {
		return linkErr(oldpath, newpath, syscall.EINVAL)
	}
	parentLoc, err := f.walk(dir, true)
	if err != nil {
		return linkErr(oldpath, newpath, err)
	}
	if parentLoc.node.kind != kindDir {
		return linkErr(oldpath, newpath, syscall.ENOTDIR)
	}
	existing := f.get(parentLoc.node, base)
	if existing == src.node {
		f.drop(parentLoc.node, existing)
		existing.name = base
		f.add(parentLoc.node, existing)
		return nil
	}
	if existing != nil && existing.kind == kindDir {
		return linkErr(oldpath, newpath, syscall.EEXIST)
	}
	if src.node.kind == kindDir && existing != nil {
		return linkErr(oldpath, newpath, syscall.ENOTDIR)
	}
	if src.node.kind == kindDir && contains(src.node, parentLoc.node) {
		return linkErr(oldpath, newpath, syscall.EINVAL)
	}
	if existing != nil {
		f.drop(parentLoc.node, existing)
	}
	f.drop(src.parent, src.node)
	src.node.name = base
	f.add(parentLoc.node, src.node)
	return nil
}

func (f *FS) RemoveAll(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	loc, err := f.walk(path, false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return pathErr("remove", path, err)
	}
	if loc.parent == nil {
		return pathErr("remove", path, syscall.EPERM)
	}
	f.remove(loc.parent, loc.node)
	return nil
}

func (f *FS) MkdirAll(path string, mode fs.FileMode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if path == "" {
		return pathErr("mkdir", path, syscall.EINVAL)
	}
	loc, err := f.walk(path, true)
	if err == nil {
		if loc.node.kind == kindDir {
			return nil
		}
		return pathErr("mkdir", path, syscall.ENOTDIR)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return pathErr("mkdir", path, err)
	}
	cur := f.root
	for _, part := range components(path) {
		switch part {
		case "", ".":
			continue
		case "..":
			if cur.parent != nil {
				cur = cur.parent
			}
			continue
		}
		if cur.kind != kindDir {
			return pathErr("mkdir", path, syscall.ENOTDIR)
		}
		child := f.get(cur, part)
		if child == nil {
			child = newDir(part, mode)
			f.add(cur, child)
			cur = child
			continue
		}
		if child.kind == kindLink {
			next, followErr := f.follow(child, 0)
			if followErr != nil {
				return pathErr("mkdir", path, followErr)
			}
			child = next
		}
		if child.kind != kindDir {
			return pathErr("mkdir", path, syscall.ENOTDIR)
		}
		cur = child
	}
	return nil
}

func newDir(name string, mode fs.FileMode) *node {
	return &node{
		name:     name,
		kind:     kindDir,
		mode:     fs.ModeDir | mode.Perm(),
		children: map[string]*node{},
	}
}

type located struct {
	node   *node
	parent *node
}

func (f *FS) walk(path string, followLast bool) (located, error) {
	return f.walkFrom(f.root, path, followLast, 0, true)
}

func (f *FS) walkFrom(start *node, path string, followLast bool, depth int, honorAbs bool) (located, error) {
	if depth > maxLinks {
		return located{}, syscall.ELOOP
	}
	cur := start
	if honorAbs && filepath.IsAbs(path) {
		cur = f.root
	}
	parts := components(path)
	if len(parts) == 0 {
		return located{node: cur, parent: cur.parent}, nil
	}
	var parent *node
	for i, part := range parts {
		last := i == len(parts)-1
		switch part {
		case "", ".":
			if last {
				return located{node: cur, parent: cur.parent}, nil
			}
			continue
		case "..":
			if cur.parent != nil {
				cur = cur.parent
			}
			if last {
				return located{node: cur, parent: cur.parent}, nil
			}
			continue
		}
		if cur.kind != kindDir {
			return located{}, syscall.ENOTDIR
		}
		child := f.get(cur, part)
		if child == nil {
			return located{}, fs.ErrNotExist
		}
		if child.kind == kindLink && (!last || followLast) {
			next, err := f.follow(child, depth)
			if err != nil {
				return located{}, err
			}
			cur = next
			parent = cur.parent
			if last {
				return located{node: cur, parent: parent}, nil
			}
			continue
		}
		parent = cur
		cur = child
		if last {
			return located{node: cur, parent: parent}, nil
		}
	}
	return located{node: cur, parent: parent}, nil
}

func (f *FS) follow(link *node, depth int) (*node, error) {
	if depth >= maxLinks {
		return nil, syscall.ELOOP
	}
	target := string(link.data)
	var (
		loc located
		err error
	)
	if filepath.IsAbs(target) {
		loc, err = f.walkFrom(f.root, target, true, depth+1, true)
	} else {
		dir := link.parent
		if dir == nil {
			dir = f.root
		}
		loc, err = f.walkFrom(dir, target, true, depth+1, false)
	}
	if err != nil {
		return nil, err
	}
	return loc.node, nil
}

func (f *FS) parentOf(path string) (*node, string, error) {
	dir, base := splitParent(path)
	if base == "" || base == "." || base == ".." {
		return nil, "", syscall.EINVAL
	}
	loc, err := f.walk(dir, true)
	if err != nil {
		return nil, "", err
	}
	if loc.node.kind != kindDir {
		return nil, "", syscall.ENOTDIR
	}
	return loc.node, base, nil
}

func (f *FS) get(dir *node, name string) *node {
	if dir == nil || dir.children == nil {
		return nil
	}
	return dir.children[f.key(name)]
}

func (f *FS) add(dir, child *node) {
	if dir.children == nil {
		dir.children = map[string]*node{}
	}
	dir.children[f.key(child.name)] = child
	child.parent = dir
}

func (f *FS) drop(dir, child *node) {
	if dir == nil || dir.children == nil || child == nil {
		return
	}
	delete(dir.children, f.key(child.name))
	child.parent = nil
}

func (f *FS) remove(parent, n *node) {
	if n.kind == kindDir {
		kids := make([]*node, 0, len(n.children))
		for _, child := range n.children {
			kids = append(kids, child)
		}
		for _, child := range kids {
			f.remove(n, child)
		}
	}
	f.drop(parent, n)
}

func (f *FS) key(name string) string {
	if f.fold {
		return strings.ToLower(name)
	}
	return name
}

func contains(dir, n *node) bool {
	for n != nil {
		if n == dir {
			return true
		}
		n = n.parent
	}
	return false
}

func (n *node) info(name string) fileInfo {
	size := int64(len(n.data))
	if n.kind == kindDir {
		size = 0
	}
	return fileInfo{name: name, mode: n.mode, size: size}
}

type fileInfo struct {
	name string
	mode fs.FileMode
	size int64
}

func (fi fileInfo) Name() string       { return fi.name }
func (fi fileInfo) Size() int64        { return fi.size }
func (fi fileInfo) Mode() fs.FileMode  { return fi.mode }
func (fi fileInfo) ModTime() time.Time { return time.Time{} }
func (fi fileInfo) IsDir() bool        { return fi.mode.IsDir() }
func (fi fileInfo) Sys() any           { return nil }

type dirEntry struct{ fileInfo }

func (d dirEntry) Name() string               { return d.name }
func (d dirEntry) IsDir() bool                { return d.mode.IsDir() }
func (d dirEntry) Type() fs.FileMode          { return d.mode.Type() }
func (d dirEntry) Info() (fs.FileInfo, error) { return d.fileInfo, nil }

func components(path string) []string {
	sep := string(filepath.Separator)
	trimmed := strings.Trim(path, sep)
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, sep)
}

func splitParent(path string) (string, string) {
	if path == "" {
		return ".", ""
	}
	sep := byte(filepath.Separator)
	end := len(path)
	for end > 1 && path[end-1] == sep {
		end--
	}
	path = path[:end]
	i := strings.LastIndexByte(path, sep)
	if i < 0 {
		return ".", path
	}
	base := path[i+1:]
	if i == 0 {
		return string(filepath.Separator), base
	}
	return path[:i], base
}

func baseName(path string) string { return filepath.Base(path) }

func pathErr(op, path string, err error) error {
	return &os.PathError{Op: op, Path: path, Err: err}
}

func linkErr(old, new string, err error) error {
	return &os.LinkError{Op: "rename", Old: old, New: new, Err: err}
}
