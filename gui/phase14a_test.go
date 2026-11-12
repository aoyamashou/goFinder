package gui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/keymap"
)

// Space is Quick Look, not a check mark: the action select.toggle is gone,
// Space (⌘Y no more) opens the item under the cursor, and pressing again ends the
// open one first.
func TestQuickLook(t *testing.T) {
	a, tt, fs, root := setup(t)
	for _, ac := range actions() {
		if ac.ID == "select.toggle" {
			t.Fatal("select.toggle still there")
		}
	}
	if id, ok := a.km.Lookup(mustChord(t, "Space")); !ok || id != "quicklook" {
		t.Fatalf("Space → %q", id)
	}
	if _, ok := a.km.Lookup(mustChord(t, "Cmd+Y")); ok {
		t.Fatal("⌘Y still bound")
	}
	a.tab().cursor = 0
	name := cursorName(a)
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if len(fs.quickLooks) != 1 || len(fs.quickLooks[0]) != 1 || fs.quickLooks[0][0] != root+"/"+name {
		t.Fatalf("quick looks %q (cursor %q)", fs.quickLooks, name)
	}
	if a.tab().sel.Len() != 0 {
		t.Fatal("space selected")
	}
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if len(fs.quickLooks) != 2 || fs.quickStops != 1 {
		t.Fatalf("looks %d stops %d", len(fs.quickLooks), fs.quickStops)
	}
	a.stopQuickLook()
	if fs.quickStops != 2 {
		t.Fatalf("quit did not end it: %d", fs.quickStops)
	}
}

// The status bar says 快速查看 for Space.
func TestQuickLookHint(t *testing.T) {
	if hintTitles["quicklook"] != "快速查看" {
		t.Fatal("hint title")
	}
}

// 复制命令 puts killall Finder on the clipboard's way and never runs it.
func TestCopyKillFinder(t *testing.T) {
	a, tt, fs, _ := setupWith(t, testConfig(), "")
	tt.Key(ui.Cmd, ui.KeyComma)
	tt.Frame()
	scrollSettings(tt)
	if err := tt.Click("复制命令"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.status != "已复制，粘贴到终端运行" || len(fs.shells) != 0 {
		t.Fatalf("status %q shells %q", a.status, fs.shells)
	}
	if killFinderCmd != "killall Finder" {
		t.Fatal(killFinderCmd)
	}
}

// 完全磁盘访问 shows the grant and opens its System Settings pane.
func TestFullDiskAccess(t *testing.T) {
	a, tt, fs, _ := setupWith(t, testConfig(), "")
	tt.Key(ui.Cmd, ui.KeyComma)
	tt.Frame()
	if !tt.HasText("未授权") {
		t.Fatalf("texts %q", tt.Texts())
	}
	scrollSettings(tt)
	if err := tt.Click("打开系统设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(fs.urls) != 1 || fs.urls[0] != "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles" {
		t.Fatalf("urls %q", fs.urls)
	}
	fs.fullDisk = true
	a.dlgOpen = false
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyComma)
	tt.Frame()
	if !tt.HasText("已授权") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

func mustChord(t *testing.T, s string) keymap.Chord {
	t.Helper()
	ch, err := keymap.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return ch
}
