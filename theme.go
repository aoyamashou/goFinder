package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// The design system (docs/design-system.md) is the one source of the
// app's colors and metrics: docs/design-tokens.json, read once at start.
// Code asks for a token by its name, never writes a hex value of its own.

//go:embed docs/design-tokens.json
var designTokensJSON []byte

// tokenColor is a color token's value in each theme.
type tokenColor struct{ light, dark ui.Color }

// designTokens is docs/design-tokens.json, parsed: colors by name (aliases
// resolved), and the lengths of spacing, radius and size in DIPs.
type designTokens struct {
	colors  map[string]tokenColor
	lengths map[string]float32
}

var ds = mustParseTokens(designTokensJSON)

func mustParseTokens(b []byte) designTokens {
	d, err := parseTokens(b)
	if err != nil {
		panic("docs/design-tokens.json: " + err.Error())
	}
	return d
}

func parseTokens(b []byte) (designTokens, error) {
	var raw struct {
		Color struct {
			Tokens []struct {
				Name  string            `json:"name"`
				Value map[string]string `json:"value"`
			} `json:"tokens"`
		} `json:"color"`
		Spacing, Radius, Size struct {
			Tokens []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"tokens"`
		}
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return designTokens{}, err
	}
	vals := map[string]map[string]string{}
	for _, t := range raw.Color.Tokens {
		vals[t.Name] = t.Value
	}
	var resolve func(name, theme string, depth int) (ui.Color, error)
	resolve = func(name, theme string, depth int) (ui.Color, error) {
		v, ok := vals[name]
		if !ok || depth > 16 {
			return ui.Color{}, fmt.Errorf("color %q: missing or circular", name)
		}
		s := v[theme]
		if s == "" {
			s = v["light"]
		}
		if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
			return resolve(s[1:len(s)-1], theme, depth+1)
		}
		if !strings.HasPrefix(s, "#") {
			return ui.Color{}, fmt.Errorf("color %q: %q is not hex", name, s)
		}
		return ui.Hex(s), nil
	}
	d := designTokens{colors: map[string]tokenColor{}, lengths: map[string]float32{}}
	for name := range vals {
		l, err := resolve(name, "light", 0)
		if err != nil {
			return d, err
		}
		k, err := resolve(name, "dark", 0)
		if err != nil {
			return d, err
		}
		d.colors[name] = tokenColor{l, k}
	}
	for _, fam := range [][]struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{raw.Spacing.Tokens, raw.Radius.Tokens, raw.Size.Tokens} {
		for _, t := range fam {
			f, err := strconv.ParseFloat(strings.TrimSuffix(t.Value, "px"), 32)
			if err != nil {
				return d, fmt.Errorf("length %q: %v", t.Name, err)
			}
			d.lengths[t.Name] = float32(f)
		}
	}
	return d, nil
}

// tokIn is a color token in theme t (light or dark).
func tokIn(t *ui.Theme, name string) ui.Color {
	if name == "focus" {
		name = "selection"
	}
	v, ok := ds.colors[name]
	if !ok {
		panic("no color token " + name)
	}
	if t.Dark {
		return v.dark
	}
	return v.light
}

// tok is a color token in the frame's theme. "focus" is "selection", the
// brand cyan the theme's Accent is too, whatever the macOS accent.
func tok(c *ui.Context, name string) ui.Color { return tokIn(c.Theme(), name) }

// px is a spacing, radius or size token in DIPs.
func px(name string) float32 {
	v, ok := ds.lengths[name]
	if !ok {
		panic("no length token " + name)
	}
	return v
}

// applyTheme sets the window's theme from the tokens over the system's
// default, which keeps the user's appearance and text scale.
func applyTheme(c *ui.Context) { applyAppearance(c, "system") }

// darkFor tells whether the appearance mode draws dark, sysDark being
// macOS's own: "light" and "system" follow their word, anything else
// ("dark", or missing) is dark.
func darkFor(mode string, sysDark bool) bool {
	switch mode {
	case "light":
		return false
	case "system":
		return sysDark
	}
	return true
}

// applyAppearance sets the window's theme from the tokens in the light or
// dark the mode asks for. Against macOS's own appearance it starts from
// MyGo's theme of that kind. The accent is the brand cyan, not macOS's
// (user, 2026-11-12).
func applyAppearance(c *ui.Context, mode string) {
	sys := c.Theme()
	t := *sys
	if dark := darkFor(mode, sys.Dark); dark != sys.Dark {
		base := ui.LightTheme()
		if dark {
			base = ui.DarkTheme()
		}
		t = *base
		t.FontSize, t.Font = sys.FontSize, sys.Font
	}
	t.Accent, t.AccentText = tokIn(&t, "selection"), tokIn(&t, "selection-ink")
	t.Background = tokIn(&t, "bg")
	t.Surface = tokIn(&t, "surface-100")
	t.SurfaceHover = tokIn(&t, "surface-200")
	t.SurfacePressed = tokIn(&t, "surface-200")
	t.Border = tokIn(&t, "separator")
	t.Text = tokIn(&t, "ink")
	t.TextMuted = tokIn(&t, "muted")
	t.Danger = tokIn(&t, "danger")
	t.Warning = tokIn(&t, "warning")
	t.Radius = px("radius-control")
	c.SetTheme(&t)
}
