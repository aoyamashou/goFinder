package gui

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/vfs"
)

type fakeSys struct {
	opened, trashed []string
	openedWith      [][2]string
	revealed        []string
	watches         map[string]func() // watched folder → its callback
	shells          [][2]string       // dir, command line
	shellErr        error
	viewer          string   // NSFileViewer
	viewerSets      []string // SetFileViewer calls
	thumbs          []string // Thumbnail calls: the source files
	thumbErr        error
	quickLooks      [][]string // QuickLook calls
	quickStops      int        // how many were ended
	fullDisk        bool
	urls            []string // OpenURL calls
	ejected         []string // Eject calls
	ejectErr        error
}

func (f *fakeSys) Open(p string) error { f.opened = append(f.opened, p); return nil }
func (f *fakeSys) Trash(p string) error {
	f.trashed = append(f.trashed, p)
	return os.RemoveAll(p)
}
func (f *fakeSys) Reveal(p string) { f.revealed = append(f.revealed, p) }
func (f *fakeSys) OpenWith(app, p string) error {
	f.openedWith = append(f.openedWith, [2]string{app, p})
	return nil
}
func (f *fakeSys) Watch(dir string, changed func()) func() {
	if f.watches == nil {
		f.watches = map[string]func(){}
	}
	f.watches[dir] = changed
	return func() { delete(f.watches, dir) }
}
func (f *fakeSys) RunShell(dir, cmd string) (string, error) {
	f.shells = append(f.shells, [2]string{dir, cmd})
	if f.shellErr != nil {
		return "line 1\nboom", f.shellErr
	}
	return "", nil
}
func (f *fakeSys) FileViewer() (string, error) { return f.viewer, nil }
func (f *fakeSys) SetFileViewer(id string) error {
	f.viewerSets = append(f.viewerSets, id)
	f.viewer = id
	return nil
}

func (f *fakeSys) QuickLook(paths []string) func() {
	f.quickLooks = append(f.quickLooks, paths)
	return func() { f.quickStops++ }
}
func (f *fakeSys) FullDiskAccess() bool { return f.fullDisk }
func (f *fakeSys) Eject(p string) error {
	f.ejected = append(f.ejected, p)
	return f.ejectErr
}

func (f *fakeSys) OpenURL(u string) error {
	f.urls = append(f.urls, u)
	return nil
}

// Thumbnail writes a 2×1 PNG, or fails with thumbErr.
func (f *fakeSys) Thumbnail(src, dst string, px int) error {
	f.thumbs = append(f.thumbs, src)
	if f.thumbErr != nil {
		return f.thumbErr
	}
	return os.WriteFile(dst, tinyPNG, 0o644)
}

// tinyPNG is a 2×1 PNG.
var tinyPNG = func() []byte {
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 1)))
	return b.Bytes()
}()

func (f *fakeSys) RunsScripts(app string) bool { return app != "NoScripts" }
func (f *fakeSys) BundleID() string            { return "io.github.aoyamashou.gofinder" }
func (f *fakeSys) InstalledApps() ([]string, error) {
	return []string{"Ghostty", "Terminal", "Visual Studio Code", "Zed"}, nil
}

// setup makes a tree and an app over it, with work running synchronously.
func setup(t *testing.T) (*app, *ui.Tester, *fakeSys, string) {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"alpha/inner", "beta"} {
		os.MkdirAll(filepath.Join(root, d), 0o755)
	}
	for _, f := range []string{"report-1.txt", "report-2.txt", "notes.md", "alpha/inside.txt"} {
		os.WriteFile(filepath.Join(root, f), []byte("hello "+f), 0o644)
	}
	km, _, _ := buildKeymap(testConfig(), "")
	fs := &fakeSys{}
	a := newApp(km, fs, testConfig(), root)
	a.async = func(f func()) { f() }
	a.gitDelay = 0
	a.start()
	applyTestAgents(a)
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	return a, tt, fs, root
}

func cursorName(a *app) string {
	if e, ok := a.tab().cursorEntry(); ok {
		return e.Name
	}
	return ""
}

