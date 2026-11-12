package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

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

// TestGoStart: ` goes to the folder 启动时从这里开始 chose, else to ~, and to
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
