package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/favorites"
	"gofinder/internal/shelf"
	"gofinder/internal/workspace"
)

func TestShelf(t *testing.T) {
	a, tt, fs, root := setup(t)
	a.sh.file = filepath.Join(t.TempDir(), "shelf.json")
	goTo(t, a, tt, "notes.md")
	tt.Key(ui.Cmd|ui.Alt, ui.KeyS)
	tt.Frame()
	if !a.sh.show || !slices.Equal(a.sh.items, []string{filepath.Join(root, "notes.md")}) || !tt.HasText("暂存架（1）") {
		t.Fatalf("⌘⌥S: show %v items %v", a.sh.show, a.sh.items)
	}
	// Dropped from Finder onto the shelf panel, on the right.
	r1 := filepath.Join(root, "report-1.txt")
	if !tt.DropFiles(1050, 300, r1) {
		t.Fatal("shelf did not take the drop")
	}
	tt.Frame()
	if len(a.sh.items) != 2 {
		t.Fatalf("after drop: %v", a.sh.items)
	}
	if saved, _ := (shelf.File{Path: a.sh.file}).Load(); !slices.Equal(saved, a.sh.items) {
		t.Fatalf("saved %v", saved)
	}

	beta := filepath.Join(root, "beta")
	a.navigate(a.tab(), beta)
	tt.Frame()
	tt.Key(ui.Cmd|ui.Ctrl, ui.KeyV) // copy here: the shelf keeps them
	tt.Frame()
	if _, err := os.Stat(filepath.Join(beta, "notes.md")); err != nil || len(a.sh.items) != 2 {
		t.Fatalf("copy: %v items %v status %q", err, a.sh.items, a.status)
	}
	a.navigate(a.tab(), filepath.Join(root, "alpha"))
	tt.Frame()
	tt.Key(ui.Cmd|ui.Ctrl, ui.KeyM) // move here: they leave the shelf
	tt.Frame()
	if _, err := os.Stat(filepath.Join(root, "alpha", "report-1.txt")); err != nil || len(a.sh.items) != 0 {
		t.Fatalf("move: %v items %v status %q", err, a.sh.items, a.status)
	}
	if _, err := os.Stat(r1); !os.IsNotExist(err) {
		t.Fatal("moved file still at its source")
	}

	// Reveal and remove from the item's menu, and the shelf loads at start.
	a.sh.items = []string{filepath.Join(root, "report-2.txt"), filepath.Join(root, "gone")}
	a.saveShelf()
	b := newApp(a.km, fs, testConfig(), root)
	b.async, b.sh.file = func(f func()) { f() }, a.sh.file
	b.loadShelf()
	if !slices.Equal(b.sh.items, []string{filepath.Join(root, "report-2.txt")}) {
		t.Fatalf("loaded shelf %v (missing files should drop)", b.sh.items)
	}
	tt.Key(ui.Cmd|ui.Alt, ui.KeyB)
	tt.Frame()
	if a.sh.show {
		t.Fatal("⌘⌥B did not hide the shelf")
	}
}

