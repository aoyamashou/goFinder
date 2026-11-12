// Package keymap is the single source of truth for goFinder's keyboard
// shortcuts: dispatch, the status-bar hints and the "?" cheat sheet all read
// from it, so what the hints show is always what the keys do.
//
// Bindings are a single key or modifiers plus one key ("Cmd+Shift+H"),
// or the same key pressed twice within DoubleTap ("c c"); there are no
// other multi-key sequences.
package keymap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Mods is a set of modifier keys.
type Mods uint8

const (
	Shift Mods = 1 << iota
	Ctrl
	Alt
	Cmd
)

// Chord is one key with the modifiers held, or, with Double, that key
// pressed twice in a row ("c c").
type Chord struct {
	Mods   Mods
	Key    string // canonical key name, see keyNames
	Double bool
}

// DoubleTap is how soon the second press of a double tap must come.
const DoubleTap = 400 * time.Millisecond

// keyNames maps accepted spellings (lowercase) to canonical names.
var keyNames = map[string]string{
	"up": "Up", "down": "Down", "left": "Left", "right": "Right",
	"home": "Home", "end": "End", "pageup": "PageUp", "pagedown": "PageDown",
	"enter": "Enter", "return": "Enter", "esc": "Escape", "escape": "Escape",
	"tab": "Tab", "space": "Space", "backspace": "Backspace", "delete": "Backspace",
	"forwarddelete": "ForwardDelete",
	"back":          "Back", "forward": "Forward", // a mouse's side buttons, a trackpad's swipes
	".": ".", ",": ",", "/": "/", "[": "[", "]": "]", "-": "-", "=": "=", ";": ";", "'": "'", "`": "`", "\\": "\\",
	"period": ".", "comma": ",", "slash": "/",
}

func init() {
	for c := 'a'; c <= 'z'; c++ {
		keyNames[string(c)] = strings.ToUpper(string(c))
	}
	for c := '0'; c <= '9'; c++ {
		keyNames[string(c)] = string(c)
	}
	for i := 1; i <= 12; i++ {
		keyNames[fmt.Sprintf("f%d", i)] = fmt.Sprintf("F%d", i)
	}
}

// Parse reads "Cmd+Shift+H", "F2", "Cmd+Backspace", "Shift+/", "c", and
// the double tap "c c" (the same key twice; no other sequence).
func Parse(s string) (Chord, error) {
	f := strings.Fields(s)
	switch len(f) {
	case 0:
		return Chord{}, fmt.Errorf("empty key")
	case 1:
		return parseOne(f[0])
	case 2:
		a, err := parseOne(f[0])
		if err != nil {
			return a, err
		}
		b, err := parseOne(f[1])
		if err != nil {
			return a, err
		}
		if a != b {
			return Chord{}, fmt.Errorf("only the same key twice is a sequence, not %q", s)
		}
		a.Double = true
		return a, nil
	}
	return Chord{}, fmt.Errorf("multi-key sequences are not supported: %q", s)
}

func parseOne(s string) (Chord, error) {
	var ch Chord
	// Split on "+" but keep a trailing "+" key out of scope (no "+" key binding).
	parts := strings.Split(s, "+")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		lp := strings.ToLower(p)
		if i < len(parts)-1 {
			switch lp {
			case "cmd", "command", "cmdorctrl", "meta", "super":
				ch.Mods |= Cmd
			case "ctrl", "control":
				ch.Mods |= Ctrl
			case "alt", "option", "opt":
				ch.Mods |= Alt
			case "shift":
				ch.Mods |= Shift
			default:
				return ch, fmt.Errorf("unknown modifier %q in %q", p, s)
			}
			continue
		}
		k, ok := keyNames[lp]
		if !ok {
			return ch, fmt.Errorf("unknown key %q in %q", p, s)
		}
		ch.Key = k
	}
	return ch, nil
}

// MustParse is Parse for built-in tables.
func MustParse(s string) Chord {
	ch, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return ch
}

