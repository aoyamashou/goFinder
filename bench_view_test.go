package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// tenK makes a folder of 10,000 empty files beside a small one.
func tenK(b *testing.B) (root, big string) {
	b.Helper()
	root = b.TempDir()
	big = filepath.Join(root, "big")
	os.Mkdir(big, 0o755)
	for i := range 10000 {
		os.WriteFile(filepath.Join(big, fmt.Sprintf("file-%05d.txt", i)), nil, 0o644)
	}
	return root, big
}

func benchApp(b *testing.B, start string) (*app, *ui.Tester) {
	b.Helper()
	cfg := defaultConfig()
	km, _, _ := buildKeymap(cfg, "")
	a := newApp(km, &fakeSys{}, cfg, start)
	a.async = func(f func()) { f() }
	a.gitDelay = 0
	a.start()
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	return a, tt
}

// Entering a folder of 10,000 files: read, sort and the first frame.
func BenchmarkEnter10k(b *testing.B) {
	root, big := tenK(b)
	a, tt := benchApp(b, root)
	for b.Loop() {
		a.navigate(a.tab(), big)
		tt.Frame()
		a.navigate(a.tab(), root)
		tt.Frame()
	}
}

// A frame with nothing changed, the folder of 10,000 files showing.
func BenchmarkIdleFrame10k(b *testing.B) {
	_, big := tenK(b)
	_, tt := benchApp(b, big)
	for b.Loop() {
		tt.Frame()
	}
}

// Moving the cursor one row down in the folder of 10,000 files.
func BenchmarkCursorDown10k(b *testing.B) {
	_, big := tenK(b)
	_, tt := benchApp(b, big)
	for b.Loop() {
		tt.Key(0, ui.KeyDown)
		tt.Frame()
	}
}
