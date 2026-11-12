package gui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/icons"
)

// Every token of docs/design-tokens.json parses, aliases resolved, in both
// themes; the ones the code names exist.
func TestDesignTokens(t *testing.T) {
	d, err := parseTokens(designTokensJSON)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"bg", "surface-100", "surface-200", "separator", "ink", "muted", "faint",
		"selection-inactive", "signal", "signal-text", "danger", "git-modified", "type-folder"} {
		if _, ok := d.colors[n]; !ok {
			t.Errorf("no color %s", n)
		}
	}
	if d.colors["selection-inactive"] != d.colors["surface-200"] {
		t.Error("alias selection-inactive → surface-200 not resolved")
	}
	if d.colors["bg"].light == d.colors["bg"].dark {
		t.Error("bg is the same in both themes")
	}
	for _, n := range []string{"row-height", "radius-row", "radius-pill", "space-1.5", "inspector-width", "focus-width"} {
		if _, ok := d.lengths[n]; !ok {
			t.Errorf("no length %s", n)
		}
	}
	if d.lengths["row-height"] != 22 || d.lengths["focus-width"] != 1.5 {
		t.Errorf("lengths %v", d.lengths)
	}
}

// A new config draws dark, whatever macOS's look; there is no toolbar.
func TestNoToolbarAndDarkByDefault(t *testing.T) {
	cfg := defaultConfig()
	if cfg.Appearance != "" || !darkFor(cfg.Appearance, false) {
		t.Fatalf("defaults: appearance %q", cfg.Appearance)
	}
	a, tt, _, _ := setupWith(t, cfg, "")
	tt.Frame()
	if tt.HasText("‹ 后退") || tt.HasText("后退") {
		t.Fatalf("a toolbar shows: %q", tt.Texts())
	}
	if _, ok := a.km.Action("toolbar"); ok {
		t.Fatal("the toolbar action is still there")
	}
	// The tester's system look is light: the window is dark anyway.
	var bg ui.Color
	probe := ui.NewTester(func(c *ui.Context) {
		applyAppearance(c, a.cfg.Appearance)
		bg = c.Theme().Background
	}, 100, 100)
	probe.Frame()
	if bg != ds.colors["bg"].dark {
		t.Fatalf("background %v, want the dark bg", bg)
	}
}

// light and system follow their word.
func TestAppearanceModes(t *testing.T) {
	if darkFor("light", true) || !darkFor("system", true) || darkFor("system", false) || !darkFor("dark", false) {
		t.Fatal("darkFor")
	}
	if appearanceIndex("") != 0 || appearanceIndex("system") != 2 {
		t.Fatal("appearanceIndex")
	}
}

// inTheme runs f in a frame of the light or the dark theme, after
// applyTheme, as view() does.
func inTheme(t *testing.T, dark bool, f func(c *ui.Context)) {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		th := *ui.LightTheme()
		if dark {
			th = *ui.DarkTheme()
		}
		c.SetTheme(&th)
		applyTheme(c)
		f(c)
		ui.Text(c, "x")
	}, 200, 100)
	tt.Frame()
}

func TestApplyThemeUsesTokens(t *testing.T) {
	for _, dark := range []bool{false, true} {
		inTheme(t, dark, func(c *ui.Context) {
			if got, want := c.Theme().Background, tok(c, "bg"); got != want {
				t.Errorf("dark=%v: Background %v, want bg %v", dark, got, want)
			}
		})
	}
}

