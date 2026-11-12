package gui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// outside makes a file in a folder of its own, as one in Finder.
func outside(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(name), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

// center is the middle of the first text s on screen.
func center(t *testing.T, tt *ui.Tester, s string) (float32, float32) {
	t.Helper()
	r, ok := tt.Find(s)
	if !ok {
		t.Fatalf("no %q on screen: %q", s, tt.Texts())
	}
	return r.X + r.W/2, r.Y + r.H/2
}

// dropAt drops src at x, y and checks it lands in dir, moved, with the
// cursor where it was.
func dropAt(t *testing.T, a *app, tt *ui.Tester, x, y float32, src, dir string) {
	t.Helper()
	cur := cursorName(a)
	if !tt.DropFiles(x, y, src) {
		t.Fatalf("drop of %s at %v,%v not taken", filepath.Base(src), x, y)
	}
	tt.Frame()
	if want := filepath.Join(dir, filepath.Base(src)); !exists(want) || exists(src) {
		t.Fatalf("%s: want it moved to %s (status %q)", filepath.Base(src), want, a.status)
	}
	if cursorName(a) != cur {
		t.Fatalf("cursor moved from %q to %q", cur, cursorName(a))
	}
}

func TestDropTargets(t *testing.T) {
	a, tt, _, root := setup(t)
	// A folder's row: into the folder.
	x, y := center(t, tt, "beta")
	dropAt(t, a, tt, x, y, outside(t, "one.txt"), filepath.Join(root, "beta"))
	// A file's row, or below the rows: into the column's folder.
	x, y = center(t, tt, "notes.md")
	dropAt(t, a, tt, x, y, outside(t, "two.txt"), root)
	dropAt(t, a, tt, x, y+300, outside(t, "three.txt"), root)
	if !tt.HasText("three.txt") {
		t.Fatalf("list not read again: %q", tt.Texts())
	}

	// In alpha, the parent column shows root: its blank space takes files.
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if a.tab().t.Dir != filepath.Join(root, "alpha") {
		t.Fatalf("in %s", a.tab().t.Dir)
	}
	x, y = center(t, tt, "report-1.txt") // a row of the parent column
	dropAt(t, a, tt, x, y+300, outside(t, "four.txt"), root)
	// The path bar's name of root.
	x, y = center(t, tt, filepath.Base(root))
	dropAt(t, a, tt, x, y, outside(t, "five.txt"), root)
	// The fourth column shows inner, the cursor's folder.
	if cursorName(a) != "inner" {
		t.Fatalf("cursor on %q", cursorName(a))
	}
	inner := filepath.Join(root, "alpha", "inner")
	r, _ := tt.Find("inside.txt")
	dropAt(t, a, tt, r.X+r.W+300, r.Y+r.H/2, outside(t, "six.txt"), inner)
}

func TestDropRefused(t *testing.T) {
	a, tt, _, root := setup(t)
	a.cacheDir = t.TempDir()
	alpha := filepath.Join(root, "alpha")
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	// alpha onto its own folder inner.
	x, y := center(t, tt, "inside.txt")
	tt.DropFiles(x-40, y-18, alpha) // the row above: inner
	tt.Frame()
	if !exists(alpha) || exists(filepath.Join(alpha, "inner", "alpha")) || !strings.Contains(a.status, "放进它自己") {
		t.Fatalf("into its own subfolder: status %q", a.status)
	}
	// Into an archive.
	tt.Key(0, ui.KeyLeft)
	tt.Frame()
	addZip(t, root)
	a.load(a.tab())
	tt.Frame()
	goTo(t, a, tt, "pack.zip")
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	src := outside(t, "seven.txt")
	x, y = center(t, tt, "top.txt")
	tt.DropFiles(x, y+300, src)
	tt.Frame()
	if !exists(src) || !strings.Contains(a.status, "只读") {
		t.Fatalf("into a zip: status %q", a.status)
	}
}

// pressDrag presses s and moves the pointer far enough to drag.
func pressDrag(t *testing.T, tt *ui.Tester, s string) {
	t.Helper()
	x, y := center(t, tt, s)
	tt.Press(x, y)
	tt.Move(x+4, y+4)
	tt.Move(x+30, y+30)
}

func TestDragOut(t *testing.T) {
	a, tt, _, root := setup(t)
	pressDrag(t, tt, "report-2.txt")
	if want := []string{filepath.Join(root, "report-2.txt")}; !slices.Equal(a.dnd.src, want) {
		t.Fatalf("dragged %v, want %v", a.dnd.src, want)
	}
	if cursorName(a) != "alpha" || a.dnd.preview == nil {
		t.Fatalf("cursor %q preview %v", cursorName(a), a.dnd.preview != nil)
	}

	// The chosen rows drag together when the row pressed is one of them.
	a, tt, _, root = setup(t)
	tt.ClickWith(ui.Cmd, "report-1.txt")
	tt.ClickWith(ui.Cmd, "notes.md")
	tt.Frame()
	pressDrag(t, tt, "report-1.txt")
	got := slices.Clone(a.dnd.src)
	slices.Sort(got)
	if want := []string{filepath.Join(root, "notes.md"), filepath.Join(root, "report-1.txt")}; !slices.Equal(got, want) {
		t.Fatalf("dragged %v, want %v", got, want)
	}
}

func TestDragOutOfZip(t *testing.T) {
	a, tt, _, root := setup(t)
	a.cacheDir = t.TempDir()
	addZip(t, root)
	a.load(a.tab())
	tt.Frame()
	goTo(t, a, tt, "pack.zip")
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	pressDrag(t, tt, "top.txt")
	if len(a.dnd.src) != 1 || !strings.HasPrefix(a.dnd.src[0], a.cacheDir) {
		t.Fatalf("dragged %v", a.dnd.src)
	}
	if b, _ := os.ReadFile(a.dnd.src[0]); string(b) != "zipped top" {
		t.Fatalf("extracted %q", b)
	}
}

func TestPathBar(t *testing.T) {
	a, tt, _, root := setup(t)
	if tt.HasText("✎") || slices.Contains(tt.Texts(), "↑") {
		t.Fatalf("old buttons still there: %q", tt.Texts())
	}
	m, ok := tt.Access("展开 " + root)
	if !ok {
		t.Fatal("no ▸ for root")
	}
	// The blank space after the last ▸ edits the path.
	tt.ClickAt(m.Bounds.X+m.Bounds.W+80, m.Bounds.Y+m.Bounds.H/2)
	tt.Frame()
	if !a.pane().addrEditing {
		t.Fatal("a click on the capsule's blank space did not edit")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.pane().addrEditing {
		t.Fatal("Escape did not end editing")
	}
	// ▸ lists the folders in it, to go to one.
	m, _ = tt.Access("展开 " + root)
	tt.ClickAt(m.Bounds.X+m.Bounds.W/2, m.Bounds.Y+m.Bounds.H/2)
	tt.Frame()
	if items := tt.Menu(); !slices.Contains(items, "alpha") || !slices.Contains(items, "beta") {
		t.Fatalf("menu %q", items)
	}
	if err := tt.ChooseMenuItem("beta"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.tab().t.Dir != filepath.Join(root, "beta") {
		t.Fatalf("went to %s", a.tab().t.Dir)
	}
	// A folder's name goes there.
	if err := tt.Click(filepath.Base(root)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.tab().t.Dir != root {
		t.Fatalf("crumb went to %s", a.tab().t.Dir)
	}
}

// A sidebar item takes files into its folder, as Finder's do.
func TestDropOnSidebarItem(t *testing.T) {
	a, tt, _, _ := setup(t)
	fav := filepath.Join(t.TempDir(), "favdrop")
	if err := os.Mkdir(fav, 0o755); err != nil {
		t.Fatal(err)
	}
	a.sb.show, a.sb.favs = true, []string{fav}
	tt.Frame()
	x, y := center(t, tt, "favdrop")
	dropAt(t, a, tt, x, y, outside(t, "seven.txt"), fav)
}