func TestListsAndNavigates(t *testing.T) {
	a, tt, fs, root := setup(t)
	if !tt.HasText("alpha") || !tt.HasText("report-1.txt") {
		t.Fatalf("listing missing: %q", tt.Texts())
	}
	if cursorName(a) != "alpha" { // folders first
		t.Fatalf("cursor on %q", cursorName(a))
	}
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if a.tab().t.Dir != filepath.Join(root, "alpha") || !tt.HasText("inside.txt") {
		t.Fatalf("Right did not enter alpha: %s", a.tab().t.Dir)
	}
	tt.Key(0, ui.KeyLeft)
	tt.Frame()
	if a.tab().t.Dir != root || cursorName(a) != "alpha" {
		t.Fatalf("Left: dir %s cursor %q", a.tab().t.Dir, cursorName(a))
	}
	// Down to a file, → opens it with the default app.
	for cursorName(a) != "notes.md" {
		tt.Key(0, ui.KeyDown)
		tt.Frame()
	}
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if len(fs.opened) != 1 || filepath.Base(fs.opened[0]) != "notes.md" {
		t.Fatalf("opened %v", fs.opened)
	}
	tt.Key(ui.Cmd, ui.KeyBracketLeft) // back: we are at root, history has alpha
	tt.Frame()
	if a.tab().t.Dir != filepath.Join(root, "alpha") {
		t.Fatalf("back went to %s", a.tab().t.Dir)
	}
}

func TestHintsFollowModifiers(t *testing.T) {
	_, tt, _, _ := setup(t)
	has := func(s string) bool {
		return slices.ContainsFunc(tt.Texts(), func(x string) bool { return strings.Contains(x, s) })
	}
	if !tt.HasText("新文件夹") || !tt.HasText("新文件") || has("⌘R") {
		t.Fatalf("default hints: %q", tt.Texts())
	}
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	if !tt.HasText("⌘R") || !tt.HasText("刷新") || has("⇧⌘H") {
		t.Fatalf("Cmd hints: %q", tt.Texts())
	}
	tt.HoldModifiers(0)
}

func TestHelpSheet(t *testing.T) {
	a, tt, _, _ := setup(t)
	tt.Key(ui.Shift, ui.KeySlash)
	tt.Frame()
	if !a.helpOpen || !tt.HasText("速查表（Esc 关闭）") || !tt.HasText("移到废纸篓") {
		t.Fatalf("help not shown: open=%v %q", a.helpOpen, tt.Texts())
	}
	tt.Type("废纸")
	tt.Frame()
	if tt.HasText("新建标签页") {
		t.Fatal("filter did not narrow the sheet")
	}
	tt.Key(0, ui.KeyEscape) // clears the search
	tt.Key(0, ui.KeyEscape) // closes
	tt.Frame()
	if a.helpOpen {
		t.Fatal("Esc did not close the sheet")
	}
}

func TestCopyPasteAcrossPanes(t *testing.T) {
	a, tt, _, root := setup(t)
	a.setLayout(layoutDual)
	tt.Frame()
	for cursorName(a) != "notes.md" {
		tt.Key(0, ui.KeyDown)
		tt.Frame()
	}
	tt.Key(ui.Cmd, ui.KeyC)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.active != 1 {
		t.Fatalf("Tab did not switch panes (active %d)", a.active)
	}
	tt.Key(0, ui.KeyRight) // into alpha (first row)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	if _, err := os.Stat(filepath.Join(root, "alpha", "notes.md")); err != nil {
		t.Fatalf("paste: %v (status %q)", err, a.status)
	}
	if cursorName(a) != "notes.md" {
		t.Fatalf("cursor after paste on %q", cursorName(a))
	}
}

func TestAddressBar(t *testing.T) {
	a, tt, _, root := setup(t)
	tt.Key(ui.Cmd, ui.KeyL)
	tt.Frame()
	if !a.pane().addrEditing {
		t.Fatal("⌘L did not edit the address")
	}
	a.pane().addrText = root + "/beta"
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.tab().t.Dir != filepath.Join(root, "beta") || a.pane().addrEditing {
		t.Fatalf("address went to %s (editing %v)", a.tab().t.Dir, a.pane().addrEditing)
	}
}

