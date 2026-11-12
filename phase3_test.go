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
	a.cfg.WorkspacesSeeded, a.cfg.DefaultWorkspace = true, "写代码" // no demo workspaces here
	alpha := filepath.Join(root, "alpha")
	a.navigate(a.tab(), alpha)
	a.tab().sort = ui.SortOrder{Column: "size", Descending: true}
	tt.Key(ui.Cmd|ui.Alt, ui.Key4)
	tt.Frame()

	save := func(name string) {
		t.Helper()
		tt.Key(0, ui.KeyW)
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
	tt.Key(0, ui.KeyW)
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
	if !slices.Equal(a.wsList, []string{"写代码", "读文档"}) || !slices.Equal(a.cfg.Workspaces, a.wsList) {
		t.Fatalf("list %v order %v", a.wsList, a.cfg.Workspaces)
	}
	// The title bar's buttons.
	if !slices.Contains(tt.Texts(), "写代码") || !slices.Contains(tt.Texts(), "读文档") {
		t.Fatalf("title bar buttons: %q", tt.Texts())
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
	tt.Key(0, ui.KeyW)
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

// The first launch makes three workspaces to show what they do: Home in
// three columns, Download side by side, Documents one above the other;
// first in the title bar, in that order, and made once only.
func TestDemoWorkspaces(t *testing.T) {
	home := t.TempDir()
	for _, d := range []string{"Downloads", "Documents"} {
		os.MkdirAll(filepath.Join(home, d), 0o755)
	}
	a, tt, _, _ := setup(t)
	a.sb.home, a.dataDir = home, t.TempDir()
	os.MkdirAll(filepath.Join(a.dataDir, "workspaces"), 0o755)
	workspace.Write(a.wsPath("Aa"), workspace.State{Layout: "columns", Panes: []workspace.Pane{{Tabs: []workspace.Tab{{Dir: home}}}}})
	a.listWorkspaces()
	tt.Frame()
	if !slices.Equal(a.wsList, []string{"默认工作区", "下载", "文档", "Aa"}) || !a.cfg.WorkspacesSeeded ||
		!loadConfig(a.dataDir).WorkspacesSeeded || loadConfig(a.dataDir).DefaultWorkspace != "默认工作区" {
		t.Fatalf("list %v seeded %v default %q", a.wsList, a.cfg.WorkspacesSeeded, a.cfg.DefaultWorkspace)
	}
	if err := tt.Click("文档"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.Key2)
	tt.Frame()
	dl := filepath.Join(home, "Downloads")
	if a.wsCur != "下载" || a.layout != layoutDual || a.panes[0].tab().t.Dir != dl || a.panes[1].tab().t.Dir != dl {
		t.Fatalf("Download: cur %q layout %d %s", a.wsCur, a.layout, a.panes[0].tab().t.Dir)
	}
	tt.Key(0, ui.Key3)
	tt.Frame()
	doc := filepath.Join(home, "Documents")
	if a.wsCur != "文档" || a.layout != layoutRows || a.panes[1].tab().t.Dir != doc {
		t.Fatalf("3: cur %q layout %d", a.wsCur, a.layout)
	}
	tt.Key(0, ui.Key1)
	tt.Frame()
	if a.wsCur != "默认工作区" || a.layout != layoutColumns || a.tab().t.Dir != home {
		t.Fatalf("1: cur %q dir %s", a.wsCur, a.tab().t.Dir)
	}
	// Deleted, not made again.
	os.Remove(a.wsPath("下载"))
	a.listWorkspaces()
	if slices.Contains(a.wsList, "下载") {
		t.Fatalf("made again: %v", a.wsList)
	}
}

// The dialog and a workspace button's menu rename and delete; the
// default workspace is renamed but never deleted.
func TestWorkspaceRenameDelete(t *testing.T) {
	home := t.TempDir()
	a, tt, fs, _ := setup(t)
	a.sb.home, a.dataDir = home, t.TempDir()
	a.listWorkspaces()
	tt.Frame()
	tt.Key(0, ui.Key3) // 文档
	tt.Frame()
	// The dialog first (a click right after a drawn menu is the menu's):
	// 删除 is off on the default, and moves the others to the Trash.
	tt.Key(0, ui.KeyW)
	tt.Frame()
	a.wsPick = "默认工作区"
	tt.Frame()
	if b, ok := tt.AccessByRole(ui.RoleButton, "删除"); !ok || !b.Disabled {
		t.Fatalf("删除 on the default: %v %+v", ok, b)
	}
	a.wsPick = "下载"
	tt.Frame()
	b, ok := tt.AccessByRole(ui.RoleButton, "删除")
	if !ok || b.Disabled {
		t.Fatalf("删除: %v %+v", ok, b)
	}
	tt.ClickAt(b.Bounds.X+b.Bounds.W/2, b.Bounds.Y+b.Bounds.H/2)
	tt.Frame()
	if slices.Contains(a.wsList, "下载") || len(fs.trashed) != 1 || slices.Contains(loadConfig(a.dataDir).Workspaces, "下载") {
		t.Fatalf("deleted: %v trashed %v status %q", a.wsList, fs.trashed, a.status)
	}
	r, ok := tt.AccessByRole(ui.RoleButton, "重命名")
	if !ok {
		t.Fatalf("no 重命名: %q", tt.Texts())
	}
	tt.ClickAt(r.Bounds.X+r.Bounds.W/2, r.Bounds.Y+r.Bounds.H/2)
	tt.Frame()
	if a.dlg != dlgWsRename || a.dlgTarget != a.wsPick {
		t.Fatalf("dialog rename: %d %q", a.dlg, a.dlgTarget)
	}
	a.dlgOpen = false
	tt.Frame()

	// The menu of a button: 重命名… opens the rename dialog.
	if err := tt.RightClick("文档"); err != nil {
		t.Fatal(err)
	}
	if m := menuLabels(tt); !slices.Contains(m, "重命名…") || !slices.Contains(m, "删除") {
		t.Fatalf("menu %q", m)
	}
	if err := tt.ChooseMenuItem("重命名…"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.dlgOpen || a.dlg != dlgWsRename || a.dlgText != "文档" {
		t.Fatalf("dialog %v %d %q", a.dlgOpen, a.dlg, a.dlgText)
	}
	a.dlgText = "我的文档"
	a.submitDialog()
	tt.Frame()
	if !slices.Equal(a.wsList, []string{"默认工作区", "我的文档"}) || a.wsCur != "我的文档" ||
		!slices.Equal(loadConfig(a.dataDir).Workspaces, a.wsList) {
		t.Fatalf("renamed: %v cur %q", a.wsList, a.wsCur)
	}
	if _, err := os.Stat(a.wsPath("我的文档")); err != nil {
		t.Fatal(err)
	}
	// A taken name is refused.
	a.renameWorkspace("我的文档", "默认工作区")
	if !strings.Contains(a.status, "已有") || !slices.Contains(a.wsList, "我的文档") {
		t.Fatalf("taken: %q %v", a.status, a.wsList)
	}
	// The default: renamed, and stays the default; never deleted.
	a.renameWorkspace("默认工作区", "主页")
	if a.cfg.DefaultWorkspace != "主页" || loadConfig(a.dataDir).DefaultWorkspace != "主页" || a.wsList[0] != "主页" {
		t.Fatalf("default renamed: %q %v", a.cfg.DefaultWorkspace, a.wsList)
	}
	a.deleteWorkspace("主页")
	if !slices.Contains(a.wsList, "主页") || len(fs.trashed) != 1 || !strings.Contains(a.status, "不能删除") {
		t.Fatalf("default deleted: %v %q", a.wsList, a.status)
	}
}

// 0.18.4's demo workspaces (Home, Download, Documents) take the Chinese
// names, in place; Home becomes the default.
func TestDemoNamesMigrate(t *testing.T) {
	home := t.TempDir()
	a, tt, _, _ := setup(t)
	a.sb.home, a.dataDir = home, t.TempDir()
	st := workspace.State{Layout: "columns", Panes: []workspace.Pane{{Tabs: []workspace.Tab{{Dir: home}}}}}
	os.MkdirAll(filepath.Join(a.dataDir, "workspaces"), 0o755)
	for _, n := range []string{"Home", "Download", "Test1"} {
		workspace.Write(a.wsPath(n), st)
	}
	a.cfg.WorkspacesSeeded, a.cfg.Workspaces, a.wsCur = true, []string{"Home", "Download", "Test1"}, "Download"
	a.listWorkspaces()
	tt.Frame()
	if !slices.Equal(a.wsList, []string{"默认工作区", "下载", "Test1"}) || a.cfg.DefaultWorkspace != "默认工作区" || a.wsCur != "下载" {
		t.Fatalf("migrated: %v default %q cur %q", a.wsList, a.cfg.DefaultWorkspace, a.wsCur)
	}
	// Once only: a later Home stays Home.
	workspace.Write(a.wsPath("Home"), st)
	a.listWorkspaces()
	if !slices.Contains(a.wsList, "Home") {
		t.Fatalf("migrated again: %v", a.wsList)
	}
}
