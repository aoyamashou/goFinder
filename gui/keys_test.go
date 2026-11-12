package gui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gofinder/internal/keymap"
	"gofinder/internal/shellcmd"
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
	press(0, ui.KeyT)
	if len(fs.openedWith) != 1 || fs.openedWith[0][0] != "Terminal" {
		t.Fatalf("t: %v", fs.openedWith)
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
// and nothing shows it.
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
		{"导航时：", "移动", "上级", "切换窗格", "回到起点", "隐藏文件", "快速查看", "过滤", "终端", "编辑器", "工作区", "速查表"},
		{"选中后：", "复制路径", "复制", "剪切", "粘贴", "暂存", "新文件夹", "新文件", "重命名", "删除", "收藏"},
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
	for _, k := range []string{"↑↓", "→", "←", "`", ",", ".", "Space", "f", "t", "a", "e", "w", "tab", "?", "v", "s", "s s"} {
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

// setupWith is setup with a config and a keymap.json of the test's.
func setupWith(t *testing.T, cfg config, kmJSON string) (*app, *ui.Tester, *fakeSys, string) {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "alpha"), 0o755)
	for _, f := range []string{"it's a file.txt", "notes.md"} {
		os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644)
	}
	data := t.TempDir()
	kmPath := keymapPath(data)
	if kmJSON != "" {
		os.WriteFile(kmPath, []byte(kmJSON), 0o644)
	}
	km, file, warns := buildKeymap(cfg, kmPath)
	fs := &fakeSys{}
	a := newApp(km, fs, cfg, root)
	a.kmFile, a.kmPath = file, kmPath
	a.setWarnings(warns)
	a.async = func(f func()) { f() }
	a.gitDelay = 0
	a.start()
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	return a, tt, fs, root
}

func hasPart(tt *ui.Tester, s string) bool {
	return slices.ContainsFunc(tt.Texts(), func(x string) bool { return strings.Contains(x, s) })
}

// What keymap.json and config.json ask for but cannot have is said at
// start, and each problem is listed in the cheat sheet.
func TestKeymapConflictsShown(t *testing.T) {
	cfg := testConfig()
	cfg.Commands = []command{{ID: "typora", Title: "用 Typora 打开", Command: "open -a Typora {selection}", Keys: []string{"Cmd+R"}}}
	cfg.ContextMenu = append(cfg.ContextMenu, "nope.menu")
	a, tt, _, _ := setupWith(t, cfg, `{"trash": ["Cmd+Alt+N"], "copy": ["Cmd+Alt+N"], "ghost": ["F5"]}`)
	if id, _ := a.km.Lookup(keymap.MustParse("Cmd+R")); id != "cmd.typora" {
		t.Fatalf("⌘R → %q", id)
	}
	if id, _ := a.km.Lookup(keymap.MustParse("Cmd+Alt+N")); id != "copy" { // copy comes first
		t.Fatalf("⌘⌥N → %q", id)
	}
	if !strings.Contains(a.status, "4 处") || !hasPart(tt, "按 ? 查看") {
		t.Fatalf("status %q, warnings %q", a.status, a.warns)
	}
	tt.Key(ui.Shift, ui.KeySlash)
	tt.Frame()
	for _, s := range []string{"原属于「刷新」，已改给「用 Typora 打开」", "后者已忽略", `未知动作 "ghost"`, `contextMenu：未知动作 "nope.menu"`} {
		if !hasPart(tt, s) {
			t.Errorf("sheet does not say %q: %q", s, a.warns)
		}
	}
	// The unknown menu entry is left out of the menu.
	tt.Key(0, ui.KeyEscape)
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if err := tt.RightClick("notes.md"); err != nil {
		t.Fatal(err)
	}
	if m := tt.Menu(); !slices.Contains(m, "复制") || slices.ContainsFunc(m, func(s string) bool { return strings.Contains(s, "nope") }) {
		t.Fatalf("menu %q", m)
	}
	tt.CloseMenu()
}

