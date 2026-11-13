// Package fileundo holds bounded metadata snapshots and safe inverse operations.
// It never permanently deletes a file and never guesses a trash location.
package fileundo

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gofinder/internal/macfile"
)

// Entry restores Current to Original, or trashes Current if Original is empty.
type Entry struct {
	Current, Original string
	info              os.FileInfo
	digest            [32]byte
}

func snapshot(path string) ([32]byte, error) {
	h := sha256.New()
	n := 0
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		n++
		if n > 10000 {
			return errors.New("目录超过 10000 项，无法安全撤销")
		}
		i, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(path, p)
		if err != nil {
			return err
		}
		// Renames change ctime; adding and undoing a child changes its parent's
		// timestamps and allocation size. Neither is evidence of an outside edit.
		fmt.Fprintf(h, "%q/%s/%d", rel, macfile.Identity(i), i.Mode())
		if !i.IsDir() {
			fmt.Fprintf(h, "/%d/%d", i.Size(), i.ModTime().UnixNano())
		}
		fmt.Fprintln(h)
		return nil
	})
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum, err
}

// Record snapshots products recursively, but only keeps identity for a move.
func Record(current, original string) (Entry, error) {
	e := Entry{Current: current, Original: original}
	var err error
	e.info, err = os.Lstat(current)
	if err == nil && original == "" {
		e.digest, err = snapshot(current)
	}
	return e, err
}

// Same verifies the object against its pre-operation identity.
func Same(path string, before os.FileInfo) bool {
	i, err := os.Lstat(path)
	return err == nil && before != nil && os.SameFile(i, before)
}

func (e Entry) Undo(trash func(string) error) error {
	if !Same(e.Current, e.info) {
		return errors.New("文件已丢失或被替换：" + e.Current)
	}
	if e.Original == "" {
		d, err := snapshot(e.Current)
		if err != nil {
			return err
		}
		if d != e.digest {
			return errors.New("文件内容或属性已改变，无法安全撤销：" + e.Current)
		}
		return trash(e.Current)
	}
	if _, err := os.Lstat(e.Original); !errors.Is(err, fs.ErrNotExist) {
		return errors.New("原位置已存在项目或不可访问：" + e.Original)
	}
	return macfile.Restore(e.Current, e.Original)
}
