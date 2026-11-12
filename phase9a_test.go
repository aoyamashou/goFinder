package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/keymap"
)

func listNames(a *app) []string {
	var out []string
	for _, e := range a.tab().t.Entries() {
		out = append(out, e.Name)
	}
	return out
}

// press sends a key and lets a double tap's wait run out.
func press9(tt *ui.Tester, mods ui.Modifiers, k ui.Key) {
	tt.Key(mods, k)
	tt.Advance(keymap.DoubleTap)
	tt.Frame()
}

// Quick Select is gone: / does nothing, the keymap has no such action,
// and the toolbar has no button for it.
func TestNoQuickSelect(t *testing.T) {
	a, tt, _, _ := setup(t)
	if _, ok := a.km.Action("quickselect"); ok {
		t.Fatal("quickselect action still exists")
	}
	if id, ok := a.km.Lookup(keymap.Chord{Key: "/"}); ok {
		t.Fatalf("/ is bound to %q", id)
	}
	tt.TypeKey(0, ui.KeySlash, "/")
	tt.Type("rep")
	tt.Frame()
	if a.tab().sel.Len() != 0 || a.filterOn || a.tab().t.Filter != "" {
		t.Fatal("/ did something")
	}
	if tt.HasText("快速选择") {
		t.Fatalf("quick select still shown: %q", tt.Texts())
	}
	if slices.Contains(a.toolbarIDs(), "@qs") {
		t.Fatal("toolbar has @qs")
	}
}

// v and ⌘V paste; p is no longer bound.
func TestPasteKeys(t *testing.T) {
	a, tt, _, root := setup(t)
	toNotes(a, tt)
	press9(tt, 0, ui.KeyC)
	press9(tt, 0, ui.KeyP)
	if _, err := os.Stat(filepath.Join(root, "notes 2.md")); err == nil {
		t.Fatal("p pasted")
	}
	if _, ok := a.km.Lookup(keymap.Chord{Key: "p"}); ok {
		t.Fatal("p is bound")
	}
	press9(tt, 0, ui.KeyV)
	if _, err := os.Stat(filepath.Join(root, "notes 2.md")); err != nil {
		t.Fatalf("v: %v", err)
	}
	press9(tt, ui.Cmd, ui.KeyV)
	if _, err := os.Stat(filepath.Join(root, "notes 3.md")); err != nil {
		t.Fatalf("⌘V: %v (%q)", err, a.status)
	}
}

// f opens the filter box; typing filters; ↩ keeps; Esc clears.
func TestFilter(t *testing.T) {
	a, tt, _, _ := setup(t)
	ts := a.tab()
	press9(tt, 0, ui.KeyF)
	if !a.filterOn || !tt.HasText("过滤") {
		t.Fatalf("f: filterOn %v", a.filterOn)
	}
	tt.Type("rep")
	tt.Frame()
	if got := listNames(a); !slices.Equal(got, []string{"report-1.txt", "report-2.txt"}) {
		t.Fatalf("rep: %v", got)
	}
	if cursorName(a) != "report-1.txt" {
		t.Fatalf("cursor %q", cursorName(a))
	}
	if !tt.HasText("过滤「rep」：2 项") || !tt.HasText("保留") || !tt.HasText("Esc") {
		t.Fatalf("hints: %q", tt.Texts())
	}
	// Keys type letters while the box has the keyboard.
	tt.Type("x")
	tt.Frame()
	if len(listNames(a)) != 0 || ts.cursor != -1 {
		t.Fatalf("repx: %v cursor %d", listNames(a), ts.cursor)
	}
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if len(listNames(a)) != 2 {
		t.Fatalf("after ⌫: %v", listNames(a))
	}
	// ↩ keeps the filter and gives the list the keyboard.
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if a.filterOn || ts.t.Filter != "rep" || len(listNames(a)) != 2 {
		t.Fatalf("↩: on %v filter %q", a.filterOn, ts.t.Filter)
	}
	if !tt.HasText("过滤：rep") {
		t.Fatalf("status bar: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if cursorName(a) != "report-2.txt" {
		t.Fatalf("down: %q", cursorName(a))
	}
	// Esc in the list clears the filter; the cursor stays on its item.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if ts.t.Filter != "" || len(listNames(a)) != 5 || cursorName(a) != "report-2.txt" {
		t.Fatalf("Esc in list: %q %v cursor %q", ts.t.Filter, listNames(a), cursorName(a))
	}
	if tt.HasText("过滤：rep") {
		t.Fatal("status still says 过滤")
	}
}

