package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/icons"
	"gofinder/internal/vfs"
)

func TestColumnWidth(t *testing.T) {
	if nameCols("abc") != 3 || nameCols("文件.md") != 7 {
		t.Fatalf("cols: %d %d", nameCols("abc"), nameCols("文件.md"))
	}
	if w := fitWidth(1, 8, 13); w != minColW {
		t.Fatalf("short names: %v", w)
	}
	if w := fitWidth(500, 8, 13); w != maxColW {
		t.Fatalf("long names: %v", w)
	}
	w20, w21 := fitWidth(20, 8, 13), fitWidth(21, 8, 13)
	if w21-w20 != 8 || w20 <= minColW || w21 >= maxColW {
		t.Fatalf("fit: %v %v", w20, w21)
	}
	var wc widthCache
	es := []vfs.Entry{{Name: "a"}, {Name: "longer-name.txt"}, {Name: "link", IsLink: true}}
	if wc.maxCols(es) != 15 {
		t.Fatalf("maxCols %d", wc.cols)
	}
	es[1].Name = "x" // the same listing: the cache stands
	if wc.maxCols(es) != 15 || wc.maxCols(es[:1]) != 1 {
		t.Fatal("cache")
	}
}

func TestFourthColumn(t *testing.T) {
	a, tt, _, root := setup(t)
	os.WriteFile(filepath.Join(root, "blob.bin"), []byte{1, 0, 2, 0}, 0o644)
	a.load(a.tab())
	tt.Frame()
	if a.layout != layoutColumns || a.showPreview {
		t.Fatalf("default layout %d, preview %v", a.layout, a.showPreview)
	}
	// A folder: its items.
	if cursorName(a) != "alpha" || a.panes[0].pv.kind != pvDir || !tt.HasText("inside.txt") || !tt.HasText("inner") {
		t.Fatalf("folder: %q kind %d %q", cursorName(a), a.panes[0].pv.kind, tt.Texts())
	}
	// The folder under the cursor, and the current one in the parent
	// column, show open; the others closed.
	if a.nerd {
		if !tt.HasText(icons.DirOpen.Glyph) {
			t.Fatal("no open folder glyph")
		}
	}
	// A binary file: its details alone.
	goTo(t, a, tt, "blob.bin")
	tt.Frame()
	if a.panes[0].pv.kind != pvBinary || !tt.HasText("二进制文件") || tt.HasText("inside.txt") {
		t.Fatalf("binary: kind %d %q", a.panes[0].pv.kind, tt.Texts())
	}
	// A text file: its text.
	goTo(t, a, tt, "notes.md")
	tt.Frame()
	if a.panes[0].pv.kind != pvText || !tt.HasText("hello notes.md") || tt.HasText("二进制文件") {
		t.Fatalf("text: kind %d %q", a.panes[0].pv.kind, tt.Texts())
	}
}

func TestNerdFontFallback(t *testing.T) {
	a, tt, _, _ := setup(t)
	a.nerd = false
	tt.Frame()
	if !tt.HasText("▾") || !tt.HasText("▸") || tt.HasText(icons.DirClosed.Glyph) {
		t.Fatalf("fallback: %q", tt.Texts())
	}
	a.nerd = true
	tt.Frame()
	if !tt.HasText(icons.DirClosed.Glyph) || !tt.HasText(icons.DirOpen.Glyph) {
		t.Fatalf("glyphs: %q", tt.Texts())
	}
}

