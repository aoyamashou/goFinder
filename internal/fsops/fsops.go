// Package fsops copies, moves and creates files. Moving to the Trash is
// the app's job (mygo.Shell.TrashItem), so nothing here deletes. Archives
// are read-only: copying out of one extracts, anything into one fails.
package fsops

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"gofinder/internal/vfs"
)

// UniqueName returns a path in dir for name that does not exist yet,
// adding " 2", " 3", … before the extension as Finder does.
func UniqueName(dir, name string) string {
	p := filepath.Join(dir, name)
	if _, err := os.Lstat(p); errors.Is(err, fs.ErrNotExist) {
		return p
	}
	ext := filepath.Ext(name)
	if strings.HasPrefix(name, ".") && strings.Count(name, ".") == 1 {
		ext = "" // ".env" has no extension
	}
	base := strings.TrimSuffix(name, ext)
	for i := 2; ; i++ {
		p = filepath.Join(dir, fmt.Sprintf("%s %d%s", base, i, ext))
		if _, err := os.Lstat(p); errors.Is(err, fs.ErrNotExist) {
			return p
		}
	}
}

// Progress is called after each source item is done.
type Progress func(done, total int, current string)

// within reports whether p is dir or inside it.
func within(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}

// Copy copies each source into dstDir, renaming on name clashes. It returns
// the new paths. Copying a folder into itself is refused; a source inside
// an archive is extracted.
func Copy(srcs []string, dstDir string, prog Progress) ([]string, error) {
	if vfs.InZip(dstDir) {
		return nil, vfs.ErrReadOnly
	}
	var out []string
	for i, src := range srcs {
		inZip := vfs.InZip(src)
		if !inZip && within(dstDir, src) {
			return out, fmt.Errorf("cannot copy %q into itself", filepath.Base(src))
		}
		dst := UniqueName(dstDir, filepath.Base(src))
		copy := copyAny
		if inZip {
			copy = vfs.Extract
		}
		if err := copy(src, dst); err != nil {
			return out, err
		}
		out = append(out, dst)
		if prog != nil {
			prog(i+1, len(srcs), src)
		}
	}
	return out, nil
}

// Move moves each source into dstDir, renaming on name clashes; across
// volumes it copies then removes the source. Moving into the same folder
// does nothing.
func Move(srcs []string, dstDir string, prog Progress) ([]string, error) {
	if vfs.InZip(dstDir) || slices.ContainsFunc(srcs, vfs.InZip) {
		return nil, vfs.ErrReadOnly
	}
	var out []string
	for i, src := range srcs {
		if filepath.Dir(src) == filepath.Clean(dstDir) {
			out = append(out, src)
			continue
		}
		if within(dstDir, src) {
			return out, fmt.Errorf("cannot move %q into itself", filepath.Base(src))
		}
		dst := UniqueName(dstDir, filepath.Base(src))
		err := os.Rename(src, dst)
		if errors.Is(err, syscall.EXDEV) {
			if err = copyAny(src, dst); err == nil {
				err = os.RemoveAll(src)
			}
		}
		if err != nil {
			return out, err
		}
		out = append(out, dst)
		if prog != nil {
			prog(i+1, len(srcs), src)
		}
	}
	return out, nil
}

// Rename renames p to newName in the same folder, refusing to overwrite.
func Rename(p, newName string) (string, error) {
	if newName == "" || strings.ContainsRune(newName, '/') {
		return p, fmt.Errorf("invalid name %q", newName)
	}
	dst := filepath.Join(filepath.Dir(p), newName)
	if dst == p {
		return p, nil
	}
	// A case-only rename on a case-insensitive volume finds dst "existing".
	if _, err := os.Lstat(dst); err == nil && !strings.EqualFold(dst, p) {
		return p, fmt.Errorf("%q already exists", newName)
	}
	return dst, os.Rename(p, dst)
}

// NewFolder creates a uniquely named folder in dir.
func NewFolder(dir, name string) (string, error) {
	p := UniqueName(dir, name)
	return p, os.Mkdir(p, 0o755)
}

// NewFile creates a uniquely named file in dir with content.
func NewFile(dir, name string, content []byte) (string, error) {
	p := UniqueName(dir, name)
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return p, err
	}
	_, err = f.Write(content)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return p, err
}

func copyAny(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	case info.IsDir():
		if err := os.Mkdir(dst, info.Mode().Perm()|0o700); err != nil {
			return err
		}
		des, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, de := range des {
			if err := copyAny(filepath.Join(src, de.Name()), filepath.Join(dst, de.Name())); err != nil {
				return err
			}
		}
		return os.Chtimes(dst, info.ModTime(), info.ModTime())
	default:
		return copyFile(src, dst, info)
	}
}

func copyFile(src, dst string, info fs.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}
