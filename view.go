package main

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/keymap"
	"gofinder/internal/pane"
	"gofinder/internal/vfs"
)

// view builds the window. It is plain on purpose: default widgets, no
// icons or animation, so it stays light and quick.
func (a *app) view(c *ui.Context) {
	defer a.endHot()
	a.svc, a.hasSvc = c.Services(), true
	a.syncWatches()
	a.syncMenu()
	a.syncSidebar()
	a.updateFocusZone(c)
	// Every popup menu (right-click, toolbar and path menus) is drawn by
	// MyGo in the theme's look, its keys at the right, as Claude
	// Desktop's; the menu bar stays the system's.
	sysDark := c.Theme().Dark
	a.vibrant = c.Vibrancy() && darkFor(a.cfg.Appearance, sysDark) == sysDark
	applyAppearance(c, a.cfg.Appearance)
	c.SetMenuPresentation(ui.PopoverMenus)
	t := c.Theme()
	if a.vibrant {
		c.Root().Background(ui.Transparent) // the sidebar's material shows
	}
	a.dnd.accent, a.dnd.danger, a.dnd.hot = t.Accent, t.Danger, ""

	ui.Column(c).Fill().ObserveInput(a.otherKey).Children(func() {
		a.titleBar(c)
		ui.Row(c).Grow(1).Gap(0).AlignItems(ui.Stretch).Children(func() {
			if a.sb.show {
				a.sidebar(c)
			}
			ui.Column(c).Grow(1).Background(t.Background).Children(func() { a.body(c) })
			if a.sh.show {
				a.shelfView(c)
			}
		})
		a.statusBar(c, t)
	})
	a.helpSheet(c, t)
	a.dialog(c)
	a.dispatch(c)
}

// body is the panes, each in three columns: one, or two side by side or
// one above the other; then the inspector, a narrow fourth column of the
// active pane's item, when it is on.
func (a *app) body(c *ui.Context) {
	paneV := func(i int) func() { return func() { a.columnsView(c, i) } }
	ui.Row(c).Fill().Gap(0).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Children(func() {
			switch a.layout {
			case layoutDual:
				ui.SplitPercent(c, &a.splitDual, paneV(0), paneV(1)).Fill()
			case layoutRows:
				ui.SplitVerticalPercent(c, &a.splitRows, paneV(0), paneV(1)).Fill()
			default:
				a.columnsView(c, 0)
			}
		})
		if a.showPreview {
			ui.Divider(c).Vertical()
			a.inspectorView(c)
		}
		a.fitInspector()
	})
}

// dispatch runs the keymap's shortcuts. Keys without Cmd/Ctrl/Alt step aside
// while a text field has the focus. A key
// bound alone and as a double tap ("n", "n n") waits keymap.DoubleTap for
// its second press: the frame c.After asks for runs the single action.
func (a *app) dispatch(c *ui.Context) {
	if a.recording != "" {
		return // the key pressed is the new shortcut, not a command
	}
	now := c.Now()
	if id, due := a.km.Due(&a.taps, now); due && id != "" {
		a.run(id)
		c.After(0) // show what it did
	}
	plainBlocked := c.FocusTakesText()
	a.keyNow, a.plainBlocked = now, plainBlocked
	seen := map[keymap.Chord]bool{}
	for _, b := range a.km.Bindings() {
		ch := b.Chord
		ch.Double = false // "n n" is pressed as n
		if seen[ch] {
			continue
		}
		seen[ch] = true
		k, ok := uiKeys[ch.Key]
		if !ok {
			continue
		}
		if ch.Mods&(keymap.Cmd|keymap.Ctrl|keymap.Alt) == 0 && plainBlocked {
			continue
		}
		if ch.Mods == 0 && ch.Key == "Enter" {
			continue // the list takes ↩ as Submitted: see enterKey
		}
		if c.Shortcut(toUIMods(ch.Mods), k) {
			for _, id := range a.km.Press(&a.taps, ch, now) {
				a.run(id)
			}
		}
	}
	if d, ok := a.taps.Waiting(now); ok {
		c.After(d)
	}
}

