package gui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"

	"gofinder/internal/keymap"
)

// menuBar lays out the menu bar: action IDs, "-" for separators, and
// "@name" for the system's items (roles) and generated submenus. Every
// action appears somewhere (TestMenuCoversActions); the user's shell
// commands get a menu of their own.
var menuBar = []struct {
	title string
	ids   []string
}{
	{"文件", []string{"newfile", "newfolder", "-", "open", "reveal", "terminal", "agent", "@agentmenu", "editor", "-", "rename", "copypath", "trash"}},
	{"编辑", []string{"@undo", "@redo", "-", "@cut", "@copy", "@paste", "@selectAll", "-", "filter", "select.none"}},
	{"显示", []string{"layout.columns", "layout.dual", "layout.rows", "-", "quicklook", "preview", "sidebar", "shelf", "-", "@sort", "sortmenu", "hidden", "refresh", "addr"}},
	{"前往", []string{"back", "forward", "up", "-", "go.start", "go.home", "go.desktop", "go.downloads", "go.documents", "go.apps", "-", "fav.add", "fav.remove"}},
	{"暂存架", []string{"shelf.add", "shelf.copy", "shelf.move", "shelf.clear"}},
	{"窗口", []string{"@minimize", "@zoom", "-", "ws.next", "ws.prev", "@ws", "workspace", "-", "pane.switch"}},
	{"帮助", []string{"help"}},
}

// appMenu is the first menu, named after the app by macOS.
var appMenu = []string{"@about", "@update", "-", "settings", "-", "@services", "-", "@hide", "@hideOthers", "@unhide", "-", "@quit"}

// roles are the system items. The Edit roles act on a focused text field
// and otherwise reach the window's shortcuts, so the app's copy, cut,
// paste and select all run through them.
var roles = map[string]struct {
	role   mygo.MenuRole
	label  string
	covers string // the action the role runs outside text fields
}{
	"@undo":       {mygo.RoleUndo, "撤销", ""},
	"@redo":       {mygo.RoleRedo, "重做", ""},
	"@cut":        {mygo.RoleCut, "剪切", "cut"},
	"@copy":       {mygo.RoleCopy, "复制", "copy"},
	"@paste":      {mygo.RolePaste, "粘贴", "paste"},
	"@selectAll":  {mygo.RoleSelectAll, "全选", "select.all"},
	"@minimize":   {mygo.RoleMinimize, "最小化", ""},
	"@zoom":       {mygo.RoleZoom, "缩放", ""},
	"@about":      {mygo.RoleAbout, "关于 goFinder", ""},
	"@services":   {mygo.RoleServices, "服务", ""},
	"@hide":       {mygo.RoleHide, "隐藏 goFinder", ""},
	"@hideOthers": {mygo.RoleHideOthers, "隐藏其他", ""},
	"@unhide":     {mygo.RoleUnhide, "全部显示", ""},
	"@quit":       {mygo.RoleQuit, "退出 goFinder", ""},
}

// menuUnsafeKeys never become menu shortcuts: the menu takes its keys
// before the window, so these would leave text fields and the file list
// without their editing and moving keys. The window's shortcuts still
// run them; the menu shows the key in the item's title.
var menuUnsafeKeys = map[string]bool{
	"Backspace": true, "ForwardDelete": true, "Up": true, "Down": true, "Left": true, "Right": true,
	"Home": true, "End": true, "PageUp": true, "PageDown": true, "Tab": true, "Space": true,
	"Enter": true, "Escape": true,
}

// roleChords are the Edit roles' own keys.
var roleChords = map[keymap.Chord]bool{
	{Mods: keymap.Cmd, Key: "C"}: true, {Mods: keymap.Cmd, Key: "X"}: true, {Mods: keymap.Cmd, Key: "V"}: true,
	{Mods: keymap.Cmd, Key: "A"}: true, {Mods: keymap.Cmd, Key: "Z"}: true, {Mods: keymap.Cmd | keymap.Shift, Key: "Z"}: true,
}

