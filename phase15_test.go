package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

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
	for _, name := range []string{"toolbar", "tb.back", "sidebar", "addr.capsule", "col.parent", "col.current", "col.preview",
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
			a.dbg(c, b, "tb.back")
			if b.Clicked() {
				clicked++
			}
		})
	}, 400, 300)
	tt.Frame()
	tt.Frame()
	badge, ok := tt.Find(idText("tb.back"))
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
	for _, s := range []string{"版本", "MyGo", mygoVersion(), "Bundle ID", fallbackBundleID, a.dataDir} {
		if !tt.HasText(s) {
			t.Errorf("about lacks %q: %q", s, tt.Texts())
		}
	}
	if err := tt.Click(a.dataDir); err != nil {
		t.Fatal(err)
	}
	if a.status != "已复制数据目录路径" || tt.Clipboard() != a.dataDir {
		t.Errorf("copy: %q %q", a.status, tt.Clipboard())
	}
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
		seg := map[string][2]int{"tb.": {1, 19}, "titlebar": {1, 19}, "toolbar": {1, 19}, "sidebar": {20, 29}, "addr.": {30, 39},
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
	b, err := os.ReadFile(filepath.Join("docs", "debug-ids.md"))
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
	a.status = "已拷贝 3 项"
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