// otherKey runs the single action of a key waiting for its double tap as
// soon as another key comes, before that key does anything.
func (a *app) otherKey(ev ui.InputEvent) {
	if ev.Kind != ui.InputKeyDown || ev.Repeat {
		return
	}
	if a.shiftedKey(ev) {
		return
	}
	waiting, on := a.taps.Pending()
	if !on {
		return
	}
	if ch, ok := chordOf(ev.Mods, ev.Key); ok && ch != waiting {
		if id := a.km.Flush(&a.taps); id != "" {
			a.run(id)
		}
	}
}

// shiftedKey runs a key bound with Shift alone on a punctuation key, as ?
// ("Shift+/"). macOS names a key by its charactersIgnoringModifiers, which
// keep Shift: ⇧/ comes as "?", which MyGo has no Key for (KeyUnknown, with
// / only as PhysicalKey), and MyGo matches shortcuts by position only with
// ⌃, ⌥ or ⌘ held, so c.Shortcut(Shift, KeySlash) never sees it. The
// binding is matched here by position instead, as the keymap writes it.
func (a *app) shiftedKey(ev ui.InputEvent) bool {
	if ev.Key != ui.KeyUnknown || ev.PhysicalKey == ui.KeyUnknown || ev.Mods != ui.Shift ||
		a.recording != "" || a.plainBlocked {
		return false
	}
	ch, ok := chordOf(ev.Mods, ev.PhysicalKey)
	if _, bound := a.km.Lookup(ch); !ok || !bound {
		return false
	}
	if id := a.km.Flush(&a.taps); id != "" {
		a.run(id)
	}
	for _, id := range a.km.Press(&a.taps, ch, a.keyNow) {
		a.run(id)
	}
	return true
}

// paneHead is the pane's address bar, and its filter box.
func (a *app) paneHead(c *ui.Context, pi int) {
	p := a.panes[pi]
	a.addressBar(c, pi, p, p.tab())
	// Always one element here, so the ones after it keep their identity
	// (and the list its focus) as the box comes and goes.
	ui.Column(c.Key("filterslot")).Children(func() {
		if a.filterOn && pi == a.active {
			a.filterBox(c, p.tab())
		}
	})
}

// paneList is the pane's file list, or why it cannot be read.
func (a *app) paneList(c *ui.Context, pi int) {
	ts := a.panes[pi].tab()
	if ts.err != "" {
		ui.Text(c, "无法读取："+ts.err).TextColor(c.Theme().Danger).Padding(12).Grow(1)
		return
	}
	a.table(c, pi, ts)
}

// sortKeys are the columns a tab sorts by, as saved in workspaces.
var sortKeys = map[string]pane.SortKey{"name": pane.ByName, "size": pane.BySize, "date": pane.ByDate, "kind": pane.ByKind}