// menuAccel picks the key the menu item shows as its shortcut, "" for
// none: a key with ⌘ or ⌃, never a double tap.
func menuAccel(chords []keymap.Chord) string {
	for _, ch := range chords {
		if ch.Mods&(keymap.Cmd|keymap.Ctrl) != 0 && !ch.Double && !menuUnsafeKeys[ch.Key] && !roleChords[ch] {
			return ch.Spec()
		}
	}
	return ""
}

// menuToggles are the items with a check mark.
var menuToggles = map[string]mygo.MenuItemType{
	"layout.dual":    mygo.MenuItemRadio,
	"layout.columns": mygo.MenuItemRadio, "layout.rows": mygo.MenuItemRadio,
	"preview": mygo.MenuItemCheckbox, "sidebar": mygo.MenuItemCheckbox,
	"shelf": mygo.MenuItemCheckbox, "hidden": mygo.MenuItemCheckbox,
}

// menuState is the menu bar built, and what it last showed.
type menuState struct {
	items   map[string]*mygo.MenuItem // by action ID
	modal   bool
	checked map[string]bool
	agents  string // signature of the agents the "Agent" submenu lists
}

// buildMenu makes the menu bar from the keymap, so its titles and keys
// are the ones the app has now.
func (a *app) buildMenu() []*mygo.MenuItem {
	a.menu = menuState{items: map[string]*mygo.MenuItem{}, checked: map[string]bool{}, agents: a.agentSig()}
	bar := []*mygo.MenuItem{{Label: "goFinder", Submenu: a.menuItems(appMenu)}}
	for _, m := range menuBar {
		bar = append(bar, &mygo.MenuItem{Label: m.title, Submenu: a.menuItems(m.ids)})
	}
	var cmds []string
	for _, act := range a.km.Actions() {
		if strings.HasPrefix(act.ID, cmdPrefix) {
			cmds = append(cmds, act.ID)
		}
	}
	if len(cmds) > 0 {
		// Before Window and Help, where macOS expects those last.
		n := len(bar) - 2
		bar = append(bar[:n], append([]*mygo.MenuItem{{Label: "命令", Submenu: a.menuItems(cmds)}}, bar[n:]...)...)
	}
	return bar
}

func (a *app) menuItems(ids []string) []*mygo.MenuItem {
	var out []*mygo.MenuItem
	for _, id := range ids {
		switch {
		case id == "-":
			out = append(out, mygo.Separator())
		case id == "@ws":
			var sub []string
			for i := 1; i <= 9; i++ {
				sub = append(sub, fmt.Sprintf("ws.%d", i))
			}
			out = append(out, &mygo.MenuItem{Label: "切换到工作区", Submenu: a.menuItems(sub)})
		case id == "@sort":
			out = append(out, &mygo.MenuItem{Label: "排序方式", Submenu: a.sortItems()})
		case id == "@update":
			if a.updates {
				out = append(out, updater.MenuItem())
			}
		case id == "@agentmenu":
			out = append(out, &mygo.MenuItem{Label: "在这里打开 Agent", Submenu: a.agentItems()})
		case strings.HasPrefix(id, "@"):
			r := roles[id]
			out = append(out, &mygo.MenuItem{Role: r.role, Label: r.label})
		default:
			out = append(out, a.actionItem(id))
		}
	}
	return out
}

func (a *app) actionItem(id string) *mygo.MenuItem {
	act, _ := a.km.Action(id)
	chords := a.km.KeysFor(id)
	// One title, one key: the action's first key, as the status bar and
	// the right-click menu show it. A ⌘/⌃ key the menu can take is the
	// accelerator; any other (a Yazi key, ↩, →) follows the title, and
	// the ⌘ key after it is left to the window's shortcuts, unshown.
	it := &mygo.MenuItem{ID: id, Label: act.Title}
	if len(chords) > 0 {
		if acc := menuAccel(chords[:1]); acc != "" {
			it.Accelerator = acc
		} else {
			it.Label += "　" + chords[0].String()
		}
	}
	if t, ok := menuToggles[id]; ok {
		it.Type = t
	}
	it.Click = func(*mygo.MenuItem, *mygo.Window) {
		a.post(func() {
			if !a.modalOpen() {
				a.run(id)
			}
		})
	}
	a.menu.items[id] = it
	return it
}

