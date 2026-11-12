package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestQuestionMarkOpensHelp presses ? as macOS reports it: its
// charactersIgnoringModifiers keep Shift, so the key is "?", which MyGo
// names no Key for (KeyUnknown), with Slash only as the physical key.
func TestQuestionMarkOpensHelp(t *testing.T) {
	a, tt, _, _ := setup(t)
	tt.KeyAt(ui.Shift, ui.KeySlash)
	tt.Type("?")
	tt.Frame()
	if !a.helpOpen {
		t.Fatalf("? did not open the cheat sheet")
	}
}

// TestEnterRenamesRightOpens: ↩ renames, → enters a folder or opens a
// file, o does nothing, and a double click still opens.
func TestEnterRenamesRightOpens(t *testing.T) {
	a, tt, fs, root := setup(t)
	for cursorName(a) != "notes.md" {
		tt.Key(0, ui.KeyDown)
		tt.Frame()
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if !a.dlgOpen || a.dlg != dlgRename || a.dlgText != "notes.md" || len(fs.opened) != 0 {
		t.Fatalf("↩: dialog %v kind %d text %q opened %v", a.dlgOpen, a.dlg, a.dlgText, fs.opened)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.dlgOpen {
		t.Fatal("Esc did not close the rename dialog")
	}
	tt.TypeKey(0, ui.KeyO, "o")
	tt.Frame()
	if len(fs.opened) != 0 || a.dlgOpen {
		t.Fatalf("o did something: opened %v", fs.opened)
	}
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if len(fs.opened) != 1 || fs.opened[0] != filepath.Join(root, "notes.md") {
		t.Fatalf("→ on a file: opened %v", fs.opened)
	}
	r, ok := tt.Find("report-1.txt")
	if !ok {
		t.Fatal("report-1.txt not shown")
	}
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if len(fs.opened) != 2 || filepath.Base(fs.opened[1]) != "report-1.txt" || a.dlgOpen {
		t.Fatalf("double click: opened %v dialog %v", fs.opened, a.dlgOpen)
	}
}

// TestTrashConfirmation: d asks first; ↩ trashes, Esc keeps; 不再提醒
// writes confirmTrash=false, after which d trashes at once.
func TestTrashConfirmation(t *testing.T) {
	a, tt, fs, root := setup(t)
	a.dataDir = t.TempDir()
	toName := func(name string) {
		for i := 0; cursorName(a) != name && i < 20; i++ {
			tt.Key(0, ui.KeyDown)
			tt.Frame()
		}
	}
	toName("notes.md")

	// Esc cancels: nothing moves.
	tt.TypeKey(0, ui.KeyD, "d")
	tt.Frame()
	if !a.dlgOpen || a.dlg != dlgTrash || !tt.HasText("将 1 个项目移到废纸篓？") || !tt.HasText("notes.md") || !tt.HasText("不再提醒") {
		t.Fatalf("d: dialog %v kind %d %q", a.dlgOpen, a.dlg, tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.dlgOpen || len(fs.trashed) != 0 {
		t.Fatalf("Esc: dialog %v trashed %v", a.dlgOpen, fs.trashed)
	}
	if _, err := os.Stat(filepath.Join(root, "notes.md")); err != nil {
		t.Fatal(err)
	}

	// ↩ confirms.
	tt.TypeKey(0, ui.KeyD, "d")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.dlgOpen || len(fs.trashed) != 1 || filepath.Base(fs.trashed[0]) != "notes.md" {
		t.Fatalf("↩: dialog %v trashed %v", a.dlgOpen, fs.trashed)
	}
	if !a.cfg.ConfirmTrash {
		t.Fatal("confirming turned the confirmation off")
	}

	// Several: at most five names, then how many more.
	for i := range 7 {
		os.WriteFile(filepath.Join(root, fmt.Sprintf("z%d.txt", i)), nil, 0o644)
	}
	a.load(a.tab())
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyBackspace)
	tt.Frame()
	n := a.tab().sel.Len()
	if !tt.HasText(fmt.Sprintf("将 %d 个项目移到废纸篓？", n)) || !tt.HasText(fmt.Sprintf("…还有 %d 项", n-5)) {
		t.Fatalf("⌘⌫ on %d: %q", n, tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	a.tab().sel.Clear()

	// 不再提醒: saved off, and the next d trashes at once.
	toName("report-1.txt")
	tt.TypeKey(0, ui.KeyD, "d")
	tt.Frame()
	if err := tt.Click("不再提醒"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.cfg.ConfirmTrash || len(fs.trashed) != 2 {
		t.Fatalf("不再提醒: confirm %v trashed %v", a.cfg.ConfirmTrash, fs.trashed)
	}
	if cfg := loadConfig(a.dataDir); cfg.ConfirmTrash {
		t.Fatal("config.json still says confirmTrash")
	}
	toName("report-2.txt")
	tt.TypeKey(0, ui.KeyD, "d")
	tt.Frame()
	if a.dlgOpen || len(fs.trashed) != 3 || filepath.Base(fs.trashed[2]) != "report-2.txt" {
		t.Fatalf("d after 不再提醒: dialog %v trashed %v", a.dlgOpen, fs.trashed)
	}

	// The settings' 通用 page turns it back on.
	a.run("settings")
	tt.Frame()
	if err := tt.Click("移到废纸篓前确认（d、⌘⌫）"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.cfg.ConfirmTrash || !loadConfig(a.dataDir).ConfirmTrash {
		t.Fatal("settings did not turn the confirmation back on")
	}
}

// TestConfirmTrashDefault: a config.json without confirmTrash asks.
func TestConfirmTrashDefault(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"terminal":"Ghostty"}`), 0o644)
	if !loadConfig(dir).ConfirmTrash {
		t.Fatal("missing confirmTrash is not on")
	}
}

// TestTwoRowStatusBar: the upper row has the last message,
// the view switch and the inspector, and no hidden items' switch; the
// lower row has the hints, ↩ 重命名 among them.
func TestTwoRowStatusBar(t *testing.T) {
	a, tt, _, _ := setup(t)
	a.status = "已复制 3 项"
	tt.Frame()
	top, ok1 := tt.Find("已复制 3 项")
	view, ok2 := tt.Find("左右")
	hint, ok3 := tt.Find("重命名")
	if !ok1 || !ok2 || !ok3 {
		t.Fatalf("status bar: %v %v %v %q", ok1, ok2, ok3, tt.Texts())
	}
	if d := top.Y - view.Y; d > 8 || d < -8 {
		t.Fatalf("counts (y %v) and view switch (y %v) not on one row", top.Y, view.Y)
	}
	if hint.Y <= top.Y+top.H/2 {
		t.Fatalf("hints (y %v) not under the counts (y %v)", hint.Y, top.Y)
	}
	if !tt.HasText("↩") {
		t.Fatalf("no ↩ cap: %q", tt.Texts())
	}
	// The status bar's eye is gone: "隐藏文件" is the hint alone.
	n := 0
	for _, s := range tt.Texts() {
		if s == "隐藏文件" {
			n++
		}
	}
	if n != 1 {
		t.Fatal("the status bar still has the hidden items' switch")
	}
	act, _ := a.km.Action("preview")
	if err := tt.Click(act.Title); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.showPreview {
		t.Fatal("inspector switch")
	}
	if err := tt.Click("上下"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.layout != layoutRows {
		t.Fatal("view switch")
	}
}

// TestDrawnMenus: the file menu is drawn in the window, each item with
// its action's first key at the right, 移到废纸篓 in red.
func TestDrawnMenus(t *testing.T) {
	a, tt, _, _ := setup(t)
	if err := tt.RightClick("notes.md"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	menu := tt.Menu()
	if !slices.Contains(menu, "复制") || !slices.Contains(menu, "移到废纸篓") {
		t.Fatalf("menu %q", menu)
	}
	// Drawn: its items are elements of the window, keys beside them.
	for _, k := range []string{"复制", "c", "x", "v", "d", "↩", "移到废纸篓"} {
		if !tt.HasText(k) {
			t.Fatalf("drawn menu lacks %q: %q", k, tt.Texts())
		}
	}
	red := func(label string) int {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no %q", label)
		}
		img := tt.Image()
		s := img.Bounds().Dx() / 1100
		n := 0
		for y := int(r.Y) * s; y < int(r.Y+r.H)*s; y++ {
			for x := int(r.X) * s; x < int(r.X+r.W)*s; x++ {
				cr, cg, cb, _ := img.At(x, y).RGBA()
				if cr>>8 > cg>>8+60 && cr>>8 > cb>>8+60 {
					n++
				}
			}
		}
		return n
	}
	if red("移到废纸篓") == 0 || red("复制") != 0 {
		t.Fatalf("red pixels: trash %d copy %d", red("移到废纸篓"), red("复制"))
	}
	if err := tt.Click("移到废纸篓"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.dlgOpen || a.dlg != dlgTrash {
		t.Fatalf("menu's 移到废纸篓 did not ask: %v %d", a.dlgOpen, a.dlg)
	}
}
