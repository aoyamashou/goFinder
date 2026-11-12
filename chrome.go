package main

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"

	"gofinder/internal/shellcmd"
	"gofinder/internal/templates"
	"gofinder/internal/vfs"
)

// The window's chrome: the title bar (the Agent button, the workspaces,
// the folder's name) above the panes, the status bar under them, and the
// menu of a column's blank space. There is no toolbar (removed at the
// user's word, 2026-11-12).

// defaultBlankMenu is the menu of a column's blank space when config.json
// does not list one: "@new" is the new-file submenu, "@sort" the sort
// submenu, "-" a separator.
var defaultBlankMenu = []string{
	"@new", "newfolder", "-", "paste", "-", "@agents", "-",
	"@sort", "hidden", "refresh", "-", "copypath",
}

// chromeItems are the blank menu's own items, beside action IDs.
var chromeItems = map[string]bool{"-": true, "@new": true, "@sort": true, agentsItem: true}

// agentsItem is the menu placeholder that expands to "open in terminal"
// and one item per enabled agent, at the top level of the menu.
const agentsItem = "@agents"

// noAgentHint is the status bar's word when no agent is enabled.
const noAgentHint = "没有启用的 Agent，去设置 › Agent 启用"

// actionSymbols are the SF Symbols of the title bar's buttons.
var actionSymbols = map[string]string{"agent": "sparkles", "workspace": "square.on.square"}

// tip is what hovering an action's button says: its name and first key.
func (a *app) tip(id, title string) string {
	if keys := a.km.KeysFor(id); len(keys) > 0 {
		return title + "  " + keys[0].String()
	}
	return title
}

// toolbarTips say what a title bar button does, in a sentence: the tip
// is "name · this · first key". Buttons with none fall back to tip.
var toolbarTips = map[string]string{
	"agent": "在当前文件夹启动 Agent", "workspace": "把当前布局保存为工作区，或切换工作区（1–9、[ ]）",
}

// toolbarTip is the tip of a title bar button: "name · what it does · key".
func (a *app) toolbarTip(id, title string) string {
	desc, ok := toolbarTips[id]
	if !ok {
		return a.tip(id, title)
	}
	tip := title + " · " + desc
	if keys := a.km.KeysFor(id); len(keys) > 0 {
		tip += " · " + keys[0].String()
	}
	return tip
}

// symButton is a toolbar button showing an SF Symbol, with no face until
// hovered; on shows it pressed, for a setting that is on. Without SF
// Symbols it shows its label.
func (a *app) symButton(c *ui.Context, sym, label, tip string, on, disabled bool) ui.Element {
	return a.symButtonTint(c, sym, label, tip, on, disabled, "")
}

// hoverDanger is the tint of a button whose symbol is ink until hovered,
// then danger: the trash button.
const hoverDanger = "@hover-danger"

// symButtonTint is symButton with the symbol in the color token tint
// ("" is ink): the Agent button's three beams are signal.
func (a *app) symButtonTint(c *ui.Context, sym, label, tip string, on, disabled bool, tint string) ui.Element {
	b := ui.ButtonBase(c).Width(toolbarButtonW).Height(toolbarButtonH).
		Radius(px("radius-control")).Label(label).Tooltip(tip).Disabled(disabled)
	switch {
	case on:
		b.Background(tok(c, "surface-200"))
	case !disabled && b.Hovered():
		b.Background(tok(c, "surface-200"))
	}
	col := tok(c, "ink")
	switch {
	case disabled:
		col = tok(c, "muted")
	case tint == hoverDanger:
		if b.Hovered() {
			col = tok(c, "danger")
		}
	case tint != "":
		col = tok(c, tint)
	}
	if !disabled && !c.WindowActive() {
		col = tok(c, "faint") // goFinder is in the background
	}
	if sym == "" || !sfsymbols.Available() {
		return b.Width(0).Padding(0, px("space-2")).TextColor(col).Children(func() { ui.Text(c, label).SingleLine() })
	}
	return b.Draw(func(p *ui.Painter, r ui.Rect) {
		const s = 16
		box := ui.Rect{X: r.X + (r.W-s)/2, Y: r.Y + (r.H-s)/2, W: s, H: s}
		if src, ok := sfsymbols.Image(sym, int(s*max(p.Scale(), 1)+0.5), col, sfsymbols.Options{Natural: true}); ok {
			p.Image(src, box, ui.Contain)
		}
	})
}

