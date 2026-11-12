// Package workspace saves and restores the window's arrangement: the
// layout, and every pane's tabs with their folders and sort order. Named
// workspaces are workspaces/<name>.json in the data folder; the state at
// quit is session.json, restored at the next launch.
package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Tab is one tab: its folder and sort column ("name", "size", "date",
// "kind").
type Tab struct {
	Dir  string `json:"dir"`
	Sort string `json:"sort,omitempty"`
	Desc bool   `json:"desc,omitempty"`
}

// Pane is a pane's tabs and which one is current.
type Pane struct {
	Tabs []Tab `json:"tabs"`
	Cur  int   `json:"cur"`
}

// State is the whole arrangement.
type State struct {
	Layout      string `json:"layout"` // "single", "dual", "columns" or "rows"
	Active      int    `json:"active"`
	ShowPreview bool   `json:"showPreview,omitempty"`
	ShowShelf   bool   `json:"showShelf,omitempty"`
	Panes       []Pane `json:"panes"`
}

// Valid fixes what a hand-edited or old file may get wrong: indexes out
// of range, panes without tabs. It reports whether anything usable is left.
func (s *State) Valid() bool {
	var panes []Pane
	for _, p := range s.Panes {
		var tabs []Tab
		for _, t := range p.Tabs {
			if t.Dir != "" {
				tabs = append(tabs, t)
			}
		}
		if len(tabs) == 0 {
			continue
		}
		p.Tabs = tabs
		if p.Cur < 0 || p.Cur >= len(tabs) {
			p.Cur = 0
		}
		panes = append(panes, p)
	}
	s.Panes = panes
	if s.Active < 0 || s.Active >= len(panes) {
		s.Active = 0
	}
	return len(panes) > 0
}

// Read loads a state file.
func Read(path string) (State, error) {
	var s State
	b, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, err
	}
	if !s.Valid() {
		return s, errors.New("没有可恢复的标签页")
	}
	return s, nil
}

// Write saves a state file through a temporary one.
func Write(path string, s State) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// CleanName makes a workspace name safe as a file name; "" when nothing
// is left.
func CleanName(name string) string {
	name = strings.TrimSpace(strings.NewReplacer("/", "-", ":", "-", "\x00", "").Replace(name))
	if strings.HasPrefix(name, ".") {
		name = strings.TrimLeft(name, ".")
	}
	return name
}

// Path is where the workspace name is saved under dir.
func Path(dir, name string) string { return filepath.Join(dir, name+".json") }

// List names the workspaces saved in dir, sorted.
func List(dir string) []string {
	des, _ := os.ReadDir(dir)
	var out []string
	for _, de := range des {
		if n, ok := strings.CutSuffix(de.Name(), ".json"); ok && !de.IsDir() && n != "" && !strings.HasPrefix(n, ".") {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out
}
