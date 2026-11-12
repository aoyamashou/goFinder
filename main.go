// goFinder is a small, keyboard-first file manager for macOS built with
// MyGo's native UI.
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"

	"gofinder/internal/favorites"
	"gofinder/internal/finder"
	"gofinder/internal/templates"
)

func main() {
	// Updates from the GitHub releases (mygo.json updates): a check 10s
	// after launch, then daily; development builds never check.
	mygo.Use(native.Plugin)
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/"
	}
	cfg, dataDir := defaultConfig(), ""
	if dir, err := mygo.App.Path(mygo.PathUserData); err == nil {
		// The data of the days of the name MygoFinder comes along.
		if err := moveOld(besideOld(dir), dir); err != nil {
			log.Printf("data folder: %v", err)
		}
		cfg, dataDir = loadConfig(dir), dir
	}
	start := startDir(os.Args[1:], home, cfg.StartDir, finder.NewWindowDir(home))
	km, kmFile, warns := buildKeymap(cfg, keymapPath(dataDir))
	for _, w := range warns {
		log.Print(w)
	}

	a := newApp(km, realSystem{}, cfg, start)
	if d := cfg.StartDir2; len(os.Args) < 2 && isDir(d) {
		a.panes[1] = &paneState{ts: newTabState(d, cfg.ShowHidden)} // its own 启动时从这里开始
	}
	a.sb.home = home // the sidebar follows Finder's
	a.updates = true // native.Plugin is used
	a.kmFile, a.kmPath = kmFile, keymapPath(dataDir)
	a.setWarnings(warns)
	if dir, err := mygo.App.Path(mygo.PathUserData); err == nil {
		a.dataDir = dir
		a.tplDir = filepath.Join(dir, "templates")
		if err := templates.Ensure(a.tplDir); err != nil {
			log.Printf("templates: %v", err)
		}
		// Favorites sync through iCloud Drive when it is there; the local
		// file of phase 2 moves in at the first load.
		local := favorites.File{Path: filepath.Join(dir, "favorites.json")}
		a.sb.store = local
		if home, err := os.UserHomeDir(); err == nil {
			if p, ok := favorites.ICloudFile(home); ok {
				if err := moveOld(besideOld(filepath.Dir(p)), filepath.Dir(p)); err != nil {
					log.Printf("iCloud favorites: %v", err)
				}
				a.sb.store, a.sb.favFrom = favorites.File{Path: p}, local
			}
		}
		if f, ok := a.sb.store.(favorites.File); ok {
			a.sb.favDir = filepath.Dir(f.Path)
		}
		a.sh.file = filepath.Join(dir, "shelf.json")
	}
	if dir, err := mygo.App.Path(mygo.PathCache); err == nil {
		a.cacheDir = dir
	}

	// Folders opened with the app (Dock, "open -a", the default file
	// viewer) open in the active pane. Those that come before the window is up
	// wait for it, and take the place of the home folder.
	var win *mygo.Window
	var pending []string
	mygo.App.OnOpenFile(func(path string) {
		mygo.RunOnMain(func() {
			if win == nil {
				pending = append(pending, path)
				return
			}
			a.openPath(path)
			win.Invalidate()
		})
	})

	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) { a.stopQuickLook() })
	mygo.App.WhenReady(func() {
		a.setMenu = func(items []*mygo.MenuItem) { mygo.App.SetMenu(mygo.NewMenu(items)) }
		a.applyMenu()
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "goFinder",
			Width:     1035, // the size the user tuned (2026-11-12)
			Height:    692,
			MinWidth:  480,
			MinHeight: 320,
			StateKey:  "main",
			Vibrancy:  mygo.VibrancySidebar, // behind the sidebar, as Finder's
			// The app draws its own title bar (titleBar), so a dark window
			// stays dark under a light macOS.
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(a.view),
		})
		a.post = win.Update
		a.growWindow = func(dw int) { growRight(win, dw) }
		a.start()
		a.detectAgents()
		for _, p := range pending {
			a.openPath(p)
		}
		pending = nil
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// startDir is where the window opens: the folder named on the command
// line, else the one chosen with 启动时从这里开始 (chosen, while it is
// there), else where Finder opens a new window (finderDir, "" for none),
// else home. Folders opened with the app (OnOpenFile) take its place
// once the window is up. The last session is not restored; workspaces
// are.
func startDir(args []string, home, chosen, finderDir string) string {
	if len(args) > 0 {
		if st, err := os.Stat(args[0]); err == nil && st.IsDir() {
			if p, err := filepath.Abs(args[0]); err == nil {
				return p
			}
		}
	}
	if chosen != "" {
		if st, err := os.Stat(chosen); err == nil && st.IsDir() {
			return chosen
		}
	}
	if finderDir != "" {
		return finderDir
	}
	return home
}

// growRight widens win by dw (narrows it, for a negative dw) to the
// right, moving it left where the screen's edge leaves no room. A full
// screen or zoomed window stays as it is.
func growRight(win *mygo.Window, dw int) {
	if win.IsFullScreen() || win.IsMaximized() {
		return
	}
	b := win.Bounds()
	b.Width = max(b.Width+dw, 480)
	for _, d := range mygo.Screen.Displays() {
		wa := d.WorkArea
		if cx := b.X + b.Width/2; cx < wa.X || cx >= wa.X+wa.Width {
			continue
		}
		b.Width = min(b.Width, wa.Width)
		if over := b.X + b.Width - (wa.X + wa.Width); over > 0 {
			b.X -= over
		}
		break
	}
	win.SetBounds(b)
}