// titleBar is the strip under the window controls (c.TitleBar), dragging
// the window: the Agent button, the workspaces' buttons, then the active
// folder's name. Its tips show at once.
func (a *app) titleBar(c *ui.Context) {
	bar := c.TitleBar()
	th := *c.Theme()
	th.TooltipDelay = -1
	ui.Themed(c.Key("titlebar.theme"), &th).Gap(0).Padding(0).Background(ui.Transparent).Shrink(0).Children(func() {
		row := ui.Row(c.Key("titlebar")).Height(max(px("toolbar-height"), bar.Height)).Gap(px("space-2")).
			Padding(px("space-1"), px("space-2")+bar.Right, px("space-1"), px("space-2")+bar.Left).
			AlignItems(ui.Center).Shrink(0).Background(tok(c, "surface-100")).DragWindow()
		a.dbg(c, row, "titlebar")
		row.Children(func() {
			a.agentSplit(c, a.tab())
			a.workspaceButtons(c)
			ui.Spacer(c)
			ui.Text(c, a.windowTitle()).FontSize(12).TextColor(titleColor(c)).SingleLine().Shrink(1).MinWidth(0)
			ui.Spacer(c)
		})
	})
	separatorLine(c.Key("titlebar.sep"))
}

// titleColor is the title strip's text color: brand cyan while goFinder is
// active (signal-text, the cyan that reads on a light bar too); in the
// background gray, black in the light theme.
func titleColor(c *ui.Context) ui.Color {
	switch {
	case c.WindowActive():
		return tok(c, "signal-text")
	case c.Theme().Dark:
		return tok(c, "muted")
	}
	return tok(c, "ink")
}

// windowTitle is the title strip's text: the active folder's name.
func (a *app) windowTitle() string {
	d := a.tab().t.Dir
	if b := filepath.Base(d); b != "" && b != "." && b != "/" {
		return b
	}
	return d
}

// agentSplit is the title bar's Agent button in two segments on one face:
// the three beams and the default agent's name, which opens it here, then
// ▾, which pops up the menu of the enabled agents. Only the segment under
// the pointer is lit.
func (a *app) agentSplit(c *ui.Context, ts *tabState) {
	act, _ := a.km.Action("agent")
	def, has := a.defaultAgentItem()
	name := "Agent"
	tip := "去设置 › Agent 启用"
	if has {
		name = def.Name
		tip = a.toolbarTip("agent", "在这里打开 "+def.Name)
	}
	ui.Row(c.Key("tb:agentsplit")).Height(toolbarButtonH).Gap(0).AlignItems(ui.Center).
		Background(tok(c, "surface-200")).Radius(px("radius-control")).Shrink(0).Children(func() {
		seg := func(b ui.Element, disabled bool) {
			if !disabled && b.Hovered() {
				b.Background(tok(c, "separator"))
			}
		}
		left := ui.ButtonBase(c.Key("tb:agent")).Height(toolbarButtonH).Radius(px("radius-control")).Padding(0, px("space-2")).
			Label(act.Title).Tooltip(tip).Disabled(!has)
		a.dbg(c, left, "tb.agent")
		seg(left, !has)
		col := tok(c, "ink")
		if !has {
			col = tok(c, "muted")
		}
		left.Children(func() {
			ui.Row(c).Gap(px("space-1")).AlignItems(ui.Center).Children(func() {
				if sfsymbols.Available() {
					beams := tok(c, "signal")
					if !has {
						beams = col
					}
					ui.Column(c).Width(16).Height(16).Draw(func(p *ui.Painter, r ui.Rect) {
						if src, ok := sfsymbols.Image(actionSymbols["agent"], int(16*max(p.Scale(), 1)+0.5), beams, sfsymbols.Options{Natural: true}); ok {
							p.Image(src, r, ui.Contain)
						}
					})
				}
				ui.Text(c, name).FontSize(fontBody).SingleLine().TextColor(col)
			})
		})
		if left.Clicked() {
			a.run("agent")
			a.wantFocus = true
		}
		ui.Row(c).Width(1).Height(px("space-3")).Background(tok(c, "separator")).Shrink(0)
		right := ui.ButtonBase(c.Key("tb:agent.more")).Width(toolbarButtonW).Height(toolbarButtonH).Radius(px("radius-control")).
			Label("Agent 菜单").Tooltip("选择其他 Agent")
		seg(right, false)
		right.Menu(func(m *ui.Menu) { a.agentsMenu(m, func() string { return a.agentDir(ts) }, nil) })
		right.Center().Children(func() {
			ui.Text(c, "▾").FontSize(fontBody).SingleLine().TextColor(tok(c, "ink"))
		})
	})
}

