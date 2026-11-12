package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// A new config hides the toolbar and draws dark, whatever macOS's look.
func TestToolbarHiddenAndDarkByDefault(t *testing.T) {
	cfg := defaultConfig()
	if cfg.ShowToolbar || cfg.Appearance != "" || !darkFor(cfg.Appearance, false) {
		t.Fatalf("defaults: toolbar %v appearance %q", cfg.ShowToolbar, cfg.Appearance)
	}
	a, tt, _, _ := setupWith(t, cfg, "")
	tt.Frame()
	if tt.HasText("‹ 后退") {
		t.Fatal("toolbar shows by default")
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

// ⌃⌘B shows and hides the toolbar, saved to config.json; light and system
// follow their word.
func TestToolbarToggleAndAppearanceModes(t *testing.T) {
	a, tt, _, _ := setupWith(t, defaultConfig(), "")
	a.dataDir = t.TempDir()
	a.run("toolbar")
	tt.Frame()
	if !a.cfg.ShowToolbar || !tt.HasText("‹ 后退") {
		t.Fatal("toolbar did not show")
	}
	if !loadConfig(a.dataDir).ShowToolbar {
		t.Fatal("not saved")
	}
	a.run("toolbar")
	tt.Frame()
	if tt.HasText("‹ 后退") {
		t.Fatal("toolbar did not hide")
	}
	if darkFor("light", true) || !darkFor("system", true) || darkFor("system", false) || !darkFor("dark", false) {
		t.Fatal("darkFor")
	}
	if appearanceIndex("") != 0 || appearanceIndex("system") != 2 {
		t.Fatal("appearanceIndex")
	}
}
