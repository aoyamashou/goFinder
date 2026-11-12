package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/keymap"
)

func TestVimSwitch(t *testing.T) {
	a, tt, _, _ := setupWith(t, testConfig(), `{"refresh":["h"],"cursor.up":["u"]}`)
	a.dataDir = filepath.Dir(a.kmPath)
	before, err := os.ReadFile(a.kmPath)
	if err != nil {
		t.Fatal(err)
	}
	if a.cfg.VimKeys || loadConfig(a.dataDir).VimKeys {
		t.Fatal("Vim keys enabled by default")
	}
	a.run("settings")
	a.setPage = 1
	tt.Frame()
	for _, vim := range []bool{true, false, true} {
		a.recording, a.recEat = "refresh", true
		a.km.Press(&a.taps, keymap.MustParse("n"), time.Now())
		if err := tt.Click("Vim 键位"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.cfg.VimKeys != vim || loadConfig(a.dataDir).VimKeys != vim {
			t.Fatalf("switch not persisted: want %v", vim)
		}
		if _, pending := a.taps.Pending(); pending || a.recording != "" || a.recEat {
			t.Fatal("switch kept recording or a pending double tap")
		}
		if id, _ := a.km.Lookup(keymap.MustParse("h")); id != "refresh" {
			t.Fatalf("custom h lost: %q", id)
		}
		if a.firstKey("cursor.up") != "u" {
			t.Fatal("custom up key lost")
		}
		if _, bound := a.km.Lookup(keymap.MustParse("j")); bound != vim {
			t.Fatalf("j bound=%v, Vim=%v", bound, vim)
		}
		if vim && len(a.warns) == 0 {
			t.Fatal("h conflict not reported")
		}
		restarted, _, _ := buildKeymap(loadConfig(a.dataDir), a.kmPath)
		if !reflect.DeepEqual(a.km.Bindings(), restarted.Bindings()) {
			t.Fatal("restart and live switch use different bindings")
		}
		after, err := os.ReadFile(a.kmPath)
		if err != nil || string(before) != string(after) {
			t.Fatalf("switch changed keymap.json: %v", err)
		}
	}
	if err := tt.Click("完成"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.cfg.VimKeys || a.dlgOpen {
		t.Fatal("closing settings reverted Vim keys")
	}
}

func TestVimSwitchUsesMemory(t *testing.T) {
	a, _, _, _ := setupWith(t, testConfig(), "")
	var jobs []func()
	a.async = func(f func()) { jobs = append(jobs, f) }
	a.setKeys("cursor.down", []string{"m"}, false) // write remains queued
	a.setVimKeys(true)
	if a.firstKey("cursor.down") != "m" {
		t.Fatal("switch reread stale keymap.json")
	}
	for _, job := range jobs {
		job()
	}
	for _, vim := range []bool{true, false} {
		a.setVimKeys(vim)
		a.setKeys("cursor.down", nil, false)
		if len(a.km.KeysFor("cursor.down")) != 0 {
			t.Fatal("clear left an extra movement key")
		}
		a.setKeys("cursor.down", nil, true)
		want := ""
		if vim {
			want = "j"
		}
		if a.firstKey("cursor.down") != want {
			t.Fatalf("mode %v reset to %q, want %q", vim, a.firstKey("cursor.down"), want)
		}
	}
}

func TestVimSaveFailure(t *testing.T) {
	a, _, _, root := setup(t)
	a.dataDir = filepath.Join(root, "notes.md", "not-a-directory")
	a.setVimKeys(true)
	if !a.cfg.VimKeys || !strings.Contains(a.status, "无法保存 config.json") {
		t.Fatalf("save failure not reported: %q", a.status)
	}
}

func TestVimCustomCommandPriority(t *testing.T) {
	cfg := testConfig()
	cfg.VimKeys = true
	cfg.Commands = []command{{ID: "custom", Title: "自定义", Command: "echo test", Keys: []string{"j"}}}
	for _, overrides := range []string{"", `{"cursor.down":["j"]}`} {
		km, _, warns := buildKeymap(cfg, "")
		if overrides != "" {
			a, _, _, _ := setupWith(t, cfg, overrides)
			km, warns = a.km, a.warns
		}
		want := "cmd.custom"
		if overrides != "" {
			want = "cursor.down"
		}
		if id, _ := km.Lookup(keymap.MustParse("j")); id != want || len(warns) == 0 {
			t.Fatalf("j=%q, want %q with conflict warning: %v", id, want, warns)
		}
	}
}

func TestVimFilteredMovement(t *testing.T) {
	a, tt, _, _ := setup(t)
	a.setVimKeys(true)
	a.setFilter(a.tab(), "report")
	tt.Frame()
	tt.Key(0, ui.KeyJ)
	tt.Key(0, ui.KeyJ) // last row stays put
	if cursorName(a) != "report-2.txt" || a.tab().sel.Len() != 1 {
		t.Fatalf("filtered j: %q (%d selected)", cursorName(a), a.tab().sel.Len())
	}
	tt.Key(0, ui.KeyK)
	if cursorName(a) != "report-1.txt" {
		t.Fatalf("filtered k: %q", cursorName(a))
	}
}

func TestVimMovementMatchesArrows(t *testing.T) {
	for _, row := range []int{-1, 0, 2, 4} {
		for _, down := range []bool{false, true} {
			t.Run(fmt.Sprintf("row%d/down%v", row, down), func(t *testing.T) {
				var cursor int
				var chosen []string
				for _, vim := range []bool{false, true} {
					a, tt, _, _ := setup(t)
					a.setVimKeys(vim)
					a.tab().cursor = row
					for _, e := range a.tab().t.Entries() {
						a.tab().sel.Add(e.Path)
					}
					tt.Frame()
					key := ui.KeyUp
					if down {
						key = ui.KeyDown
					}
					if vim {
						key = ui.KeyK
						if down {
							key = ui.KeyJ
						}
					}
					tt.Key(0, key)
					tt.Frame()
					var names []string
					for _, e := range a.tab().chosen() {
						names = append(names, e.Name)
					}
					if !vim {
						cursor, chosen = a.tab().cursor, names
					} else if cursor != a.tab().cursor || !slices.Equal(chosen, names) {
						t.Fatalf("arrows (%d,%v), Vim (%d,%v)", cursor, chosen, a.tab().cursor, names)
					}
				}
			})
		}
	}
}

func TestVimNavigation(t *testing.T) {
	a, tt, fs, root := setup(t)
	a.setVimKeys(true)
	tt.Frame()
	tt.Key(0, ui.KeyL)
	if a.tab().t.Dir != filepath.Join(root, "alpha") {
		t.Fatal("l did not enter directory")
	}
	tt.Key(0, ui.KeyH)
	if a.tab().t.Dir != root {
		t.Fatal("h did not return to parent")
	}
	tt.Key(0, ui.KeyJ)
	if a.tab().cursor != 1 {
		t.Fatalf("j did not move exactly once immediately: %d", a.tab().cursor)
	}
	tt.KeyRepeat(0, ui.KeyJ)
	tt.Frame()
	if cursorName(a) != "notes.md" || a.pane().pv.path != filepath.Join(root, "notes.md") {
		t.Fatalf("repeat/preview: cursor=%q preview=%q", cursorName(a), a.pane().pv.path)
	}
	tt.Key(0, ui.KeyL)
	if len(fs.opened) != 1 || fs.opened[0] != filepath.Join(root, "notes.md") {
		t.Fatalf("l opened %v", fs.opened)
	}
	tt.Key(0, ui.KeyDown)
	if a.tab().cursor != 3 {
		t.Fatal("native down stopped working")
	}
	a.setKeys("cursor.down", nil, false)
	tt.Frame()
	tt.Key(0, ui.KeyJ)
	if a.tab().cursor != 3 || !tt.HasText("↑↓") {
		t.Fatal("cleared j still moved or native movement hint missing")
	}
	tt.Key(0, ui.KeyDown)
	if a.tab().cursor != 4 {
		t.Fatal("native down was cleared with j")
	}
	a.navigate(a.tab(), filepath.Join(root, "beta"))
	tt.Frame()
	tt.Key(0, ui.KeyK)
	if a.tab().cursor != -1 {
		t.Fatal("empty directory gained a cursor")
	}
}

func TestVimFocusAndTyping(t *testing.T) {
	for _, target := range []string{"address", "filter", "rename", "settings", "help", "sidebar", "parent", "menu"} {
		t.Run(target, func(t *testing.T) {
			a, tt, _, root := setup(t)
			a.setVimKeys(true)
			tt.Frame()
			switch target {
			case "address":
				tt.Key(ui.Cmd, ui.KeyL)
			case "filter":
				tt.TypeKey(0, ui.KeyF, "f")
			case "rename":
				a.run("rename")
			case "settings":
				a.run("settings")
				a.setPage, a.focusKeyQ = 1, true
			case "help":
				a.run("help")
			case "sidebar":
				a.wantSidebar = true
			case "parent":
				tt = ui.NewTester(func(c *ui.Context) {
					a.view(c)
					a.panes[0].parent.list.Focus(c)
				}, 1100, 700)
			case "menu":
				if err := tt.RightClick("alpha"); err != nil {
					t.Fatal(err)
				}
			}
			tt.Frame()
			tt.Frame()
			cursor := a.tab().cursor
			for _, ch := range []struct {
				key  ui.Key
				text string
			}{{ui.KeyH, "h"}, {ui.KeyJ, "j"}, {ui.KeyK, "k"}, {ui.KeyL, "l"}} {
				tt.TypeKey(0, ch.key, ch.text)
			}
			tt.Frame()
			if a.tab().t.Dir != root || target != "filter" && a.tab().cursor != cursor {
				t.Fatalf("navigation escaped %s: dir=%q cursor=%d (was %d)", target, a.tab().t.Dir, a.tab().cursor, cursor)
			}
			if target == "filter" && a.filterText != "hjkl" || target == "settings" && a.helpQuery != "hjkl" {
				t.Fatalf("text lost: filter=%q search=%q", a.filterText, a.helpQuery)
			}
			if target == "address" && !strings.Contains(a.pane().addrText, "hjkl") ||
				target == "rename" && !strings.Contains(a.dlgText, "hjkl") ||
				target == "help" && a.helpQuery != "hjkl" {
				t.Fatalf("text lost: address=%q rename=%q help=%q", a.pane().addrText, a.dlgText, a.helpQuery)
			}
		})
	}
}

func TestVimDualAndRows(t *testing.T) {
	for _, layout := range []int{layoutDual, layoutRows} {
		a, tt, _, _ := setup(t)
		a.setVimKeys(true)
		a.setLayout(layout)
		a.active, a.wantFocus = 1, true
		tt.Frame()
		tt.Frame()
		left, right := a.panes[0].ts.cursor, a.panes[1].ts.cursor
		tt.Key(0, ui.KeyJ)
		if a.panes[0].ts.cursor != left || a.panes[1].ts.cursor != right+1 {
			t.Fatalf("layout %d: moved wrong pane", layout)
		}
	}
}

func TestVimScrollAndShiftAnchor(t *testing.T) {
	a, tt, _, root := setup(t)
	for i := range 80 {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("z%03d.txt", i)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a.load(a.tab())
	a.setVimKeys(true)
	tt.Frame()
	for range 60 {
		tt.KeyRepeat(0, ui.KeyJ)
	}
	tt.Frame()
	first, last := a.tab().list.Visible()
	if a.tab().cursor != 60 || first == 0 || a.tab().cursor < first || a.tab().cursor > last {
		t.Fatalf("cursor %d outside visible %d..%d", a.tab().cursor, first, last)
	}
	tt.Key(0, ui.KeyHome) // reset native pivot to first row
	tt.Key(0, ui.KeyJ)
	tt.Key(0, ui.KeyJ)
	tt.Key(ui.Shift, ui.KeyDown)
	// MyGo v0.8.3 has no public setter for pivot: the approved fallback
	// extends from the last native navigation, not the last j/k row.
	if a.tab().cursor != 3 || a.tab().sel.Len() != 4 {
		t.Fatalf("native Shift anchor changed: cursor=%d selected=%d", a.tab().cursor, a.tab().sel.Len())
	}
}

func TestVimShots(t *testing.T) {
	dir := os.Getenv("MYGO_SHOTS")
	if dir == "" {
		t.Skip("MYGO_SHOTS not set")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"light", "dark"} {
		a, tt, _, _ := setup(t)
		a.cfg.Appearance = mode
		for _, vim := range []bool{false, true} {
			a.setVimKeys(vim)
			tt.Frame()
			prefix := fmt.Sprintf("%s-vim-%v", mode, vim)
			writePNG(t, filepath.Join(dir, prefix+"-main.png"), tt.Image())
			a.run("settings")
			a.setPage = 1
			tt.Frame()
			writePNG(t, filepath.Join(dir, prefix+"-settings.png"), tt.Image())
			a.dlgOpen, a.wantFocus = false, true
			tt.Frame()
		}
	}
}
