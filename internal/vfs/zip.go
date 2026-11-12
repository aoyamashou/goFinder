package vfs

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Zip archives are browsed as read-only folders. A path inside one is the
// archive's path, "!", then the path inside it: /x/a.zip!/sub/dir. The
// archive's root is /x/a.zip! — what filepath.Clean leaves of /x/a.zip!/.

// ZipSep separates an archive from the path inside it.
const ZipSep = "!"

// IsZipName reports whether name looks like a zip archive.
func IsZipName(name string) bool { return strings.EqualFold(filepath.Ext(name), ".zip") }

// SplitZip splits p into the archive and the slash-separated path inside it
// ("" for the root). ok is false when p is not inside an archive.
func SplitZip(p string) (archive, inner string, ok bool) {
	lp := strings.ToLower(p)
	from := 0
	for {
		i := strings.Index(lp[from:], ".zip!")
		if i < 0 {
			return "", "", false
		}
		end := from + i + len(".zip")
		rest := p[end+1:]
		if rest == "" || rest[0] == '/' {
			return p[:end], strings.Trim(rest, "/"), true
		}
		from = end
	}
}

// InZip reports whether p is inside an archive (its root included).
func InZip(p string) bool { _, _, ok := SplitZip(p); return ok }

// ZipRoot is the path of an archive's root folder.
func ZipRoot(archive string) string { return archive + ZipSep }

func zipPath(archive, inner string) string {
	if inner == "" {
		return ZipRoot(archive)
	}
	return ZipRoot(archive) + "/" + inner
}

// RealDir is the folder on disk that holds p: p itself outside archives,
// else the archive's folder.
func RealDir(p string) string {
	if a, _, ok := SplitZip(p); ok {
		return filepath.Dir(a)
	}
	return p
}

// zipIndex is an archive's listing, built once per archive version.
type zipIndex struct {
	mod   time.Time
	size  int64
	dirs  map[string][]Entry // inner dir -> its entries
	files map[string]Entry   // inner path -> entry (files and dirs)
}

var zipCache struct {
	sync.Mutex
	archive string
	idx     *zipIndex
}

// cleanInner makes an archive member name a safe relative path, or "" for
// one to skip (absolute, escaping, or macOS's __MACOSX resource forks).
func cleanInner(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	p := path.Clean("/" + name)[1:]
	if p == "" || strings.HasPrefix(name, "/") || strings.Contains("/"+name+"/", "/../") {
		return ""
	}
	if p == "__MACOSX" || strings.HasPrefix(p, "__MACOSX/") {
		return ""
	}
	return p
}

func indexZip(archive string) (*zipIndex, error) {
	st, err := os.Stat(archive)
	if err != nil {
		return nil, err
	}
	zipCache.Lock()
	defer zipCache.Unlock()
	if zipCache.archive == archive && zipCache.idx.mod.Equal(st.ModTime()) && zipCache.idx.size == st.Size() {
		return zipCache.idx, nil
	}
	r, err := zip.OpenReader(archive)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	idx := &zipIndex{mod: st.ModTime(), size: st.Size(), dirs: map[string][]Entry{"": nil}, files: map[string]Entry{}}
	var addDir func(inner string, mod time.Time)
	add := func(e Entry, inner string) {
		parent := path.Dir(inner)
		if parent == "." {
			parent = ""
		}
		if parent != "" {
			addDir(parent, st.ModTime())
		}
		idx.files[inner] = e
		idx.dirs[parent] = append(idx.dirs[parent], e)
	}
	addDir = func(inner string, mod time.Time) {
		if _, ok := idx.files[inner]; ok {
			return
		}
		idx.dirs[inner] = idx.dirs[inner] // exists even when empty
		add(Entry{Name: path.Base(inner), Path: zipPath(archive, inner), IsDir: true, ModTime: mod, Mode: fs.ModeDir | 0o755}, inner)
	}
	for _, f := range r.File {
		inner := cleanInner(f.Name)
		if inner == "" {
			continue
		}
		if strings.HasSuffix(f.Name, "/") || f.FileInfo().IsDir() {
			addDir(inner, f.Modified)
			continue
		}
		if _, ok := idx.files[inner]; ok {
			continue // a duplicate name: keep the first
		}
		add(Entry{Name: path.Base(inner), Path: zipPath(archive, inner), Size: int64(f.UncompressedSize64), ModTime: f.Modified, Mode: f.Mode().Perm()}, inner)
	}
	zipCache.archive, zipCache.idx = archive, idx
	return idx, nil
}

