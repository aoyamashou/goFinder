package gui

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// copyPathOn right-clicks name and chooses 复制路径.
func copyPathOn(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	if err := tt.RightClick(name); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.ChooseMenuItem("复制路径"); err != nil {
		t.Fatalf("%v: menu %q", err, tt.Menu())
	}
	tt.Frame()
}

// TestRightClickParentRow: a right-click on a row of the parent column
// acts on that row, not on the column's folder.
func TestRightClickParentRow(t *testing.T) {
	a, tt, _, root := setup(t)
	tt.Key(0, ui.KeyRight) // into alpha: root is the parent column
	tt.Frame()
	copyPathOn(t, tt, "beta")
	if want := filepath.Join(root, "beta"); a.copied != want {
		t.Fatalf("copied %q, want %q", a.copied, want)
	}
	if a.tab().t.Dir != root || cursorName(a) != "beta" {
		t.Fatalf("dir %s cursor %q", a.tab().t.Dir, cursorName(a))
	}
}

// TestRightClickCurrentRow: a right-click on a row not under the cursor
// acts on it.
func TestRightClickCurrentRow(t *testing.T) {
	a, tt, _, root := setup(t)
	copyPathOn(t, tt, "notes.md")
	if want := filepath.Join(root, "notes.md"); a.copied != want {
		t.Fatalf("copied %q, want %q", a.copied, want)
	}
}

// TestRightClickThirdRow: a right-click on a row of the third column acts
// on it.
func TestRightClickThirdRow(t *testing.T) {
	a, tt, _, root := setup(t)
	copyPathOn(t, tt, "inside.txt")
	if want := filepath.Join(root, "alpha", "inside.txt"); a.copied != want {
		t.Fatalf("copied %q, want %q", a.copied, want)
	}
}

// TestMouseBackForward: a mouse's side buttons and a trackpad's swipes go
// back and forward.
func TestMouseBackForward(t *testing.T) {
	a, tt, _, root := setup(t)
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	tt.Key(0, ui.KeyBack)
	tt.Frame()
	if a.tab().t.Dir != root {
		t.Fatalf("back: %s", a.tab().t.Dir)
	}
	tt.Key(0, ui.KeyForward)
	tt.Frame()
	if a.tab().t.Dir != filepath.Join(root, "alpha") {
		t.Fatalf("forward: %s", a.tab().t.Dir)
	}
}

// TestBlankClickClears: a click on the list's blank space below its rows
// chooses nothing, the cursor kept.
func TestBlankClickClears(t *testing.T) {
	a, tt, _, _ := setup(t)
	tt.ClickWith(ui.Cmd, "notes.md")
	tt.Frame()
	tt.ClickWith(ui.Cmd, "report-1.txt")
	tt.Frame()
	ts := a.tab()
	if ts.sel.Len() < 2 {
		t.Fatalf("chose %d", ts.sel.Len())
	}
	cur := cursorName(a)
	r, _ := tt.Find("report-2.txt")
	tt.ClickAt(r.X+10, r.Y+r.H+120)
	tt.Frame()
	if ts.sel.Len() != 0 {
		t.Fatalf("still chose %d", ts.sel.Len())
	}
	if cursorName(a) != cur {
		t.Fatalf("cursor %q, want %q", cursorName(a), cur)
	}
}