// agentSig identifies the enabled agents and the default, to rebuild the
// menu bar when they change.
func (a *app) agentSig() string {
	var b strings.Builder
	if d, ok := a.defaultAgentItem(); ok {
		b.WriteString(d.ID)
	}
	for _, ag := range a.enabledAgents() {
		b.WriteString("\x00" + ag.ID + "\x01" + ag.Name)
	}
	return b.String()
}

// agentItems is the "open Agent" submenu: one item per enabled agent.
func (a *app) agentItems() []*mygo.MenuItem {
	var out []*mygo.MenuItem
	for _, ag := range a.enabledAgents() {
		id := "agent." + ag.ID
		it := &mygo.MenuItem{ID: id, Label: ag.Name}
		it.Click = func(*mygo.MenuItem, *mygo.Window) {
			a.post(func() {
				if !a.modalOpen() {
					a.openAgent(ag, a.agentDir(a.tab()))
				}
			})
		}
		a.menu.items[id] = it
		out = append(out, it)
	}
	if len(out) == 0 {
		out = append(out, &mygo.MenuItem{Label: "没有启用的 Agent", Disabled: true})
	}
	return out
}

// sortOrder is the sort submenu: the list has no header to click since
// it shows names alone.
var sortOrder = []struct{ col, title string }{
	{"name", "名称"}, {"size", "大小"}, {"date", "修改日期"}, {"kind", "类型"},
}

func (a *app) sortItems() []*mygo.MenuItem {
	var out []*mygo.MenuItem
	add := func(id, label string, typ mygo.MenuItemType, f func(ts *tabState)) {
		it := &mygo.MenuItem{ID: id, Label: label, Type: typ}
		it.Click = func(*mygo.MenuItem, *mygo.Window) {
			a.post(func() {
				if !a.modalOpen() {
					f(a.tab())
				}
			})
		}
		a.menu.items[id] = it
		out = append(out, it)
	}
	for _, s := range sortOrder {
		add("sort."+s.col, s.title, mygo.MenuItemRadio, func(ts *tabState) { ts.sort.Column = s.col })
	}
	out = append(out, mygo.Separator())
	add("sort.desc", "降序", mygo.MenuItemCheckbox, func(ts *tabState) { ts.sort.Descending = !ts.sort.Descending })
	return out
}

func (a *app) modalOpen() bool { return a.helpOpen || a.dlgOpen }

// menuChecks is what the check marks should show.
func (a *app) menuChecks() map[string]bool {
	return map[string]bool{
		"layout.dual":    a.layout == layoutDual,
		"layout.columns": a.layout == layoutColumns, "layout.rows": a.layout == layoutRows,
		"preview": a.showPreview, "sidebar": a.sb.show, "shelf": a.sh.show,
		"hidden":    a.tab().t.ShowHidden,
		"sort.name": a.tab().sort.Column == "name", "sort.size": a.tab().sort.Column == "size",
		"sort.date": a.tab().sort.Column == "date", "sort.kind": a.tab().sort.Column == "kind",
		"sort.desc": a.tab().sort.Descending,
	}
}

// syncMenu brings the menu bar in line with the app, changing only what
// changed. While a dialog or the cheat sheet is open the app's items are
// disabled, so their keys reach the dialog (recording a shortcut too).
func (a *app) syncMenu() {
	if a.menu.items == nil {
		return
	}
	if sig := a.agentSig(); sig != a.menu.agents {
		a.menu.agents = sig // the bar is rebuilt, then
		a.applyMenu()
		a.menu.modal = false // the new items start enabled
	}
	if m := a.modalOpen(); m != a.menu.modal {
		a.menu.modal = m
		for _, it := range a.menu.items {
			it.SetEnabled(!m)
		}
	}
	for id, on := range a.menuChecks() {
		if it := a.menu.items[id]; it != nil {
			if was, ok := a.menu.checked[id]; !ok || was != on || it.IsChecked() != on {
				a.menu.checked[id] = on
				it.SetChecked(on)
			}
		}
	}
}

// applyMenu installs the menu bar; it runs again when keys change.
func (a *app) applyMenu() {
	if a.setMenu != nil {
		a.setMenu(a.buildMenu())
	}
}
