package shelf

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestAddRemoveSave(t *testing.T) {
	l, n := Add(nil, "/a", "/b/", "/a", "")
	if n != 2 || !slices.Equal(l, []string{"/a", "/b"}) {
		t.Fatalf("Add = %v %d", l, n)
	}
	l, n = Add(l, "/c", "/b")
	if n != 1 || !slices.Equal(l, []string{"/a", "/b", "/c"}) {
		t.Fatalf("Add again = %v %d", l, n)
	}
	r, n := Remove(l, "/b", "/zzz")
	if n != 1 || !slices.Equal(r, []string{"/a", "/c"}) || len(l) != 3 {
		t.Fatalf("Remove = %v %d (orig %v)", r, n, l)
	}

	f := File{Path: filepath.Join(t.TempDir(), "x", "shelf.json")}
	if got, err := f.Load(); err != nil || got != nil {
		t.Fatalf("missing: %v %v", got, err)
	}
	if err := f.Save(r); err != nil {
		t.Fatal(err)
	}
	if got, err := f.Load(); err != nil || !slices.Equal(got, r) {
		t.Fatalf("Load = %v %v", got, err)
	}
}

func TestExisting(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "f")
	os.WriteFile(p, nil, 0o644)
	if got := Existing([]string{p, filepath.Join(d, "gone")}); !slices.Equal(got, []string{p}) {
		t.Fatalf("Existing = %v", got)
	}
}