// table is the tab's file list: icons and names alone, as Yazi shows them.
func (a *app) table(c *ui.Context, pi int, ts *tabState) {
	if k := sortKeys[ts.sort.Column]; k != ts.t.Sort || ts.sort.Descending != ts.t.Desc {
		ts.t.SetSort(k, ts.sort.Descending)
	}
	es := ts.t.Entries()
	gst, groot := a.gitStatus(ts), ts.gitRoot
	focused := a.focusZone == fmt.Sprintf("pane%d", pi)

	ts.hits = ts.hits[:0] // the rows built say where they are
	box := ui.Column(c).Grow(1).ObserveInput(func(ev ui.InputEvent) {
		a.active = pi
		switch ev.Kind {
		case ui.InputKeyDown:
			a.enterKey = ev.Key == ui.KeyEnter && ev.Mods == 0
		case ui.InputPointerDown:
			a.enterKey = false
			a.pointerDown(ts, ev)
		}
	})
	ts.box = box.Bounds()
	if len(es) == 0 && a.popOpen && pi == a.active {
		a.popAt(box) // no row to open at
	}
	box.Children(func() {
		l, room := a.fileList(c.Key(ts), &ts.list, es, focused)
		l = l.Rows(func(r ui.ListRow) {
			e, cur := es[r.Index], r.Index == ts.cursor
			// While a side column's row menu shows, that row has the accent.
			look := lookOf(cur, r.Selected(), r.ListFocused() && c.WindowActive() && a.menuHot == "")
			row := a.fileRow(r.Context, e, look, cur && e.IsDir, gitMark(gst, groot, e), room)
			a.dropRow(row, &ts.list, e)
			ts.hits = append(ts.hits, rowHit{e.Path, row.Bounds()})
			if cur && a.popOpen && pi == a.active {
				// Drawn, the keyboard's menu and the right-click's would
				// share the row's one menu: the row has only this one
				// while it is open.
				a.popAt(row)
				a.dbg(r.Context, row, "menu.context", pi)
			} else {
				row.ContextMenu(func(m *ui.Menu) { a.contextMenu(m, pi, ts, e) })
			}
		}).Label("文件").ContextMenu(func(m *ui.Menu) { a.blankMenu(m, pi, ts.t.Dir) })
		a.acceptDrop(a.dragFiles(l, &ts.list, ts.t.Dir, func(hot string) []string {
			if ts.sel.Has(hot) {
				return paths(ts.chosen())
			}
			return []string{hot}
		}, false), ts.t.Dir)
		if l.Submitted() {
			switch {
			case a.enterKey:
				// ↩ runs what the keymap binds to it (重命名); a double
				// click opens.
				a.enterKey = false
				a.active = pi
				if id, ok := a.km.Lookup(keymap.Chord{Key: "Enter"}); ok {
					a.run(id)
				}
			default:
				if e, ok := ts.cursorEntry(); ok {
					a.active = pi
					a.openEntry(ts, e)
				}
			}
		}
		if !ts.loaded {
			ui.Text(c, "读取中…").TextColor(tok(c, "muted")).Padding(8)
		} else if len(es) == 0 {
			ui.Text(c, "空文件夹").TextColor(tok(c, "muted")).Padding(8)
		}
	})
	if !a.wantFocus && !a.wantSidebar && ts.list.FocusWithin(c) && a.active != pi {
		a.active = pi
		a.filterOn = false
	}
	if a.wantFocus && pi == a.active && !a.helpOpen && !a.dlgOpen && !a.pane().addrEditing {
		ts.list.Focus(c)
		a.wantFocus = false
	}
}

// popAt opens the keyboard's menu (a.pop) on e, the cursor row.
func (a *app) popAt(e ui.Element) {
	e.PopoverMenuButton(a.popMenu, ui.PopoverMenuButtonOptions{
		At: ui.AnchorBottomStart, Open: &a.popOpen,
	})
}

func humanSize(n int64) string {
	const unit = 1000 // as Finder counts
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	v, i := float64(n), 0
	for v >= unit && i < 4 {
		v /= unit
		i++
	}
	return fmt.Sprintf("%.1f %cB", v, "KMGT"[i-1])
}

// contextMenu lists config.json's contextMenu, or defaultMenu; on a
// folder, 启动时从这里开始 last.
func (a *app) contextMenu(m *ui.Menu, pi int, ts *tabState, e vfs.Entry) {
	a.contextMenuPre(m, pi, ts, e, nil)
}

// contextMenuPre is contextMenu, running pre (when not nil) before the
// item chosen: the third column enters the folder its row is in then.
func (a *app) contextMenuPre(m *ui.Menu, pi int, ts *tabState, e vfs.Entry, pre func()) {
	defer a.startHereItem(m, pi, e)
	a.active = pi
	ids := a.cfg.ContextMenu
	if ids == nil {
		ids = defaultMenu
	}
	sep := false // a separator is due before the next item
	n := 0
	hasAgents := slices.Contains(ids, agentsItem)
	for _, id := range ids {
		if id == "-" {
			sep = n > 0
			continue
		}
		if _, ok := a.km.Action(id); !ok && id != agentsItem || id == "terminal" && hasAgents {
			continue
		}
		if sep {
			m.Separator()
			sep = false
		}
		n++
		if id == agentsItem {
			a.agentsMenu(m, func() string { return a.agentDir(ts) }, func() { a.run("terminal") })
			continue
		}
		it := a.menuItem(m, id, a.actionTitle(id))
		switch id {
		case "paste":
			it.Disabled(len(a.clip) == 0)
		case "trash":
			it.Danger()
		}
		if it.Chosen() {
			a.dropHot()
			if pre != nil {
				pre()
			}
			a.run(id)
		}
	}
}

// The address bar is a capsule capsuleH high, editing or not.
const capsuleH = 28 // the design system gives the path pill no height token

