package gui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/icons"
)

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
