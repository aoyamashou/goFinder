package gui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"gofinder/internal/keymap"
)

func TestTrashUndoKeyAndFeedback(t *testing.T) {
	a, tt, sys, root := setup(t)
	toNotes(a, tt)
	p := filepath.Join(root, "notes.md")
	a.trash(a.tab())
	tt.Frame()
	if sys.trashSounds != 1 || !tt.HasText("已移到废纸篓 1 项") || len(a.undoSteps) != 1 {
		t.Fatal(a.status, sys.trashSounds, len(a.undoSteps))
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	if _, err := os.Stat(p); err != nil {
		t.Fatal("Cmd+Z failed", err, a.status)
	}
	if len(a.undoSteps) != 0 || sys.trashSounds != 1 {
		t.Fatal("undo history/sound", a.undoSteps, sys.trashSounds)
	}
	a.helpOpen = true
	a.helpQuery = "撤销"
	tt.Frame()
	if !tt.HasText("撤销文件操作") {
		t.Fatal("cheat sheet missing undo", tt.Texts())
	}
	if key := a.firstKey("undo"); key != keymap.MustParse("Cmd+Z").String() {
		t.Fatal(key)
	}
	a.buildMenu()
	if !strings.HasPrefix(a.menu.items["settings"].Label, "设置...") || roles["@undo"].covers != "undo" {
		t.Fatal("menu wiring")
	}
}

func TestTrashPartialFailureAndRetryUndo(t *testing.T) {
	a, tt, sys, root := setup(t)
	toNotes(a, tt)
	p, q := filepath.Join(root, "notes.md"), filepath.Join(root, "report-1.txt")
	a.tab().sel.Add(p)
	a.tab().sel.Add(q)
	sys.trashFail = map[string]error{q: errors.New("no permission")}
	a.trash(a.tab())
	if sys.trashSounds != 1 || !strings.Contains(a.status, "失败") || len(a.undoSteps) != 1 {
		t.Fatal(a.status)
	}
	if err := os.WriteFile(p, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	a.undoFiles()
	if len(a.undoSteps) != 1 || !strings.Contains(a.status, "失败") {
		t.Fatal("conflict consumed history", a.status)
	}
	if err := os.Rename(p, p+".conflict"); err != nil {
		t.Fatal(err)
	}
	a.undoFiles()
	if len(a.undoSteps) != 0 {
		t.Fatal("retry did not finish", a.status)
	}
	if data, err := os.ReadFile(p); err != nil || !strings.HasPrefix(string(data), "hello") {
		t.Fatal(string(data), err)
	}
	toNotes(a, tt)
	sys.trashFail[p] = errors.New("failed")
	a.trash(a.tab())
	if sys.trashSounds != 1 {
		t.Fatal("all-failed batch sounded")
	}
}

func TestUndoNewCopyMoveRenameAndBusy(t *testing.T) {
	a, _, _, root := setup(t)
	a.run("newfile")
	a.popOpen = false
	a.openDialog(dlgNewFile, a.tab(), "new.txt", "")
	a.dlgTpl = blankTemplate
	a.submitDialog()
	p := filepath.Join(root, "new.txt")
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
	a.undoFiles()
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("new file not undone", err)
	}
	p = filepath.Join(root, "notes.md")
	a.openDialog(dlgRename, a.tab(), "renamed.md", p)
	a.submitDialog()
	a.undoFiles()
	if _, err := os.Stat(p); err != nil {
		t.Fatal("rename undo", err)
	}
	for _, move := range []bool{false, true} {
		a.clip, a.clipCut = []string{p}, move
		a.paste(a.tab(), filepath.Join(root, "beta"))
		a.undoFiles()
		if _, err := os.Stat(p); err != nil {
			t.Fatal("source lost", err)
		}
		if _, err := os.Stat(filepath.Join(root, "beta", "notes.md")); !os.IsNotExist(err) {
			t.Fatal("destination remained", err)
		}
	}
	a.fileOps = 1
	a.undoFiles()
	if a.toast != "有文件操作进行中" {
		t.Fatal(a.toast)
	}
	a.fileOps = 0
	a.undoing = true
	if a.beginFileOp() {
		t.Fatal("started operation during undo")
	}
	a.undoing = false
}

func TestUndoTextInputAndRemap(t *testing.T) {
	a, tt, _, root := setup(t)
	toNotes(a, tt)
	a.trash(a.tab())
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyL)
	tt.Frame()
	tt.Type("some-text")
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	if len(a.undoSteps) != 1 {
		t.Fatal("text undo consumed file history")
	}
	if _, err := os.Stat(filepath.Join(root, "notes.md")); !os.IsNotExist(err) {
		t.Fatal("text undo restored file")
	}
	warnings := a.km.Override(map[string][]string{"undo": {"Cmd+Alt+U"}})
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	if a.firstKey("undo") != keymap.MustParse("Cmd+Alt+U").String() {
		t.Fatal("remap ignored")
	}
}

func TestTrashFeedbackShots(t *testing.T) {
	dir := os.Getenv("MYGO_SHOTS")
	if dir == "" {
		t.Skip("MYGO_SHOTS not set")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"light", "dark"} {
		a, tt, _, _ := setup(t)
		a.cfg.Appearance = mode
		toNotes(a, tt)
		a.trash(a.tab())
		tt.Frame()
		tt.Frame()
		writePNG(t, filepath.Join(dir, "trash-feedback-"+mode+".png"), tt.Image())
	}
}
