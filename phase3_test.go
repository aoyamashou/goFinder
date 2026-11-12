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

func TestWorkspaceSaveRestore(t *testing.T) {
	a, tt, _, root := setup(t)
	a.dataDir = t.TempDir()
	alpha := filepath.Join(root, "alpha")
	tt.Key(ui.Cmd, ui.KeyT)
	tt.Frame()
	a.navigate(a.tab(), alpha)
	a.tab().sort = ui.SortOrder{Column: "size", Descending: true}
	tt.Key(ui.Cmd|ui.Alt, ui.Key4)
	tt.Frame()
	want := a.snapshot()

	tt.Key(ui.Cmd|ui.Alt, ui.KeyW)
	tt.Frame()
	if !a.dlgOpen || a.dlg != dlgWorkspace || !tt.HasText("还没有保存的 Workspace") {
		t.Fatalf("⌘⌥W dialog: open %v %q", a.dlgOpen, tt.Texts())
	}
	a.dlgText = "写代码"
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if _, err := os.Stat(a.wsPath("写代码")); err != nil {
		t.Fatalf("not saved: %v (%s)", err, a.status)
	}

	// Change everything, then restore from the dialog.
	tt.Key(ui.Cmd|ui.Alt, ui.Key3)
	tt.Key(ui.Cmd, ui.KeyW)
	tt.Frame()
	tt.Key(ui.Cmd|ui.Alt, ui.KeyW)
	tt.Frame()
	if a.wsPick != "写代码" {
		t.Fatalf("dialog lists %v", a.wsNames)
	}
	if err := tt.Click("恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	got := a.snapshot()
	if got.Layout != "rows" || len(got.Panes[0].Tabs) != 2 || got.Panes[0].Cur != 1 ||
		got.Panes[0].Tabs[1] != (workspace.Tab{Dir: alpha, Sort: "size", Desc: true}) {
		t.Fatalf("restored %+v, want %+v", got, want)
	}
	if a.tab().t.Dir != alpha || !tt.HasText("inside.txt") {
		t.Fatalf("restored tab not loaded: %s", a.tab().t.Dir)
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