// newItems are the new-file menu's items in dir, a folder pane pi shows:
// a blank file, each template, and a folder.
func (a *app) newItems(m *ui.Menu, pi int, dir string) {
	ro := vfs.InZip(dir)
	if m.Item(blankTemplate).Disabled(ro).Chosen() {
		a.newIn(pi, dir, dlgNewFile, blankTemplate)
	}
	for _, name := range a.tplNames {
		if m.Item(name).Disabled(ro).Chosen() {
			a.newIn(pi, dir, dlgNewFile, name)
		}
	}
	m.Separator()
	if a.menuItem(m, "newfolder", "新建文件夹").Disabled(ro).Chosen() {
		a.newIn(pi, dir, dlgNewFolder, "")
	}
}

// sortMenu chooses the tab's sort order.
func (a *app) sortMenu(m *ui.Menu, ts *tabState) {
	for _, s := range sortOrder {
		if m.Item(s.title).Radio(ts.sort.Column == s.col).Chosen() {
			ts.sort.Column = s.col
		}
	}
	m.Separator()
	if m.Item("降序").Checked(ts.sort.Descending).Chosen() {
		ts.sort.Descending = !ts.sort.Descending
	}
}

// newIn asks for the name of a new file (from template tpl) or folder in
// dir, which pane pi shows in one of its columns.
func (a *app) newIn(pi int, dir string, kind int, tpl string) {
	a.active = pi
	ts := a.panes[pi].tab()
	if kind == dlgNewFolder {
		a.openDialog(dlgNewFolder, ts, "未命名文件夹", "")
	} else {
		a.newFileDialog(ts)
		if tpl != blankTemplate {
			a.dlgTpl, a.dlgTplSeen, a.dlgText = tpl, tpl, tpl
		}
	}
	a.dlgDir = dir
}

// loadTemplates lists the templates for the new-file menus, off the main
// thread.
func (a *app) loadTemplates() {
	dir := a.tplDir
	if dir == "" {
		return
	}
	a.async(func() {
		names := templates.List(dir)
		a.post(func() { a.tplNames = names })
	})
}

