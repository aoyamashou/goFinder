package gui

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/favorites"
	"gofinder/internal/finder"
	"gofinder/internal/finder/findertest"
	"gofinder/internal/templates"
	"gofinder/internal/vfs"
)

// addZip writes root/pack.zip holding docs/readme.md and top.txt.
func addZip(t *testing.T, root string) string {
	t.Helper()
	p := filepath.Join(root, "pack.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, body := range map[string]string{"docs/readme.md": "zipped readme", "top.txt": "zipped top"} {
		fw, _ := w.Create(name)
		fw.Write([]byte(body))
	}
	w.Close()
	f.Close()
	return p
}

func goTo(t *testing.T, a *app, tt *ui.Tester, name string) {
	t.Helper()
	for i := 0; cursorName(a) != name; i++ {
		if i > 50 {
			t.Fatalf("no row %q (cursor %q)", name, cursorName(a))
		}
		tt.Key(0, ui.KeyDown)
		tt.Frame()
	}
}

func TestZipBrowse(t *testing.T) {
	a, tt, fs, root := setup(t)
	a.cacheDir = t.TempDir()
	z := addZip(t, root)
	a.load(a.tab())
	tt.Frame()
	goTo(t, a, tt, "pack.zip")

	tt.Key(0, ui.KeyRight) // into the archive, as a folder
	tt.Frame()
	if a.tab().t.Dir != vfs.ZipRoot(z) || !tt.HasText("docs") || !tt.HasText("top.txt") {
		t.Fatalf("into zip: %s %q", a.tab().t.Dir, tt.Texts())
	}
	if !tt.HasText("pack.zip") { // the crumb names the archive, without "!"
		t.Fatalf("crumb: %q", tt.Texts())
	}

	// Read-only: no new file, no trash.
	tt.Key(ui.Cmd, ui.KeyN)
	tt.Frame()
	if a.dlgOpen || !tt.HasText("压缩包内只读：可以复制（c）后到别处粘贴来解压") {
		t.Fatalf("newfile in zip: open %v status %q", a.dlgOpen, a.status)
	}

	// The filter and preview work inside.
	tt.Key(0, ui.KeyF)
	tt.Frame()
	tt.Type("top")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if cursorName(a) != "top.txt" {
		t.Fatalf("filter in zip: %q", cursorName(a))
	}
	tt.Key(0, ui.KeyEscape) // clear the filter
	tt.Frame()
	tt.Key(ui.Cmd|ui.Alt, ui.KeyP)
	tt.Frame()
	if !tt.HasText("zipped top") {
		t.Fatalf("zip preview: %q", tt.Texts())
	}

	// → on a file extracts it to the cache and opens it.
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if len(fs.opened) != 1 {
		t.Fatalf("opened %v (status %q)", fs.opened, a.status)
	}
	if b, err := os.ReadFile(fs.opened[0]); err != nil || string(b) != "zipped top" || !filepath.IsAbs(fs.opened[0]) {
		t.Fatalf("extracted %s: %q %v", fs.opened[0], b, err)
	}

	// Copy docs out: pasting outside extracts.
	tt.Key(0, ui.KeyHome)
	tt.Frame()
	goTo(t, a, tt, "docs")
	tt.Key(ui.Cmd, ui.KeyC)
	tt.Frame()
	tt.Key(0, ui.KeyLeft) // up out of the archive, onto it
	tt.Frame()
	if a.tab().t.Dir != root || cursorName(a) != "pack.zip" {
		t.Fatalf("out of zip: %s cursor %q", a.tab().t.Dir, cursorName(a))
	}
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	if b, err := os.ReadFile(filepath.Join(root, "docs", "readme.md")); err != nil || string(b) != "zipped readme" {
		t.Fatalf("paste out of zip: %q %v (status %q)", b, err, a.status)
	}
	tt.Key(ui.Cmd, ui.KeyBracketLeft) // back into the archive
	tt.Frame()
	if a.tab().t.Dir != vfs.ZipRoot(z) {
		t.Fatalf("back: %s", a.tab().t.Dir)
	}
}