// addressBar is one capsule, as QSpace's path bar: ‹ › before it, then
// "▸" (the folders in /), each folder's name with a ▸ menu of the folders
// in it, and the Git branch at the end. A click on the capsule's blank
// space or ⌘L types a path instead, with completion, in the same capsule.
func (a *app) addressBar(c *ui.Context, pi int, p *paneState, ts *tabState) {
	ui.Row(c).Gap(2).Padding(4, 6).AlignItems(ui.Center).Children(func() {
		if p.addrEditing {
			a.addressInput(c, pi, p, ts)
			return
		}
		capsule := ui.Row(c.Key("capsule")).Grow(1).Shrink(1).Height(capsuleH).Gap(0).Padding(0, px("space-2")).AlignItems(ui.Center).
			Background(tok(c, "surface-100")).Radius(px("radius-pill")).Font(a.listFont()).FontSize(float32(a.listSize())).Tooltip("点击空白处或按 " + a.firstKey("addr") + " 输入路径")
		if capsule.Clicked() {
			a.active = pi
			a.run("addr")
		}
		capsule.Children(func() {
			a.dbg(c, ui.Row(c).Grow(1).Shrink(1).Gap(0).AlignItems(ui.Center).Children(func() {
				a.crumbs(c, pi, ts)
			}), "addr.crumbs", pi)
			if br := a.gitBranch(ts); br != "" {
				a.dbg(c, ui.Text(c, br).Font("").FontSize(11).SingleLine().TextColor(tok(c, "muted")).Padding(0, px("space-1")).Tooltip("Git 分支 · 有改动的文件数"), "addr.git", pi)
			}
		})
		a.dbg(c, capsule, "addr.capsule", pi)
	})
}

// addressInput types a path, completing folder names, in the capsule's
// shape.
func (a *app) addressInput(c *ui.Context, pi int, p *paneState, ts *tabState) {
	if p.addrText != p.addrSuggFor {
		p.addrSugg, p.addrSuggFor = vfs.Complete(p.addrText, false), p.addrText
	}
	in := ui.Autocomplete(c.Key("addr"), &p.addrText, p.addrSugg).Label("地址").AutoFocus().Grow(1).Height(capsuleH).Radius(px("radius-pill")).
		Background(tok(c, "surface-100")).Border(1, tok(c, "focus")).Font(a.listFont()).FontSize(float32(a.listSize()))
	switch {
	case in.Submitted():
		p.addrEditing = false
		a.active = pi
		a.goTyped(ts, p.addrText)
		a.wantFocus = true
	case in.Shortcut(0, ui.KeyEscape), in.FocusLeft():
		p.addrEditing = false
		a.wantFocus = true
	}
}

// crumbs are the capsule's path: a ▸ menu for /, then each folder's name
// and its ▸ menu. Deep paths keep the last levels; the middle folds into
// a "…" menu so the bar never overflows. A folder's name takes files
// dropped on it.
func (a *app) crumbs(c *ui.Context, pi int, ts *tabState) {
	const tail = 3
	crumbs := vfs.Crumbs(ts.t.Dir)
	hidden := ts.t.ShowHidden
	for i, name := range crumbs {
		dir := vfs.CrumbPath(crumbs, i)
		if i > 0 && i < len(crumbs)-tail {
			if i == 1 {
				a.flatButton(c.Key("fold"), "…", "更上层的文件夹", false).Menu(func(m *ui.Menu) {
					for j := len(crumbs) - tail - 1; j >= 1; j-- {
						d := vfs.CrumbPath(crumbs, j)
						if m.Item(d).Chosen() {
							a.active = pi
							a.navigate(ts, d)
						}
					}
				})
			}
			continue
		}
		if z, inner, ok := vfs.SplitZip(dir); ok && inner == "" {
			name = filepath.Base(z) // the archive's root: "a.zip", not "a.zip!"
		}
		if i > 0 {
			b := a.flatButton(c.Key("c"+dir), name, dir, false)
			a.acceptDrop(b, dir)
			if b.Clicked() {
				a.active = pi
				a.navigate(ts, dir)
			}
		}
		a.flatButton(c.Key("m"+dir), "▸", "展开 "+dir, false).Label("展开 " + dir).TextColor(tok(c, "muted")).Menu(func(m *ui.Menu) {
			subs := vfs.Subdirs(dir, hidden)
			if len(subs) == 0 {
				m.Item("（没有子文件夹）").Disabled(true)
			}
			for j, sub := range subs {
				if j == 500 {
					m.Item("…").Disabled(true)
					break
				}
				if m.Item(sub).Chosen() {
					a.active = pi
					a.navigate(ts, filepath.Join(dir, sub))
				}
			}
		})
	}
}

