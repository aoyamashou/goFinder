package gui

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/keymap"
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
		a.setPage = pageAbout
		save("about")
		tt.Scroll(550, 400, 0, 2000)
		save("about-end")
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

// The default is 14; a config without the field, or with a size the
// settings do not offer, reads as 14.
func TestListSizeDefault(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"terminal":"Terminal","version":7}`), 0o644)
	cfg := loadConfig(dir)
	a, _, _, _ := setupWith(t, cfg, "")
	if cfg.ListFontSize != 0 || a.listSize() != 14 {
		t.Fatalf("old config: %d → %d", cfg.ListFontSize, a.listSize())
	}
	a.cfg.ListFontSize = 17
	if a.listSize() != 14 {
		t.Fatal("an unknown size is not 14")
	}
	if rowHeight(14) != 22 || rowHeight(13) != 22 || rowHeight(18) != 23 {
		t.Fatalf("row heights %v %v %v", rowHeight(13), rowHeight(14), rowHeight(18))
	}
}

// The settings' 列表字号 changes the row height, the glyph's size and the
// columns' measure at once, and writes config.json.
func TestListSizeSetting(t *testing.T) {
	a, tt, _, _ := setupWith(t, testConfig(), "")
	a.dataDir = t.TempDir()
	rowGap := func() float32 {
		r1, ok1 := tt.Find("alpha")
		r2, ok2 := tt.Find("notes.md")
		if !ok1 || !ok2 {
			t.Fatalf("rows missing: %q", tt.Texts())
		}
		return r2.Y - r1.Y
	}
	tt.Frame()
	h14, cw14 := rowGap(), a.charW
	if h14 != 2*rowHeight(14) { // alpha, it's a file.txt, notes.md
		t.Fatalf("14pt rows: %v apart", h14)
	}
	a.run("settings")
	tt.Frame()
	if err := tt.Click("18"); err != nil {
		t.Fatal(err)
	}
	a.run("settings") // close the dialog
	tt.Frame()
	tt.Frame()
	if a.cfg.ListFontSize != 18 || a.listSize() != 18 {
		t.Fatalf("size %d", a.cfg.ListFontSize)
	}
	if got := loadConfig(a.dataDir).ListFontSize; got != 18 {
		t.Fatalf("saved %d", got)
	}
	if h18 := rowGap(); h18 != 2*rowHeight(18) || h18 <= h14 {
		t.Fatalf("18pt rows: %v apart (14pt %v)", h18, h14)
	}
	if a.charW <= cw14 {
		t.Fatalf("column width did not follow: %v → %v", cw14, a.charW)
	}
	if glyphSize(18) != 21 || math.Abs(float64(iconW(14))-20) > 1 || iconW(18) <= iconW(14) {
		t.Fatalf("glyph %d, icon column %v", glyphSize(18), iconW(14))
	}
	if fitWidth(20, 8, 18) <= fitWidth(20, 8, 14) {
		t.Fatal("the chrome of a row does not grow with the size")
	}
}

// The cheat sheet is 440 wide; bindings are apart with a "/", the caps of
// one binding 2px apart.
func TestHelpSheetKeyGroups(t *testing.T) {
	if helpSheetW != 440 {
		t.Fatalf("width %d", helpSheetW)
	}
	a, tt, _, _ := setupWith(t, testConfig(), "")
	a.run("help")
	tt.Frame()
	tt.Frame()
	q, ok1 := tt.Find("速查表（Esc 关闭）")
	b, ok2 := tt.Access("在设置中修改")
	if !ok1 || !ok2 {
		t.Fatalf("sheet missing: %q", tt.Texts())
	}
	// The search box starts the column, the button ends it.
	if w := b.Bounds.X + b.Bounds.W - q.X; w < 439 || w > 441 {
		t.Fatalf("sheet content is %v wide (%v, %v)", w, q, b.Bounds)
	}
	if !tt.HasText("/") {
		t.Fatal("no / between bindings")
	}

	// "n n" is one binding of two caps 2px apart; two bindings are apart
	// with a /.
	nn := keymap.MustParse("n n")
	g := ui.NewTester(func(c *ui.Context) { applyTheme(c); keyGroups(c, []keymap.Chord{nn}) }, 1100, 40)
	g.SetDark(false)
	g.Frame()
	g.Frame()
	if g.HasText("/") {
		t.Fatal("a single binding has a /")
	}
	if gap := capGap(g.Image(), token("bg", false)); gap != 2 {
		t.Fatalf("caps of one binding %vpx apart", gap)
	}
	g2 := ui.NewTester(func(c *ui.Context) { applyTheme(c); keyGroups(c, []keymap.Chord{nn, keymap.MustParse("Cmd+Z")}) }, 1100, 40)
	g2.SetDark(false)
	g2.Frame()
	if !g2.HasText("/") {
		t.Fatalf("two bindings: %q", g2.Texts())
	}
}

// capGap is the width of the first run of col (the window's color) after
// the first cap, on a line through the caps: the space between two caps.
func capGap(img *image.RGBA, col ui.Color) int {
	s := img.Bounds().Dx() / 1100
	y := 9 // through the caps (they start at the top)
	clear := func(x int) bool { return near(pixel(img, float32(x)/float32(s), float32(y)/float32(s)), col) }
	x, w := 0, img.Bounds().Dx()
	for x < w && clear(x) {
		x++
	}
	for x < w && !clear(x) {
		x++
	}
	n := 0
	for x < w && clear(x) {
		x++
		n++
	}
	return n / s
}

// Tips of the title bar show at once; the window's others after the
// default delay; a title bar tip says what the button does and its key.
func TestTitleBarTips(t *testing.T) {
	a, tt, _, _ := setupWith(t, testConfig(), "")
	tt.Frame()
	for _, id := range []string{"agent", "workspace"} {
		act, _ := a.km.Action(id)
		tip := a.toolbarTip(id, act.Title)
		if !strings.Contains(tip, " · "+toolbarTips[id]) {
			t.Errorf("tip %q", tip)
		}
		if keys := a.km.KeysFor(id); len(keys) > 0 && !strings.HasSuffix(tip, " · "+keys[0].String()) {
			t.Errorf("tip %q lacks its key", tip)
		}
	}
	r, ok := tt.Find("工作区…")
	if !ok {
		t.Fatalf("no workspace button: %q", tt.Texts())
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	tt.Advance(20 * time.Millisecond)
	tt.Frame()
	if !hasPart(tt, "切换、保存、重命名、删除工作区") {
		t.Fatalf("no tip at once: %q", tt.Texts())
	}
	// Elsewhere (the path capsule's blank end) the default delay holds.
	tt.Move(1090, 690)
	tt.Advance(5 * time.Second)
	tt.Frame()
	crumb, ok := tt.Find("展开 /")
	if !ok {
		t.Fatalf("no crumb: %q", tt.Texts())
	}
	tt.Move(1000, crumb.Y+crumb.H/2)
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	if tt.HasText("点击空白处或按 g 输入路径") {
		t.Fatal("the window's tip shows at once")
	}
	tt.Advance(time.Second)
	tt.Frame()
	if !tt.HasText("点击空白处或按 g 输入路径") {
		t.Fatalf("the window's tip never shows: %q", tt.Texts())
	}
}

// The Agent button is one split button: the left opens the default agent,
// the right pops up the menu; with none enabled the left is disabled.
func TestAgentSplitButton(t *testing.T) {
	withAgents(t, "codex", agClaude, agCodex)
	a, tt, fs, _ := setup(t)
	a.cacheDir = t.TempDir()
	if !tt.HasText("Codex") {
		t.Fatalf("no name on the button: %q", tt.Texts())
	}
	left, _ := tt.Find("Codex")
	more, _ := tt.Find("▾")
	if more.X <= left.X+left.W {
		t.Fatalf("segments out of order: %v %v", left, more)
	}
	if err := tt.Click("在这里打开默认 Agent"); err != nil || len(fs.openedWith) != 1 {
		t.Fatalf("left: %v, opened %d", err, len(fs.openedWith))
	}
	if err := tt.Click("▾"); err != nil {
		t.Fatal(err)
	}
	if got := menuLabels(tt); len(got) != 2 {
		t.Fatalf("menu: %q", got)
	}
	tt.CloseMenu()
	if len(fs.openedWith) != 1 {
		t.Fatal("the right segment opened an agent")
	}

	withAgents(t, "" /* none */)
	a.cfg.Agents = map[string]agentCfg{"claude": {}, "codex": {}}
	tt.Frame()
	tt.Frame()
	if len(a.enabledAgents()) != 0 {
		t.Fatalf("still enabled: %v", agentIDs(a))
	}
	n := len(fs.openedWith)
	tt.Click("在这里打开默认 Agent")
	if len(fs.openedWith) != n {
		t.Fatal("a disabled left segment opened an agent")
	}
	info, ok := tt.Access("在这里打开默认 Agent")
	if ok && !info.Disabled {
		t.Fatal("left segment not disabled")
	}
}

// TestRightClickHotRow: a right-click on a row of the parent column lights
// it up as chosen while its menu shows; choosing an item or closing the
// menu puts it out.
func TestRightClickHotRow(t *testing.T) {
	a, tt, _, root := setup(t)
	tt.Key(0, ui.KeyRight) // into alpha: root is the parent column
	tt.Frame()
	if err := tt.RightClick("beta"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if want := filepath.Join(root, "beta"); a.menuHot != want {
		t.Fatalf("hot %q, want %q", a.menuHot, want)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if a.menuHot != "" {
		t.Fatalf("still hot after Escape: %q", a.menuHot)
	}

	copyPathOn(t, tt, "beta")
	tt.Frame()
	if a.menuHot != "" || a.copied != filepath.Join(root, "beta") {
		t.Fatalf("hot %q copied %q", a.menuHot, a.copied)
	}
}

// TestEjectVolume: a volume in the sidebar has an eject button, as
// Finder's; a tab inside the volume goes home first, and a failure shows
// in the status bar.
func TestEjectVolume(t *testing.T) {
	a, tt, fs, _ := setup(t)
	home, vols := t.TempDir(), t.TempDir()
	usb := filepath.Join(vols, "USBDISK")
	os.Mkdir(usb, 0o755)
	a.sb.show = false
	tt.Frame()
	a.sb.show, a.sb.home, a.sb.volumesDir = true, home, vols
	tt.Frame()
	tt.Frame()
	if !tt.HasText("USBDISK") {
		t.Fatalf("no volume in %q", tt.Texts())
	}
	a.navigate(a.tab(), usb)
	tt.Frame()
	if err := tt.Click("推出"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if len(fs.ejected) != 1 || fs.ejected[0] != usb {
		t.Fatalf("ejected %v", fs.ejected)
	}
	if a.tab().t.Dir != home {
		t.Fatalf("tab still in %s", a.tab().t.Dir)
	}
	if !strings.Contains(a.status, "已推出") {
		t.Fatalf("status %q", a.status)
	}

	fs.ejectErr = errors.New("busy")
	a.eject(usb)
	tt.Frame()
	if !strings.Contains(a.status, "无法推出") || !strings.Contains(a.status, "busy") {
		t.Fatalf("status %q", a.status)
	}
}

// TestCommonApps: the terminal and editor lists offer the common ones
// installed, not every app, and keep the current choice.
func TestCommonApps(t *testing.T) {
	installed := []string{"Ghostty", "Google Chrome", "Kaku", "Otty", "Terminal", "Visual Studio Code", "Zed"}
	if got := commonApps(installed, commonTerminals, "Terminal"); !slices.Equal(got, []string{"Terminal", "Ghostty", "Kaku", "Otty"}) {
		t.Fatalf("terminals %v", got)
	}
	if got := commonApps(installed, commonEditors, "MyEditor"); !slices.Equal(got, []string{"MyEditor", "Visual Studio Code", "Zed"}) {
		t.Fatalf("editors %v", got)
	}
}

// TestMenuSingleKeys: the right-click menu shows each action's first key,
// a double tap too: "c c" by 复制路径, not ⌥⌘C.
func TestMenuSingleKeys(t *testing.T) {
	_, tt, _, _ := setup(t)
	if err := tt.RightClick("notes.md"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("c c") || tt.HasText("⌥⌘C") {
		t.Fatalf("menu texts %q", tt.Texts())
	}
}

// scrollSettings scrolls the settings page to its end.
func scrollSettings(tt *ui.Tester) {
	r, _ := tt.Find("终端和编辑器")
	tt.Scroll(r.X+20, r.Y+20, 0, 2000)
	tt.Frame()
}

// TestSettingsFixedSize: the settings panel is one size on every page,
// its buttons inside it.
func TestSettingsFixedSize(t *testing.T) {
	a, tt, _, _ := setup(t)
	tt.Key(ui.Cmd, ui.KeyComma)
	tt.Frame()
	var tabs []ui.Rect
	for p := 0; p <= pageAbout; p++ {
		a.setPage = p
		tt.Frame()
		r, ok := tt.Find("完成")
		if p == 0 {
			r, ok = tt.Find("保存")
		}
		if !ok {
			t.Fatalf("page %d: no button in %q", p, tt.Texts())
		}
		tabs = append(tabs, r)
	}
	for p, r := range tabs {
		if math.Abs(float64(r.Y-tabs[0].Y)) > 2 || r.Y+r.H > 700 {
			t.Fatalf("page %d: button at %v, page 0's at %v", p, r, tabs[0])
		}
	}
}

// TestInspectorWidensWindow: the inspector widens the window to the right
// by its width, and hiding it gives the width back.
func TestInspectorWidensWindow(t *testing.T) {
	a, tt, _, _ := setup(t)
	var got []int
	a.growWindow = func(dw int) { got = append(got, dw) }
	a.run("preview")
	tt.Frame()
	a.run("preview")
	tt.Frame()
	w := int(px("inspector-width")) + 1
	if !slices.Equal(got, []int{w, -w}) {
		t.Fatalf("grew %v, want %d then %d", got, w, -w)
	}
}

// TestBrandAccentAndTitle: the accent is the brand cyan, whatever macOS's;
// the title strip's folder name is cyan while goFinder is active, gray in
// the background (black in the light theme).
func TestBrandAccentAndTitle(t *testing.T) {
	for _, dark := range []bool{true, false} {
		a, tt := themed(t, dark)
		a.cfg.Appearance = "system" // the tester's light or dark
		tt.SetTitleBar(ui.TitleBar{Height: 28, Left: 80})
		tt.Frame()
		tt.Frame()
		// The focused column's border is the accent.
		if colorPx(tt, ui.Rect{X: 0, Y: 0, W: 1100, H: 600}, token("selection", dark)) == 0 {
			t.Errorf("dark=%v: no brand cyan accent", dark)
		}
		r, ok := tt.Find(a.windowTitle())
		if !ok || r.Y > 28 {
			t.Fatalf("no title strip: %v %v", r, ok)
		}
		if colorPx(tt, r, token("signal-text", dark)) == 0 {
			t.Errorf("dark=%v: title not cyan", dark)
		}
		tt.SetWindowActive(false)
		tt.Frame()
		off := token("muted", dark)
		if !dark {
			off = token("ink", dark)
		}
		if colorPx(tt, r, token("signal-text", dark)) != 0 || colorPx(tt, r, off) == 0 {
			t.Errorf("dark=%v: background title not gray/black", dark)
		}
	}
}

func debugApp(t *testing.T) (*app, *ui.Tester) {
	t.Helper()
	a, _ := themed(t, false)
	tt := ui.NewTester(func(c *ui.Context) { applyTheme(c); a.view(c) }, 1100, 700)
	tt.Frame()
	tt.Frame()
	return a, tt
}

func idText(name string) string { return fmt.Sprint(debugIDs[name]) }

// TestDebugBadges: on, the main elements show their numbers and the
// status bar its Debug tag; off, they are gone and the layout is the same.
func TestDebugBadges(t *testing.T) {
	a, tt := debugApp(t)
	a.showPreview, a.sh.show = true, true
	tt.Frame()
	tt.Frame()
	before := strings.Join(tt.Texts(), "|")
	list0, _ := tt.Find("文件")
	cap0, _ := tt.Find("Debug")
	if cap0 != (ui.Rect{}) {
		t.Fatal("Debug tag while off")
	}
	a.debugOn = true
	tt.Frame()
	tt.Frame()
	for _, name := range []string{"titlebar", "tb.agent", "sidebar", "addr.capsule", "col.parent", "col.current", "col.preview",
		"inspector", "shelf", "status.msg", "status.hints", "status.layout", "status.insp", "status.debug"} {
		if !tt.HasText(idText(name)) {
			t.Errorf("no badge %s (%s): %q", name, idText(name), tt.Texts())
		}
	}
	if !tt.HasText("Debug") {
		t.Error("no Debug tag")
	}
	// Layout is unchanged by the badges (the tag adds one item to the
	// status row, which does not move the panes).
	list1, _ := tt.Find("文件")
	if list0 != list1 {
		t.Errorf("list bounds %v -> %v", list0, list1)
	}
	// Clicking the Debug tag turns it off.
	if err := tt.Click("Debug"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.debugOn || strings.Join(tt.Texts(), "|") != before {
		t.Errorf("still on after the tag: %v", a.debugOn)
	}
	list2, _ := tt.Find("文件")
	if list0 != list2 {
		t.Errorf("list bounds after off %v -> %v", list0, list2)
	}
}

// TestDebugBadgeNoClick: a press on the part of a badge that lies over its
// element goes through to the element.
func TestDebugBadgeNoClick(t *testing.T) {
	a := &app{debugOn: true}
	clicked := 0
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Padding(60).Children(func() {
			b := ui.Button(c.Key("b"), "Target").Height(40)
			a.dbg(c, b, "tb.agent")
			if b.Clicked() {
				clicked++
			}
		})
	}, 400, 300)
	tt.Frame()
	tt.Frame()
	badge, ok := tt.Find(idText("tb.agent"))
	if !ok {
		t.Fatalf("no badge: %q", tt.Texts())
	}
	// The button starts at 60 DIPs; the badge reaches 4 DIPs into it.
	if badge.Y+badge.H < 62 {
		t.Fatalf("badge %v does not reach the button", badge)
	}
	tt.ClickAt(badge.X+badge.W/2, 62)
	tt.Frame()
	if clicked != 1 {
		t.Fatalf("clicked %d times, badge %v", clicked, badge)
	}
}

// TestDebugPaneSuffix: with two panes the number carries the pane's.
func TestDebugPaneSuffix(t *testing.T) {
	a, tt := debugApp(t)
	a.debugOn = true
	a.setLayout(layoutDual)
	tt.Frame()
	tt.Frame()
	for _, s := range []string{idText("col.current") + "·1", idText("col.current") + "·2"} {
		if !tt.HasText(s) {
			t.Errorf("no %q in %q", s, tt.Texts())
		}
	}
}

// TestAboutPage: version, MyGo, bundle id, data folder, the Debug switch.
func TestAboutPage(t *testing.T) {
	a, tt := debugApp(t)
	a.dataDir = t.TempDir()
	a.run("settings")
	tt.Frame()
	if err := tt.Click("关于"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	for _, s := range []string{"版本 " + appVersion(), "MyGo", mygoVersion(), "Bundle ID", fallbackBundleID, a.dataDir,
		"项目仓库", "作者网站", "X 主页", "小红书", "GitHub", "致谢开源项目", "Yazi", "声明"} {
		if !tt.HasText(s) {
			t.Errorf("about lacks %q: %q", s, tt.Texts())
		}
	}
	if authorAvatar() == nil {
		t.Error("the avatar does not decode")
	}
	sys := a.sys.(*fakeSys)
	for _, l := range []string{"项目仓库", "小红书", "Yazi"} {
		if err := tt.Click(l); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	want := []string{repoURL, "https://xhslink.cn/o/2M9LOaUzn01", "https://github.com/sxyazi/yazi"}
	if !slices.Equal(sys.urls, want) {
		t.Errorf("opened %q, want %q", sys.urls, want)
	}
	if err := tt.Click(a.dataDir); err != nil {
		t.Fatal(err)
	}
	if a.status != "已复制数据目录路径" || tt.Clipboard() != a.dataDir {
		t.Errorf("copy: %q %q", a.status, tt.Clipboard())
	}
	tt.Scroll(550, 400, 0, 2000)
	tt.Frame()
	if err := tt.Click("Debug 编号"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !a.debugOn || !tt.HasText(idText("dlg.set.about")) {
		t.Errorf("switch: on=%v %q", a.debugOn, tt.Texts())
	}
}

// TestDebugIDsDoc: docs/debug-ids.md lists exactly the numbers of debugIDs,
// each once, with the element's name, and the numbers are unique and in
// the segments the plan gives.
func TestDebugIDsDoc(t *testing.T) {
	seen := map[int]string{}
	for n, id := range debugIDs {
		if o, dup := seen[id]; dup {
			t.Errorf("number %d is %s and %s", id, o, n)
		}
		seen[id] = n
		if id < 1 || id > 89 {
			t.Errorf("%s: %d out of 1–89", n, id)
		}
		seg := map[string][2]int{"tb.": {1, 19}, "titlebar": {1, 19}, "sidebar": {20, 29}, "addr.": {30, 39},
			"col.": {40, 59}, "inspector": {40, 59}, "shelf": {40, 59}, "status.": {60, 69}, "dlg.": {70, 89}, "menu.": {70, 89}}
		ok := false
		for p, r := range seg {
			if strings.HasPrefix(n, p) && id >= r[0] && id <= r[1] {
				ok = true
			}
		}
		if !ok {
			t.Errorf("%s: %d is outside its segment", n, id)
		}
	}
	b, err := os.ReadFile(filepath.Join("..", "docs", "debug-ids.md"))
	if err != nil {
		t.Fatal(err)
	}
	row := regexp.MustCompile("(?m)^\\| (\\d+) \\| `([^`]+)` \\|")
	doc := map[string]int{}
	for _, m := range row.FindAllStringSubmatch(string(b), -1) {
		id, _ := strconv.Atoi(m[1])
		if _, dup := doc[m[2]]; dup {
			t.Errorf("doc lists %s twice", m[2])
		}
		doc[m[2]] = id
	}
	for n, id := range debugIDs {
		if got, ok := doc[n]; !ok {
			t.Errorf("doc lacks %s (%d)", n, id)
		} else if got != id {
			t.Errorf("doc has %s as %d, table %d", n, got, id)
		}
	}
	for n := range doc {
		if _, ok := debugIDs[n]; !ok {
			t.Errorf("doc lists unknown %s", n)
		}
	}
}

// Screenshots for the review, when MYGO_SHOTS names a folder.
func TestDebugShots(t *testing.T) {
	dir := os.Getenv("MYGO_SHOTS")
	if dir == "" {
		t.Skip("MYGO_SHOTS not set")
	}
	os.MkdirAll(dir, 0o755)
	a, tt := themed(t, false)
	a.dataDir = t.TempDir()
	a.debugOn, a.showPreview, a.sh.show = true, true, true
	a.shelve([]string{"/tmp/a.txt", "/tmp/b.txt"})
	a.status = "已复制 3 项"
	save := func(n string) {
		tt.Frame()
		tt.Frame()
		writePNG(t, filepath.Join(dir, "debug-"+n+".png"), tt.Image())
	}
	save("main")
	a.setLayout(layoutDual)
	save("dual")
	a.setLayout(layoutColumns)
	a.run("settings")
	save("settings")
	tt.Click("关于")
	save("about")
}
