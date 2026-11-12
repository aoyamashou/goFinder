package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/favorites"
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
