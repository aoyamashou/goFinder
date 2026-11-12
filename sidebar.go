package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"

	"gofinder/internal/favorites"
	"gofinder/internal/finder"
)

// sidebarState is the sidebar, as Finder's: Finder's favorites, the
// locations (iCloud Drive, cloud drives, volumes), then this app's own
// favorites. While it shows, Finder's lists, /Volumes and the favorites'
// folder are watched.
//
// An item's ID is "p:" (Finder's favorites), "l:" (locations) or "f:"
// (own favorites) then its folder's path: sidebarDir reads it back.
type sidebarState struct {
	show  bool
	open  [3]bool // sections unfolded
	sel   string  // item chosen
	shown string  // sel as this app last set it, to spot a click
	store favorites.Store
	favs  []string
	// favBase is the list last read from or saved to the store: what
	// this Mac and the others last agreed on, for Merge.
	favBase []string
	// favFrom is the local favorites.json of phase 2, moved into the
	// iCloud store at the first load; favDir is the store's folder,
	// watched while the sidebar shows so other Macs' edits come in.
	favFrom  favorites.File
	favDir   string
	favReady bool
	favStop  func()

	// home is whose Finder settings, iCloud Drive and cloud drives show;
	// "" reads none of them (tests), and Finder's favorites fall back
	// to places().
	home      string
	finder    []finder.Item // Finder's favorites; nil: not read
	finderOK  bool
	finderSeq int
	findStop  func()

	locs       []finder.Item
	volumesDir string
	volStop    func()
	volSeq     int

	h ui.Handle // the sidebar, for its focus
}

// places stand in for Finder's favorites when they can't be read.
func places() []finder.Item {
	home, _ := os.UserHomeDir()
	return []finder.Item{
		{Name: "主目录", Path: home, Kind: finder.Home},
		{Name: "桌面", Path: filepath.Join(home, "Desktop"), Kind: finder.Desktop},
		{Name: "下载", Path: filepath.Join(home, "Downloads"), Kind: finder.Downloads},
		{Name: "文稿", Path: filepath.Join(home, "Documents"), Kind: finder.Documents},
		{Name: "应用程序", Path: "/Applications", Kind: finder.Applications},
	}
}

// finderItems is the first section: Finder's favorites, or places().
func (a *app) finderItems() []finder.Item {
	if a.sb.finderOK {
		return a.sb.finder
	}
	return places()
}

// sidebarDir is the folder of a sidebar item's ID.
func sidebarDir(id string) string {
	if len(id) < 2 {
		return ""
	}
	return id[2:]
}

// sidebarID is the item showing dir, "" for none.
func (a *app) sidebarID(dir string) string {
	has := func(items []finder.Item) bool {
		return slices.ContainsFunc(items, func(it finder.Item) bool { return it.Path == dir })
	}
	switch {
	case has(a.finderItems()):
		return "p:" + dir
	case slices.Contains(a.sb.favs, dir):
		return "f:" + dir
	case has(a.sb.locs):
		return "l:" + dir
	}
	return ""
}

func (a *app) loadFavorites() {
	st, from, dir := a.sb.store, a.sb.favFrom, a.sb.favDir
	if st == nil {
		return
	}
	a.async(func() {
		var err error
		if to, ok := st.(favorites.File); ok && from.Path != "" {
			err = favorites.Migrate(from, to)
		}
		if dir != "" {
			os.MkdirAll(dir, 0o755) // so it can be watched
		}
		favs, lerr := st.Load()
		a.post(func() {
			a.sb.favReady = true
			if err == nil {
				err = lerr
			}
			if err != nil {
				a.status = "无法读取收藏夹：" + err.Error()
			}
			if lerr == nil {
				a.sb.favs, a.sb.favBase = favs, favs
			}
		})
	})
}

// syncFavorites reads the store again after another Mac changed it, and
// merges (favorites.Merge): their additions, removals and order come in,
// and what this Mac changed since stays. It saves when the merge differs
// from the file.
func (a *app) syncFavorites() {
	st := a.sb.store
	if st == nil {
		return
	}
	a.async(func() {
		disk, err := st.Load()
		if err != nil {
			return // a half-synced file: the next change reads it again
		}
		a.post(func() {
			merged := favorites.Merge(a.sb.favBase, a.sb.favs, disk)
			a.sb.favs, a.sb.favBase = merged, disk
			if !slices.Equal(merged, disk) {
				a.saveFavorites()
			}
		})
	})
}

func (a *app) saveFavorites() {
	st, favs := a.sb.store, slices.Clone(a.sb.favs)
	a.sb.favBase = favs
	a.async(func() {
		if err := st.Save(favs); err != nil {
			a.post(func() { a.status = "无法保存收藏夹：" + err.Error() })
		}
	})
}

