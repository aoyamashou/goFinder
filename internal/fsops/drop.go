package fsops

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"gofinder/internal/vfs"
)

// DropMod is the modifier held as files drop: none, Option (copy) or
// Command (move).
type DropMod int

const (
	DropAuto DropMod = iota // Finder's rule: same volume moves, another copies
	DropCopy                // ⌥
	DropMove                // ⌘
)

// Op is what a drop does with its files.
type Op int

const (
	OpCopy Op = iota
	OpMove
)

// DropOp decides, as Finder does, whether files dropped on dst move or
// copy: ⌥ copies and ⌘ moves; with neither, files on dst's volume move and
// any on another volume make them all copy. Items inside an archive only
// copy; anything unreadable copies, which never loses a file.
func DropOp(srcs []string, dst string, mod DropMod) Op {
	for _, s := range srcs {
		if vfs.InZip(s) {
			return OpCopy
		}
	}
	switch mod {
	case DropCopy:
		return OpCopy
	case DropMove:
		return OpMove
	}
	dd, ok := device(dst)
	if !ok || len(srcs) == 0 {
		return OpCopy
	}
	for _, s := range srcs {
		// The item's folder: a symlink lives on its folder's volume.
		if sd, ok := device(filepath.Dir(s)); !ok || sd != dd {
			return OpCopy
		}
	}
	return OpMove
}

func device(p string) (uint64, bool) {
	fi, err := os.Stat(p)
	if err != nil {
		return 0, false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(st.Dev), true
}

// CheckDrop refuses a drop Finder refuses: into an archive, or a folder
// into itself or a folder inside it.
func CheckDrop(srcs []string, dst string) error {
	if vfs.InZip(dst) {
		return vfs.ErrReadOnly
	}
	for _, s := range srcs {
		if !vfs.InZip(s) && within(filepath.Clean(dst), filepath.Clean(s)) {
			return fmt.Errorf("不能把「%s」放进它自己里面", filepath.Base(s))
		}
	}
	return nil
}
