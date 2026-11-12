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