func TestRenameNewTrash(t *testing.T) {
	a, tt, fs, root := setup(t)
	for cursorName(a) != "notes.md" {
		tt.Key(0, ui.KeyDown)
		tt.Frame()
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if !a.dlgOpen || a.dlgText != "notes.md" {
		t.Fatalf("rename dialog: open %v text %q", a.dlgOpen, a.dlgText)
	}
	a.dlgText = "todo.md"
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if _, err := os.Stat(filepath.Join(root, "todo.md")); err != nil || cursorName(a) != "todo.md" {
		t.Fatalf("rename: %v cursor %q status %q", err, cursorName(a), a.status)
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyN)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if st, err := os.Stat(filepath.Join(root, "未命名文件夹")); err != nil || !st.IsDir() {
		t.Fatalf("new folder: %v", err)
	}
	if cursorName(a) != "未命名文件夹" {
		t.Fatalf("cursor on %q", cursorName(a))
	}
	tt.Key(ui.Cmd, ui.KeyBackspace)
	tt.Frame()
	tt.Key(0, ui.KeyEnter) // the confirmation
	tt.Frame()
	if len(fs.trashed) != 1 || filepath.Base(fs.trashed[0]) != "未命名文件夹" {
		t.Fatalf("trashed %v", fs.trashed)
	}
	tt.Key(ui.Cmd|ui.Alt, ui.KeyT)
	tt.Frame()
	if len(fs.openedWith) != 1 || fs.openedWith[0] != [2]string{"Terminal", root} {
		t.Fatalf("terminal: %v", fs.openedWith)
	}
}

func TestLayoutsAndPreview(t *testing.T) {
	a, tt, _, _ := setup(t)
	for cursorName(a) != "notes.md" {
		tt.Key(0, ui.KeyDown)
		tt.Frame()
	}
	tt.Key(ui.Cmd|ui.Alt, ui.KeyP)
	tt.Frame()
	if !tt.HasText("hello notes.md") {
		t.Fatalf("text preview missing: %q", tt.Texts())
	}
	tt.Key(ui.Cmd|ui.Alt, ui.Key3)
	tt.Frame()
	tt.Frame()
	if a.layout != layoutColumns || !tt.HasText("hello notes.md") {
		t.Fatalf("columns layout: %d %q", a.layout, tt.Texts())
	}
	tt.Key(ui.Cmd|ui.Alt, ui.Key2)
	tt.Frame()
	if a.layout != layoutDual {
		t.Fatal("dual layout")
	}
}

// testConfig is the defaults the older tests were written for: the
// window in macOS's own appearance.
func testConfig() config {
	cfg := defaultConfig()
	cfg.Appearance = "system"
	return cfg
}

// The old data folder moves to the new, empty one; a new one in use is
// left alone.
func TestMoveOld(t *testing.T) {
	root := t.TempDir()
	old, dir := filepath.Join(root, oldName), filepath.Join(root, "goFinder")
	os.MkdirAll(old, 0o755)
	os.WriteFile(filepath.Join(old, "config.json"), []byte("{}"), 0o644)
	os.MkdirAll(dir, 0o755) // made by MyGo on first use
	if besideOld(dir) != old {
		t.Fatal(besideOld(dir))
	}
	if err := moveOld(old, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Fatal("not moved")
	}
	os.MkdirAll(old, 0o755)
	os.WriteFile(filepath.Join(old, "x"), nil, 0o644)
	moveOld(old, dir)
	if _, err := os.Stat(filepath.Join(old, "x")); err != nil {
		t.Error("moved over a folder in use")
	}
	if err := moveOld(filepath.Join(root, "none"), dir); err != nil {
		t.Error(err)
	}
}

// → on an app runs it, as Finder's double click; the list stays put.
func TestOpenAppRuns(t *testing.T) {
	a, tt, fs, root := setup(t)
	app := filepath.Join(root, "Demo.app")
	os.MkdirAll(filepath.Join(app, "Contents"), 0o755)
	a.run("refresh")
	tt.Frame()
	ts := a.tab()
	ts.cursor = slices.IndexFunc(ts.t.Entries(), func(e vfs.Entry) bool { return e.Name == "Demo.app" })
	if ts.cursor < 0 {
		t.Fatalf("no Demo.app: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if a.tab().t.Dir != root {
		t.Errorf("entered the app: %s", a.tab().t.Dir)
	}
	if !slices.Contains(fs.opened, app) {
		t.Errorf("not run: %v", fs.opened)
	}
}