func TestRowColors(t *testing.T) {
	for _, dark := range []bool{false, true} {
		inTheme(t, dark, func(c *ui.Context) {
			// Selected with focus: selection fill, selection-ink everywhere.
			if bg, ok := rowFill(c, lookAccent); !ok || bg != tok(c, "selection") {
				t.Errorf("accent row fill %v %v", bg, ok)
			}
			g, n, m := rowColors(c, lookAccent, icons.KindCode, false, "M")
			ink := tok(c, "selection-ink")
			if g != ink || n != ink || m != ink {
				t.Errorf("accent row colors %v %v %v, want %v", g, n, m, ink)
			}
			// Without focus: selection-inactive fill, ink name, type-colored glyph.
			if bg, ok := rowFill(c, lookGray); !ok || bg != tok(c, "selection-inactive") {
				t.Errorf("gray row fill %v %v", bg, ok)
			}
			g, n, m = rowColors(c, lookGray, icons.KindCode, false, "M")
			if g != tok(c, "type-code") || n != tok(c, "ink") || m != tok(c, "git-modified") {
				t.Errorf("gray row colors %v %v %v", g, n, m)
			}
			// Plain: no fill; a hidden name is faint, never tinted.
			if _, ok := rowFill(c, lookPlain); ok {
				t.Error("plain row has a fill")
			}
			if _, n, _ = rowColors(c, lookPlain, icons.KindFolder, true, ""); n != tok(c, "faint") {
				t.Errorf("hidden name %v", n)
			}
			if _, n, _ = rowColors(c, lookPlain, icons.KindFolder, false, ""); n != tok(c, "ink") {
				t.Errorf("folder name %v is tinted", n)
			}
		})
	}
}

func TestGitAndKindTokens(t *testing.T) {
	inTheme(t, false, func(c *ui.Context) {
		for mark, name := range map[string]string{"M": "git-modified", "A": "git-added", "?": "git-untracked", "U": "git-conflict"} {
			if gitColor(c, mark) != tok(c, name) {
				t.Errorf("git %q is not %s", mark, name)
			}
		}
	})
	for k := icons.KindDoc; k <= icons.KindConfig; k++ {
		tok0 := kindToken(k)
		if _, ok := ds.colors[tok0]; !ok {
			t.Errorf("kind %d: no token %q", k, tok0)
		}
	}
}

// TestWordmarkPNG writes the settings' wordmark ("goFinder." as the 关于
// page shows it) with a transparent background, for the README: one for
// light pages, one for dark. Run with MYGO_WORDMARK set to a folder.
func TestWordmarkPNG(t *testing.T) {
	dir := os.Getenv("MYGO_WORDMARK")
	if dir == "" {
		t.Skip("MYGO_WORDMARK not set")
	}
	for _, dark := range []bool{false, true} {
		shot := func(bg ui.Color) *image.RGBA {
			tt := ui.NewTester(func(c *ui.Context) {
				applyAppearance(c, map[bool]string{false: "light", true: "dark"}[dark])
				ui.Column(c).Grow(1).Padding(8).Background(bg).AlignItems(ui.Start).Children(func() { wordmark(c) })
			}, 200, 50)
			tt.SetScale(4)
			tt.Frame()
			tt.Frame()
			return tt.Image()
		}
		w, b := shot(ui.Color{R: 255, G: 255, B: 255, A: 255}), shot(ui.Color{A: 255})
		r := w.Bounds()
		out := image.NewNRGBA(r)
		box := image.Rectangle{Min: r.Max, Max: r.Min}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				pw, pb := w.RGBAAt(x, y), b.RGBAAt(x, y)
				// Difference matting: over white and black, a = 1 - (w - b).
				d := (int(pw.R) - int(pb.R) + int(pw.G) - int(pb.G) + int(pw.B) - int(pb.B)) / 3
				a := max(0, min(255, 255-d))
				if a == 0 {
					continue
				}
				un := func(v uint8) uint8 { return uint8(min(255, int(v)*255/a)) }
				out.SetNRGBA(x, y, color.NRGBA{R: un(pb.R), G: un(pb.G), B: un(pb.B), A: uint8(a)})
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
		crop := out.SubImage(box.Inset(-4))
		name := map[bool]string{false: "wordmark-light.png", true: "wordmark-dark.png"}[dark]
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, crop); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}