// flatButton is a button with no face until hovered, as in a toolbar:
// the path's names and arrows, and ‹ ›.
func (a *app) flatButton(c *ui.Context, label, tip string, disabled bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Padding(1, 5).Radius(t.Radius).Label(label).Tooltip(tip).Disabled(disabled)
	if !disabled && b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	color := t.Text
	if disabled {
		color = t.TextMuted
	}
	return b.TextColor(color).Children(func() {
		ui.Text(c, label).SingleLine()
	})
}

// helpSheet lists every shortcut, read-only; "在设置中修改" goes to the
// settings' 快捷键 page.
// helpSheetW is the cheat sheet's width.
const helpSheetW = 440

func (a *app) helpSheet(c *ui.Context, t *ui.Theme) {
	if !a.helpOpen {
		if a.toKeys {
			// After the sheet closed, which gives the keyboard back to the
			// list, so the page's search can take it.
			a.toKeys = false
			a.settingsDialog()
			a.setPage, a.focusKeyQ = 1, true
		}
		return
	}
	dialogModal(c, &a.helpOpen, func() {
		hs := ui.Column(c).Width(helpSheetW).Gap(px("space-2"))
		defer func() { a.dbg(c, hs, "dlg.help") }()
		hs.Children(func() {
			ui.Text(c, "速查表（Esc 关闭）").FontSize(fontTitle).FontWeight(weightTitle)
			for _, w := range a.warns {
				uiCaption(c, "⚠ "+w, "danger")
			}
			a.keyList(c, false)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				uiCaption(c, "快捷键保存在数据目录的 keymap.json，例如 {\"trash\": [\"d\"]}；同键双击写作 \"c c\"", "muted").Grow(1)
				if ui.Button(c.Key("tosettings"), "在设置中修改").Clicked() {
					a.helpOpen, a.toKeys = false, true
					c.After(0)
				}
			})
		})
	})
	if !a.helpOpen && !a.toKeys {
		a.wantFocus = true
	}
}

// keyList is every action with its keys as caps, by group, with a search;
// edit adds 改键 to each row, and, while one records, 清除, 默认 and 取消.
func (a *app) keyList(c *ui.Context, edit bool) {
	q := ui.SearchField(c.Key("helpq"), &a.helpQuery).Label("搜索快捷键").Placeholder("搜索：名称或按键").
		AutoFocus().ReadOnly(a.recording != "" || a.recEat)
	if !edit {
		// the cheat sheet's; the settings' page has the handle
	} else if q.Bind(&a.keyQ); a.focusKeyQ {
		a.focusKeyQ = false
		a.keyQ.Focus(c)
	}
	rows := a.km.Sheet(a.helpQuery)
	ui.Scroll(c).Height(420).Children(func() {
		group := ""
		for _, h := range rows {
			if h.Group != group {
				group = h.Group
				ui.Text(c, group).Bold().Padding(px("space-2"), 0, px("space-0.5"), 0)
			}
			ui.Row(c.Key("hr:"+h.ID)).Gap(px("space-1.5")).Padding(px("space-0.5"), px("space-1")).AlignItems(ui.Center).Children(func() {
				ui.Text(c, h.Title).SingleLine().Grow(1).Shrink(1)
				if edit && a.recording == h.ID {
					ui.Text(c, "请按下新的按键…").TextColor(tok(c, "selection"))
					if ui.Button(c.Key("unbind"), "清除").Tooltip("不用快捷键").Clicked() {
						a.recording = ""
						a.setKeys(h.ID, nil, false)
					}
					if ui.Button(c.Key("reset"), "默认").Tooltip("恢复默认快捷键").Clicked() {
						a.recording = ""
						a.setKeys(h.ID, nil, true)
					}
					if ui.Button(c.Key("cancel"), "取消").Clicked() {
						a.recording = ""
					}
					return
				}
				keyGroups(c, a.km.KeysFor(h.ID))
				if edit && ui.Button(c.Key("rec"), "改键").Tooltip("录制新的快捷键，单键也可以").Clicked() {
					a.recording = h.ID
				}
			})
		}
		if len(rows) == 0 {
			ui.Text(c, "没有匹配的快捷键").TextColor(tok(c, "muted"))
		}
	})
}

