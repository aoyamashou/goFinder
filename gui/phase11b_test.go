package gui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// themed is the app's tester with the design theme applied, in the
// light or the dark appearance.
func themed(t *testing.T, dark bool) (*app, *ui.Tester) {
	t.Helper()
	a, _, _, _ := setup(t)
	tt := ui.NewTester(func(c *ui.Context) { applyTheme(c); a.view(c) }, 1100, 700)
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	return a, tt
}

// token is a color token in the tester's appearance.
func token(name string, dark bool) ui.Color {
	v := ds.colors[name]
	if dark {
		return v.dark
	}
	return v.light
}

// near tells whether the pixel p is within a few levels of col.
func near(p, col ui.Color) bool {
	d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
	return d(p.R, col.R) < 6 && d(p.G, col.G) < 6 && d(p.B, col.B) < 6
}

// colorPx is how many pixels of r (in DIPs) are col.
func colorPx(tt *ui.Tester, r ui.Rect, col ui.Color) int {
	img := tt.Image()
	s := img.Bounds().Dx() / 1100
	n := 0
	for y := int(r.Y) * s; y < int(r.Y+r.H)*s; y++ {
		for x := int(r.X) * s; x < int(r.X+r.W)*s; x++ {
			p := img.RGBAAt(x, y)
			if near(ui.Color{R: p.R, G: p.G, B: p.B, A: 255}, col) {
				n++
			}
		}
	}
	return n
}

func pixel(img *image.RGBA, x, y float32) ui.Color {
	s := float32(img.Bounds().Dx() / 1100)
	p := img.RGBAAt(int(x*s), int(y*s))
	return ui.Color{R: p.R, G: p.G, B: p.B, A: 255}
}

// The status bar is two rows on surface-100, a separator between.
func TestStatusBarTwoRows(t *testing.T) {
	for _, dark := range []bool{false, true} {
		a, tt := themed(t, dark)
		top, ok1 := tt.Find("回到起点")
		if !ok1 {
			t.Fatalf("no hint row: %q", tt.Texts())
		}
		a.status = "已复制 3 项"
		tt.Frame()
		msg, ok2 := tt.Find("已复制 3 项")
		if !ok2 || msg.Y >= top.Y {
			t.Fatalf("rows: message %v, hints %v", msg, top)
		}
		img := tt.Image()
		s100 := token("surface-100", dark)
		sep := token("separator", dark)
		if p := pixel(img, 3, msg.Y+msg.H/2); !near(p, s100) {
			t.Errorf("dark=%v: top row %v, want surface-100 %v", dark, p, s100)
		}
		if p := pixel(img, 3, top.Y+top.H/2); !near(p, s100) {
			t.Errorf("dark=%v: hint row %v, want surface-100 %v", dark, p, s100)
		}
		// Between the rows runs one separator line.
		found := false
		for y := msg.Y + msg.H/2; y < top.Y; y += 0.5 {
			if near(pixel(img, 3, y), sep) {
				found = true
			}
		}
		if !found {
			t.Errorf("dark=%v: no separator between the status rows", dark)
		}
	}
}

// The done dot shows with a result, and not without.
func TestStatusDoneDot(t *testing.T) {
	a, tt := themed(t, false)
	signal := token("signal", false)
	// The status bar's top row, under the list, whose cursor row is the
	// same cyan (the accent): only the row's own pixels count.
	a.status = "已复制 3 项"
	tt.Frame()
	msg, _ := tt.Find("已复制 3 项")
	row := ui.Rect{X: 0, Y: msg.Y - 2, W: 400, H: msg.H + 4}
	if n := colorPx(tt, row, signal); n < 8 {
		t.Fatalf("no dot with a result: %d px", n)
	}
	a.status = ""
	tt.Frame()
	if n := colorPx(tt, row, signal); n != 0 {
		t.Fatalf("a dot without a result: %d px", n)
	}
	a.status = "无法打开：没有权限"
	tt.Frame()
	if n := colorPx(tt, row, signal); n != 0 {
		t.Fatalf("a dot on an error: %d px", n)
	}
	// The filter sentence is signal-text.
	a.status = ""
	a.tab().t.Filter = "txt"
	tt.Frame()
	r, ok := tt.Find("过滤：txt")
	if !ok || colorPx(tt, r, token("signal-text", false)) == 0 {
		t.Fatalf("filter mark: %v %v", r, ok)
	}
}

