package keymap

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

var testActions = []Action{
	{ID: "tab.new", Title: "New Tab", Group: "Tabs", Hint: true},
	{ID: "go.home", Title: "Home", Group: "Go"},
	{ID: "trash", Title: "Move to Trash", Group: "File", Hint: true},
	{ID: "help", Title: "Shortcuts", Group: "Help", Hint: true},
}

func newTest(t *testing.T) *Keymap {
	t.Helper()
	km, err := New(testActions, map[string][]string{
		"tab.new": {"Cmd+T"},
		"go.home": {"Cmd+Shift+H"},
		"trash":   {"Cmd+Backspace"},
		"help":    {"Shift+/", "F1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return km
}

func TestParse(t *testing.T) {
	cases := map[string]Chord{
		"Cmd+Shift+H":  {Mods: Cmd | Shift, Key: "H"},
		"cmdorctrl+t":  {Mods: Cmd, Key: "T"},
		"F2":           {Mods: 0, Key: "F2"},
		"Option+Cmd+1": {Mods: Alt | Cmd, Key: "1"},
		"Cmd+Delete":   {Mods: Cmd, Key: "Backspace"},
		"Cmd+Shift+.":  {Mods: Cmd | Shift, Key: "."},
		"Return":       {Mods: 0, Key: "Enter"},
		"Cmd+[":        {Mods: Cmd, Key: "["},
	}
	for in, want := range cases {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "Hyper+A", "Cmd+Nope"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) succeeded", bad)
		}
	}
}

func TestString(t *testing.T) {
	if s := MustParse("Cmd+Shift+H").String(); s != "⇧⌘H" {
		t.Errorf("got %q", s)
	}
	if s := MustParse("Cmd+Backspace").String(); s != "⌘⌫" {
		t.Errorf("got %q", s)
	}
}

func TestLookupAndConflict(t *testing.T) {
	km := newTest(t)
	if id, ok := km.Lookup(MustParse("Cmd+T")); !ok || id != "tab.new" {
		t.Fatalf("lookup = %q %v", id, ok)
	}
	if _, ok := km.Lookup(MustParse("Cmd+Y")); ok {
		t.Fatal("unbound chord found")
	}
	_, err := New(testActions, map[string][]string{"tab.new": {"Cmd+T"}, "go.home": {"Cmd+T"}})
	if err == nil {
		t.Fatal("conflict not detected")
	}
	if _, err := New(testActions, map[string][]string{"nope": {"Cmd+T"}}); err == nil {
		t.Fatal("unknown action accepted")
	}
}

func TestOverride(t *testing.T) {
	km := newTest(t)
	if w := km.Override(map[string][]string{"tab.new": {"Cmd+Alt+N"}}); len(w) != 0 {
		t.Fatal(w)
	}
	if _, ok := km.Lookup(MustParse("Cmd+T")); ok {
		t.Error("old binding survived")
	}
	if id, _ := km.Lookup(MustParse("Cmd+Alt+N")); id != "tab.new" {
		t.Error("new binding missing")
	}
	if id, _ := km.Lookup(MustParse("Cmd+Shift+H")); id != "go.home" {
		t.Error("untouched binding lost")
	}
}

// Conflicts are applied as far as possible and each is explained.
func TestOverrideConflicts(t *testing.T) {
	km := newTest(t)
	w := km.Override(map[string][]string{
		"go.home": {"Cmd+Backspace", "Hyper+X"}, // takes trash's key; a bad key
		"nope":    {"Cmd+K"},
	})
	if id, _ := km.Lookup(MustParse("Cmd+Backspace")); id != "go.home" {
		t.Fatalf("override did not take the key: %q", id)
	}
	if len(km.KeysFor("trash")) != 0 {
		t.Fatal("trash kept the key it lost")
	}
	want := []string{"「Move to Trash」", "未知动作 \"nope\"", "Hyper"}
	for _, s := range want {
		found := false
		for _, x := range w {
			found = found || strings.Contains(x, s)
		}
		if !found {
			t.Errorf("no warning mentions %q: %q", s, w)
		}
	}
	// Two overrides asking for one key: the first in action order wins.
	w = km.Override(map[string][]string{"tab.new": {"F5"}, "help": {"F5"}})
	if id, _ := km.Lookup(MustParse("F5")); id != "tab.new" || len(w) != 1 || !strings.Contains(w[0], "「Shortcuts」") {
		t.Fatalf("F5 → %q, warnings %q", id, w)
	}
}

func TestOverridesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "keymap.json")
	if m, err := ReadOverrides(p); err != nil || len(m) != 0 {
		t.Fatal(m, err)
	}
	if err := WriteOverrides(p, map[string][]string{"trash": {MustParse("Cmd+Alt+Backspace").Spec()}}); err != nil {
		t.Fatal(err)
	}
	m, err := ReadOverrides(p)
	if err != nil || len(m["trash"]) != 1 || m["trash"][0] != "Cmd+Alt+Backspace" {
		t.Fatal(m, err)
	}
	os.WriteFile(p, []byte("{"), 0o644)
	if _, err := ReadOverrides(p); err == nil {
		t.Fatal("bad JSON accepted")
	}
}