func TestFocusBorderFollows(t *testing.T) {
	a, tt, _, _ := setup(t)
	if !a.sb.show || a.focusZone != "pane0" {
		t.Fatalf("start: sidebar %v, focus %q", a.sb.show, a.focusZone)
	}
	tt.Key(0, ui.KeyTab) // columns: on to the sidebar
	tt.Frame()
	tt.Frame()
	if a.focusZone != "sidebar" {
		t.Fatalf("Tab: focus %q", a.focusZone)
	}
	tt.Key(0, ui.KeyRight) // → from the sidebar: back to the files
	tt.Frame()
	tt.Frame()
	if a.focusZone != "pane0" {
		t.Fatalf("Right: focus %q", a.focusZone)
	}
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	tt.Key(0, ui.KeyTab) // and Tab again
	tt.Frame()
	tt.Frame()
	if a.focusZone != "pane0" {
		t.Fatalf("Tab back: focus %q", a.focusZone)
	}

	// Two panes: Tab goes between them.
	a.setLayout(layoutDual)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	tt.Frame()
	if a.focusZone != "pane1" {
		t.Fatalf("dual: focus %q", a.focusZone)
	}
	// A click in the sidebar leaves the outline on the pane it came from
	// (user, 2026-11-12).
	if err := tt.Click("桌面"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.focusZone != "pane1" {
		t.Fatalf("sidebar click: focus %q", a.focusZone)
	}
}

func TestStartsAtHome(t *testing.T) {
	home, dir := t.TempDir(), t.TempDir()
	if got := startDir(nil, home, "", ""); got != home {
		t.Fatalf("no args: %s", got)
	}
	if got := startDir([]string{dir}, home, "", ""); got != dir {
		t.Fatalf("folder asked: %s", got)
	}
	if got := startDir([]string{filepath.Join(dir, "nope")}, home, "", ""); got != home {
		t.Fatalf("missing folder: %s", got)
	}
	// A session saved by an earlier version is not restored.
	data := t.TempDir()
	os.WriteFile(filepath.Join(data, "session.json"), []byte(`{"layout":"single","panes":[{"tabs":[{"dir":"/"}]}]}`), 0o644)
	km, _, _ := buildKeymap(testConfig(), "")
	a := newApp(km, &fakeSys{}, testConfig(), home)
	a.dataDir = data
	a.async = func(f func()) { f() }
	a.start()
	if a.tab().t.Dir != home || a.layout != layoutColumns {
		t.Fatalf("started at %s, layout %d", a.tab().t.Dir, a.layout)
	}
}

func TestMiddleTruncation(t *testing.T) {
	cases := []struct {
		name string
		room int
		want string
	}{
		{"short.go", 0, "short.go"},
		{"short.go", 8, "short.go"},
		{"build.gradle.kts", 14, "build.gra….kts"},
		{"build.gradle.kts", 13, "build.gr….kts"},
		{"gradle-wrapper.properties", 16, "grad….properties"},
		{"unlocknote-content", 15, "unlocknote-con…"},
		{"README.md", 1, "…"},
		{"文件名很长的文档.md", 10, "文件名….md"},
		{"archive.tar.gz", 4, "arc…"}, // no room for ".gz" and a start
	}
	for _, c := range cases {
		got := midTrunc(c.name, c.room)
		if got != c.want {
			t.Errorf("midTrunc(%q, %d) = %q, want %q", c.name, c.room, got, c.want)
		}
		if c.room > 0 && nameCols(got) > c.room {
			t.Errorf("midTrunc(%q, %d) = %q is %d wide", c.name, c.room, got, nameCols(got))
		}
	}
	if nameRoom(0, 8, 13) != 0 || nameRoom(rowChrome(13)+80, 8, 13) != 10 {
		t.Fatalf("room: %d %d", nameRoom(0, 8, 13), nameRoom(rowChrome(13)+80, 8, 13))
	}
	// A name of the widest column's length fits it uncut.
	if nameRoom(fitWidth(20, 8, 13), 8, 13) != 20 {
		t.Fatal("fitWidth and nameRoom disagree")
	}
}

func TestLongNameCutInList(t *testing.T) {
	a, tt, _, root := setup(t)
	long := "a-really-long-file-name-that-will-not-fit-in-a-column.properties"
	os.WriteFile(filepath.Join(root, long), nil, 0o644)
	a.load(a.tab())
	tt.Frame()
	tt.Frame() // the list's width is known from the second layout on
	found := ""
	for _, s := range tt.Texts() {
		if len(s) > 12 && s[len(s)-11:] == ".properties" {
			found = s
		}
	}
	if found == "" || found == long || !containsRune(found, '…') || !strings.HasPrefix(found, "a-really") {
		t.Fatalf("long name shown as %q in %q", found, tt.Texts())
	}
}

func containsRune(s string, r rune) bool {
	for _, x := range s {
		if x == r {
			return true
		}
	}
	return false
}

func TestRowLook(t *testing.T) {
	cases := []struct {
		cursor, chosen, focused bool
		want                    rowLook
	}{
		{false, false, true, lookPlain},
		{true, false, true, lookAccent}, // the cursor's row after a load: not chosen
		{false, true, true, lookAccent},
		{true, false, false, lookGray},
		{false, true, false, lookGray},
		{false, false, false, lookPlain},
	}
	for _, c := range cases {
		if got := lookOf(c.cursor, c.chosen, c.focused); got != c.want {
			t.Errorf("lookOf(%v, %v, %v) = %d, want %d", c.cursor, c.chosen, c.focused, got, c.want)
		}
	}
}

// The cursor's row is highlighted, never white on white: in the accent
// color while the list has the keyboard, in gray once the sidebar takes
// it or the window is inactive.
func TestCursorRowHighlight(t *testing.T) {
	a, tt, _, _ := setup(t)
	tt.Frame()
	if a.tab().cursor != 0 || a.tab().sel.Len() != 0 {
		t.Fatalf("after a load the cursor is on row 0, unchosen: %d %d", a.tab().cursor, a.tab().sel.Len())
	}
	// The row's background, right of the cursor's name.
	bg := func() (r, g, b uint8) {
		tt.Frame()
		rc, ok := tt.Find("report-2.txt") // the widest name: the column fits it
		if !ok {
			t.Fatal("no report-2.txt")
		}
		alpha, _ := tt.Find("alpha")
		x, y := int(rc.X+rc.W+4), int(alpha.Y+alpha.H/2)
		img := tt.Image()
		s := img.Bounds().Dx() / 1100
		p := img.RGBAAt(x*s, y*s)
		return p.R, p.G, p.B
	}
	if r, g, b := bg(); !(int(b) > int(r)+60) {
		t.Fatalf("focused: row %d,%d,%d is not in the accent color", r, g, b)
	}
	tt.Key(0, ui.KeyTab) // the sidebar takes the keyboard
	tt.Frame()
	if a.focusZone != "sidebar" {
		t.Fatalf("focus %q", a.focusZone)
	}
	isGray := func(r, g, b uint8) bool {
		d := max(r, g, b) - min(r, g, b)
		return d < 12 && r < 245 && r > 150
	}
	if r, g, b := bg(); !isGray(r, g, b) {
		t.Fatalf("unfocused: row %d,%d,%d is not gray", r, g, b)
	}
	tt.Key(0, ui.KeyTab) // back to the files, then the window goes inactive
	tt.Frame()
	tt.SetWindowActive(false)
	if r, g, b := bg(); !isGray(r, g, b) {
		t.Fatalf("inactive window: row %d,%d,%d is not gray", r, g, b)
	}
}
