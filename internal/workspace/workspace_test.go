package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := State{
		Layout: "dual", Active: 1, ShowPreview: true, ShowShelf: true,
		Panes: []Pane{
			{Tabs: []Tab{{Dir: "/a"}, {Dir: "/b", Sort: "size", Desc: true}}, Cur: 1},
			{Tabs: []Tab{{Dir: "/x.zip!/sub", Sort: "date"}}},
		},
	}
	p := Path(dir, "工作")
	if err := Write(p, s); err != nil {
		t.Fatal(err)
	}
	got, err := Read(p)
	if err != nil || !reflect.DeepEqual(got, s) {
		t.Fatalf("Read = %+v %v", got, err)
	}
	Write(Path(dir, "b"), s)
	os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0o644)
	if l := List(dir); !slices.Equal(l, []string{"b", "工作"}) {
		t.Fatalf("List = %v", l)
	}
}

func TestValidFixes(t *testing.T) {
	s := State{Active: 5, Panes: []Pane{{Cur: 3, Tabs: []Tab{{Dir: ""}, {Dir: "/a"}}}, {}}}
	if !s.Valid() || len(s.Panes) != 1 || s.Panes[0].Cur != 0 || s.Active != 0 || len(s.Panes[0].Tabs) != 1 {
		t.Fatalf("Valid fixed to %+v", s)
	}
	empty := State{Panes: []Pane{{}}}
	if empty.Valid() {
		t.Fatal("empty state valid")
	}
	p := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(p, []byte(`{"panes":[]}`), 0o644)
	if _, err := Read(p); err == nil {
		t.Fatal("Read of empty state did not fail")
	}
}

func TestCleanName(t *testing.T) {
	for in, want := range map[string]string{" a/b ": "a-b", "..x": "x", "  ": "", "前端:1": "前端-1"} {
		if got := CleanName(in); got != want {
			t.Errorf("CleanName(%q) = %q, want %q", in, got, want)
		}
	}
}
