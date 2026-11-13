package gui

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"gofinder/internal/vfs"
)

func installFixture(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, "Contents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "Contents", "Info.plist"), []byte("fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func selectInstallFixture(t *testing.T, a *app, tt *ui.Tester, root string) string {
	t.Helper()
	path := installFixture(t, filepath.Join(root, "Demo.app"))
	a.run("refresh")
	tt.Frame()
	ts := a.tab()
	ts.cursor = slices.IndexFunc(ts.t.Entries(), func(e vfs.Entry) bool { return e.Path == path })
	if ts.cursor < 0 {
		t.Fatal("missing app")
	}
	tt.Frame()
	return path
}

func TestAppInstallOpenMenu(t *testing.T) {
	for _, choice := range []string{"打开", installTitle, installUnquarantineTitle, "cancel"} {
		t.Run(choice, func(t *testing.T) {
			a, tt, fs, root := setup(t)
			path := selectInstallFixture(t, a, tt, root)
			tt.Key(0, ui.KeyRight)
			tt.Frame()
			if len(fs.opened) != 0 || len(fs.installs) != 0 || a.tab().t.Dir != root {
				t.Fatal("open caused side effect")
			}
			for _, label := range []string{"打开", installTitle, installUnquarantineTitle} {
				if !tt.HasText(label) {
					t.Fatalf("missing %s: %q", label, tt.Texts())
				}
			}
			if choice == "cancel" {
				tt.Key(0, ui.KeyEscape)
				tt.Frame()
				if len(fs.installs) != 0 || len(fs.opened) != 0 {
					t.Fatal("cancel installed")
				}
				return
			}
			if err := tt.ChooseMenuItem(choice); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if choice == "打开" {
				if !slices.Contains(fs.opened, path) || len(fs.installs) != 0 {
					t.Fatal("open")
				}
			} else {
				if len(fs.installs) != 1 || fs.installs[0].path != path || fs.installs[0].unquarantine != (choice == installUnquarantineTitle) {
					t.Fatal(fs.installs)
				}
				if !strings.Contains(a.status, "已复制到 /Applications/Demo.app") || len(fs.opened) != 0 {
					t.Fatal(a.status)
				}
			}
		})
	}
}

func TestAppInstallContextMenus(t *testing.T) {
	for _, column := range []string{"current", "parent", "child"} {
		t.Run(column, func(t *testing.T) {
			a, tt, fs, root := setup(t)
			var path string
			switch column {
			case "child":
				path = installFixture(t, filepath.Join(root, "alpha", "Demo.app"))
				a.afterNav(a.tab())
			default:
				path = selectInstallFixture(t, a, tt, root)
			}
			if column == "parent" {
				a.navigate(a.tab(), filepath.Join(root, "alpha"))
			}
			a.cfg.ContextMenu = []string{"copy.path"} // install entries survive old/custom menu settings.
			tt.Frame()
			if err := tt.RightClick("Demo.app"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if err := tt.ChooseMenuItem(installUnquarantineTitle); err != nil {
				t.Fatalf("%v %q", err, tt.Menu())
			}
			tt.Frame()
			if len(fs.installs) != 1 || fs.installs[0].path != path || !fs.installs[0].unquarantine {
				t.Fatal(fs.installs)
			}
		})
	}
}

func TestInstalledAppStillOpens(t *testing.T) {
	a, tt, fs, root := setup(t)
	a.sb.home = filepath.Join(root, "home")
	path := installFixture(t, filepath.Join(a.sb.home, "Applications", "Demo.app"))
	a.navigate(a.tab(), filepath.Dir(path))
	tt.Frame()
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if !slices.Contains(fs.opened, path) || a.popOpen {
		t.Fatalf("opened=%v pop=%v", fs.opened, a.popOpen)
	}
	if err := tt.RightClick("Demo.app"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText(installTitle) {
		t.Fatal("installed app has install menu")
	}
}

func TestAppInstallBackgroundAndFailure(t *testing.T) {
	a, tt, fs, root := setup(t)
	path := selectInstallFixture(t, a, tt, root)
	var jobs []func()
	a.async = func(f func()) { jobs = append(jobs, f) }
	a.installApp(path, true)
	a.installApp(path, false)
	if len(jobs) != 1 || len(fs.installs) != 0 || !strings.Contains(a.status, "正在复制") {
		t.Fatalf("jobs %d status %s", len(jobs), a.status)
	}
	// Moving the cursor doesn't change the captured installation request.
	a.tab().cursor = 0
	fs.installErr = errors.New("没有权限写入 /Applications")
	jobs[0]()
	if len(fs.installs) != 1 || fs.installs[0].path != path || a.installing[path] || !strings.Contains(a.status, "没有权限") {
		t.Fatal(a.status, fs.installs)
	}
	a.installApp(path, false)
	if len(jobs) != 2 {
		t.Fatal("failed job stayed busy")
	}
}

func TestAppInstallShots(t *testing.T) {
	dir := os.Getenv("MYGO_SHOTS")
	if dir == "" {
		t.Skip("MYGO_SHOTS not set")
	}
	os.MkdirAll(dir, 0755)
	for _, mode := range []string{"light", "dark"} {
		a, tt, _, root := setup(t)
		a.cfg.Appearance = mode
		selectInstallFixture(t, a, tt, root)
		tt.Key(0, ui.KeyRight)
		tt.Frame()
		tt.Frame()
		writePNG(t, filepath.Join(dir, "app-install-"+mode+"-open.png"), tt.Image())
		tt.CloseMenu()
		tt.Frame()
		if err := tt.RightClick("Demo.app"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		tt.Frame()
		writePNG(t, filepath.Join(dir, "app-install-"+mode+"-context.png"), tt.Image())
	}
}

func TestAppInstallOpenEntrypoints(t *testing.T) {
	for _, entry := range []string{"vim", "double-click", "child-double-click"} {
		t.Run(entry, func(t *testing.T) {
			a, tt, fs, root := setup(t)
			if entry == "child-double-click" {
				installFixture(t, filepath.Join(root, "alpha", "Demo.app"))
				a.afterNav(a.tab())
				tt.Frame()
			} else {
				selectInstallFixture(t, a, tt, root)
			}
			if entry == "vim" {
				a.setVimKeys(true)
				tt.Frame()
				tt.Key(0, ui.KeyL)
			} else {
				if err := tt.Click("Demo.app"); err != nil {
					t.Fatal(err)
				}
				if err := tt.Click("Demo.app"); err != nil {
					t.Fatal(err)
				}
			}
			tt.Frame()
			if !tt.HasText(installTitle) || len(fs.installs) != 0 || len(fs.opened) != 0 {
				t.Fatalf("menu absent or side effect: %q installs=%v opened=%v", tt.Menu(), fs.installs, fs.opened)
			}
		})
	}
}