func TestFilterEscInBox(t *testing.T) {
	a, tt, _, _ := setup(t)
	press9(tt, 0, ui.KeyF)
	tt.Type("notes")
	tt.Frame()
	if got := listNames(a); !slices.Equal(got, []string{"notes.md"}) {
		t.Fatalf("notes: %v", got)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if a.filterOn || a.tab().t.Filter != "" || len(listNames(a)) != 5 {
		t.Fatalf("Esc: on %v filter %q", a.filterOn, a.tab().t.Filter)
	}
	// Focus is back on the list: keys are commands again.
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if cursorName(a) == "" {
		t.Fatal("no cursor")
	}
}

func TestFilterSmartCase(t *testing.T) {
	a, tt, _, root := setup(t)
	os.WriteFile(filepath.Join(root, "README"), []byte("x"), 0o644)
	a.refreshLists()
	tt.Frame()
	press9(tt, 0, ui.KeyF)
	tt.Type("read")
	tt.Frame()
	if got := listNames(a); !slices.Equal(got, []string{"README"}) {
		t.Fatalf("read: %v", got)
	}
	tt.Type("M") // a capital: exact now, "readM" matches nothing
	tt.Frame()
	if len(listNames(a)) != 0 {
		t.Fatalf("readM: %v", listNames(a))
	}
	for range 5 {
		tt.Key(0, ui.KeyBackspace)
		tt.Frame()
	}
	tt.Type("RE")
	tt.Frame()
	if got := listNames(a); !slices.Equal(got, []string{"README"}) {
		t.Fatalf("RE: %v", got)
	}
}

// The cursor stays on its item when it survives the filter, else goes
// to the first; going to another folder clears the filter.
func TestFilterCursorAndNavigation(t *testing.T) {
	a, tt, _, root := setup(t)
	for cursorName(a) != "report-2.txt" {
		tt.Key(0, ui.KeyDown)
		tt.Frame()
	}
	press9(tt, 0, ui.KeyF)
	tt.Type("txt")
	tt.Frame()
	if cursorName(a) != "report-2.txt" {
		t.Fatalf("kept item: %q", cursorName(a))
	}
	for range 3 {
		tt.Key(0, ui.KeyBackspace)
		tt.Frame()
	}
	tt.Type("1")
	tt.Frame()
	if cursorName(a) != "report-1.txt" {
		t.Fatalf("filtered out: %q", cursorName(a))
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	a.navigate(a.tab(), filepath.Join(root, "alpha"))
	tt.Frame()
	if a.tab().t.Filter != "" || a.filterOn || !slices.Contains(listNames(a), "inside.txt") {
		t.Fatalf("navigate: filter %q %v", a.tab().t.Filter, listNames(a))
	}
}

// The default hints, in order, on two lines: looking and going, then
// editing.
func TestDefaultHintsOrder(t *testing.T) {
	_, tt, _, _ := setup(t)
	lines := [][]string{
		{"过滤", "回到起点", "启动智能体", "帮助"},
		{"选中后：", "拷贝", "拷贝路径", "剪切", "粘贴", "新文件夹", "新文件", "重命名", "删除"},
	}
	var ys []float64
	for _, want := range lines {
		at, y := -1.0, -1.0
		for _, w := range want {
			r, ok := tt.Find(w)
			if !ok {
				t.Fatalf("hint %q missing: %q", w, tt.Texts())
			}
			if float64(r.X) <= at {
				t.Fatalf("hint %q out of order (x %v after %v)", w, r.X, at)
			}
			if y >= 0 && float64(r.Y) != y {
				t.Fatalf("hint %q off its line (y %v, not %v)", w, r.Y, y)
			}
			at, y = float64(r.X), float64(r.Y)
		}
		ys = append(ys, y)
	}
	if ys[0] >= ys[1] {
		t.Fatalf("lines at %v", ys)
	}
	for _, k := range []string{"f", "v", "?"} {
		if !tt.HasText(k) {
			t.Errorf("no keycap %q", k)
		}
	}
	if tt.HasText("快速选择") || tt.HasText("选择") {
		t.Fatalf("old hint: %q", tt.Texts())
	}
}

// The status bar's upper row has no counts.
func TestStatusBarNoCounts(t *testing.T) {
	a, tt, _, _ := setup(t)
	for _, s := range tt.Texts() {
		if strings.HasSuffix(s, " 项") || strings.HasPrefix(s, "已选") {
			t.Fatalf("count in the status bar: %q", s)
		}
	}
	a.tab().sel.Add(a.tab().t.Entries()[2].Path)
	a.tab().sel.Add(a.tab().t.Entries()[3].Path)
	tt.Frame()
	for _, s := range tt.Texts() {
		if strings.HasPrefix(s, "已选") {
			t.Fatalf("selection count: %q", s)
		}
	}
}

// The right-click menus copy paths instead of showing in Finder.
func TestContextMenuCopyPath(t *testing.T) {
	a, tt, fs, root := setup(t)
	if err := tt.RightClick("notes.md"); err != nil {
		t.Fatal(err)
	}
	if m := tt.Menu(); slices.Contains(m, "在 Finder 中显示") || !slices.Contains(m, "复制路径") {
		t.Fatalf("row menu %q", m)
	}
	if err := tt.ChooseMenuItem("复制路径"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.copied != filepath.Join(root, "notes.md") || a.status != "已复制路径" {
		t.Fatalf("single: %q status %q", a.copied, a.status)
	}
	// In a selection: every chosen path, a line each.
	ts := a.tab()
	ts.sel.Clear()
	ts.sel.Add(filepath.Join(root, "notes.md"))
	ts.sel.Add(filepath.Join(root, "report-1.txt"))
	tt.Frame()
	if err := tt.RightClick("notes.md"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("复制路径"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	got := strings.Split(a.copied, "\n")
	slices.Sort(got)
	if !slices.Equal(got, []string{filepath.Join(root, "notes.md"), filepath.Join(root, "report-1.txt")}) {
		t.Fatalf("multi: %q", a.copied)
	}
	// Blank space: the column's folder.
	ts.sel.Clear()
	tt.Frame()
	r, _ := tt.Find("文件")
	tt.RightClickAt(r.X+r.W/2, r.Y+r.H-8)
	if m := tt.Menu(); slices.Contains(m, "在 Finder 中显示") || !slices.Contains(m, "复制路径") {
		t.Fatalf("blank menu %q", m)
	}
	if err := tt.ChooseMenuItem("复制路径"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.copied != root || len(fs.revealed) != 0 {
		t.Fatalf("blank: %q revealed %v", a.copied, fs.revealed)
	}
	// The action stays in the menu bar.
	if _, ok := a.km.Action("reveal"); !ok {
		t.Fatal("reveal gone")
	}
}