var keyGlyphs = map[string]string{
	"Up": "↑", "Down": "↓", "Left": "←", "Right": "→", "Enter": "↩", "Escape": "Esc",
	"Tab": "⇥", "Space": "Space", "Backspace": "⌫", "ForwardDelete": "⌦",
	"PageUp": "PgUp", "PageDown": "PgDn", "Back": "鼠标后退", "Forward": "鼠标前进",
}

// String renders the chord the macOS way: "⇧⌘H"; a plain letter is
// lowercase, as typed ("c"), and a double tap is the key twice ("c c").
func (ch Chord) String() string {
	if ch.Double {
		one := ch
		one.Double = false
		return one.String() + " " + one.String()
	}
	if ch == (Chord{Mods: Shift, Key: "/"}) {
		return "?"
	}
	if ch.plainLetter() {
		return strings.ToLower(ch.Key)
	}
	var b strings.Builder
	if ch.Mods&Ctrl != 0 {
		b.WriteString("⌃")
	}
	if ch.Mods&Alt != 0 {
		b.WriteString("⌥")
	}
	if ch.Mods&Shift != 0 {
		b.WriteString("⇧")
	}
	if ch.Mods&Cmd != 0 {
		b.WriteString("⌘")
	}
	if g, ok := keyGlyphs[ch.Key]; ok {
		b.WriteString(g)
	} else {
		b.WriteString(ch.Key)
	}
	return b.String()
}

// Action is a command the user can run.
type Action struct {
	ID    string
	Title string
	Group string
	// Hint puts the action in the status bar's always-on hints.
	Hint bool
}

// Binding ties a chord to an action.
type Binding struct {
	Chord  Chord
	Action string
}

// Keymap holds the actions and their bindings.
type Keymap struct {
	actions []Action
	byID    map[string]int
	binds   []Binding
	lookup  map[Chord]string

	defaults map[string][]string
}

// New builds a keymap from actions and default bindings ("Cmd+T" -> id).
func New(actions []Action, defaults map[string][]string) (*Keymap, error) {
	km := &Keymap{actions: actions, byID: map[string]int{}, defaults: defaults}
	for i, a := range actions {
		if _, dup := km.byID[a.ID]; dup {
			return nil, fmt.Errorf("duplicate action %q", a.ID)
		}
		km.byID[a.ID] = i
	}
	if err := km.setBindings(defaults); err != nil {
		return nil, err
	}
	return km, nil
}

func (km *Keymap) setBindings(m map[string][]string) error {
	for id := range m {
		if _, ok := km.byID[id]; !ok {
			return fmt.Errorf("binding for unknown action %q", id)
		}
	}
	// Action order, then keys as listed: the first key is the one hints show.
	var binds []Binding
	for _, a := range km.actions {
		for _, k := range m[a.ID] {
			ch, err := Parse(k)
			if err != nil {
				return err
			}
			binds = append(binds, Binding{ch, a.ID})
		}
	}
	lookup := map[Chord]string{}
	for _, b := range binds {
		if prev, ok := lookup[b.Chord]; ok && prev != b.Action {
			return fmt.Errorf("%s is bound to both %q and %q", b.Chord, prev, b.Action)
		}
		lookup[b.Chord] = b.Action
	}
	km.binds, km.lookup = binds, lookup
	return nil
}

