package gui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gofinder/internal/finder"
	"gofinder/internal/finder/findertest"
)

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
