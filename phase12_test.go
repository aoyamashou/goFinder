package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func tbButton(t *testing.T, tt *ui.Tester, name string) ui.AccessInfo {
	t.Helper()
	i, ok := tt.AccessByRole(ui.RoleButton, name)
	if !ok {
		t.Fatalf("no button %q: %q", name, tt.Texts())
	}
	return i
}

func clickBtn(tt *ui.Tester, name string) error {
	i, ok := tt.AccessByRole(ui.RoleButton, name)
	if !ok || i.Disabled {
		return fmt.Errorf("button %q missing or disabled", name)
	}
	tt.ClickAt(i.Bounds.X+i.Bounds.W/2, i.Bounds.Y+i.Bounds.H/2)
	return nil
}

func TestDefaultToolbarOrder(t *testing.T) {
	want := []string{"back", "forward", "up", "-", "@new", "@sort", "hidden", "-", "trash", "-", "terminal", "agent", "@flex", "workspace", "shelf", "filter"}
	if !slices.Equal(defaultToolbar, want) {
		t.Fatalf("default %q", defaultToolbar)
	}
	a, tt, _, _ := setup(t)
	var xs []float32
	for _, n := range []string{"‹ 后退", "前进 ›", "上级目录", "废纸篓", "Workspace…", "显示/隐藏暂存架", "过滤当前文件夹"} {
		xs = append(xs, tbButton(t, tt, n).Bounds.X)
	}
	if !slices.IsSorted(xs) {
		t.Fatalf("buttons out of order: %v", xs)
	}
	_ = a
}

func TestToolbarFilterButton(t *testing.T) {
	a, tt, _, _ := setup(t)
	if err := clickBtn(tt, "过滤当前文件夹"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.filterOn {
		t.Fatal("filter box not open")
	}
	a.filterText = "report"
	a.tab().t.Filter = "report"
	a.filterOn = false
	tt.Frame()
	r := tbButton(t, tt, "过滤当前文件夹").Bounds
	if colorPx(tt, r, token("surface-200", false)) == 0 {
		t.Fatal("no on state while filtering")
	}
}

func TestToolbarUpAndTrashDisabled(t *testing.T) {
	a, tt, _, root := setup(t)
	if tbButton(t, tt, "上级目录").Disabled {
		t.Fatal("up disabled below root")
	}
	if tbButton(t, tt, "废纸篓").Disabled {
		t.Fatal("trash disabled with a cursor item")
	}
	if err := clickBtn(tt, "废纸篓"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.dlgOpen || a.dlg != dlgTrash {
		t.Fatalf("no confirm: %v %d", a.dlgOpen, a.dlg)
	}
	a.dlgOpen = false
	tt.Frame()
	// An empty folder: nothing to trash.
	a.navigate(a.tab(), filepath.Join(root, "beta"))
	tt.Frame()
	tt.Frame()
	if !tbButton(t, tt, "废纸篓").Disabled {
		t.Fatal("trash enabled in an empty folder")
	}
	a.navigate(a.tab(), "/")
	tt.Frame()
	tt.Frame()
	if !tbButton(t, tt, "上级目录").Disabled {
		t.Fatal("up enabled at /")
	}
}

func TestToolbarSettingsPage(t *testing.T) {
	a, tt, _, _ := setup(t)
	dir := t.TempDir()
	a.dataDir = dir
	saved := func() []string { return loadConfig(dir).Toolbar }
	a.cfg.Toolbar = []string{"back", "forward", "up"}
	a.settingsDialog()
	a.setPage = pageToolbar
	tt.Frame()
	tt.Frame()
	if !tt.HasText("当前工具栏") || !tt.HasText("可添加") {
		t.Fatalf("page: %q", tt.Texts())
	}
	// Move the first item down.
	if err := clickBtn(tt, "下移"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if want := []string{"forward", "back", "up"}; !slices.Equal(a.cfg.Toolbar, want) || !slices.Equal(saved(), want) {
		t.Fatalf("down: %q saved %q", a.cfg.Toolbar, saved())
	}
	// Up: row 1 is selected after the move; its 上移 is the first enabled.
	if err := func() error {
		a.toolbarKey(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyUp, Mods: ui.Cmd})
		return nil
	}(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if want := []string{"back", "forward", "up"}; !slices.Equal(saved(), want) {
		t.Fatalf("up: %q", saved())
	}
	// Remove the first; it returns to the choices.
	if err := clickBtn(tt, "移除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if want := []string{"forward", "up"}; !slices.Equal(saved(), want) {
		t.Fatalf("remove: %q", saved())
	}
	if !slices.Contains(a.toolbarChoices(), "back") || slices.Contains(a.toolbarChoices(), "up") {
		t.Fatalf("choices %q", a.toolbarChoices())
	}
	if !slices.Contains(a.toolbarChoices(), "-") {
		t.Fatal("a gap can always be added")
	}
	// Add: the first choice goes to the end; the toolbar follows.
	first := a.toolbarChoices()[0]
	if err := clickBtn(tt, "添加"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := saved(); len(got) != 3 || got[2] != first {
		t.Fatalf("add: %q (first %q)", got, first)
	}
	// Keyboard: ↓ chooses, ⌘↑ moves, ⌫ removes.
	a.tbSel = 0
	a.toolbarKey(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyDown})
	a.toolbarKey(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyUp, Mods: ui.Cmd})
	if got := saved(); got[0] != "up" || a.tbSel != 0 {
		t.Fatalf("keys: %q sel %d", got, a.tbSel)
	}
	a.toolbarKey(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyBackspace})
	if got := saved(); len(got) != 2 {
		t.Fatalf("backspace: %q", got)
	}
	// Reset.
	if err := clickBtn(tt, "恢复默认"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.cfg.Toolbar != nil || loadConfig(dir).Toolbar != nil && !slices.Equal(loadConfig(dir).Toolbar, defaultToolbar) {
		t.Fatalf("reset: %v saved %v", a.cfg.Toolbar, saved())
	}
	if !slices.Equal(a.toolbarIDs(), defaultToolbar) {
		t.Fatal("not the default")
	}
}

// A toolbar the user wrote keeps working as written.
func TestOldToolbarConfigKept(t *testing.T) {
	a, tt, _, _ := setup(t)
	a.cfg.Toolbar = []string{"back", "shelf", "terminal"}
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleButton, "过滤当前文件夹"); ok {
		t.Fatal("new default leaked into an own toolbar")
	}
	if _, ok := tt.AccessByRole(ui.RoleButton, "显示/隐藏暂存架"); !ok {
		t.Fatal("own toolbar lost")
	}
}
