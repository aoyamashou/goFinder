package main

import (
	"path/filepath"
	"testing"
)

// 设为起点 on a folder sets startDir, saved; startDir opens there
// while the folder is there; choosing it again clears it.
func TestStartHere(t *testing.T) {
	a, tt, _, root := setup(t)
	a.dataDir = t.TempDir()
	dir := filepath.Join(root, "beta")
	choose := func() {
		t.Helper()
		if err := tt.RightClick("beta"); err != nil {
			t.Fatal(err)
		}
		if err := tt.ChooseMenuItem("设为起点"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	choose()
	if a.cfg.StartDir != dir {
		t.Fatalf("startDir %q", a.cfg.StartDir)
	}
	if got := loadConfig(a.dataDir).StartDir; got != dir {
		t.Fatalf("saved %q", got)
	}
	home := t.TempDir()
	if got := startDir(nil, home, dir, ""); got != dir {
		t.Errorf("start %q", got)
	}
	if got := startDir(nil, home, filepath.Join(dir, "gone"), ""); got != home {
		t.Errorf("gone folder: %q", got)
	}
	if got := startDir([]string{home}, "/", dir, ""); got != home {
		t.Errorf("command line first: %q", got)
	}
	choose()
	if a.cfg.StartDir != "" {
		t.Errorf("not cleared: %q", a.cfg.StartDir)
	}
}
