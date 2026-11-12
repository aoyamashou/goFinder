package gui

import (
	"os"
	"path/filepath"
	"testing"
)

// The old data folder moves to the new, empty one; a new one in use is
// left alone.
func TestMoveOld(t *testing.T) {
	root := t.TempDir()
	old, dir := filepath.Join(root, oldName), filepath.Join(root, "goFinder")
	os.MkdirAll(old, 0o755)
	os.WriteFile(filepath.Join(old, "config.json"), []byte("{}"), 0o644)
	os.MkdirAll(dir, 0o755) // made by MyGo on first use
	if besideOld(dir) != old {
		t.Fatal(besideOld(dir))
	}
	if err := moveOld(old, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Fatal("not moved")
	}
	os.MkdirAll(old, 0o755)
	os.WriteFile(filepath.Join(old, "x"), nil, 0o644)
	moveOld(old, dir)
	if _, err := os.Stat(filepath.Join(old, "x")); err != nil {
		t.Error("moved over a folder in use")
	}
	if err := moveOld(filepath.Join(root, "none"), dir); err != nil {
		t.Error(err)
	}
}
