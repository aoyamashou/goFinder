package pane

import (
	"slices"
	"testing"
	"time"

	"gofinder/internal/vfs"
)

func names(es []vfs.Entry) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.Name)
	}
	return out
}

func sample() []vfs.Entry {
	t0 := time.Unix(1000, 0)
	return []vfs.Entry{
		{Name: "b.txt", Size: 30, ModTime: t0.Add(2 * time.Hour)},
		{Name: "A.md", Size: 10, ModTime: t0.Add(3 * time.Hour)},
		{Name: "zdir", IsDir: true, ModTime: t0},
		{Name: ".env", Size: 1, ModTime: t0},
		{Name: "Adir", IsDir: true, ModTime: t0.Add(time.Hour)},
	}
}

func TestSortAndHidden(t *testing.T) {
	tb := New("/x")
	tb.SetEntries(sample())
	if got := names(tb.Entries()); !slices.Equal(got, []string{"Adir", "zdir", "A.md", "b.txt"}) {
		t.Fatalf("by name: %v", got)
	}
	tb.SetSort(BySize, true)
	if got := names(tb.Entries()); !slices.Equal(got, []string{"zdir", "Adir", "b.txt", "A.md"}) {
		t.Fatalf("by size desc: %v", got)
	}
	tb.SetSort(ByName, false)
	tb.ToggleHidden()
	if got := names(tb.Entries()); !slices.Contains(got, ".env") {
		t.Fatalf("hidden not shown: %v", got)
	}
}

func TestHistory(t *testing.T) {
	tb := New("/a/b")
	tb.Go("/a/b/c")
	if !tb.Up() || tb.Dir != "/a/b" || tb.Focus != "c" {
		t.Fatalf("up: %q focus %q", tb.Dir, tb.Focus)
	}
	if !tb.Back() || tb.Dir != "/a/b/c" {
		t.Fatalf("back: %q", tb.Dir)
	}
	if !tb.Back() || tb.Dir != "/a/b" || tb.Focus != "c" {
		t.Fatalf("back 2: %q focus %q", tb.Dir, tb.Focus)
	}
	if !tb.Forward() || tb.Dir != "/a/b/c" {
		t.Fatalf("forward: %q", tb.Dir)
	}
	if tb.Go("/a/b/c") {
		t.Error("Go to same dir recorded history")
	}
	root := New("/")
	if root.Up() {
		t.Error("up from root")
	}
}

func TestFilter(t *testing.T) {
	tb := New("/x")
	tb.SetEntries(sample())
	tb.SetFilter("a") // smart case: no capital, so case is ignored
	if got := names(tb.Entries()); !slices.Equal(got, []string{"Adir", "A.md"}) {
		t.Fatalf("filter a: %v", got)
	}
	tb.SetFilter("A") // a capital: exact
	if got := names(tb.Entries()); !slices.Equal(got, []string{"Adir", "A.md"}) {
		t.Fatalf("filter A: %v", got)
	}
	tb.SetFilter("TXT")
	if got := names(tb.Entries()); len(got) != 0 {
		t.Fatalf("filter TXT: %v", got)
	}
	tb.SetFilter("txt")
	if got := names(tb.Entries()); !slices.Equal(got, []string{"b.txt"}) {
		t.Fatalf("filter txt: %v", got)
	}
	tb.SetFilter("")
	if len(tb.Entries()) != 4 {
		t.Fatalf("no filter: %v", names(tb.Entries()))
	}
	if tb.Index("zdir") != 1 || tb.Index("nope") != -1 {
		t.Error("Index wrong")
	}
}

func TestFilterKeepsHiddenRulesAndClearsOnGo(t *testing.T) {
	tb := New("/x")
	tb.SetEntries(sample())
	tb.SetFilter("e")
	if got := names(tb.Entries()); len(got) != 0 { // .env stays hidden
		t.Fatalf("hidden: %v", got)
	}
	tb.ToggleHidden()
	if got := names(tb.Entries()); !slices.Equal(got, []string{".env"}) {
		t.Fatalf("shown: %v", got)
	}
	tb.Go("/y")
	if tb.Filter != "" {
		t.Error("Go kept the filter")
	}
	tb.SetFilter("q")
	tb.Back()
	if tb.Filter != "" {
		t.Error("Back kept the filter")
	}
	tb.SetFilter("q")
	tb.Forward()
	if tb.Filter != "" {
		t.Error("Forward kept the filter")
	}
	tb.SetFilter("q")
	tb.Up()
	if tb.Filter != "" {
		t.Error("Up kept the filter")
	}
}

func TestMatchName(t *testing.T) {
	for _, c := range []struct {
		n, q string
		want bool
	}{{"Readme.MD", "", true}, {"Readme.MD", "read", true}, {"Readme.MD", "Read", true},
		{"Readme.MD", "READ", false}, {"readme", "Read", false}, {"readme", "me", true},
		{"模型查询.txt", "mxcx", true}, {"模型查询.txt", "cx", true}, {"模型查询.txt", "MXCX", false},
		{"模型查询.txt", "查询", true}, {"模型查询.txt", "mxcxt", false}, {"模型查询.txt", "mxcx.txt", true}} {
		if MatchName(c.n, c.q) != c.want {
			t.Errorf("MatchName(%q,%q) != %v", c.n, c.q, c.want)
		}
	}
}

func TestZipNavigation(t *testing.T) {
	tb := New("/x")
	tb.Go("/x/a.zip!/docs")
	if tb.Title() != "docs" {
		t.Fatalf("title %q", tb.Title())
	}
	tb.Up()
	if tb.Dir != "/x/a.zip!" || tb.Focus != "docs" || tb.Title() != "a.zip" {
		t.Fatalf("up to root: %q focus %q title %q", tb.Dir, tb.Focus, tb.Title())
	}
	tb.Up()
	if tb.Dir != "/x" || tb.Focus != "a.zip" {
		t.Fatalf("up out of zip: %q focus %q", tb.Dir, tb.Focus)
	}
	tb.Forward() // nothing: Up cleared it
	tb.Go("/x/a.zip!/docs/sub")
	tb.Go("/x")
	tb.Back()
	tb.Back()
	if tb.Dir != "/x" {
		t.Fatalf("back: %q", tb.Dir)
	}
	tb.Forward()
	tb.Back()
	if tb.Focus != "a.zip" {
		t.Fatalf("back out of zip focuses %q", tb.Focus)
	}
}
