package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

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