// Override replaces the bindings of the actions named in m (an empty list
// unbinds); other actions keep theirs. It applies what it can and returns
// what it could not, in words for the user:
// unknown actions and keys are ignored; a key another action had moves to
// the overriding action (the other loses it); a key two overriding actions
// ask for goes to the first in action order.
func (km *Keymap) Override(m map[string][]string) []string {
	var warns []string
	title := func(id string) string { return "「" + km.actions[km.byID[id]].Title + "」" }
	want := map[string][]Chord{}
	for id, keys := range m {
		if _, ok := km.byID[id]; !ok {
			warns = append(warns, fmt.Sprintf("未知动作 %q，已忽略", id))
			continue
		}
		want[id] = []Chord{}
		for _, k := range keys {
			ch, err := Parse(k)
			if err != nil {
				warns = append(warns, fmt.Sprintf("%s：%v，已忽略", title(id), err))
				continue
			}
			want[id] = append(want[id], ch)
		}
	}
	// Overriding actions take their keys first, in action order.
	owner := map[Chord]string{}
	for _, a := range km.actions {
		chs, ok := want[a.ID]
		if !ok {
			continue
		}
		var kept []Chord
		for _, ch := range chs {
			if prev, taken := owner[ch]; taken {
				if prev != a.ID {
					warns = append(warns, fmt.Sprintf("%s 同时设给了%s和%s，后者已忽略", ch, title(prev), title(a.ID)))
				}
				continue
			}
			owner[ch] = a.ID
			kept = append(kept, ch)
		}
		want[a.ID] = kept
	}
	var binds []Binding
	for _, a := range km.actions {
		if chs, ok := want[a.ID]; ok {
			for _, ch := range chs {
				binds = append(binds, Binding{ch, a.ID})
			}
			continue
		}
		for _, b := range km.binds {
			if b.Action != a.ID {
				continue
			}
			if o, taken := owner[b.Chord]; taken {
				warns = append(warns, fmt.Sprintf("%s 原属于%s，已改给%s", b.Chord, title(a.ID), title(o)))
				continue
			}
			binds = append(binds, b)
		}
	}
	lookup := map[Chord]string{}
	for _, b := range binds {
		lookup[b.Chord] = b.Action
	}
	km.binds, km.lookup = binds, lookup
	return warns
}

// Defaults returns the built-in keys of an action.
func (km *Keymap) Defaults(id string) []string { return km.defaults[id] }

// ReadOverrides reads a JSON object {"action.id": ["Cmd+K"]} from path.
// A missing file is an empty map.
func ReadOverrides(path string) (map[string][]string, error) {
	m := map[string][]string{}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string][]string{}, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return m, nil
}

// WriteOverrides saves m to path through a temporary file.
func WriteOverrides(path string, m map[string][]string) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path+".tmp", b, 0o644); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

// plainLetter is a letter with no modifier, shown lowercase.
func (ch Chord) plainLetter() bool {
	return ch.Mods == 0 && len(ch.Key) == 1 && ch.Key[0] >= 'A' && ch.Key[0] <= 'Z'
}

// Spec is the parseable form of a chord, as keymap.json takes it.
func (ch Chord) Spec() string {
	if ch.Double {
		one := ch
		one.Double = false
		return one.Spec() + " " + one.Spec()
	}
	if ch.plainLetter() {
		return strings.ToLower(ch.Key)
	}
	var parts []string
	if ch.Mods&Cmd != 0 {
		parts = append(parts, "Cmd")
	}
	if ch.Mods&Ctrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if ch.Mods&Alt != 0 {
		parts = append(parts, "Alt")
	}
	if ch.Mods&Shift != 0 {
		parts = append(parts, "Shift")
	}
	return strings.Join(append(parts, ch.Key), "+")
}

// Lookup returns the action bound to ch.
func (km *Keymap) Lookup(ch Chord) (string, bool) {
	id, ok := km.lookup[ch]
	return id, ok
}

// Bindings returns every binding, in action order.
func (km *Keymap) Bindings() []Binding { return km.binds }

// Action returns the action with id.
func (km *Keymap) Action(id string) (Action, bool) {
	i, ok := km.byID[id]
	if !ok {
		return Action{}, false
	}
	return km.actions[i], true
}

// Actions returns all actions in declaration order.
func (km *Keymap) Actions() []Action { return km.actions }

// KeysFor returns the chords bound to an action.
func (km *Keymap) KeysFor(id string) []Chord {
	var out []Chord
	for _, b := range km.binds {
		if b.Action == id {
			out = append(out, b.Chord)
		}
	}
	return out
}

// Hint is one entry of the status bar or cheat sheet.
type Hint struct {
	Keys  string // "⌘T"
	Title string
	Group string
	ID    string
}

