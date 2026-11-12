package main

import (
	"image"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/keymap"
)

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
	q, ok1 := tt.Find("快捷键（Esc 关闭）")
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

// Tips of the toolbar show at once; the window's others after the default
// delay; a toolbar tip says what the button does and its key.
func TestToolbarTips(t *testing.T) {
	a, tt, _, _ := setupWith(t, testConfig(), "")
	tt.Frame()
	for _, id := range append(append([]string{}, defaultToolbar...), a.toolbarChoices()...) {
		act, ok := a.km.Action(id)
		if id == "-" || id == "@flex" || id == "agent" || strings.HasPrefix(id, "@") || !ok {
			continue
		}
		if _, has := toolbarTips[id]; !has {
			if _, sym := actionSymbols[id]; sym {
				t.Errorf("toolbar button %q has no tip", id)
			}
			continue
		}
		tip := a.toolbarTip(id, act.Title)
		if !strings.Contains(tip, " · "+toolbarTips[id]) || strings.Count(tip, " · ") < 1 {
			t.Errorf("tip %q", tip)
		}
		if keys := a.km.KeysFor(id); len(keys) > 0 && !strings.HasSuffix(tip, " · "+keys[0].String()) {
			t.Errorf("tip %q lacks its key", tip)
		}
	}
	if tip := a.toolbarTip("trash", "废纸篓"); tip != "废纸篓 · 删除选中项（可在废纸篓恢复） · "+a.km.KeysFor("trash")[0].String() {
		t.Fatalf("trash tip %q", tip)
	}
	if tip := a.toolbarTip("filter", "过滤"); !strings.HasPrefix(tip, "过滤 · 只显示名称包含输入文字的项 · f") {
		t.Fatalf("filter tip %q", tip)
	}

	r, ok := tt.Find("排序")
	if !ok {
		t.Fatalf("no sort button: %q", tt.Texts())
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	tt.Advance(20 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("选择文件的排序方式") {
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
	if tt.HasText("点击空白处输入路径") {
		t.Fatal("the window's tip shows at once")
	}
	tt.Advance(time.Second)
	tt.Frame()
	if !tt.HasText("点击空白处输入路径") {
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