// blankMenu is the menu of a column's blank space, acting on dir, the
// folder the column shows in pane pi: config.json's blankMenu, or
// defaultBlankMenu.
func (a *app) blankMenu(m *ui.Menu, pi int, dir string) {
	ids := a.cfg.BlankMenu
	if ids == nil {
		ids = defaultBlankMenu
	}
	ts := a.panes[pi].tab()
	ro := vfs.InZip(dir)
	sep, n := false, 0
	hasAgents := slices.Contains(ids, agentsItem)
	for _, id := range ids {
		if id == "-" {
			sep = n > 0
			continue
		}
		if _, ok := a.km.Action(id); !ok && !chromeItems[id] || id == "terminal" && hasAgents {
			continue
		}
		if sep {
			m.Separator()
			sep = false
		}
		n++
		switch id {
		case "@new":
			m.Submenu("新建文件", func(m *ui.Menu) {
				if m.Item(blankTemplate).Disabled(ro).Chosen() {
					a.newIn(pi, dir, dlgNewFile, blankTemplate)
				}
				for _, name := range a.tplNames {
					if m.Item(name).Disabled(ro).Chosen() {
						a.newIn(pi, dir, dlgNewFile, name)
					}
				}
			}).Disabled(ro)
			continue
		case "@sort":
			m.Submenu("排序方式", func(m *ui.Menu) { a.sortMenu(m, ts) })
			continue
		case agentsItem:
			a.agentsMenu(m, func() string { return dir }, func() { a.active = pi; a.openFolderWith("terminal", dir) })
			continue
		}
		it := a.menuItem(m, id, a.actionTitle(id))
		switch id {
		case "newfile", "newfolder":
			it.Disabled(ro)
		case "paste":
			it.Disabled(ro || len(a.clip) == 0)
		case "hidden":
			it.Checked(ts.t.ShowHidden)
		}
		if !it.Chosen() {
			continue
		}
		a.active = pi
		switch id {
		case "newfile":
			a.newIn(pi, dir, dlgNewFile, blankTemplate)
		case "newfolder":
			a.newIn(pi, dir, dlgNewFolder, "")
		case "paste":
			a.paste(ts, dir)
		case "terminal", "editor":
			a.openFolderWith(id, dir)
		case "refresh":
			a.refreshLists()
		case "reveal":
			p := dir
			if archive, _, in := vfs.SplitZip(p); in {
				p = archive
			}
			a.sys.Reveal(p)
		case "copypath":
			a.copyPathText(dir) // the column's folder, not the selection
		default:
			a.run(id)
		}
	}
}

// openFolderWith opens dir in the terminal or the editor of config.json.
func (a *app) openFolderWith(id, dir string) {
	appName := a.cfg.Terminal
	if id == "editor" {
		appName = a.cfg.Editor
	}
	dir = vfs.RealDir(dir)
	a.async(func() {
		if err := a.sys.OpenWith(appName, dir); err != nil {
			a.post(func() { a.status = fmt.Sprintf("无法用 %s 打开（⌘, 选择其他应用）", appName) })
		}
	})
}

// fallbackTerminal runs agents when the chosen terminal cannot run a
// .command script.
const fallbackTerminal = "Terminal"

// loginShell is the user's login shell, for the script's first line.
func loginShell() string {
	if sh := os.Getenv("SHELL"); strings.HasPrefix(sh, "/") && !strings.ContainsAny(sh, " \t\n") {
		return sh
	}
	return "/bin/zsh"
}

// agentScript is the .command script that runs cmd, the user's command
// line as typed, in dir, quoted for sh. A login shell runs it, so that
// ~/.local/bin and the like are in PATH.
func agentScript(shell, dir, cmd string) string {
	return "#!" + shell + " -l\ncd " + shellcmd.Quote(dir) + " && exec " + cmd + "\n"
}

// agentDir is the folder an agent opens in: the folder under the cursor,
// else the folder of the list (a file's own folder).
func (a *app) agentDir(ts *tabState) string {
	if e, ok := ts.cursorEntry(); ok && e.IsDir {
		return e.Path
	}
	return ts.t.Dir
}

