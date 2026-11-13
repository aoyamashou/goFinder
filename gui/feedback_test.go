package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"gofinder/internal/keymap"
	"gofinder/internal/vfs"
)

func TestFeedbackCopyKeys(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		a, tt, _, _ := setup(t)
		toNotes(a, tt)
		tt.Key(0, ui.KeyC)
		tt.Frame()
		if len(a.clip) != 0 || tt.HasText("已复制 1 项") {
			t.Fatal("single copy ran before double-tap deadline")
		}
		tt.Advance(keymap.DoubleTap + time.Millisecond)
		tt.Frame()
		if !tt.HasText("已复制 1 项") || len(a.clip) != 1 {
			t.Fatalf("copy: %q %v", tt.Texts(), a.clip)
		}
	})
	t.Run("double", func(t *testing.T) {
		a, tt, _, _ := setup(t)
		toNotes(a, tt)
		tt.Key(0, ui.KeyC)
		tt.Key(0, ui.KeyC)
		tt.Frame()
		if !tt.HasText("已复制路径") || len(a.clip) != 0 || tt.HasText("已复制 1 项") {
			t.Fatal(tt.Texts(), a.clip)
		}
	})
}

func TestFeedbackEmptyAndZip(t *testing.T) {
	a, tt, _, root := setup(t)
	a.clip = []string{"previous"}
	a.clipCut = true
	a.tab().t.SetEntries(nil)
	a.tab().cursor = -1
	a.tab().sel.Clear()
	a.run("copy")
	tt.Frame()
	if len(a.clip) != 1 || a.clip[0] != "previous" || !a.clipCut || !tt.HasText("没有选中的项目") {
		t.Fatal(a.clip, a.clipCut, tt.Texts())
	}
	addZip(t, root)
	a.navigate(a.tab(), vfs.ZipRoot(filepath.Join(root, "pack.zip")))
	tt.Frame()
	a.run("cut")
	tt.Frame()
	if !tt.HasText("压缩包内不能剪切") || a.clip[0] != "previous" {
		t.Fatal(tt.Texts(), a.clip)
	}
}

func TestFeedbackStartAndHidden(t *testing.T) {
	a, tt, _, root := setup(t)
	*a.startDirOf(a.active) = root
	a.run("go.start")
	tt.Frame()
	if !tt.HasText("已在起点") {
		t.Fatal(tt.Texts())
	}
	*a.startDirOf(a.active) = filepath.Join(root, "alpha")
	a.run("go.start")
	tt.Frame()
	if !tt.HasText("已回到起点") {
		t.Fatal(tt.Texts())
	}
	*a.startDirOf(a.active) = filepath.Join(root, "missing")
	a.run("go.start")
	tt.Frame()
	if !tt.HasText("起点不可用，已回到主目录") {
		t.Fatal(tt.Texts())
	}
	a.run("hidden")
	tt.Frame()
	if !tt.HasText("已显示隐藏文件") {
		t.Fatal(tt.Texts())
	}
	a.run("hidden")
	tt.Frame()
	if !tt.HasText("已隐藏隐藏文件") {
		t.Fatal(tt.Texts())
	}
}

func TestFeedbackSortingAndRemap(t *testing.T) {
	a, tt, _, _ := setup(t)
	a.run("sortmenu")
	tt.Frame()
	if a.feedbackShowing {
		t.Fatal("opening sort displayed feedback")
	}
	if err := tt.ChooseMenuItem("名称"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.feedbackShowing {
		t.Fatal("same order displayed feedback")
	}
	a.run("sortmenu")
	tt.Frame()
	if err := tt.ChooseMenuItem("大小"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("按大小升序") {
		t.Fatal(tt.Texts())
	}
	a.buildMenu()
	a.menu.items["sort.desc"].Click(nil, nil)
	tt.Frame()
	if !tt.HasText("按大小降序") {
		t.Fatal(tt.Texts())
	}
	b, bt, _, _ := setupWith(t, testConfig(), `{"hidden":["z"]}`)
	bt.Key(0, ui.KeyZ)
	bt.Frame()
	if !b.tab().t.ShowHidden || !bt.HasText("已显示隐藏文件") {
		t.Fatal(bt.Texts())
	}
}

func TestFeedbackExpiryAndPointer(t *testing.T) {
	a, tt, _, _ := setup(t)
	a.run("hidden")
	tt.Frame()
	if !tt.HasText("已显示隐藏文件") {
		t.Fatal(tt.Texts())
	}
	tt.Advance(time.Second)
	a.run("hidden")
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if !tt.HasText("已隐藏隐藏文件") {
		t.Fatal("replacement did not reset duration")
	}
	tt.Advance(501 * time.Millisecond)
	tt.Frame()
	if tt.HasText("已隐藏隐藏文件") {
		t.Fatal("toast did not expire")
	}
	// Aim at a file row while the pass-through viewport is showing.
	a.run("hidden")
	tt.Frame()
	if err := tt.Click("notes.md"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if cursorName(a) != "notes.md" {
		t.Fatalf("click blocked: %q", cursorName(a))
	}
}

func TestFeedbackShots(t *testing.T) {
	dir := os.Getenv("MYGO_SHOTS")
	if dir == "" {
		t.Skip("MYGO_SHOTS not set")
	}
	os.MkdirAll(dir, 0755)
	for _, mode := range []string{"light", "dark"} {
		a, tt, _, _ := setup(t)
		a.cfg.Appearance = mode
		a.run("hidden")
		tt.Frame()
		tt.Frame()
		if !tt.HasText("已显示隐藏文件") {
			t.Fatal(strings.Join(tt.Texts(), ","))
		}
		writePNG(t, filepath.Join(dir, "shortcut-feedback-"+mode+".png"), tt.Image())
	}
}

func TestFeedbackClickThroughToast(t *testing.T) {
	a, tt, _, root := setup(t)
	for i := 0; i < 35; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("file-%02d.txt", i)), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	a.run("refresh")
	tt.SetSize(700, 400)
	tt.Frame()
	a.run("hidden")
	tt.Frame()
	toast, ok := tt.Find("已显示隐藏文件")
	if !ok {
		t.Fatal("missing toast")
	}
	x, y := toast.X+toast.W/2, toast.Y+toast.H/2
	var target string
	for i := 0; i < 35; i++ {
		name := fmt.Sprintf("file-%02d.txt", i)
		r, ok := tt.Find(name)
		if ok && x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
			target = name
			break
		}
	}
	if target == "" {
		t.Fatalf("no visible file label beneath toast at %.0f,%.0f", x, y)
	}
	tt.ClickAt(x, y)
	tt.Frame()
	if cursorName(a) != target {
		t.Fatalf("toast blocked %s: cursor=%s", target, cursorName(a))
	}
}

func TestFeedbackHoverBehavior(t *testing.T) {
	a, tt, _, _ := setup(t)
	a.run("hidden")
	tt.Frame()
	r, ok := tt.Find("已显示隐藏文件")
	if !ok {
		t.Fatal("missing toast")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	tt.Advance(1501 * time.Millisecond)
	tt.Frame()
	if tt.HasText("已显示隐藏文件") {
		t.Fatal("pass-through toast unexpectedly held open")
	}
}