// setFavorite adds dir to the favorites, or removes it, and saves them.
func (a *app) setFavorite(dir string, add bool) {
	var changed bool
	if add {
		a.sb.favs, changed = favorites.Add(a.sb.favs, dir)
		a.status = "已收藏 " + filepath.Base(dir)
		if !changed {
			a.status = filepath.Base(dir) + " 已在收藏夹中"
		}
	} else {
		a.sb.favs, changed = favorites.Remove(a.sb.favs, dir)
		a.status = "已从收藏夹移除 " + filepath.Base(dir)
		if !changed {
			a.status = filepath.Base(dir) + " 不在收藏夹中"
		}
	}
	if !changed || a.sb.store == nil {
		return
	}
	a.saveFavorites()
}

// loadFinder reads Finder's favorites; when it can't, the first section
// shows places().
func (a *app) loadFinder() {
	if a.sb.home == "" {
		return
	}
	a.sb.finderSeq++
	seq, home := a.sb.finderSeq, a.sb.home
	a.async(func() {
		items, err := finder.Favorites(home)
		a.post(func() {
			if a.sb.finderSeq == seq {
				a.sb.finder, a.sb.finderOK = items, err == nil
			}
		})
	})
}

// loadLocations lists iCloud Drive, the cloud drives and the volumes
// Finder shows.
func (a *app) loadLocations() {
	a.sb.volSeq++
	seq, home, dir := a.sb.volSeq, a.sb.home, a.sb.volumesDir
	a.async(func() {
		locs := finder.Locations(home, dir)
		a.post(func() {
			if a.sb.volSeq == seq {
				a.sb.locs = locs
			}
		})
	})
}

// syncSidebar watches Finder's lists, /Volumes and the favorites' folder
// while the sidebar shows.
func (a *app) syncSidebar() {
	switch {
	case a.sb.show && a.sb.favStop == nil && a.sb.favReady && a.sb.favDir != "":
		a.syncFavorites()
		a.sb.favStop = a.sys.Watch(a.sb.favDir, func() { a.post(a.syncFavorites) })
	case !a.sb.show && a.sb.favStop != nil:
		a.sb.favStop()
		a.sb.favStop = nil
	}
	switch {
	case a.sb.show && a.sb.findStop == nil && a.sb.home != "":
		a.loadFinder()
		a.sb.findStop = a.sys.Watch(finder.ListDir(a.sb.home), func() { a.post(a.loadFinder) })
	case !a.sb.show && a.sb.findStop != nil:
		a.sb.findStop()
		a.sb.findStop = nil
	}
	switch {
	case a.sb.show && a.sb.volStop == nil:
		a.loadLocations()
		a.sb.volStop = a.sys.Watch(a.sb.volumesDir, func() { a.post(a.loadLocations) })
	case !a.sb.show && a.sb.volStop != nil:
		a.sb.volStop()
		a.sb.volStop = nil
	}
}

// symbols are the SF Symbols of the items, as Finder's.
var symbols = map[finder.Kind][]string{
	finder.Home:         {"house"},
	finder.Desktop:      {"menubar.dock.rectangle"},
	finder.Downloads:    {"arrow.down.circle"},
	finder.Documents:    {"doc"},
	finder.Applications: {"a.square", "square.grid.2x2"},
	finder.Pictures:     {"photo"},
	finder.Music:        {"music.note"},
	finder.Movies:       {"film"},
	finder.ICloud:       {"icloud"},
	finder.Cloud:        {"cloud"},
	finder.Volume:       {"externaldrive"},
	finder.Folder:       {"folder"},
}

const sidebarIcon = 16 // DIPs

// sidebarItem is an item with its SF Symbol. ui.SidebarItem takes only
// an SVG icon, so the symbol is painted in room left before the label.
func (a *app) sidebarItem(c *ui.Context, id string, kind finder.Kind, label string, focused bool) ui.Element {
	col := tok(c, "muted")
	switch {
	case !c.WindowActive():
		col = tok(c, "faint") // goFinder is in the background
	case focused && a.sb.sel == id:
		col = tok(c, "selection-ink") // over the selection color
	}
	names := symbols[kind]
	if names == nil {
		names = symbols[finder.Folder]
	}
	left, gap := px("space-2"), px("space-1.5")
	return ui.SidebarItem(c, id, nil, label).Height(px("row-height")).Radius(px("radius-row")).
		Padding(0, left).PaddingStart(left + sidebarIcon + gap).Draw(func(p *ui.Painter, r ui.Rect) {
		box := ui.Rect{X: r.X + left, Y: r.Y + (r.H-sidebarIcon)/2, W: sidebarIcon, H: sidebarIcon}
		pxl := int(sidebarIcon*max(p.Scale(), 1) + 0.5)
		for _, n := range names {
			if src, ok := sfsymbols.Image(n, pxl, col, sfsymbols.Options{Natural: true}); ok {
				p.Image(src, box, ui.Contain)
				return
			}
		}
	})
}

