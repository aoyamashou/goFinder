package fsops

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"gofinder/internal/vfs"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestUniqueName(t *testing.T) {
	d := t.TempDir()
	if p := UniqueName(d, "a.txt"); p != filepath.Join(d, "a.txt") {
		t.Fatal(p)
	}
	write(t, filepath.Join(d, "a.txt"), "")
	write(t, filepath.Join(d, "a 2.txt"), "")
	if p := UniqueName(d, "a.txt"); p != filepath.Join(d, "a 3.txt") {
		t.Fatal(p)
	}
	write(t, filepath.Join(d, ".env"), "")
	if p := UniqueName(d, ".env"); p != filepath.Join(d, ".env 2") {
		t.Fatal(p)
	}
}

func TestCopyTreeAndClash(t *testing.T) {
	d := t.TempDir()
	src := filepath.Join(d, "src")
	write(t, filepath.Join(src, "x.txt"), "hello")
	write(t, filepath.Join(src, "sub", "y.txt"), "deep")
	dst := filepath.Join(d, "dst")
	os.Mkdir(dst, 0o755)
	var calls int
	out, err := Copy([]string{src, src}, dst, func(done, total int, _ string) { calls++ })
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[1] != filepath.Join(dst, "src 2") || calls != 2 {
		t.Fatalf("out %v calls %d", out, calls)
	}
	if read(t, filepath.Join(dst, "src 2", "sub", "y.txt")) != "deep" {
		t.Fatal("deep file not copied")
	}
	if _, err := Copy([]string{src}, filepath.Join(src, "sub"), nil); err == nil {
		t.Fatal("copied a folder into itself")
	}
}

func TestMove(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "a", "f.txt"), "1")
	write(t, filepath.Join(d, "b", "f.txt"), "2")
	out, err := Move([]string{filepath.Join(d, "a", "f.txt")}, filepath.Join(d, "b"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if out[0] != filepath.Join(d, "b", "f 2.txt") || read(t, out[0]) != "1" {
		t.Fatalf("moved to %v", out)
	}
	if _, err := os.Stat(filepath.Join(d, "a", "f.txt")); !os.IsNotExist(err) {
		t.Fatal("source still there")
	}
	// Same folder: nothing happens.
	out, err = Move([]string{filepath.Join(d, "b", "f.txt")}, filepath.Join(d, "b"), nil)
	if err != nil || out[0] != filepath.Join(d, "b", "f.txt") {
		t.Fatalf("same-folder move: %v %v", out, err)
	}
}

func TestRenameAndCreate(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "a.txt"), "")
	write(t, filepath.Join(d, "b.txt"), "")
	if _, err := Rename(filepath.Join(d, "a.txt"), "b.txt"); err == nil {
		t.Fatal("rename overwrote")
	}
	if _, err := Rename(filepath.Join(d, "a.txt"), "x/y"); err == nil {
		t.Fatal("slash accepted")
	}
	p, err := Rename(filepath.Join(d, "a.txt"), "c.txt")
	if err != nil || p != filepath.Join(d, "c.txt") {
		t.Fatal(p, err)
	}
	f, err := NewFile(d, "c.txt", []byte("tpl"))
	if err != nil || f != filepath.Join(d, "c 2.txt") || read(t, f) != "tpl" {
		t.Fatal(f, err)
	}
	dir, err := NewFolder(d, "untitled folder")
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(dir); !st.IsDir() {
		t.Fatal("not a dir")
	}
}

func TestCopyOutOfZipExtracts(t *testing.T) {
	d := t.TempDir()
	z := filepath.Join(d, "a.zip")
	f, _ := os.Create(z)
	w := zip.NewWriter(f)
	fw, _ := w.Create("dir/x.txt")
	fw.Write([]byte("x"))
	fw, _ = w.Create("y.txt")
	fw.Write([]byte("y"))
	w.Close()
	f.Close()
	root := vfs.ZipRoot(z)
	out, err := Copy([]string{root + "/dir", root + "/y.txt"}, d, nil)
	if err != nil || len(out) != 2 {
		t.Fatalf("copy: %v %v", out, err)
	}
	if b, _ := os.ReadFile(filepath.Join(d, "dir", "x.txt")); string(b) != "x" {
		t.Fatalf("dir/x.txt = %q", b)
	}
	if _, err := Copy([]string{filepath.Join(d, "y.txt")}, root, nil); err != vfs.ErrReadOnly {
		t.Fatalf("copy into zip: %v", err)
	}
	if _, err := Move([]string{root + "/y.txt"}, d, nil); err != vfs.ErrReadOnly {
		t.Fatalf("move out of zip: %v", err)
	}
}
