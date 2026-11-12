package gui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gofinder/internal/keymap"
)

// walk visits every item of a menu tree.
func walk(items []*mygo.MenuItem, fn func(*mygo.MenuItem)) {
	for _, it := range items {
		fn(it)
		walk(it.Submenu, fn)
	}
}

func TestMenuCoversActions(t *testing.T) {
	a, _, _, _ := setup(t)
	shown := map[string]bool{}
	walk(a.buildMenu(), func(it *mygo.MenuItem) {
		if it.ID != "" {
			shown[it.ID] = true
		}
	})
	for _, r := range roles {
		if r.covers != "" {
			shown[r.covers] = true
		}
	}
	for _, act := range a.km.Actions() {
		if !shown[act.ID] {
			t.Errorf("action %q (%s) is in no menu", act.ID, act.Title)
		}
	}
}

func TestMenuAccelerators(t *testing.T) {
	a, _, _, _ := setup(t)
	byID := map[string]*mygo.MenuItem{}
	walk(a.buildMenu(), func(it *mygo.MenuItem) {
		if it.ID != "" {
			byID[it.ID] = it
		}
	})
	cases := map[string]string{
		"workspace": "", // w alone carries no Cmd
		"go.home":   "Cmd+Shift+H",
		"settings":  "Cmd+,",
		"hidden":    "", // first key ., its ⇧⌘. is the window's alone
		"ws.next":   "", // first key ]
		"open":      "", // first key →, which the list needs
		"trash":     "", // Cmd+Backspace edits text in fields
		"copy":      "", // the Edit menu's role holds Cmd+C
		"help":      "", // ? and F1 carry no Cmd
	}
	for id, want := range cases {
		it := byID[id]
		if id == "copy" {
			if it != nil {
				t.Errorf("copy has its own item besides the Edit role")
			}
			continue
		}
		if it == nil || it.Accelerator != want {
			t.Errorf("%s: accelerator %q, want %q", id, it.Accelerator, want)
		}
	}
	if !strings.HasSuffix(byID["hidden"].Label, "　.") || !strings.HasSuffix(byID["rename"].Label, "　↩") ||
		!strings.HasSuffix(byID["open"].Label, "　→") {
		t.Errorf("first keys missing from titles: %q, %q, %q", byID["hidden"].Label, byID["rename"].Label, byID["open"].Label)
	}
	if !strings.HasSuffix(byID["trash"].Label, "　d") || !strings.HasSuffix(byID["help"].Label, "?") || !strings.HasSuffix(byID["newfile"].Label, "n n") {
		t.Errorf("keys missing from titles: %q, %q", byID["trash"].Label, byID["help"].Label)
	}
	if menuAccel([]keymap.Chord{keymap.MustParse("Ctrl+Tab"), keymap.MustParse("Cmd+Shift+]")}) != "Cmd+Shift+]" {
		t.Error("menuAccel did not skip Ctrl+Tab")
	}
}

func TestMenuRunsActionsAndFollowsState(t *testing.T) {
	a, tt, _, _ := setup(t)
	a.setMenu = func([]*mygo.MenuItem) {}
	a.applyMenu()
	tt.Frame()
	root := a.tab().t.Dir
	up := a.menu.items["up"]
	a.navigate(a.tab(), filepath.Join(root, "alpha"))
	tt.Frame()
	up.Click(up, nil)
	tt.Frame()
	if a.tab().t.Dir != root {
		t.Fatalf("menu Up: %s", a.tab().t.Dir)
	}
	a.menu.items["layout.rows"].Click(nil, nil)
	tt.Frame()
	if a.layout != layoutRows || !a.menu.items["layout.rows"].IsChecked() || a.menu.items["layout.columns"].IsChecked() {
		t.Fatalf("layout %d, checks rows %v columns %v", a.layout,
			a.menu.items["layout.rows"].IsChecked(), a.menu.items["layout.columns"].IsChecked())
	}
	// While the cheat sheet is open the items are off, and a click does nothing.
	tt.Key(ui.Shift, ui.KeySlash)
	tt.Frame()
	if a.menu.items["up"].IsEnabled() {
		t.Fatal("items enabled under the cheat sheet")
	}
	a.navigate(a.tab(), filepath.Join(root, "alpha"))
	up.Click(up, nil)
	tt.Frame()
	if a.tab().t.Dir != filepath.Join(root, "alpha") {
		t.Fatal("menu ran an action behind the cheat sheet")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if !a.menu.items["up"].IsEnabled() {
		t.Fatal("items stayed disabled after the sheet closed")
	}
}

// TestMenuOneKeyPerItem: each item shows one key at most, its action's
// first key, either as the accelerator or after the title, never both.
func TestMenuOneKeyPerItem(t *testing.T) {
	a, _, _, _ := setup(t)
	walk(a.buildMenu(), func(it *mygo.MenuItem) {
		if it.ID == "" {
			return
		}
		keys := a.km.KeysFor(it.ID)
		suffix := strings.Contains(it.Label, "　")
		if suffix && it.Accelerator != "" {
			t.Errorf("%s: two keys, %q and %q", it.ID, it.Label, it.Accelerator)
		}
		if strings.Count(it.Label, "　") > 1 {
			t.Errorf("%s: title %q", it.ID, it.Label)
		}
		if len(keys) == 0 {
			if suffix || it.Accelerator != "" {
				t.Errorf("%s has no key but shows one: %q %q", it.ID, it.Label, it.Accelerator)
			}
			return
		}
		switch {
		case it.Accelerator != "":
			if it.Accelerator != keys[0].Spec() {
				t.Errorf("%s: accelerator %q is not the first key %q", it.ID, it.Accelerator, keys[0].Spec())
			}
		case !strings.HasSuffix(it.Label, "　"+keys[0].String()):
			t.Errorf("%s: title %q lacks the first key %q", it.ID, it.Label, keys[0].String())
		}
	})
}