// ejectButton is a volume's ⏏ at the end of its sidebar item, as
// Finder's; onAccent draws it over the chosen item's accent.
func (a *app) ejectButton(c *ui.Context, path string, onAccent bool) {
	const s = 14
	b := ui.ButtonBase(c).Key("eject").Width(s + 4).Height(s + 4).Radius(px("radius-control")).
		Label("推出").Tooltip("推出")
	col := tok(c, "muted")
	switch {
	case !c.WindowActive():
		col = tok(c, "faint")
	case onAccent:
		col = tok(c, "selection-ink")
	case b.Hovered():
		col = tok(c, "ink")
	}
	if b.Clicked() {
		a.eject(path)
	}
	if !sfsymbols.Available() {
		b.Width(0).TextColor(col).Children(func() { ui.Text(c, "⏏").FontSize(11) })
		return
	}
	b.Draw(func(p *ui.Painter, r ui.Rect) {
		box := ui.Rect{X: r.X + (r.W-s)/2, Y: r.Y + (r.H-s)/2, W: s, H: s}
		if src, ok := sfsymbols.Image("eject", int(s*max(p.Scale(), 1)+0.5), col, sfsymbols.Options{Natural: true}); ok {
			p.Image(src, box, ui.Contain)
		}
	})
}

// eject ejects the volume at path off the main thread. Tabs inside it go
// home first, so nothing of goFinder keeps the volume busy.
func (a *app) eject(path string) {
	for _, p := range a.panes {
		if d := p.ts.t.Dir; d == path || strings.HasPrefix(d, path+"/") {
			a.navigate(p.ts, a.sb.home)
		}
	}
	a.syncWatches()
	name := filepath.Base(path)
	a.status = "正在推出“" + name + "”…"
	a.async(func() {
		err := a.sys.Eject(path)
		a.post(func() {
			if err != nil {
				a.status = "无法推出“" + name + "”：" + err.Error()
			} else {
				a.status = "已推出“" + name + "”"
			}
			a.loadLocations()
		})
	})
}

func (a *app) sidebar(c *ui.Context) {
	ts := a.tab()
	want := a.sidebarID(ts.t.Dir)
	if a.sb.sel != a.sb.shown && a.sb.sel != "" {
		// An item was chosen since the last pass: go there.
		want = a.sb.sel
		a.navigate(ts, sidebarDir(a.sb.sel))
		if a.focusZone == "sidebar" && a.sb.h.FocusWithin(c) {
			a.wantFocus = false // reached by Tab: the keyboard stays in the sidebar
		}
	}
	// A click puts the keyboard in the sidebar; it, and the outline, go
	// back to the files the user left (user, 2026-11-12).
	if !a.wantSidebar && a.focusZone != "sidebar" && a.sb.h.FocusWithin(c) {
		a.wantFocus = true
	}
	if a.wantSidebar {
		a.sb.h.Focus(c)
		a.wantSidebar = false
	}
	a.sb.sel, a.sb.shown = want, want
	focused := a.sb.h.FocusWithin(c)
	muted := func(s string) { ui.Text(c, s).FontSize(11).TextColor(tok(c, "muted")).Padding(2, 8) }
	sb := ui.Sidebar(c.Key("sidebar"), &a.sb.sel, func() {
		a.dbg(c, ui.SidebarSection(c, "个人收藏", &a.sb.open[0], func() {
			for _, it := range a.finderItems() {
				a.acceptDrop(a.sidebarItem(c, "p:"+it.Path, it.Kind, it.Name, focused), it.Path).Tooltip(it.Path)
			}
		}), "sidebar.favs")
		a.dbg(c, ui.SidebarSection(c, "位置", &a.sb.open[1], func() {
			if len(a.sb.locs) == 0 {
				muted("（无）")
			}
			for _, it := range a.sb.locs {
				e := a.acceptDrop(a.sidebarItem(c, "l:"+it.Path, it.Kind, it.Name, focused), it.Path).Tooltip(it.Path)
				if it.Kind == finder.Volume {
					p := it.Path
					e.Children(func() { a.ejectButton(c, p, focused && a.sb.sel == "l:"+p) })
					e.ContextMenu(func(m *ui.Menu) {
						if m.Item("推出“" + it.Name + "”").Chosen() {
							a.eject(p)
						}
					})
				}
			}
		}), "sidebar.locs")
		a.dbg(c, ui.SidebarSection(c, "收藏夹", &a.sb.open[2], func() {
			if len(a.sb.favs) == 0 {
				hint := "（空）"
				if keys := a.km.KeysFor("fav.add"); len(keys) > 0 {
					hint = keys[0].String() + " 收藏当前文件夹"
				}
				muted(hint)
			}
			for _, f := range a.sb.favs {
				a.acceptDrop(a.sidebarItem(c, "f:"+f, finder.Folder, filepath.Base(f), focused), f).Tooltip(f).ContextMenu(func(m *ui.Menu) {
					if m.Item("从收藏夹移除").Chosen() {
						a.setFavorite(f, false)
					}
				})
			}
		}), "sidebar.mine")
	}).Width(px("sidebar-width")).Label("侧边栏").Bind(&a.sb.h)
	if a.vibrant {
		sb.Background(ui.Transparent) // the window's sidebar material shows
	} else {
		sb.Background(tok(c, "sidebar"))
	}
	a.dbg(c, sb, "sidebar")
	a.focusBorder(c, sb, a.focusZone == "sidebar")
}