func TestHints(t *testing.T) {
	km := newTest(t)
	h := km.Hints(0)
	if len(h) != 3 { // tab.new, trash, help (once, though bound twice)
		t.Fatalf("hints(0) = %+v", h)
	}
	cmd := km.Hints(Cmd)
	if len(cmd) != 2 || cmd[0].ID != "tab.new" || cmd[1].ID != "trash" {
		t.Fatalf("hints(Cmd) = %+v", cmd)
	}
	if got := km.Hints(Cmd | Shift); len(got) != 1 || got[0].ID != "go.home" {
		t.Fatalf("hints(Cmd|Shift) = %+v", got)
	}
}

func TestSheet(t *testing.T) {
	km := newTest(t)
	if n := len(km.Sheet("")); n != 4 {
		t.Fatalf("sheet has %d rows", n)
	}
	got := km.Sheet("trash")
	if len(got) != 1 || got[0].Keys != "⌘⌫" {
		t.Fatalf("sheet(trash) = %+v", got)
	}
	if got := km.Sheet("help"); len(got) != 1 || got[0].Keys != "?  F1" {
		t.Fatalf("sheet(help) = %+v", got)
	}
}

func TestHintsGrouped(t *testing.T) {
	acts := []Action{
		{ID: "a1", Title: "A1", Group: "甲"},
		{ID: "b1", Title: "B1", Group: "乙"},
		{ID: "a2", Title: "A2", Group: "甲"},
	}
	// Bound in an order that mixes the groups.
	km, err := New(acts, map[string][]string{"b1": {"Cmd+B"}, "a2": {"Cmd+C"}, "a1": {"Cmd+A"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, h := range km.Hints(Cmd) {
		got = append(got, h.Group+h.ID)
	}
	var sheet []string
	for _, h := range km.Sheet("") {
		sheet = append(sheet, h.Group+h.ID)
	}
	if len(got) != 3 || got[0][:3] != "甲" || got[1][:3] != "甲" || got[2] != "乙b1" {
		t.Fatalf("hints not grouped: %v", got)
	}
	if sheet[0][:3] != "甲" || sheet[2] != "乙b1" {
		t.Fatalf("sheet order %v", sheet)
	}
}

func TestDoubleTapParse(t *testing.T) {
	ch, err := Parse("c c")
	if err != nil || ch != (Chord{Key: "C", Double: true}) {
		t.Fatalf("Parse(c c) = %v, %v", ch, err)
	}
	if ch.String() != "c c" || ch.Spec() != "c c" {
		t.Fatalf("String %q Spec %q", ch.String(), ch.Spec())
	}
	if s := MustParse("N").String(); s != "n" {
		t.Fatalf("plain letter shows %q", s)
	}
	if s := MustParse("Cmd+N").Spec(); s != "Cmd+N" {
		t.Fatalf("Cmd+N spec %q", s)
	}
	for _, bad := range []string{"c n", "c c c", "g g g"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) succeeded", bad)
		}
	}
}

func TestDoubleTap(t *testing.T) {
	acts := []Action{{ID: "newfolder"}, {ID: "newfile"}, {ID: "copy"}, {ID: "rename"}}
	km, err := New(acts, map[string][]string{"newfolder": {"n"}, "newfile": {"n n"}, "copy": {"c"}, "rename": {"r"}})
	if err != nil {
		t.Fatal(err)
	}
	n, r := MustParse("n"), MustParse("r")
	t0 := time.Unix(0, 0)
	var tp Taps

	// A key with no double tap runs at once.
	if got := km.Press(&tp, r, t0); !slices.Equal(got, []string{"rename"}) {
		t.Fatalf("r: %v", got)
	}
	// Twice in time: the double tap.
	if got := km.Press(&tp, n, t0); len(got) != 0 {
		t.Fatalf("first n ran %v", got)
	}
	if d, ok := tp.Waiting(t0.Add(100 * time.Millisecond)); !ok || d != 300*time.Millisecond {
		t.Fatalf("waiting %v %v", d, ok)
	}
	if got := km.Press(&tp, n, t0.Add(300*time.Millisecond)); !slices.Equal(got, []string{"newfile"}) {
		t.Fatalf("n n: %v", got)
	}
	// Once, then the time runs out: the single action.
	km.Press(&tp, n, t0)
	if _, due := km.Due(&tp, t0.Add(399*time.Millisecond)); due {
		t.Fatal("due too soon")
	}
	if id, due := km.Due(&tp, t0.Add(DoubleTap)); !due || id != "newfolder" {
		t.Fatalf("timeout: %q %v", id, due)
	}
	// Once, then another key: the single action first, then that key's.
	km.Press(&tp, n, t0)
	if got := km.Press(&tp, r, t0.Add(50*time.Millisecond)); !slices.Equal(got, []string{"newfolder", "rename"}) {
		t.Fatalf("n r: %v", got)
	}
	// A second press too late is a new first press.
	km.Press(&tp, n, t0)
	if got := km.Press(&tp, n, t0.Add(time.Second)); !slices.Equal(got, []string{"newfolder"}) {
		t.Fatalf("late n: %v", got)
	}
	if id := km.Flush(&tp); id != "newfolder" {
		t.Fatalf("flush %q", id)
	}
}
