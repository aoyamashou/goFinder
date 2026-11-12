package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/workspace"
)

func count(tt *ui.Tester, s string) int {
	n := 0
	for _, x := range tt.Texts() {
		if x == s {
			n++
		}
	}
	return n
}

// Each pane shows three columns, in every layout; Tab goes between them.
func TestEveryPaneHasThreeColumns(t *testing.T) {
	a, tt, _, _ := setup(t)
	for _, c := range []struct {
		id     string
		layout int
		panes  int
	}{{"layout.columns", layoutColumns, 1}, {"layout.dual", layoutDual, 2}, {"layout.rows", layoutRows, 2}} {
		a.run(c.id)
		tt.Frame()
		tt.Frame()
		if a.layout != c.layout {
			t.Fatalf("%s: layout %d", c.id, a.layout)
		}
		// The parent column, the file list and the fourth column (alpha's
		// items: the cursor is on it) in each pane.
		if count(tt, "上级文件夹") != c.panes || count(tt, "文件") != c.panes || count(tt, "子项") != c.panes {
			t.Fatalf("%s: columns %d %d %d, want %d each", c.id,
				count(tt, "上级文件夹"), count(tt, "文件"), count(tt, "子项"), c.panes)
		}
	}
	a.run("layout.dual")
	tt.Frame()
	if a.active != 0 || a.focusZone != "pane0" {
		t.Fatalf("active %d zone %q", a.active, a.focusZone)
	}
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.active != 1 || a.focusZone != "pane1" {
		t.Fatalf("Tab: active %d zone %q", a.active, a.focusZone)
	}
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.active != 0 || a.focusZone != "pane0" {
		t.Fatalf("Tab back: active %d zone %q", a.active, a.focusZone)
	}
	// The preview panel shows beside one pane only.
	a.run("preview")
	tt.Frame()
	if tt.HasText("没有选中项") || count(tt, "上级文件夹") != 2 {
		t.Fatalf("preview panel in two panes: %q", tt.Texts())
	}
}

// The single list of before is now one pane in three columns.
func TestSingleLayoutIsColumns(t *testing.T) {
	if layoutOf("single") != layoutColumns || layoutOf("dual") != layoutDual || layoutOf("rows") != layoutRows {
		t.Fatal("layoutOf")
	}
	a, tt, _, root := setup(t)
	a.run("layout.dual")
	a.applyState(workspace.State{Layout: "single", Panes: []workspace.Pane{{Tabs: []workspace.Tab{{Dir: root}}}}})
	a.loadAll()
	tt.Frame()
	if a.layout != layoutColumns || a.snapshot().Layout != "columns" {
		t.Fatalf("layout %d", a.layout)
	}
	if _, ok := a.km.Action("layout.single"); ok {
		t.Fatal("layout.single is still an action")
	}
}

// The status bar's view switch runs the layout actions.
func TestStatusBarSwitchesLayout(t *testing.T) {
	a, tt, _, _ := setup(t)
	for _, c := range []struct {
		label  string
		layout int
	}{{"左右", layoutDual}, {"上下", layoutRows}, {"三列", layoutColumns}} {
		if err := tt.Click(c.label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.layout != c.layout {
			t.Fatalf("%s: layout %d", c.label, a.layout)
		}
	}
}

// There is no toolbar: the title bar has the Agent button and the
// workspaces, and no back or new buttons.
func TestNoToolbar(t *testing.T) {
	_, tt, _, _ := setup(t)
	if !tt.HasText("在这里打开默认 Agent") && !hasPart(tt, "Agent") {
		t.Fatalf("no Agent button: %q", tt.Texts())
	}
	for _, l := range []string{"后退", "前进", "新建"} {
		if tt.HasText(l) {
			t.Fatalf("a toolbar button %q: %q", l, tt.Texts())
		}
	}
}

// A blank space's menu acts on the folder its column shows.
func TestBlankMenu(t *testing.T) {
	withAgents(t, "claude", agClaude, agCodex)
	a, tt, fs, root := setup(t)
	a.navigate(a.tab(), filepath.Join(root, "alpha"))
	tt.Frame()
	tt.Frame()
	r, ok := tt.Find("上级文件夹") // shows root
	if !ok {
		t.Fatalf("no parent column: %q", tt.Texts())
	}
	tt.RightClickAt(r.X+r.W/2, r.Y+r.H-8)
	m := tt.Menu()
	want := []string{"新建文件", "新建文件夹", "粘贴", "在终端中打开", "在这里打开 Claude", "在这里打开 Codex", "排序方式", "显示/隐藏隐藏文件", "刷新", "复制路径"}
	var got []string
	for _, s := range m {
		if s != "" && s != "-" {
			got = append(got, s)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("menu %q, want %q", m, want)
	}
	if err := tt.ChooseMenuItem("新建文件夹"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.dlgOpen || a.dlg != dlgNewFolder {
		t.Fatalf("dialog %v %d", a.dlgOpen, a.dlg)
	}
	a.dlgText = "from-parent"
	a.submitDialog()
	tt.Frame()
	if _, err := os.Stat(filepath.Join(root, "from-parent")); err != nil {
		t.Fatalf("not in the parent column's folder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "from-parent")); err == nil {
		t.Fatal("made in the current folder")
	}

	// The current list's blank space: a new file there, and the terminal.
	r, _ = tt.Find("文件")
	tt.RightClickAt(r.X+r.W/2, r.Y+r.H-8)
	if err := tt.ChooseMenuItem("新建文件", blankTemplate); err != nil {
		t.Fatalf("%v: %q", err, tt.Menu())
	}
	tt.Frame()
	a.dlgText = "blank.txt"
	a.submitDialog()
	tt.Frame()
	if _, err := os.Stat(filepath.Join(root, "alpha", "blank.txt")); err != nil {
		t.Fatalf("new file: %v", err)
	}
	tt.RightClickAt(r.X+r.W/2, r.Y+r.H-8)
	if err := tt.ChooseMenuItem("在终端中打开"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if n := len(fs.openedWith); n == 0 || fs.openedWith[n-1] != [2]string{"Terminal", filepath.Join(root, "alpha")} {
		t.Fatalf("terminal %v", fs.openedWith)
	}
	// config.json's blankMenu replaces it.
	a.cfg.BlankMenu = []string{"refresh", "-", "reveal"}
	tt.RightClickAt(r.X+r.W/2, r.Y+r.H-8)
	if m := tt.Menu(); !slices.Equal(slices.DeleteFunc(m, func(s string) bool { return s == "" || s == "-" }), []string{"刷新", "在 Finder 中显示"}) {
		t.Fatalf("blankMenu: %q", m)
	}
	if err := tt.ChooseMenuItem("在 Finder 中显示"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if n := len(fs.revealed); n == 0 || fs.revealed[n-1] != filepath.Join(root, "alpha") {
		t.Fatalf("revealed %v", fs.revealed)
	}
}

// An old config.json's "toolbar" is ignored; the blank menu's unknown
// items are warned of.
func TestConfigOldToolbarIgnored(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"terminal": "Kaku", "toolbar": ["back"], "showToolbar": true}`), 0o644)
	if c := loadConfig(dir); c.Terminal != "Kaku" {
		t.Fatalf("old file: %+v", c)
	}
	km, _, warns := buildKeymap(config{BlankMenu: []string{"@sort", "-", "zzz"}}, "")
	_ = km
	if len(warns) != 1 || !strings.Contains(warns[0], "zzz") {
		t.Fatalf("warnings %q", warns)
	}
}