func readZipDir(archive, inner string) ([]Entry, error) {
	idx, err := indexZip(archive)
	if err != nil {
		return nil, err
	}
	es, ok := idx.dirs[inner]
	if !ok {
		return nil, fmt.Errorf("%s: %w", zipPath(archive, inner), fs.ErrNotExist)
	}
	return append([]Entry(nil), es...), nil
}

// Stat describes p, inside an archive or not.
func Stat(p string) (Entry, error) {
	if a, inner, ok := SplitZip(p); ok {
		if inner == "" {
			if _, err := indexZip(a); err != nil {
				return Entry{}, err
			}
			st, _ := os.Stat(a)
			return Entry{Name: filepath.Base(a), Path: ZipRoot(a), IsDir: true, ModTime: st.ModTime(), Mode: fs.ModeDir | 0o755}, nil
		}
		idx, err := indexZip(a)
		if err != nil {
			return Entry{}, err
		}
		e, ok := idx.files[inner]
		if !ok {
			return Entry{}, fmt.Errorf("%s: %w", p, fs.ErrNotExist)
		}
		return e, nil
	}
	st, err := os.Stat(p)
	if err != nil {
		return Entry{}, err
	}
	return Entry{Name: filepath.Base(p), Path: p, IsDir: st.IsDir(), Size: st.Size(), ModTime: st.ModTime(), Mode: st.Mode()}, nil
}

// Open opens the file p for reading, inside an archive or not.
func Open(p string) (io.ReadCloser, error) {
	a, inner, ok := SplitZip(p)
	if !ok {
		return os.Open(p)
	}
	r, err := zip.OpenReader(a)
	if err != nil {
		return nil, err
	}
	for _, f := range r.File {
		if cleanInner(f.Name) == inner && !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				r.Close()
				return nil, err
			}
			return &zipFile{rc, r}, nil
		}
	}
	r.Close()
	return nil, fmt.Errorf("%s: %w", p, fs.ErrNotExist)
}

type zipFile struct {
	io.ReadCloser
	r *zip.ReadCloser
}

func (f *zipFile) Close() error {
	err := f.ReadCloser.Close()
	if err2 := f.r.Close(); err == nil {
		err = err2
	}
	return err
}

// Extract writes the archive member p — a file, or a folder with all it
// holds — to dst, which must not exist yet.
func Extract(p, dst string) error {
	a, inner, ok := SplitZip(p)
	if !ok {
		return fmt.Errorf("%s is not inside an archive", p)
	}
	r, err := zip.OpenReader(a)
	if err != nil {
		return err
	}
	defer r.Close()
	found := false
	for _, f := range r.File {
		name := cleanInner(f.Name)
		if name == "" {
			continue
		}
		var rel string
		switch {
		case inner == "":
			rel = name
		case name == inner:
			rel = ""
		case strings.HasPrefix(name, inner+"/"):
			rel = name[len(inner)+1:]
		default:
			continue
		}
		found = true
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if strings.HasSuffix(f.Name, "/") || f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := extractFile(f, target); err != nil {
			return err
		}
	}
	if !found {
		if inner == "" {
			return os.MkdirAll(dst, 0o755) // an empty archive
		}
		return fmt.Errorf("%s: %w", p, fs.ErrNotExist)
	}
	return nil
}

func extractFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	perm := f.Mode().Perm() | 0o600
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		os.Remove(target)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if !f.Modified.IsZero() {
		os.Chtimes(target, f.Modified, f.Modified)
	}
	return nil
}

// ErrReadOnly is returned for changes inside an archive.
var ErrReadOnly = errors.New("压缩包内只读")
