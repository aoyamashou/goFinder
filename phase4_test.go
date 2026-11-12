package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/keymap"
	"gofinder/internal/shellcmd"
)

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
	if m := tt.Menu(); !slices.Contains(m, "拷贝") || slices.ContainsFunc(m, func(s string) bool { return strings.Contains(s, "nope") }) {
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
	cfg.Toolbar = []string{"layout.rows", "cmd.typora"}
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
	if err := tt.Click("用 Typora 打开"); err != nil { // the toolbar button
		t.Fatal(err)
	}
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
		t.Fatal("toolbar did not switch layout")
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
