package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/finfo"
)

// inspectOn shows the inspector with the cursor on name.
func inspectOn(t *testing.T, name string) (*app, *ui.Tester, string) {
	t.Helper()
	a, tt, _, root := setup(t)
	goTo(t, a, tt, name)
	a.run("preview")
	tt.Frame()
	tt.Frame()
	return a, tt, filepath.Join(root, name)
}

// TestInspectorChmod: clicking a permission box changes the file.
func TestInspectorChmod(t *testing.T) {
	a, tt, p := inspectOn(t, "notes.md")
	os.Chmod(p, 0o644)
	a.pane().insp.path = ""
	tt.Frame()
	tt.Frame()
	if !tt.HasText("644") || !tt.HasText("rw-r--r--") {
		t.Fatalf("octal: %q", tt.Texts())
	}
	// Owner X on: 744.
	if err := tt.Click("所有者 X"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o744 {
		t.Fatalf("mode %v", st.Mode())
	}
	if !tt.HasText("744") || !tt.HasText("rwxr--r--") {
		t.Fatalf("shown: %q", tt.Texts())
	}
	// Others R off: 740.
	tt.Click("其他 R")
	tt.Frame()
	tt.Frame()
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o740 {
		t.Fatalf("mode %v", st.Mode())
	}
}

// TestInspectorFlags: the 隐藏 and 锁定 boxes set the flags.
func TestInspectorFlags(t *testing.T) {
	a, tt, p := inspectOn(t, "notes.md")
	if err := tt.Click("隐藏 开关"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	in, _ := finfo.Stat(p)
	if !in.Hidden {
		t.Fatalf("not hidden: %+v", in)
	}
	tt.Click("锁定 开关")
	tt.Frame()
	tt.Frame()
	if in, _ = finfo.Stat(p); !in.Locked {
		t.Fatalf("not locked: %+v", in)
	}
	if !a.pane().insp.info.Locked || !a.pane().insp.info.Hidden {
		t.Fatalf("inspector info %+v", a.pane().insp.info)
	}
	finfo.SetLocked(p, false) // so the temp dir can go
}

// TestInspectorFold: a group's title folds its rows.
func TestInspectorFold(t *testing.T) {
	_, tt, _ := inspectOn(t, "notes.md")
	tt.Click("权限")
	tt.Frame()
	if tt.HasText("八进制") || !tt.HasText("路径") {
		t.Fatalf("after folding: %q", tt.Texts())
	}
	tt.Click("权限")
	tt.Frame()
	if !tt.HasText("八进制") {
		t.Fatal("did not open again")
	}
}

// TestThirdColumnClicks: a click on an item of the third column enters
// its folder with the cursor on it; a double click opens it.
func TestThirdColumnClicks(t *testing.T) {
	var pending []func()
	old := pvAfter
	pvAfter = func(_ time.Duration, f func()) { pending = append(pending, f) }
	defer func() { pvAfter = old }()
	flush := func(tt *ui.Tester) {
		for _, f := range pending {
			f()
		}
		pending = nil
		tt.Frame()
		tt.Frame()
	}

	a, tt, fs, root := setup(t)
	if cursorName(a) != "alpha" {
		t.Fatalf("cursor %q", cursorName(a))
	}
	if err := tt.Click("inside.txt"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	flush(tt)
	if a.tab().t.Dir != filepath.Join(root, "alpha") || cursorName(a) != "inside.txt" {
		t.Fatalf("dir %s cursor %q", a.tab().t.Dir, cursorName(a))
	}

	// A double click on a file opens it, without entering the folder.
	a, tt, fs, root = setup(t)
	tt.Click("inside.txt")
	tt.Click("inside.txt")
	tt.Frame()
	flush(tt)
	if a.tab().t.Dir != root {
		t.Fatalf("double click entered %s", a.tab().t.Dir)
	}
	if len(fs.opened) != 1 || fs.opened[0] != filepath.Join(root, "alpha", "inside.txt") {
		t.Fatalf("opened %v", fs.opened)
	}

	// A double click on a folder enters it.
	a, tt, _, root = setup(t)
	tt.Click("inner")
	tt.Click("inner")
	tt.Frame()
	flush(tt)
	if a.tab().t.Dir != filepath.Join(root, "alpha", "inner") {
		t.Fatalf("dir %s", a.tab().t.Dir)
	}
}

// TestThirdColumnRightClick: the row's menu is the file row's, and what
// it chooses acts on that row.
func TestThirdColumnRightClick(t *testing.T) {
	a, tt, fs, root := setup(t)
	if err := tt.RightClick("inside.txt"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	m := tt.Menu()
	if !slicesHas(m, "复制") || !slicesHas(m, "移到废纸篓") && !slicesHas(m, "删除") {
		t.Fatalf("menu %q", m)
	}
	if err := tt.ChooseMenuItem("复制"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.tab().t.Dir != filepath.Join(root, "alpha") || cursorName(a) != "inside.txt" {
		t.Fatalf("dir %s cursor %q", a.tab().t.Dir, cursorName(a))
	}
	if len(a.clip) != 1 || a.clip[0] != filepath.Join(root, "alpha", "inside.txt") {
		t.Fatalf("clip %v", a.clip)
	}
	_ = fs
}

func slicesHas(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// TestInspectorShots writes screenshots of the inspector to MYGO_SHOTS.
func TestInspectorShots(t *testing.T) {
	dir := os.Getenv("MYGO_SHOTS")
	if dir == "" {
		t.Skip("MYGO_SHOTS not set")
	}
	os.MkdirAll(dir, 0o755)
	for _, dark := range []bool{false, true} {
		name := map[bool]string{false: "light", true: "dark"}[dark]
		a, tt := themed(t, dark)
		a.showPreview = true
		a.tab().cursor = a.tab().t.Index("notes.md")
		tt.Frame()
		writePNG(t, filepath.Join(dir, "inspector-"+name+"-file.png"), tt.Image())
		tt.Click("通用")
		tt.Frame()
		writePNG(t, filepath.Join(dir, "inspector-"+name+"-folded.png"), tt.Image())
	}
}