// agentsMenu adds "open in terminal" (term, or nothing when nil) and one
// item per enabled agent, each with its symbol; the default agent shows
// the key of the agent action. dir is read when an item is chosen.
func (a *app) agentsMenu(m *ui.Menu, dir func() string, term func()) {
	if term != nil && a.menuItem(m, "terminal", a.actionTitle("terminal")).Chosen() {
		term()
	}
	def, hasDef := a.defaultAgentItem()
	for _, ag := range a.enabledAgents() {
		title := "在这里打开 " + ag.Name
		var it *ui.MenuItem
		if hasDef && ag.ID == def.ID {
			it = a.menuItem(m, "agent", title)
		} else {
			it = m.Item(title)
		}
		if sfsymbols.Available() && ag.Symbol != "" {
			it.Content(func(x ui.MenuItemContent) {
				ui.Row(x.Context).Gap(6).AlignItems(ui.Center).Children(func() {
					sfsymbols.Symbol(x.Context, ag.Symbol).Color(tok(x.Context, "signal"))
					ui.Text(x.Context, title).SingleLine()
				})
			})
		}
		if it.Chosen() {
			a.openAgent(ag, dir())
		}
	}
}

// openAgent runs the agent's command in dir, in the terminal of
// config.json: it writes a .command script to the cache folder, one per
// folder and agent and rewritten each time, and opens it with the
// terminal, or with Terminal.app when that one cannot run scripts.
func (a *app) openAgent(ag agentItem, dir string) {
	dir = vfs.RealDir(dir)
	cmd := strings.TrimSpace(ag.Command)
	if cmd == "" {
		a.status = noAgentHint
		return
	}
	term := a.cfg.Terminal
	if term == "" {
		term = fallbackTerminal
	}
	base := a.cacheDir
	if base == "" {
		base = os.TempDir()
	}
	h := fnv.New64a()
	h.Write([]byte(dir + "\x00" + ag.ID))
	script := filepath.Join(base, "agent", fmt.Sprintf("%x.command", h.Sum64()))
	body := agentScript(loginShell(), dir, cmd)
	name := ag.Name
	a.async(func() {
		err := os.MkdirAll(filepath.Dir(script), 0o755)
		if err == nil {
			err = os.WriteFile(script, []byte(body), 0o755)
		}
		if err == nil {
			err = os.Chmod(script, 0o755) // an older script keeps its mode
		}
		use := term
		if err == nil && !a.sys.RunsScripts(term) {
			use = fallbackTerminal
		}
		if err == nil {
			err = a.sys.OpenWith(use, script)
		}
		a.post(func() {
			switch {
			case err != nil:
				a.status = "无法打开 Agent：" + err.Error()
			case use != term:
				a.status = fmt.Sprintf("%s 不能运行 .command 脚本，已改用 %s 打开 %s", term, use, name)
			default:
				a.status = fmt.Sprintf("已在 %s 中运行 %s", use, cmd)
			}
		})
	})
}

// layoutButtons are the status bar's view switch: action, symbol, label.
var layoutButtons = []struct{ id, sym, label string }{
	{"layout.columns", "rectangle.split.3x1", "三列"},
	{"layout.dual", "rectangle.split.2x1", "左右"},
	{"layout.rows", "rectangle.split.1x2", "上下"},
}

