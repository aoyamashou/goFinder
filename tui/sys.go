package tui

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"gofinder/internal/macfile"
)

// Config holds gf's standalone terminal preferences, completely independent
// from the GUI app's configuration.
type Config struct {
	VimKeys      bool   `json:"vimKeys"`
	ShowHidden   bool   `json:"showHidden"`
	ConfirmTrash *bool  `json:"confirmTrash,omitempty"`
	Editor       string `json:"editor,omitempty"`
	NerdFont     *bool  `json:"nerdFont,omitempty"`
}

// WantConfirmTrash reports whether trash requires confirmation (default true).
func (c Config) WantConfirmTrash() bool {
	if c.ConfirmTrash == nil {
		return true
	}
	return *c.ConfirmTrash
}

// WantNerdFont reports whether Nerd Font icons should be shown (default true).
func (c Config) WantNerdFont() bool {
	if c.NerdFont == nil {
		return true
	}
	return *c.NerdFont
}

// DefaultConfigDir returns $XDG_CONFIG_HOME/gf, falling back to ~/.config/gf.
func DefaultConfigDir() string {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "gf")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "gf")
}

// LoadConfig reads dir/config.json if present, returning defaults otherwise.
func LoadConfig(dir string) Config {
	var cfg Config
	if dir == "" {
		return cfg
	}
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	return cfg
}

// System abstracts OS interactions so TUI behavior can be unit tested cleanly.
type System interface {
	Trash(path string) error
	Open(path string) error
	CopyText(text string) error
	Watch(dir string, onChange func()) (stop func())
}

type realSystem struct{}

func (realSystem) Trash(path string) error {
	_, err := macfile.Trash(path)
	return err
}

func (realSystem) Open(path string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", path).Start()
	}
	return exec.Command("xdg-open", path).Start()
}

func (realSystem) CopyText(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	default:
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		}
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (realSystem) Watch(dir string, onChange func()) func() {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return func() {}
	}
	if err := w.Add(dir); err != nil {
		_ = w.Close()
		return func() {}
	}
	done := make(chan struct{})
	var once sync.Once
	stop := func() {
		once.Do(func() {
			close(done)
			_ = w.Close()
		})
	}
	go func() {
		var timer *time.Timer
		var timerC <-chan time.Time
		defer func() {
			if timer != nil {
				timer.Stop()
			}
		}()
		for {
			select {
			case <-done:
				return
			case _, ok := <-w.Events:
				if !ok {
					return
				}
				if timer == nil {
					timer = time.NewTimer(120 * time.Millisecond)
					timerC = timer.C
				} else {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(120 * time.Millisecond)
				}
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			case <-timerC:
				timerC = nil
				onChange()
			}
		}
	}()
	return stop
}
