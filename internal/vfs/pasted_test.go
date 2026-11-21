package vfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPastedPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	for in, want := range map[string]string{
		"/Users/me/Docs":                    "/Users/me/Docs",
		"  /Users/me/Docs/  \n":             "/Users/me/Docs",
		"~/Downloads":                       filepath.Join(home, "Downloads"),
		`/Users/me/My\ Files/a\(1\).txt`:    "/Users/me/My Files/a(1).txt",
		`"/Users/me/My Files"`:              "/Users/me/My Files",
		"'/tmp/x y'":                        "/tmp/x y",
		"file:///Users/me/My%20Files/a.txt": "/Users/me/My Files/a.txt",
		"file://localhost/tmp":              "/tmp",
	} {
		if got, ok := PastedPath(in); !ok || got != want {
			t.Errorf("PastedPath(%q) = %q %v, want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "hello", "Docs/a.txt", "/a\n/b", "https://example.com/a", "file://server/x"} {
		if got, ok := PastedPath(in); ok {
			t.Errorf("PastedPath(%q) = %q, want none", in, got)
		}
	}
}
