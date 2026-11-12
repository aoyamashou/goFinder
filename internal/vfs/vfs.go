// Package vfs lists directories: the local file system, and zip archives
// as read-only folders (see zip.go).
package vfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Entry is one item of a directory listing.
type Entry struct {
	Name    string
	Path    string
	IsDir   bool // a directory, or a symlink to one
	IsLink  bool
	Size    int64
	ModTime time.Time
	Mode    fs.FileMode
}

// Hidden reports a dot file.
func (e Entry) Hidden() bool { return strings.HasPrefix(e.Name, ".") }

// Ext is the lowercase extension without the dot ("" for directories).
func (e Entry) Ext() string {
	if e.IsDir {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(e.Name), "."))
}

// ReadDir lists dir, a folder or a folder inside an archive. Entries that
// vanish while it reads are skipped.
func ReadDir(dir string) ([]Entry, error) {
	if a, inner, ok := SplitZip(dir); ok {
		return readZipDir(a, inner)
	}
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(des))
	for _, de := range des {
		info, err := de.Info() // lstat
		if err != nil {
			continue
		}
		e := Entry{
			Name:    de.Name(),
			Path:    filepath.Join(dir, de.Name()),
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Mode:    info.Mode(),
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			e.IsLink = true
			if st, err := os.Stat(e.Path); err == nil && st.IsDir() {
				e.IsDir = true
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// Subdirs returns the names of the directories in dir, hidden ones only
// when hidden is true. It is meant for menus and completion, so it skips
// the per-entry stat ReadDir does (symlinked directories are left out).
func Subdirs(dir string, hidden bool) []string {
	if a, inner, ok := SplitZip(dir); ok {
		es, _ := readZipDir(a, inner)
		var out []string
		for _, e := range es {
			if e.IsDir && (hidden || !e.Hidden()) {
				out = append(out, e.Name)
			}
		}
		slices.Sort(out)
		return out
	}
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, de := range des {
		if !de.IsDir() || (!hidden && strings.HasPrefix(de.Name(), ".")) {
			continue
		}
		out = append(out, de.Name())
	}
	return out
}

// ExpandHome turns a leading "~" into the home directory.
func ExpandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[1:])
		}
	}
	return p
}

// Complete returns completions of a typed path: directories in the typed
// path's parent whose names start with its last element (case-insensitive),
// as full paths ending in "/". "~" is kept as typed.
func Complete(typed string, hidden bool) []string {
	if typed == "" {
		return nil
	}
	exp := ExpandHome(typed)
	parent, prefix := exp, ""
	if !strings.HasSuffix(exp, "/") {
		parent, prefix = filepath.Dir(exp), filepath.Base(exp)
	}
	// What the user typed up to the last slash, to keep "~/" as typed.
	typedParent := typed[:strings.LastIndex(typed, "/")+1]
	if !strings.Contains(typed, "/") {
		typedParent = ""
	}
	lp := strings.ToLower(prefix)
	var out []string
	for _, name := range Subdirs(parent, strings.HasPrefix(prefix, ".")) {
		if strings.HasPrefix(strings.ToLower(name), lp) {
			out = append(out, typedParent+name+"/")
		}
	}
	return out
}

// Crumbs splits an absolute path into breadcrumb items: "/" first, then each
// element. The full path of crumb i is CrumbPath(crumbs, i).
func Crumbs(p string) []string {
	p = filepath.Clean(p)
	if p == "/" {
		return []string{"/"}
	}
	return append([]string{"/"}, strings.Split(strings.TrimPrefix(p, "/"), "/")...)
}

// CrumbPath is the path of crumbs[:i+1].
func CrumbPath(crumbs []string, i int) string {
	if i <= 0 {
		return "/"
	}
	return "/" + filepath.Join(crumbs[1:i+1]...)
}
