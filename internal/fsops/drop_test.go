package fsops

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gofinder/internal/vfs"
)

func TestDropOp(t *testing.T) {
	root := t.TempDir()
	a, dst := filepath.Join(root, "a.txt"), filepath.Join(root, "dst")
	os.WriteFile(a, nil, 0o644)
	os.Mkdir(dst, 0o755)
	same := []string{a}
	if got := DropOp(same, dst, DropAuto); got != OpMove {
		t.Fatalf("same volume: %v, want move", got)
	}
	if got := DropOp(same, dst, DropCopy); got != OpCopy {
		t.Fatalf("⌥: %v, want copy", got)
	}
	if got := DropOp(same, dst, DropMove); got != OpMove {
		t.Fatalf("⌘: %v, want move", got)
	}
	// /dev is devfs: another volume than the temp folder.
	if dd, _ := device("/dev"); dd != 0 {
		if rd, _ := device(root); rd != dd {
			if got := DropOp(same, "/dev", DropAuto); got != OpCopy {
				t.Fatalf("other volume: %v, want copy", got)
			}
			if got := DropOp(same, "/dev", DropMove); got != OpMove {
				t.Fatalf("other volume with ⌘: %v, want move", got)
			}
			mixed := []string{a, "/dev/null"}
			if got := DropOp(mixed, dst, DropAuto); got != OpCopy {
				t.Fatalf("one item on another volume: %v, want copy", got)
			}
		}
	}
	z := vfs.ZipRoot(filepath.Join(root, "x.zip")) + "/in.txt"
	if got := DropOp([]string{z}, dst, DropMove); got != OpCopy {
		t.Fatalf("from an archive with ⌘: %v, want copy", got)
	}
	if got := DropOp(same, filepath.Join(root, "gone"), DropAuto); got != OpCopy {
		t.Fatalf("unreadable destination: %v, want copy", got)
	}
}

func TestCheckDrop(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "dir")
	if err := CheckDrop([]string{dir}, dir); err == nil {
		t.Fatal("into itself allowed")
	}
	if err := CheckDrop([]string{dir}, filepath.Join(dir, "sub")); err == nil {
		t.Fatal("into its own subfolder allowed")
	}
	if err := CheckDrop([]string{dir}, root+"/dir2"); err != nil {
		t.Fatalf("sibling with a shared prefix refused: %v", err)
	}
	if err := CheckDrop([]string{dir}, vfs.ZipRoot(filepath.Join(root, "a.zip"))); !errors.Is(err, vfs.ErrReadOnly) {
		t.Fatalf("into an archive: %v", err)
	}
	if err := CheckDrop([]string{filepath.Join(root, "f")}, root); err != nil {
		t.Fatalf("plain drop refused: %v", err)
	}
}