// The trash confirmation lists names in mono muted and its button is danger.
func TestTrashDialogDanger(t *testing.T) {
	for _, dark := range []bool{false, true} {
		a, tt := themed(t, dark)
		tt.TypeKey(0, ui.KeyD, "d")
		tt.Frame()
		if !a.dlgOpen || a.dlg != dlgTrash {
			t.Fatal("no dialog")
		}
		r, ok := tt.Find("移到废纸篓")
		if !ok {
			t.Fatalf("no button: %q", tt.Texts())
		}
		if n := colorPx(tt, r, token("danger", dark)); n == 0 {
			t.Errorf("dark=%v: the button's text is not danger", dark)
		}
		c, _ := tt.Find("取消")
		if n := colorPx(tt, c, token("danger", dark)); n != 0 {
			t.Errorf("dark=%v: 取消 is danger", dark)
		}
		tt.Key(0, ui.KeyEscape)
	}
}

// The settings header is the wordmark, its period signal.
func TestSettingsWordmark(t *testing.T) {
	a, tt := themed(t, false)
	a.km.Override(map[string][]string{"hidden": {"Cmd+Shift+."}}) // the hints' "." cap would come first
	a.run("settings")
	tt.Frame()
	if !tt.HasText("goFinder") || !tt.HasText(".") {
		t.Fatalf("no wordmark: %q", tt.Texts())
	}
	dot, _ := tt.Find(".")
	if colorPx(tt, dot, token("signal", false)) == 0 {
		t.Errorf("the period is not signal: %v", dot)
	}
	name, _ := tt.Find("goFinder")
	if colorPx(tt, name, token("signal", false)) != 0 {
		t.Error("the name is signal")
	}
}

// The default agent's row sits on signal-soft; others do not.
func TestAgentDefaultRow(t *testing.T) {
	a, tt := themed(t, false)
	a.dataDir = t.TempDir()
	installed(a, "claude", "codex")
	a.setDefaultAgent("codex")
	a.run("settings")
	a.setPage = 2
	tt.Frame()
	tt.Frame()
	soft := token("signal-soft", false)
	// The title bar's Agent button also says "Codex", and every row has a
	// 默认 radio: the Codex row is the one under Claude's.
	cl, ok := tt.Find("Claude")
	if !ok {
		t.Fatalf("no rows: %q", tt.Texts())
	}
	img := tt.Image()
	found := false
	for y := cl.Y + cl.H; y < cl.Y+cl.H+80; y++ {
		if near(pixel(img, cl.X+cl.W+30, y), soft) {
			found = true
		}
	}
	if !found {
		t.Errorf("no signal-soft default row under Claude's")
	}
	if p := pixel(img, cl.X+cl.W+30, cl.Y+cl.H/2); near(p, soft) {
		t.Errorf("other row is signal-soft: %v", p)
	}
}

// Screenshots for the review, when MYGO_SHOTS names a folder.
func TestChromeShots(t *testing.T) {
	dir := os.Getenv("MYGO_SHOTS")
	if dir == "" {
		t.Skip("MYGO_SHOTS not set")
	}
	os.MkdirAll(dir, 0o755)
	for _, dark := range []bool{false, true} {
		name := map[bool]string{false: "light", true: "dark"}[dark]
		a, tt := themed(t, dark)
		a.dataDir = t.TempDir()
		installed(a, "claude", "codex")
		a.setDefaultAgent("codex")
		a.status = "已复制 3 项"
		a.showPreview = true
		a.sh.show = true
		a.shelve([]string{"/tmp/a.txt", "/tmp/b.txt"})
		tt.Frame()
		save := func(n string) {
			tt.Frame()
			writePNG(t, filepath.Join(dir, name+"-"+n+".png"), tt.Image())
		}
		save("main")
		tt.Key(ui.Shift, ui.KeySlash)
		save("help")
		tt.Key(0, ui.KeyEscape)
		tt.Key(0, ui.KeyEscape)
		tt.RightClick("notes.md")
		save("menu")
		tt.CloseMenu()
		tt.TypeKey(0, ui.KeyD, "d")
		save("trash")
		tt.Key(0, ui.KeyEscape)
		a.run("settings")
		save("settings")
		a.setPage = 2
		save("agents")
	}
}

func writePNG(t *testing.T, path string, img *image.RGBA) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
