package vfs

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func mkTree(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	for _, p := range []string{"Docs/a", "Downloads", ".hidden", "dl-file"} {
		if p == "dl-file" {
			os.WriteFile(filepath.Join(d, p), []byte("x"), 0o644)
			continue
		}
		os.MkdirAll(filepath.Join(d, p), 0o755)
	}
	os.Symlink(filepath.Join(d, "Docs"), filepath.Join(d, "link"))
	return d
}

func TestReadDir(t *testing.T) {
	d := mkTree(t)
	es, err := ReadDir(d)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Entry{}
	for _, e := range es {
		byName[e.Name] = e
	}
	if len(es) != 5 {
		t.Fatalf("got %d entries", len(es))
	}
	if !byName["Docs"].IsDir || byName["dl-file"].IsDir || byName["dl-file"].Size != 1 {
		t.Errorf("bad entries %+v", byName)
	}
	if l := byName["link"]; !l.IsLink || !l.IsDir {
		t.Errorf("symlink to dir: %+v", l)
	}
	if !byName[".hidden"].Hidden() {
		t.Error("hidden not detected")
	}
}

func TestComplete(t *testing.T) {
	d := mkTree(t)
	got := Complete(d+"/d", false)
	slices.Sort(got)
	want := []string{d + "/Docs/", d + "/Downloads/"}
	if !slices.Equal(got, want) {
		t.Errorf("Complete = %v, want %v", got, want)
	}
	if got := Complete(d+"/.h", false); len(got) != 1 {
		t.Errorf("hidden completion = %v", got)
	}
	if got := Complete(d+"/Docs/", false); len(got) != 1 || got[0] != d+"/Docs/a/" {
		t.Errorf("trailing slash completion = %v", got)
	}
}

func TestCrumbs(t *testing.T) {
	c := Crumbs("/Users/ada/Documents")
	if !slices.Equal(c, []string{"/", "Users", "ada", "Documents"}) {
		t.Fatalf("crumbs %v", c)
	}
	if p := CrumbPath(c, 2); p != "/Users/ada" {
		t.Errorf("CrumbPath = %q", p)
	}
	if p := CrumbPath(c, 0); p != "/" {
		t.Errorf("CrumbPath root = %q", p)
	}
	if c := Crumbs("/"); len(c) != 1 {
		t.Errorf("root crumbs %v", c)
	}
}