// statusBar is two rows of statusbar-row each, on surface-100 with a
// separator between. Above: the last message (a signal dot before it
// when it reports something done), the filter, then the font notice, the
// view switch and the inspector. Below: the shortcut hints.
func (a *app) statusBar(c *ui.Context, t *ui.Theme) {
	ts := a.tab()
	sbar := ui.Column(c.Key("statusbar")).Gap(0).Shrink(0).Background(tok(c, "surface-100"))
	sbar.Children(func() {
		separatorLine(c.Key("sb:edge"))
		ui.Row(c.Key("sb:top")).Height(px("statusbar-row")).Gap(px("space-3")).Padding(0, px("space-2")).AlignItems(ui.Center).Children(func() {
			if a.debugOn {
				// 点一下关闭 Debug；标记本身带 64 号。
				tag := a.flatButton(c.Key("sb:debug"), "Debug", "点击关闭 Debug 编号", false).TextColor(tok(c, "signal-text")).Shrink(0)
				a.dbg(c, tag, "status.debug")
				if tag.Clicked() {
					a.debugOn = false
				}
			}
			msg := ui.Row(c.Key("sb:msg")).Gap(px("space-1.5")).AlignItems(ui.Center).Grow(1).Shrink(1).MinWidth(0)
			msg.Children(func() {
				if ts.t.Filter != "" {
					uiCaption(c.Key("sb:filter"), "过滤："+ts.t.Filter, "signal-text").SingleLine().Shrink(0)
					if a.status != "" {
						uiCaption(c.Key("sb:dot2"), "·", "muted").Shrink(0)
					}
				}
				if a.status == "" {
					return
				}
				if statusIsDone(a.status) {
					ui.Row(c.Key("sb:done")).Size(doneDotSize, doneDotSize).Shrink(0).
						Radius(px("radius-pill")).Background(tok(c, "signal"))
				}
				uiCaption(c.Key("sb:text"), a.status, "muted").SingleLine().Shrink(1).MinWidth(0)
			})
			a.dbg(c, msg, "status.msg")
			uiCaption(c.Key("sb:start"), a.startLabel(), "muted").SingleLine().Shrink(1).MinWidth(0).Tooltip(a.tip("go.start", "按此键回到起点"))
			a.fontNotice(c)
			layoutSel := 0
			for i, l := range layoutButtons {
				if l.id == layoutIDs[a.layout] {
					layoutSel = i
				}
			}
			labels := make([]string, len(layoutButtons))
			for i, l := range layoutButtons {
				labels[i] = l.label
			}
			i := segmentedWith(c.Key("sb:layout"), &layoutSel, "视图", func(t ui.Element) { a.dbg(c, t, "status.layout") }, labels...)
			if i >= 0 {
				a.run(layoutButtons[i].id)
			}
			act, _ := a.km.Action("preview")
			pv := a.symButton(c.Key("sb:preview"), "sidebar.right", act.Title, a.tip("preview", act.Title), a.showPreview, a.visiblePanes() > 1)
			a.dbg(c, pv, "status.insp")
			if pv.Clicked() {
				a.run("preview")
			}
		})
		separatorLine(c.Key("sb:mid"))
		a.dbg(c, ui.Row(c.Key("sb:hints")).Height(px("statusbar-row")).Padding(0, px("space-2")).AlignItems(ui.Center).Children(func() { a.statusHints(c, 0) }), "status.hints")
		ui.Row(c.Key("sb:hints2")).Height(px("statusbar-row")).Padding(0, px("space-2")).AlignItems(ui.Center).Children(func() { a.statusHints(c, 1) })
	})
}

// failWords mark a status sentence that reports a failure or a refusal,
// not something done: it gets no "done" dot.
var failWords = []string{"无法", "失败", "不能", "没有", "错误", "只读", "放进它自己", "⚠"}

// statusIsDone tells whether the status sentence reports something done.
func statusIsDone(s string) bool {
	if s == "" {
		return false
	}
	for _, w := range failWords {
		if strings.Contains(s, w) {
			return false
		}
	}
	return true
}

// separatorLine is the app's one line: 1 DIP of separator.
func separatorLine(c *ui.Context) {
	ui.Row(c).Height(1).Shrink(0).Background(tok(c, "separator"))
}

// layoutIDs are the actions choosing each layout.
var layoutIDs = map[int]string{layoutColumns: "layout.columns", layoutDual: "layout.dual", layoutRows: "layout.rows"}

// chromeWarnings says which items of config.json's blankMenu are unknown.
func chromeWarnings(cfg config, known func(string) bool) []string {
	var warns []string
	for _, list := range []struct {
		name string
		ids  []string
	}{{"blankMenu", cfg.BlankMenu}} {
		for _, id := range list.ids {
			if !known(id) && !chromeItems[id] {
				warns = append(warns, fmt.Sprintf("config.json %s：未知项 %q，已忽略", list.name, id))
			}
		}
	}
	return slices.Clip(warns)
}
