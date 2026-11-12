package gui

import "testing"

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