func TestNewFileFromTemplate(t *testing.T) {
	a, tt, _, root := setup(t)
	a.tplDir = filepath.Join(t.TempDir(), "templates")
	if err := templates.Ensure(a.tplDir); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyN) // the new menu: a blank file, or a template
	tt.Frame()
	if !a.popOpen || !tt.HasText("main.go") {
		t.Fatalf("new menu: %v %q", a.popOpen, tt.Texts())
	}
	if err := tt.Click("空白文件"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.dlgOpen || !tt.HasText("模板") {
		t.Fatalf("dialog: %v %q", a.dlgOpen, tt.Texts())
	}
	if !slices.Equal(a.dlgTpls, []string{blankTemplate, "main.go", "Untitled.md", "Untitled.txt"}) {
		t.Fatalf("templates %v", a.dlgTpls)
	}
	a.dlgTpl = "Untitled.md" // as the Select would
	tt.Frame()
	if a.dlgText != "Untitled.md" {
		t.Fatalf("name follows template: %q", a.dlgText)
	}
	a.dlgText = "plan.md"
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	b, err := os.ReadFile(filepath.Join(root, "plan.md"))
	if err != nil || string(b) != "# 标题\n\n" || cursorName(a) != "plan.md" {
		t.Fatalf("plan.md: %q %v cursor %q", b, err, cursorName(a))
	}
}