// Workspaces take the place of tabs: saved from the dialog, switched with
// 1–9, [ ] and the strip; the one left is saved as it was.
func TestWorkspaceSaveRestore(t *testing.T) {
	a, tt, _, root := setup(t)
	a.dataDir = t.TempDir()
	alpha := filepath.Join(root, "alpha")
	a.navigate(a.tab(), alpha)
	a.tab().sort = ui.SortOrder{Column: "size", Descending: true}
	tt.Key(ui.Cmd|ui.Alt, ui.Key4)
	tt.Frame()

	save := func(name string) {
		t.Helper()
		tt.Key(ui.Cmd|ui.Alt, ui.KeyW)
		tt.Frame()
		if !a.dlgOpen || a.dlg != dlgWorkspace {
			t.Fatalf("⌘⌥W dialog: open %v", a.dlgOpen)
		}
		a.dlgText = name
		tt.Frame()
		tt.Key(0, ui.KeyEnter)
		tt.Frame()
		if _, err := os.Stat(a.wsPath(name)); err != nil || a.wsCur != name {
			t.Fatalf("not saved: %v cur %q (%s)", err, a.wsCur, a.status)
		}
	}
	tt.Key(ui.Cmd|ui.Alt, ui.KeyW)
	tt.Frame()
	if !tt.HasText("还没有保存的工作区") {
		t.Fatalf("empty dialog %q", tt.Texts())
	}
	a.dlgOpen = false
	tt.Frame()
	save("写代码")

	// A second one: three columns at the root.
	tt.Key(ui.Cmd|ui.Alt, ui.Key3)
	a.navigate(a.tab(), root)
	tt.Frame()
	save("读文档")
	if !slices.Equal(a.wsList, []string{"写代码", "读文档"}) || !tt.HasText("1 写代码") {
		t.Fatalf("list %v, strip missing", a.wsList)
	}

	tt.Key(0, ui.Key1)
	tt.Frame()
	got := a.snapshot()
	if a.wsCur != "写代码" || got.Layout != "rows" || got.Panes[0].Tabs[0] != (workspace.Tab{Dir: alpha, Sort: "size", Desc: true}) {
		t.Fatalf("1: cur %q %+v", a.wsCur, got)
	}
	if a.tab().t.Dir != alpha || !tt.HasText("inside.txt") {
		t.Fatalf("restored pane not loaded: %s", a.tab().t.Dir)
	}
	// Left as it is now, and found so on coming back.
	a.navigate(a.tab(), root)
	tt.Frame()
	tt.Key(0, ui.KeyBracketRight)
	tt.Frame()
	if a.wsCur != "读文档" || a.layout != layoutColumns {
		t.Fatalf("]: cur %q layout %d", a.wsCur, a.layout)
	}
	tt.Key(0, ui.KeyBracketLeft)
	tt.Frame()
	if a.wsCur != "写代码" || a.tab().t.Dir != root || a.layout != layoutRows {
		t.Fatalf("[: cur %q dir %s", a.wsCur, a.tab().t.Dir)
	}
	tt.Key(ui.Cmd, ui.Key2)
	tt.Frame()
	if a.wsCur != "读文档" {
		t.Fatalf("⌘2: cur %q", a.wsCur)
	}
	tt.Key(0, ui.Key9)
	tt.Frame()
	if a.wsCur != "读文档" || !strings.Contains(a.status, "没有第 9 个工作区") {
		t.Fatalf("9: cur %q status %q", a.wsCur, a.status)
	}
	// From the dialog.
	tt.Key(ui.Cmd|ui.Alt, ui.KeyW)
	tt.Frame()
	a.wsPick = "写代码"
	tt.Frame()
	if err := tt.Click("切换"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.wsCur != "写代码" || a.layout != layoutRows {
		t.Fatalf("dialog: cur %q", a.wsCur)
	}
}

func TestFavoritesICloudSync(t *testing.T) {
	a, tt, fs, _ := setup(t)
	cloudDir := filepath.Join(t.TempDir(), "iCloud", "goFinder")
	cloud := favorites.File{Path: filepath.Join(cloudDir, "favorites.json")}
	local := favorites.File{Path: filepath.Join(t.TempDir(), "favorites.json")}
	local.Save([]string{"/old"})
	a.sb.store, a.sb.favFrom, a.sb.favDir = cloud, local, cloudDir
	a.loadFavorites()
	if !slices.Equal(a.sb.favs, []string{"/old"}) {
		t.Fatalf("migrated favs %v", a.sb.favs)
	}
	a.sb.show = true
	tt.Frame()
	changed := fs.watches[cloudDir]
	if changed == nil {
		t.Fatalf("favorites folder not watched: %v", fs.watches)
	}
	// Here: add /mine. Meanwhile another Mac removed /old and added /theirs.
	a.setFavorite("/mine", true)
	cloud.Save([]string{"/theirs"})
	a.sb.favBase = []string{"/old"} // as if their write raced ours
	changed()
	tt.Frame()
	want := []string{"/theirs", "/mine"}
	if !slices.Equal(a.sb.favs, want) || !tt.HasText("theirs") {
		t.Fatalf("merged %v", a.sb.favs)
	}
	if disk, _ := cloud.Load(); !slices.Equal(disk, want) {
		t.Fatalf("merge not saved: %v", disk)
	}
	a.sb.show = false
	tt.Frame()
	if fs.watches[cloudDir] != nil {
		t.Fatal("still watching with the sidebar hidden")
	}
}

func TestGitStatusMarks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	a, tt, _, root := setup(t)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "trunk")
	git("add", "report-1.txt", "alpha/inside.txt")
	git("commit", "-q", "-m", "init")
	os.WriteFile(filepath.Join(root, "report-1.txt"), []byte("changed"), 0o644)
	os.WriteFile(filepath.Join(root, "alpha", "inside.txt"), []byte("changed"), 0o644)

	a.load(a.tab())
	tt.Frame()
	has := func(s string) bool { return slices.Contains(tt.Texts(), s) }
	if !has("M") || !has("?") || !has("•") {
		t.Fatalf("marks missing: %q", tt.Texts())
	}
	if !slices.ContainsFunc(tt.Texts(), func(s string) bool { return strings.HasPrefix(s, "⎇ trunk · ") }) {
		t.Fatalf("branch missing: %q", tt.Texts())
	}
	// Inside a zip, or outside any repository, git does not run.
	a.navigate(a.tab(), t.TempDir())
	tt.Frame()
	if a.tab().gitRoot != "" || a.gitBranch(a.tab()) != "" {
		t.Fatalf("outside a repo: root %q", a.tab().gitRoot)
	}
}
