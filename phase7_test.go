package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/keymap"
)

func toNotes(a *app, tt *ui.Tester) {
	a.tab().cursor = a.tab().t.Index("notes.md")
	tt.Frame()
}

// Each single Yazi key reaches its action.
func TestYaziKeys(t *testing.T) {
	a, tt, fs, root := setup(t)
	toNotes(a, tt)
	press := func(mods ui.Modifiers, k ui.Key) {
		t.Helper()
		tt.Key(mods, k)
		tt.Advance(keymap.DoubleTap) // a key that may be doubled waits
		tt.Frame()
	}

	press(0, ui.KeyC)
	if len(a.clip) != 1 || a.clipCut || filepath.Base(a.clip[0]) != "notes.md" {
		t.Fatalf("c: clip %v cut %v", a.clip, a.clipCut)
	}
	press(0, ui.KeyX)
	if !a.clipCut {
		t.Fatal("x did not cut")
	}
	a.clipCut = false // paste as a copy
	press(0, ui.KeyV)
	if _, err := os.Stat(filepath.Join(root, "notes 2.md")); err != nil {
		entries, _ := os.ReadDir(root)
		t.Fatalf("v: %v %v (status %q)", err, entries, a.status)
	}
	press(0, ui.KeyPeriod)
	if !a.tab().t.ShowHidden {
		t.Fatal(". did not show hidden files")
	}
	press(0, ui.KeyPeriod)
	press(0, ui.KeyComma)
	if !a.popOpen || a.pop != "sort" || !tt.HasText("修改日期") {
		t.Fatalf(",: pop %v %q", a.popOpen, a.pop)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	a.popOpen = false
	tt.Frame()
	press(0, ui.KeySemicolon)
	if len(fs.openedWith) != 1 || fs.openedWith[0][0] != "Terminal" {
		t.Fatalf("; : %v", fs.openedWith)
	}
	press(ui.Cmd, ui.KeyA)
	if a.tab().sel.Len() != len(a.tab().t.Entries()) {
		t.Fatalf("⌘A selected %d", a.tab().sel.Len())
	}
	press(0, ui.KeyEscape)
	if a.tab().sel.Len() != 0 {
		t.Fatalf("Esc left %d selected", a.tab().sel.Len())
	}
	press(0, ui.KeyG)
	if !a.pane().addrEditing {
		t.Fatal("g did not open the address")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	toNotes(a, tt)
	press(0, ui.KeyR)
	if !a.dlgOpen || a.dlg != dlgRename {
		t.Fatalf("r: dialog %v %d", a.dlgOpen, a.dlg)
	}
	a.dlgOpen = false
	tt.Frame()
	tt.Frame()
	toNotes(a, tt)
	press(0, ui.KeyD)
	press(0, ui.KeyEnter) // the confirmation
	if len(fs.trashed) != 1 || filepath.Base(fs.trashed[0]) != "notes.md" {
		t.Fatalf("d: %v", fs.trashed)
	}
	press(ui.Shift, ui.KeySlash)
	if !a.helpOpen {
		t.Fatal("? did not open the cheat sheet")
	}
	a.helpOpen = false
	tt.Frame()
	tt.Frame()
	press(0, ui.KeySlash)
	if a.filterOn {
		t.Fatal("/ still does something")
	}
}

// n and n n: the double tap, the time running out, and another key.
func TestDoubleTapKeys(t *testing.T) {
	a, tt, _, root := setup(t)
	a.tplDir = ""

	// n n: the new menu, at once.
	tt.Key(0, ui.KeyN)
	tt.Key(0, ui.KeyN)
	tt.Frame()
	if !a.popOpen || a.pop != "new" || !tt.HasText(blankTemplate) || a.dlgOpen {
		t.Fatalf("n n: pop %v %q dialog %v", a.popOpen, a.pop, a.dlgOpen)
	}
	if err := tt.Click(blankTemplate); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.dlgOpen || a.dlg != dlgNewFile {
		t.Fatalf("blank file: dialog %v %d", a.dlgOpen, a.dlg)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if _, err := os.Stat(filepath.Join(root, "未命名.txt")); err != nil {
		t.Fatalf("new file: %v", err)
	}

	// n alone: nothing until the time runs out, then a new folder.
	tt.Frame()
	tt.Key(0, ui.KeyN)
	tt.Frame()
	if a.dlgOpen {
		t.Fatal("n ran before its double tap could come")
	}
	tt.Advance(keymap.DoubleTap + 10*time.Millisecond)
	if !a.dlgOpen || a.dlg != dlgNewFolder {
		t.Fatalf("n timeout: dialog %v %d", a.dlgOpen, a.dlg)
	}
	a.dlgOpen = false
	tt.Frame()
	tt.Frame()

	// c then v at once: c's copy first, then v's paste.
	toNotes(a, tt)
	tt.Key(0, ui.KeyC)
	tt.Key(0, ui.KeyV)
	tt.Frame()
	if _, err := os.Stat(filepath.Join(root, "notes 2.md")); err != nil {
		t.Fatalf("c v: %v (clip %v status %q)", err, a.clip, a.status)
	}

	// c c: the path.
	tt.Frame()
	toNotes(a, tt)
	a.clip = nil
	tt.Key(0, ui.KeyC)
	tt.Key(0, ui.KeyC)
	tt.Frame()
	if a.status != "已复制路径" || a.clip != nil {
		t.Fatalf("c c: status %q clip %v", a.status, a.clip)
	}
}

// Plain keys type in text fields, and are letters while Quick Select types.
func TestNoKeysWhileTyping(t *testing.T) {
	a, tt, fs, _ := setup(t)
	toNotes(a, tt)
	tt.Key(ui.Cmd, ui.KeyL)
	tt.Frame()
	tt.TypeKey(0, ui.KeyD, "d")
	tt.Advance(keymap.DoubleTap)
	tt.Frame()
	if len(fs.trashed) != 0 || !a.pane().addrEditing {
		t.Fatalf("d in the address bar: trashed %v", fs.trashed)
	}
	for a.pane().addrEditing { // the completions close first
		tt.Key(0, ui.KeyEscape)
		tt.Frame()
	}
	tt.Frame()
	tt.TypeKey(0, ui.KeyF, "f")
	tt.Frame()
	tt.TypeKey(0, ui.KeyN, "n")
	tt.Advance(keymap.DoubleTap)
	tt.Frame()
	if a.dlgOpen || a.filterText != "n" {
		t.Fatalf("n in the filter box: dialog %v text %q", a.dlgOpen, a.filterText)
	}
}
