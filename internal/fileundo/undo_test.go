package fileundo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProductRejectsChangedChildAndSymlink(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "product")
	if err := os.Mkdir(p, 0700); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(p, "child")
	if err := os.WriteFile(f, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	e, err := Record(p, "")
	if err != nil {
		t.Fatal(err)
	}
	called := false
	trash := func(string) error { called = true; return nil }
	if err := os.WriteFile(f, []byte("after"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := e.Undo(trash); err == nil || called {
		t.Fatal("modified product was trashed")
	}
	link := filepath.Join(d, "link")
	if err := os.Symlink(p, link); err != nil {
		t.Fatal(err)
	}
	e, err = Record(link, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte("target may change"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := e.Undo(trash); err != nil || !called {
		t.Fatal("followed link target", err)
	}
}

func TestRestoreConflictAndReplacement(t *testing.T) {
	d := t.TempDir()
	src, dst := filepath.Join(d, "source"), filepath.Join(d, "original")
	if err := os.WriteFile(src, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	e, err := Record(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := e.Undo(nil); err == nil {
		t.Fatal("restored over conflict")
	}
	if err := os.Rename(src, src+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := e.Undo(nil); err == nil {
		t.Fatal("restored replacement")
	}
}
