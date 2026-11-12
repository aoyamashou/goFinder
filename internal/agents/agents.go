package agents

import (
	"os"
	"path/filepath"
	"strings"
)

// Found is an installed agent and the path of the binary that proved it.
type Found struct {
	Entry Entry
	Path  string
}

// ByID returns the catalog entry with that ID.
func ByID(id string) (Entry, bool) {
	for _, e := range Catalog {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// DefaultCommand is the command that starts e with its YOLO arguments.
func DefaultCommand(e Entry) string {
	if e.Yolo == "" {
		return e.Launch
	}
	return e.Launch + " " + e.Yolo
}

// FirstWord is the command's first word: the binary it runs.
func FirstWord(cmd string) string {
	f := strings.Fields(cmd)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

// Detect finds the catalog's installed agents, in catalog order. dirs are
// searched in order for each binary; exists tells whether a path is an
// executable file.
func Detect(dirs []string, exists func(string) bool) []Found {
	find := func(name string) string {
		for _, d := range dirs {
			if d == "" {
				continue
			}
			if p := filepath.Join(d, name); exists(p) {
				return p
			}
		}
		return ""
	}
	var out []Found
	for _, e := range Catalog {
		path := ""
		for _, n := range e.Detect {
			if path = find(n); path != "" {
				break
			}
		}
		if path == "" {
			continue
		}
		ok := true
		for _, r := range e.Requires {
			if find(r) == "" {
				ok = false
			}
		}
		if ok {
			out = append(out, Found{e, path})
		}
	}
	return out
}

// IsExecutable is Detect's exists for the real disk.
func IsExecutable(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular() && st.Mode()&0o111 != 0
}

// SearchDirs is the login shell's PATH (loginPath), then the places agents
// install to that a Dock-launched app's short PATH misses (from Orca),
// without repeats. home is the user's home folder.
func SearchDirs(loginPath, home string) []string {
	var dirs []string
	seen := map[string]bool{}
	add := func(d string) {
		if d != "" && !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	for _, d := range strings.Split(loginPath, ":") {
		add(d)
	}
	for _, d := range []string{
		".local/bin", ".bun/bin", ".volta/bin", ".asdf/shims", ".local/share/mise/shims",
		"Library/pnpm", ".yarn/bin", ".config/yarn/global/node_modules/.bin", ".npm-global/bin",
		".nix-profile/bin", ".cargo/bin", "go/bin", ".deno/bin",
	} {
		add(filepath.Join(home, d))
	}
	for _, d := range []string{"/opt/homebrew/bin", "/usr/local/bin", "/run/current-system/sw/bin", "/nix/var/nix/profiles/default/bin"} {
		add(d)
	}
	return dirs
}