// "改键" on the settings' 快捷键 page records the next key, takes it from
// the action that had it, and saves keymap.json. The cheat sheet only
// shows the keys, and leads there.
func TestRecordKey(t *testing.T) {
	a, tt, _, _ := setupWith(t, testConfig(), "")
	tt.Key(ui.Shift, ui.KeySlash)
	tt.Frame()
	if tt.HasText("改键") {
		t.Fatal("the cheat sheet records keys")
	}
	if err := tt.Click("在设置中修改"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.helpOpen || !a.dlgOpen || a.dlg != dlgSettings || a.setPage != 1 {
		t.Fatalf("to settings: help %v dlg %v/%d page %d", a.helpOpen, a.dlgOpen, a.dlg, a.setPage)
	}
	tt.Frame()
	tt.Type("刷新")
	tt.Frame()
	if a.helpQuery != "刷新" {
		t.Fatalf("search on the 快捷键 page: %q", a.helpQuery)
	}
	if err := tt.Click("改键"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.recording != "refresh" || !tt.HasText("请按下新的按键…") {
		t.Fatalf("recording %q", a.recording)
	}
	tt.TypeKey(0, ui.KeyK, "k") // a single key is fine, and types nothing
	tt.Frame()
	if a.recording != "" || a.helpQuery != "刷新" {
		t.Fatalf("plain k: recording %q query %q", a.recording, a.helpQuery)
	}
	if m, _ := keymap.ReadOverrides(a.kmPath); !slices.Equal(m["refresh"], []string{"k"}) {
		t.Fatalf("keymap.json after k: %v", m)
	}
	if err := tt.Click("改键"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyL) // ⌘L: the address's, and must not open it
	tt.Frame()
	if a.recording != "" || a.pane().addrEditing {
		t.Fatalf("recording %q address %v", a.recording, a.pane().addrEditing)
	}
	if id, _ := a.km.Lookup(keymap.MustParse("Cmd+L")); id != "refresh" {
		t.Fatalf("⌘L → %q", id)
	}
	m, err := keymap.ReadOverrides(a.kmPath)
	if err != nil || !slices.Equal(m["refresh"], []string{"Cmd+L"}) || slices.Contains(m["addr"], "Cmd+L") {
		t.Fatalf("keymap.json %v %v", m, err)
	}
	// Back to the defaults: ⌘R; addr keeps g alone until reset.
	if err := tt.Click("改键"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("默认"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if id, _ := a.km.Lookup(keymap.MustParse("Cmd+R")); id != "refresh" {
		t.Fatalf("reset: ⌘R → %q", id)
	}
	if m, _ := keymap.ReadOverrides(a.kmPath); m["refresh"] != nil {
		t.Fatalf("reset kept refresh in keymap.json: %v", m)
	}
}

// A shell command of config.json: a shortcut, in the cheat sheet and the
// hints, quoted placeholders, failures in the status bar.
func TestCustomCommand(t *testing.T) {
	cfg := testConfig()
	cfg.Commands = []command{{ID: "typora", Title: "用 Typora 打开", Command: "open -a Typora {selection} --dir {path}", Keys: []string{"Cmd+Alt+Y"}}}
	a, tt, fs, root := setupWith(t, cfg, "")
	for cursorName(a) != "it's a file.txt" {
		tt.Key(0, ui.KeyDown)
		tt.Frame()
	}
	tt.Key(ui.Cmd|ui.Alt, ui.KeyY)
	tt.Frame()
	file := filepath.Join(root, "it's a file.txt")
	want := "open -a Typora " + shellcmd.Quote(file) + " --dir " + shellcmd.Quote(root)
	if len(fs.shells) != 1 || fs.shells[0] != [2]string{root, want} {
		t.Fatalf("ran %q, want %q", fs.shells, want)
	}
	if !strings.Contains(a.status, "已运行「用 Typora 打开」") {
		t.Fatalf("status %q", a.status)
	}
	fs.shellErr = errors.New("exit status 1")
	a.run("cmd.typora")
	tt.Frame()
	if len(fs.shells) != 2 || !strings.Contains(a.status, "失败") || !strings.Contains(a.status, "boom") {
		t.Fatalf("failure: %d runs, status %q", len(fs.shells), a.status)
	}
	if rows := a.km.Sheet("Typora"); len(rows) != 1 || rows[0].Keys != "⌥⌘Y" || rows[0].Group != "自定义命令" {
		t.Fatalf("sheet %+v", rows)
	}
	tt.HoldModifiers(ui.Cmd | ui.Alt)
	tt.Frame()
	if !tt.HasText("⌥⌘Y") {
		t.Fatalf("hints: %q", tt.Texts())
	}
	tt.HoldModifiers(0)
	if err := tt.Click("上下"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.layout != layoutRows {
		t.Fatal("the view switch did not switch layout")
	}
	// In the context menu, when listed there.
	a.cfg.ContextMenu = []string{"-", "cmd.typora", "-", "-", "copy"}
	tt.Frame()
	if err := tt.RightClick("notes.md"); err != nil {
		t.Fatal(err)
	}
	if m := tt.Menu(); len(m) < 2 || !strings.Contains(m[0], "用 Typora 打开") {
		t.Fatalf("menu %q", m)
	}
	tt.CloseMenu()
}

// The top/bottom layout shows two panes, Tab moves between them, and a
// workspace keeps it.
func TestRowsLayout(t *testing.T) {
	a, tt, _, _ := setupWith(t, testConfig(), "")
	tt.Key(ui.Cmd|ui.Alt, ui.Key4)
	tt.Frame()
	if a.layout != layoutRows || a.visiblePanes() != 2 {
		t.Fatalf("layout %d", a.layout)
	}
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.active != 1 {
		t.Fatal("Tab did not switch panes")
	}
	s := a.snapshot()
	if s.Layout != "rows" {
		t.Fatalf("snapshot layout %q", s.Layout)
	}
	tt.Key(ui.Cmd|ui.Alt, ui.Key1)
	tt.Frame()
	a.applyState(s)
	if a.layout != layoutRows || a.active != 1 {
		t.Fatalf("restored layout %d active %d", a.layout, a.active)
	}
}

// The settings switch sets NSFileViewer to the app's identifier, and
// back to Finder; the real defaults command never runs in tests.
func TestDefaultFileViewer(t *testing.T) {
	a, tt, fs, _ := setupWith(t, testConfig(), "")
	tt.Key(ui.Cmd, ui.KeyComma)
	tt.Frame()
	if !tt.HasText("当前：Finder") {
		t.Fatalf("settings: %q", tt.Texts())
	}
	if err := tt.Click("设为默认"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !slices.Equal(fs.viewerSets, []string{"io.github.aoyamashou.gofinder"}) || !strings.Contains(a.status, "注销") {
		t.Fatalf("sets %q status %q", fs.viewerSets, a.status)
	}
	if !hasPart(tt, "当前：goFinder") {
		t.Fatalf("settings after: %q", tt.Texts())
	}
	if err := tt.Click("恢复 Finder"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !slices.Equal(fs.viewerSets, []string{"io.github.aoyamashou.gofinder", ""}) || !tt.HasText("当前：Finder") {
		t.Fatalf("sets %q", fs.viewerSets)
	}
}

// A folder the system opens comes up in the active pane (no tabs); a
// file, in its folder with the file chosen.
func TestOpenPath(t *testing.T) {
	a, tt, _, root := setupWith(t, testConfig(), "")
	a.openPath(filepath.Join(root, "alpha"))
	tt.Frame()
	if a.tab().t.Dir != filepath.Join(root, "alpha") {
		t.Fatalf("dir %s", a.tab().t.Dir)
	}
	a.openPath(filepath.Join(root, "notes.md"))
	tt.Frame()
	if a.tab().t.Dir != root || cursorName(a) != "notes.md" {
		t.Fatalf("file: dir %s cursor %q", a.tab().t.Dir, cursorName(a))
	}
}

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

// walk visits every item of a menu tree.
func walk(items []*mygo.MenuItem, fn func(*mygo.MenuItem)) {
	for _, it := range items {
		fn(it)
		walk(it.Submenu, fn)
	}
}

func TestMenuCoversActions(t *testing.T) {
	a, _, _, _ := setup(t)
	shown := map[string]bool{}
	walk(a.buildMenu(), func(it *mygo.MenuItem) {
		if it.ID != "" {
			shown[it.ID] = true
		}
	})
	for _, r := range roles {
		if r.covers != "" {
			shown[r.covers] = true
		}
	}
	for _, act := range a.km.Actions() {
		if !shown[act.ID] {
			t.Errorf("action %q (%s) is in no menu", act.ID, act.Title)
		}
	}
}

func TestMenuAccelerators(t *testing.T) {
	a, _, _, _ := setup(t)
	byID := map[string]*mygo.MenuItem{}
	walk(a.buildMenu(), func(it *mygo.MenuItem) {
		if it.ID != "" {
			byID[it.ID] = it
		}
	})
	cases := map[string]string{
		"workspace": "", // w alone carries no Cmd
		"go.home":   "Cmd+Shift+H",
		"settings":  "Cmd+,",
		"hidden":    "", // first key ., its ⇧⌘. is the window's alone
		"ws.next":   "", // first key ]
		"open":      "", // first key →, which the list needs
		"trash":     "", // Cmd+Backspace edits text in fields
		"copy":      "", // the Edit menu's role holds Cmd+C
		"help":      "", // ? and F1 carry no Cmd
	}
	for id, want := range cases {
		it := byID[id]
		if id == "copy" {
			if it != nil {
				t.Errorf("copy has its own item besides the Edit role")
			}
			continue
		}
		if it == nil || it.Accelerator != want {
			t.Errorf("%s: accelerator %q, want %q", id, it.Accelerator, want)
		}
	}
	if !strings.HasSuffix(byID["hidden"].Label, "　.") || !strings.HasSuffix(byID["rename"].Label, "　↩") ||
		!strings.HasSuffix(byID["open"].Label, "　→") {
		t.Errorf("first keys missing from titles: %q, %q, %q", byID["hidden"].Label, byID["rename"].Label, byID["open"].Label)
	}
	if !strings.HasSuffix(byID["trash"].Label, "　d") || !strings.HasSuffix(byID["help"].Label, "?") || !strings.HasSuffix(byID["newfile"].Label, "n n") {
		t.Errorf("keys missing from titles: %q, %q", byID["trash"].Label, byID["help"].Label)
	}
	if menuAccel([]keymap.Chord{keymap.MustParse("Ctrl+Tab"), keymap.MustParse("Cmd+Shift+]")}) != "Cmd+Shift+]" {
		t.Error("menuAccel did not skip Ctrl+Tab")
	}
}

func TestMenuRunsActionsAndFollowsState(t *testing.T) {
	a, tt, _, _ := setup(t)
	a.setMenu = func([]*mygo.MenuItem) {}
	a.applyMenu()
	tt.Frame()
	root := a.tab().t.Dir
	up := a.menu.items["up"]
	a.navigate(a.tab(), filepath.Join(root, "alpha"))
	tt.Frame()
	up.Click(up, nil)
	tt.Frame()
	if a.tab().t.Dir != root {
		t.Fatalf("menu Up: %s", a.tab().t.Dir)
	}
	a.menu.items["layout.rows"].Click(nil, nil)
	tt.Frame()
	if a.layout != layoutRows || !a.menu.items["layout.rows"].IsChecked() || a.menu.items["layout.columns"].IsChecked() {
		t.Fatalf("layout %d, checks rows %v columns %v", a.layout,
			a.menu.items["layout.rows"].IsChecked(), a.menu.items["layout.columns"].IsChecked())
	}
	// While the cheat sheet is open the items are off, and a click does nothing.
	tt.Key(ui.Shift, ui.KeySlash)
	tt.Frame()
	if a.menu.items["up"].IsEnabled() {
		t.Fatal("items enabled under the cheat sheet")
	}
	a.navigate(a.tab(), filepath.Join(root, "alpha"))
	up.Click(up, nil)
	tt.Frame()
	if a.tab().t.Dir != filepath.Join(root, "alpha") {
		t.Fatal("menu ran an action behind the cheat sheet")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if !a.menu.items["up"].IsEnabled() {
		t.Fatal("items stayed disabled after the sheet closed")
	}
}

// TestMenuOneKeyPerItem: each item shows one key at most, its action's
// first key, either as the accelerator or after the title, never both.
func TestMenuOneKeyPerItem(t *testing.T) {
	a, _, _, _ := setup(t)
	walk(a.buildMenu(), func(it *mygo.MenuItem) {
		if it.ID == "" {
			return
		}
		keys := a.km.KeysFor(it.ID)
		suffix := strings.Contains(it.Label, "　")
		if suffix && it.Accelerator != "" {
			t.Errorf("%s: two keys, %q and %q", it.ID, it.Label, it.Accelerator)
		}
		if strings.Count(it.Label, "　") > 1 {
			t.Errorf("%s: title %q", it.ID, it.Label)
		}
		if len(keys) == 0 {
			if suffix || it.Accelerator != "" {
				t.Errorf("%s has no key but shows one: %q %q", it.ID, it.Label, it.Accelerator)
			}
			return
		}
		switch {
		case it.Accelerator != "":
			if it.Accelerator != keys[0].Spec() {
				t.Errorf("%s: accelerator %q is not the first key %q", it.ID, it.Accelerator, keys[0].Spec())
			}
		case !strings.HasSuffix(it.Label, "　"+keys[0].String()):
			t.Errorf("%s: title %q lacks the first key %q", it.ID, it.Label, keys[0].String())
		}
	})
}