// recordKey takes the key pressed while a row of the 快捷键 page records:
// any key, a single letter too; Esc cancels.
func (a *app) recordKey(ev ui.InputEvent) {
	if ev.Kind == ui.InputText && a.recEat {
		a.recEat = false // the recorded key's text; the field is read-only for it
		return
	}
	if ev.Kind != ui.InputKeyDown || ev.Repeat {
		return
	}
	a.recEat = false
	if a.recording == "" {
		return
	}
	ch, ok := chordOf(ev.Mods, ev.Key)
	if !ok {
		return // a modifier alone: wait for the key
	}
	id := a.recording
	a.recording = ""
	if ch == (keymap.Chord{Key: "Escape"}) {
		return // Esc cancels the recording
	}
	a.recEat = true
	a.setKeys(id, []string{ch.Spec()}, false)
}

func (a *app) dialog(c *ui.Context) {
	if !a.dlgOpen {
		return
	}
	if a.dlg == dlgSettings {
		a.settingsView(c)
		return
	}
	if a.dlg == dlgWorkspace {
		a.workspaceView(c)
		return
	}
	if a.dlg == dlgTrash {
		a.trashView(c)
		return
	}
	title := map[int]string{dlgRename: "重命名", dlgNewFile: "新建文件", dlgNewFolder: "新建文件夹", dlgWsRename: "重命名工作区"}[a.dlg]
	ui.Modal(c, &a.dlgOpen, func() {
		dc := ui.Column(c).Width(360).Gap(10)
		defer func() {
			a.dbg(c, dc, map[int]string{dlgRename: "dlg.rename", dlgNewFile: "dlg.newfile", dlgNewFolder: "dlg.newfolder", dlgWsRename: "dlg.workspace"}[a.dlg])
		}()
		dc.Children(func() {
			ui.Text(c, title).Bold()
			if a.dlg == dlgNewFile && len(a.dlgTpls) > 1 {
				a.templateChosen()
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "模板")
					ui.Select(c.Key("tpl"), &a.dlgTpl, a.dlgTpls).Label("模板").Grow(1)
				})
			}
			if ui.TextInput(c.Key("dlg"), &a.dlgText).Label("名称").AutoFocus().Submitted() {
				a.submitDialog()
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					a.dlgOpen = false
				}
				if ui.PrimaryButton(c, "确定").Clicked() {
					a.submitDialog()
				}
			})
		})
	})
	if !a.dlgOpen {
		a.wantFocus = true
	}
}

// commonTerminals and commonEditors are the apps the settings offer as
// terminal and editor, by bundle name, when installed: not every app.
var (
	commonTerminals = []string{"Terminal", "iTerm", "Ghostty", "Kaku", "Warp", "WezTerm", "Alacritty",
		"kitty", "Hyper", "Tabby", "Wave", "Rio"}
	commonEditors = []string{"TextEdit", "Xcode", "Visual Studio Code", "Cursor", "Windsurf", "Antigravity",
		"Kiro", "Trae", "Zed", "Sublime Text", "CotEditor", "BBEdit", "Nova", "TextMate", "MacVim",
		"Neovide", "Emacs", "Typora", "Fleet", "IntelliJ IDEA", "GoLand", "PyCharm", "WebStorm", "Android Studio"}
)

// commonApps lists the installed apps among common, in common's order,
// cur first when it is not among them.
func commonApps(installed, common []string, cur string) []string {
	var out []string
	for _, n := range common {
		if slices.Contains(installed, n) {
			out = append(out, n)
		}
	}
	if cur != "" && !slices.Contains(out, cur) {
		out = append([]string{cur}, out...)
	}
	return out
}

