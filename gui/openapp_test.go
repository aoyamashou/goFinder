package gui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/vfs"
)

// → on an app runs it, as Finder's double click; the list stays put.
func TestOpenAppRuns(t *testing.T) {
	a, tt, fs, root := setup(t)
	app := filepath.Join(root, "Demo.app")
	os.MkdirAll(filepath.Join(app, "Contents"), 0o755)
	a.run("refresh")
	tt.Frame()
	ts := a.tab()
	ts.cursor = slices.IndexFunc(ts.t.Entries(), func(e vfs.Entry) bool { return e.Name == "Demo.app" })
	if ts.cursor < 0 {
		t.Fatalf("no Demo.app: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if a.tab().t.Dir != root {
		t.Errorf("entered the app: %s", a.tab().t.Dir)
	}
	if !slices.Contains(fs.opened, app) {
		t.Errorf("not run: %v", fs.opened)
	}
}
