package main

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

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
	installed := []string{"Ghostty", "Google Chrome", "Kaku", "Terminal", "Visual Studio Code", "Zed"}
	if got := commonApps(installed, commonTerminals, "Terminal"); !slices.Equal(got, []string{"Terminal", "Ghostty", "Kaku"}) {
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