// settingsView chooses the apps ⌘⌥T and ⌘⌥E open the folder in.
func (a *app) settingsView(c *ui.Context) {
	// One size for every page: the page scrolls in it, its buttons stay
	// at the bottom, inside the panel even in a short window.
	dialogModalSized(c, &a.dlgOpen, settingsW, settingsH, func() {
		col := ui.Column(c).Grow(1).MinHeight(0).Gap(px("space-3")).ObserveInput(a.recordKey)
		defer func() {
			a.dbg(c, col, []string{"dlg.set.general", "dlg.set.keys", "dlg.set.agent", "dlg.set.about"}[a.setPage])
		}()
		col.Children(func() {
			wordmark(c)
			if ui.Tabs(c.Key("setpage"), &a.setPage, "通用", "快捷键", "Agent", "关于").Changed() {
				a.recording, a.focusKeyQ = "", a.setPage == 1
			}
			ui.Scroll(c.Key("setbody")).Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).Gap(px("space-3")).Children(func() { a.settingsPage(c) })
			})
			a.settingsFooter(c)
		})
	})
	if !a.dlgOpen {
		a.recording = ""
		a.wantFocus = true
	}
}

// pageAbout is the settings' 关于 page.
const pageAbout = 3

// settingsW and settingsH are the settings panel's size, the same on
// every page.
const settingsW, settingsH = 640, 600

// settingsFooter is the page's buttons, at the panel's bottom.
func (a *app) settingsFooter(c *ui.Context) {
	ui.Row(c.Key("setfoot")).Gap(px("space-2")).Justify(ui.End).Shrink(0).Children(func() {
		switch a.setPage {
		case 0:
			if ui.Button(c, "取消").Clicked() {
				a.dlgOpen = false
			}
			if ui.PrimaryButton(c, "保存").Clicked() {
				a.submitDialog()
			}
			return
		}
		if ui.PrimaryButton(c.Key("setdone"), "完成").Clicked() {
			a.dlgOpen = false
		}
	})
}

// settingsPage is the body of the settings' page.
func (a *app) settingsPage(c *ui.Context) {
	if a.setPage == 1 {
		a.keyList(c, true)
		uiCaption(c, "改键即时写入数据目录的 keymap.json；同键双击（如 \"c c\"）可在该文件里设置", "muted")
		return
	}
	if a.setPage == 2 {
		a.agentsPage(c)
		return
	}
	if a.setPage == pageAbout {
		a.aboutPage(c)
		return
	}
	ui.Text(c, "终端和编辑器").Bold()
	if a.setLoading {
		uiCaption(c, "正在读取已安装的应用…", "muted")
	}
	for _, f := range []struct {
		label  string
		v      *string
		common []string
	}{{"终端", &a.setTerm, commonTerminals}, {"编辑器", &a.setEditor, commonEditors}} {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, f.label).Width(56)
			ui.Select(c.Key("set"+f.label), f.v, commonApps(a.setApps, f.common, *f.v)).Label(f.label).Grow(1)
		})
	}
	ui.Checkbox(c.Key("set.confirmTrash"), &a.setConfirm, "移到废纸篓前确认（"+a.keysText("trash")+"）")
	a.appearanceSettings(c)
	uiCaption(c, "保存到数据目录的 config.json", "muted")
	ui.Divider(c)
	a.fontSettings(c)
	ui.Divider(c)
	ui.Text(c, "默认文件浏览器").Bold()
	cur := "Finder"
	if a.setViewer != "" {
		cur = a.setViewer
	}
	isMe := a.setViewer != "" && a.setViewer == a.bundleID
	if isMe {
		cur = "goFinder（" + a.setViewer + "）"
	}
	ui.Text(c, "当前："+cur).FontSize(12)
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c.Key("viewer.on"), "设为默认").Disabled(a.setLoading || isMe).Clicked() {
			a.setFileViewer(true)
		}
		if ui.Button(c.Key("viewer.off"), "恢复 Finder").Disabled(a.setLoading || a.setViewer == "").Clicked() {
			a.setFileViewer(false)
		}
	})
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		uiCaption(c, "改的是系统设置 NSFileViewer，注销重新登录（或在终端运行 killall Finder）后生效", "muted").Grow(1)
		if ui.Button(c.Key("viewer.copy"), "复制命令").Clicked() {
			a.copyKillFinder()
		}
	})
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		state := "未授权"
		if a.fullDisk {
			state = "已授权"
		}
		ui.Text(c, "完全磁盘访问").Bold()
		ui.Text(c, state).FontSize(12)
		uiCaption(c, "在列表里打开 goFinder 的开关，然后重启 goFinder", "muted").Grow(1)
		if ui.Button(c.Key("disk.open"), "打开系统设置").Clicked() {
			a.openFullDiskSettings()
		}
	})
}