// Hints returns what the status bar shows: with no modifier held, the
// actions marked Hint; with modifiers held, every binding using exactly
// those modifiers, so holding ⌘ lists all ⌘ shortcuts.
func (km *Keymap) Hints(held Mods) []Hint {
	var out []Hint
	seen := map[string]bool{}
	for _, b := range km.binds {
		a := km.actions[km.byID[b.Action]]
		if held == 0 {
			if !a.Hint || seen[a.ID] {
				continue
			}
		} else if b.Chord.Mods != held {
			continue
		}
		seen[a.ID] = true
		out = append(out, Hint{Keys: b.Chord.String(), Title: a.Title, Group: a.Group, ID: a.ID})
	}
	if held != 0 {
		// Grouped as the cheat sheet is, its groups in the same order.
		rank := km.groupRank()
		slices.SortStableFunc(out, func(x, y Hint) int { return rank[x.Group] - rank[y.Group] })
	}
	return out
}

// groupRank orders the groups as the actions first name them, the order
// both the status bar and the cheat sheet show them in.
func (km *Keymap) groupRank() map[string]int {
	rank := map[string]int{}
	for _, a := range km.actions {
		if _, ok := rank[a.Group]; !ok {
			rank[a.Group] = len(rank)
		}
	}
	return rank
}

// Sheet returns every action with its keys, grouped in declaration order,
// keeping those whose title, group or keys contain query (case-insensitive).
// Unbound actions are listed too, with empty Keys.
func (km *Keymap) Sheet(query string) []Hint {
	q := strings.ToLower(strings.TrimSpace(query))
	var groups []string
	byGroup := map[string][]Hint{}
	for _, a := range km.actions {
		var keys []string
		for _, ch := range km.KeysFor(a.ID) {
			keys = append(keys, ch.String())
		}
		h := Hint{Keys: strings.Join(keys, "  "), Title: a.Title, Group: a.Group, ID: a.ID}
		if q != "" && !strings.Contains(strings.ToLower(h.Title+" "+h.Group+" "+h.Keys+" "+a.ID), q) {
			continue
		}
		if !slices.Contains(groups, a.Group) {
			groups = append(groups, a.Group)
		}
		byGroup[a.Group] = append(byGroup[a.Group], h)
	}
	var out []Hint
	for _, g := range groups {
		out = append(out, byGroup[g]...)
	}
	return out
}

// Taps holds a key pressed once while a double tap of it may follow.
type Taps struct {
	ch Chord // the key, Double false
	at time.Time
	on bool
}

// Waiting reports how long until the key pressed once times out.
func (t *Taps) Waiting(now time.Time) (time.Duration, bool) {
	if !t.on {
		return 0, false
	}
	return max(t.at.Add(DoubleTap).Sub(now), 0), true
}

// Pending is the key waiting for its second press.
func (t *Taps) Pending() (Chord, bool) { return t.ch, t.on }

// Press resolves a key pressed (ch with Double false) at now into the
// actions to run, in order. A key bound as a double tap waits for its
// second press instead of running its single action: the second press
// within DoubleTap runs the double tap's action; another key runs the
// waiting key's single action first, then its own.
func (km *Keymap) Press(t *Taps, ch Chord, now time.Time) []string {
	ch.Double = false
	var out []string
	if t.on {
		t.on = false
		if t.ch == ch && now.Sub(t.at) < DoubleTap {
			dbl := ch
			dbl.Double = true
			if id, ok := km.lookup[dbl]; ok {
				return []string{id}
			}
		} else if id, ok := km.lookup[t.ch]; ok {
			out = append(out, id)
		}
	}
	dbl := ch
	dbl.Double = true
	if _, ok := km.lookup[dbl]; ok {
		t.ch, t.at, t.on = ch, now, true
		return out
	}
	if id, ok := km.lookup[ch]; ok {
		out = append(out, id)
	}
	return out
}

// Due runs out the wait: once DoubleTap has passed with no second press,
// the waiting key's single action ("" for none) is due.
func (km *Keymap) Due(t *Taps, now time.Time) (string, bool) {
	if !t.on || now.Sub(t.at) < DoubleTap {
		return "", false
	}
	return km.Flush(t), true
}

// Flush ends the wait at once, as another key comes: it returns the
// waiting key's single action, "" for none.
func (km *Keymap) Flush(t *Taps) string {
	if !t.on {
		return ""
	}
	t.on = false
	return km.lookup[t.ch]
}
