package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"gofinder/internal/keymap"
)

// keyMsgToChord translates a Bubble Tea key event into a keymap.Chord.
func keyMsgToChord(msg tea.KeyMsg) (keymap.Chord, bool) {
	var mods keymap.Mods
	if msg.Alt {
		mods |= keymap.Alt
	}

	switch msg.Type {
	case tea.KeyUp:
		return keymap.Chord{Mods: mods, Key: "Up"}, true
	case tea.KeyDown:
		return keymap.Chord{Mods: mods, Key: "Down"}, true
	case tea.KeyLeft:
		return keymap.Chord{Mods: mods, Key: "Left"}, true
	case tea.KeyRight:
		return keymap.Chord{Mods: mods, Key: "Right"}, true
	case tea.KeyShiftUp:
		return keymap.Chord{Mods: mods | keymap.Shift, Key: "Up"}, true
	case tea.KeyShiftDown:
		return keymap.Chord{Mods: mods | keymap.Shift, Key: "Down"}, true
	case tea.KeyShiftLeft:
		return keymap.Chord{Mods: mods | keymap.Shift, Key: "Left"}, true
	case tea.KeyShiftRight:
		return keymap.Chord{Mods: mods | keymap.Shift, Key: "Right"}, true
	case tea.KeyHome:
		return keymap.Chord{Mods: mods, Key: "Home"}, true
	case tea.KeyEnd:
		return keymap.Chord{Mods: mods, Key: "End"}, true
	case tea.KeyPgUp:
		return keymap.Chord{Mods: mods, Key: "PageUp"}, true
	case tea.KeyPgDown:
		return keymap.Chord{Mods: mods, Key: "PageDown"}, true
	case tea.KeyEnter:
		return keymap.Chord{Mods: mods, Key: "Enter"}, true
	case tea.KeyEsc:
		return keymap.Chord{Mods: mods, Key: "Escape"}, true
	case tea.KeyTab:
		return keymap.Chord{Mods: mods, Key: "Tab"}, true
	case tea.KeyShiftTab:
		return keymap.Chord{Mods: mods | keymap.Shift, Key: "Tab"}, true
	case tea.KeySpace:
		return keymap.Chord{Mods: mods, Key: "Space"}, true
	case tea.KeyBackspace:
		return keymap.Chord{Mods: mods, Key: "Backspace"}, true
	case tea.KeyDelete:
		return keymap.Chord{Mods: mods, Key: "ForwardDelete"}, true
	case tea.KeyF1:
		return keymap.Chord{Mods: mods, Key: "F1"}, true
	case tea.KeyF2:
		return keymap.Chord{Mods: mods, Key: "F2"}, true
	case tea.KeyF3:
		return keymap.Chord{Mods: mods, Key: "F3"}, true
	case tea.KeyF4:
		return keymap.Chord{Mods: mods, Key: "F4"}, true
	case tea.KeyF5:
		return keymap.Chord{Mods: mods, Key: "F5"}, true
	case tea.KeyF6:
		return keymap.Chord{Mods: mods, Key: "F6"}, true
	case tea.KeyF7:
		return keymap.Chord{Mods: mods, Key: "F7"}, true
	case tea.KeyF8:
		return keymap.Chord{Mods: mods, Key: "F8"}, true
	case tea.KeyF9:
		return keymap.Chord{Mods: mods, Key: "F9"}, true
	case tea.KeyF10:
		return keymap.Chord{Mods: mods, Key: "F10"}, true
	case tea.KeyF11:
		return keymap.Chord{Mods: mods, Key: "F11"}, true
	case tea.KeyF12:
		return keymap.Chord{Mods: mods, Key: "F12"}, true
	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			return keymap.Chord{}, false
		}
		return runeToChord(msg.Runes[0], mods)
	}

	// Ctrl+A .. Ctrl+Z (excluding Tab/Enter/Backspace/Esc handled above).
	if s := msg.String(); strings.HasPrefix(s, "ctrl+") {
		rest := strings.TrimPrefix(s, "ctrl+")
		if len(rest) == 1 && rest[0] >= 'a' && rest[0] <= 'z' {
			return keymap.Chord{Mods: mods | keymap.Ctrl, Key: strings.ToUpper(rest)}, true
		}
	}
	return keymap.Chord{}, false
}

func runeToChord(r rune, mods keymap.Mods) (keymap.Chord, bool) {
	switch {
	case r == ' ':
		return keymap.Chord{Mods: mods, Key: "Space"}, true
	case r == '?':
		return keymap.Chord{Mods: mods | keymap.Shift, Key: "/"}, true
	case r >= 'a' && r <= 'z':
		return keymap.Chord{Mods: mods, Key: strings.ToUpper(string(r))}, true
	case r >= 'A' && r <= 'Z':
		return keymap.Chord{Mods: mods | keymap.Shift, Key: string(r)}, true
	case r >= '0' && r <= '9':
		return keymap.Chord{Mods: mods, Key: string(r)}, true
	case strings.ContainsRune(".,/[]-='`\\;", r):
		return keymap.Chord{Mods: mods, Key: string(r)}, true
	default:
		return keymap.Chord{}, false
	}
}