func TestSidebarNavigation(t *testing.T) {
	a, tt, _, root := setup(t)
	vols := t.TempDir()
	os.Mkdir(filepath.Join(vols, "USBDISK"), 0o755)
	os.Symlink("/", filepath.Join(vols, "Macintosh HD"))
	a.sb.volumesDir = vols
	store := favorites.File{Path: filepath.Join(t.TempDir(), "favorites.json")}
	a.sb.store = store
	a.dataDir = t.TempDir()
	a.sb.show = false // on by default: turn it on again below
	tt.Frame()

	tt.Key(ui.Cmd|ui.Ctrl, ui.KeyS)
	tt.Frame()
	if !a.sb.show || !tt.HasText("个人收藏") || !tt.HasText("USBDISK") || tt.HasText("Macintosh HD") {
		t.Fatalf("sidebar: %v %q", a.sb.show, tt.Texts())
	}
	var cfg config
	if b, err := os.ReadFile(filepath.Join(a.dataDir, "config.json")); err != nil || json.Unmarshal(b, &cfg) != nil || !cfg.ShowSidebar {
		t.Fatalf("config not saved: %s %v", b, err)
	}

	// s s adds the current folder; it saves and shows.
	a.navigate(a.tab(), filepath.Join(root, "alpha"))
	tt.Frame()
	tt.Key(0, ui.KeyS)
	tt.Key(0, ui.KeyS)
	tt.Frame()
	if got, _ := store.Load(); !slices.Equal(got, []string{filepath.Join(root, "alpha")}) {
		t.Fatalf("favorites saved %v", got)
	}
	a.navigate(a.tab(), filepath.Join(root, "beta"))
	tt.Frame()
	if err := tt.Click("alpha"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.tab().t.Dir != filepath.Join(root, "alpha") {
		t.Fatalf("click favorite went to %s", a.tab().t.Dir)
	}
	// The outline stays on the files the click came from.
	tt.Frame()
	if a.focusZone != "pane0" || a.sb.h.FocusWithin(nil) {
		t.Fatalf("focus after a sidebar click: %q", a.focusZone)
	}
	if err := tt.Click("USBDISK"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.tab().t.Dir != filepath.Join(vols, "USBDISK") {
		t.Fatalf("click volume went to %s", a.tab().t.Dir)
	}
	a.tab().t.Back()
	a.afterNav(a.tab())
	tt.Frame()
	tt.Key(ui.Cmd|ui.Ctrl, ui.KeyBackspace)
	tt.Frame()
	if got, _ := store.Load(); len(got) != 0 {
		t.Fatalf("after remove %v", got)
	}
	tt.Key(ui.Cmd|ui.Ctrl, ui.KeyS)
	tt.Frame()
	if a.sb.show || tt.HasText("个人收藏") {
		t.Fatal("sidebar did not hide")
	}
}

func TestSettingsDialog(t *testing.T) {
	a, tt, fs, root := setup(t)
	a.dataDir = t.TempDir()
	tt.Key(ui.Cmd, ui.KeyComma)
	tt.Frame()
	if !a.dlgOpen || a.dlg != dlgSettings || !slices.Contains(a.setApps, "Ghostty") {
		t.Fatalf("settings: %v %v", a.dlgOpen, a.setApps)
	}
	a.setTerm = "Ghostty" // as the Select would
	tt.Frame()
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	var cfg config
	b, _ := os.ReadFile(filepath.Join(a.dataDir, "config.json"))
	if json.Unmarshal(b, &cfg) != nil || cfg.Terminal != "Ghostty" || cfg.Editor != "Visual Studio Code" {
		t.Fatalf("config %s", b)
	}
	tt.Key(ui.Cmd|ui.Alt, ui.KeyT)
	tt.Frame()
	if len(fs.openedWith) != 1 || fs.openedWith[0] != [2]string{"Ghostty", root} {
		t.Fatalf("terminal: %v", fs.openedWith)
	}
}

// TestFilterPinyin: the filter finds a name in Chinese by the first letters
// of its readings.
func TestFilterPinyin(t *testing.T) {
	a, tt, _, root := setup(t)
	os.WriteFile(filepath.Join(root, "模型查询.txt"), nil, 0o644)
	a.run("refresh")
	tt.Frame()
	tt.TypeKey(0, ui.KeyF, "f")
	tt.Frame()
	tt.Type("mxcx")
	tt.Frame()
	tt.Frame()
	es := a.tab().t.Entries()
	if len(es) != 1 || es[0].Name != "模型查询.txt" {
		t.Fatalf("filter %q left %d entries", a.tab().t.Filter, len(es))
	}
}

// TestGoStart: ` goes to the folder 设为起点 chose, else to ~, and to
// ~ when the folder is gone.
func TestGoStart(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	a, tt, _, root := setup(t)
	press := func() {
		tt.TypeKey(0, ui.KeyBackquote, "`")
		tt.Frame()
	}
	press()
	if a.tab().t.Dir != home {
		t.Fatalf("no start dir: %s, want %s", a.tab().t.Dir, home)
	}
	start := filepath.Join(root, "alpha")
	a.cfg.StartDir = start
	press()
	if a.tab().t.Dir != start {
		t.Fatalf("start dir: %s", a.tab().t.Dir)
	}
	a.navigate(a.tab(), root)
	os.RemoveAll(start)
	press()
	if a.tab().t.Dir != home {
		t.Fatalf("start dir gone: %s", a.tab().t.Dir)
	}
}

// TestInactiveGray: with goFinder in the background, the focus border of
// the current column is gray; back in front, it is blue again.
func TestInactiveGray(t *testing.T) {
	_, tt, _, _ := setup(t)
	r, _ := tt.Find("alpha")
	y := r.Y + 200 // below the rows: the border alone
	blue := func(c ui.Color) bool { return int(c.B) > int(c.R)+80 }
	bx := float32(-1)
	for x := r.X - 60; x < r.X; x++ {
		if blue(pixel(tt.Image(), x, y)) {
			bx = x
			break
		}
	}
	if bx < 0 {
		t.Fatal("no focus border while active")
	}
	tt.SetWindowActive(false)
	tt.Frame()
	if p := pixel(tt.Image(), bx, y); blue(p) {
		t.Fatalf("border %v while inactive", p)
	}
	tt.SetWindowActive(true)
	tt.Frame()
	if p := pixel(tt.Image(), bx, y); !blue(p) {
		t.Fatalf("border %v after coming back", p)
	}
}

// TestGoStartPerPane: the two panes keep their own start folder, for `
// and the status bar's 左：… 右：… (上：… 下：… one above the other);
// a pane with none goes home.
func TestGoStartPerPane(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	a, tt, _, root := setup(t)
	a.sb.home = home
	a.dataDir = t.TempDir()
	alpha, beta := filepath.Join(root, "alpha"), filepath.Join(root, "beta")
	if got := a.startLabel(); got != "起点：~" {
		t.Fatalf("single: %q", got)
	}
	a.setLayout(layoutDual)
	tt.Frame()
	a.setStartDir(0, alpha, true)
	a.setStartDir(1, beta, true)
	if a.cfg.StartDir != alpha || a.cfg.StartDir2 != beta || loadConfig(a.dataDir).StartDir2 != beta {
		t.Fatalf("start dirs %q %q", a.cfg.StartDir, a.cfg.StartDir2)
	}
	tt.Frame()
	if want := "左：" + alpha + "　右：" + beta; !tt.HasText(want) {
		t.Fatalf("status lacks %q", want)
	}
	press := func() {
		tt.TypeKey(0, ui.KeyBackquote, "`")
		tt.Frame()
	}
	tab := func(want int) {
		t.Helper()
		tt.Key(0, ui.KeyTab)
		tt.Frame()
		if a.active != want {
			t.Fatalf("Tab: pane %d", a.active)
		}
	}
	tab(1)
	a.navigate(a.tab(), root)
	press()
	if a.tab().t.Dir != beta || a.panes[0].tab().t.Dir == beta {
		t.Fatalf("right `: %s, left %s", a.tab().t.Dir, a.panes[0].tab().t.Dir)
	}
	tab(0)
	press()
	if a.tab().t.Dir != alpha {
		t.Fatalf("left `: %s", a.tab().t.Dir)
	}
	a.setStartDir(1, beta, false)
	a.setLayout(layoutRows)
	if got := a.startLabel(); got != "上："+alpha+"　下：~" {
		t.Fatalf("rows: %q", got)
	}
	tt.Frame()
	tab(1)
	press()
	if a.tab().t.Dir != home {
		t.Fatalf("right with none: %s", a.tab().t.Dir)
	}
}

// The sidebar follows Finder: its favorites in Finder's order, then the
// locations, then this app's favorites; it falls back to places() and
// reads Finder's list again when it changes.
func TestFinderSidebar(t *testing.T) {
	a, tt, fs, root := setup(t)
	home, vols := t.TempDir(), t.TempDir()
	for _, d := range []string{"Desktop", "dev"} {
		os.Mkdir(filepath.Join(home, d), 0o755)
	}
	os.MkdirAll(finder.ICloudDrive(home), 0o755)
	os.MkdirAll(filepath.Join(home, "Library", "CloudStorage", "GoogleDrive-me@example.com"), 0o755)
	os.Mkdir(filepath.Join(vols, "USBDISK"), 0o755)
	os.Mkdir(filepath.Join(vols, "77421E38-6F22-4598-BE16-E79E7AAA6B1F"), 0o755)

	// No Finder list yet: places() stand in.
	a.sb.show = false
	tt.Frame()
	a.sb.show, a.sb.home, a.sb.volumesDir = true, home, vols
	tt.Frame()
	tt.Frame()
	if a.sb.finderOK || !tt.HasText("个人收藏") || !tt.HasText("应用程序") {
		t.Fatalf("fallback: %v %q", a.sb.finderOK, tt.Texts())
	}
	for _, s := range []string{"位置", "iCloud云盘", "Google Drive", "USBDISK", "收藏夹"} {
		if !tt.HasText(s) {
			t.Fatalf("no %q in %q", s, tt.Texts())
		}
	}
	if tt.HasText("77421E38-6F22-4598-BE16-E79E7AAA6B1F") {
		t.Fatal("a UUID volume shows")
	}
	list := finder.ListDir(home)
	if fs.watches[list] == nil || fs.watches[vols] == nil {
		t.Fatalf("not watching Finder's list: %v", fs.watches)
	}

	// Finder's list comes in, in its order and names, and changes.
	os.MkdirAll(list, 0o755)
	write := func(es ...findertest.Entry) {
		os.WriteFile(filepath.Join(list, "com.apple.LSSharedFileList.FavoriteItems.sfl4"), findertest.List(es), 0o644)
		fs.watches[list]()
		tt.Frame()
		tt.Frame()
	}
	dev, desk := filepath.Join(home, "dev"), filepath.Join(home, "Desktop")
	write(findertest.Entry{Path: dev}, findertest.Entry{Path: desk, BookmarkName: "桌面"}, findertest.Entry{Path: root})
	got := func() (ps []string) {
		for _, it := range a.finderItems() {
			ps = append(ps, it.Path)
		}
		return ps
	}
	if !a.sb.finderOK || !slices.Equal(got(), []string{dev, desk, root}) || tt.HasText("应用程序") || !tt.HasText("桌面") {
		t.Fatalf("finder items %v %q", got(), tt.Texts())
	}
	// The current folder is chosen by its "p:" ID; a click goes there.
	if a.sb.sel != "p:"+root {
		t.Fatalf("chosen %q", a.sb.sel)
	}
	if err := tt.Click("dev"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.tab().t.Dir != dev || sidebarDir(a.sb.sel) != dev {
		t.Fatalf("click: %s %q", a.tab().t.Dir, a.sb.sel)
	}
	write(findertest.Entry{Path: desk, BookmarkName: "桌面"})
	if !slices.Equal(got(), []string{desk}) || a.sidebarID(dev) != "" {
		t.Fatalf("after Finder removed dev: %v", got())
	}
	if err := tt.Click("Google Drive"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if want := filepath.Join(home, "Library", "CloudStorage", "GoogleDrive-me@example.com"); a.tab().t.Dir != want || a.sb.sel != "l:"+want {
		t.Fatalf("location click: %s %q", a.tab().t.Dir, a.sb.sel)
	}

	// A broken list falls back again.
	os.WriteFile(filepath.Join(list, "com.apple.LSSharedFileList.FavoriteItems.sfl4"), []byte("junk"), 0o644)
	fs.watches[list]()
	tt.Frame()
	if a.sb.finderOK || !tt.HasText("应用程序") {
		t.Fatal("broken list: no fallback")
	}

	// Hidden, the sidebar stops watching.
	a.sb.show = false
	tt.Frame()
	if fs.watches[list] != nil || fs.watches[vols] != nil {
		t.Fatal("still watching with the sidebar hidden")
	}
}

// The window opens at the folder asked for, else Finder's new-window
// folder, else home.
func TestStartDirPriority(t *testing.T) {
	home, asked, fdir := t.TempDir(), t.TempDir(), t.TempDir()
	if got := startDir([]string{asked}, home, "", fdir); got != asked {
		t.Fatalf("asked: %s", got)
	}
	if got := startDir(nil, home, "", fdir); got != fdir {
		t.Fatalf("finder: %s", got)
	}
	if got := startDir([]string{filepath.Join(asked, "nope")}, home, "", fdir); got != fdir {
		t.Fatalf("missing folder: %s", got)
	}
	if got := startDir(nil, home, "", ""); got != home {
		t.Fatalf("home: %s", got)
	}
	// Finder's preference picks the folder, home when it is gone.
	prefs := filepath.Join(home, "Library", "Preferences")
	os.MkdirAll(prefs, 0o755)
	os.WriteFile(filepath.Join(prefs, "com.apple.finder.plist"), findertest.Plist(map[string]any{"NewWindowTarget": "PfDe"}), 0o644)
	if got := startDir(nil, home, "", finder.NewWindowDir(home)); got != home {
		t.Fatalf("no Desktop: %s", got)
	}
	os.Mkdir(filepath.Join(home, "Desktop"), 0o755)
	if got := startDir(nil, home, "", finder.NewWindowDir(home)); got != filepath.Join(home, "Desktop") {
		t.Fatalf("PfDe: %s", got)
	}
}

// 设为起点 on a folder sets startDir, saved; startDir opens there
// while the folder is there; choosing it again clears it.
func TestStartHere(t *testing.T) {
	a, tt, _, root := setup(t)
	a.dataDir = t.TempDir()
	dir := filepath.Join(root, "beta")
	choose := func() {
		t.Helper()
		if err := tt.RightClick("beta"); err != nil {
			t.Fatal(err)
		}
		if err := tt.ChooseMenuItem("设为起点"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	choose()
	if a.cfg.StartDir != dir {
		t.Fatalf("startDir %q", a.cfg.StartDir)
	}
	if got := loadConfig(a.dataDir).StartDir; got != dir {
		t.Fatalf("saved %q", got)
	}
	home := t.TempDir()
	if got := startDir(nil, home, dir, ""); got != dir {
		t.Errorf("start %q", got)
	}
	if got := startDir(nil, home, filepath.Join(dir, "gone"), ""); got != home {
		t.Errorf("gone folder: %q", got)
	}
	if got := startDir([]string{home}, "/", dir, ""); got != home {
		t.Errorf("command line first: %q", got)
	}
	choose()
	if a.cfg.StartDir != "" {
		t.Errorf("not cleared: %q", a.cfg.StartDir)
	}
}
