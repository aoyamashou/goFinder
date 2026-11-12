package main

import (
	"slices"
	"strconv"

	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"
)

// The settings' 工具栏 page: the toolbar's items on the left, in order,
// each moved or removed with buttons; what can still be added on the right.
// Every change is written to config.json at once and the toolbar follows.

const (
	pageToolbar   = 3 // the settings page's index
	pageAbout     = 4 // 关于
	itemGap       = "-"
	itemFlex      = "@flex"
	toolbarListH  = 300
	toolbarRowGap = 1
)

// toolbarItemTitle is the name of a toolbar item in the settings.
func (a *app) toolbarItemTitle(id string) string {
	switch id {
	case itemGap:
		return "分组间隔"
	case itemFlex:
		return "推到右侧"
	case "@new":
		return "新建"
	case "@sort":
		return "排序"
	}
	if act, ok := a.km.Action(id); ok {
		return act.Title
	}
	return id
}

// toolbarChoices are the items that can still be added: every action with
// a symbol, "@new" and "@sort", the push to the right, and a gap (always).
func (a *app) toolbarChoices() []string {
	have := a.toolbarIDs()
	var out []string
	add := func(id string) {
		if !slices.Contains(have, id) {
			out = append(out, id)
		}
	}
	for _, id := range []string{"@new", "@sort"} {
		add(id)
	}
	for _, act := range a.km.Actions() {
		if _, ok := actionSymbols[act.ID]; ok {
			add(act.ID)
		}
	}
	add(itemFlex)
	return append(out, itemGap)
}

// setToolbar makes ids the toolbar and writes config.json.
func (a *app) setToolbar(ids []string) {
	a.cfg.Toolbar = append([]string{}, ids...)
	a.saveConfig()
}

// toolbarMove moves item i by d places.
func (a *app) toolbarMove(i, d int) {
	ids := slices.Clone(a.toolbarIDs())
	j := i + d
	if i < 0 || i >= len(ids) || j < 0 || j >= len(ids) {
		return
	}
	ids[i], ids[j] = ids[j], ids[i]
	a.tbSel = j
	a.setToolbar(ids)
}

// toolbarRemove removes item i.
func (a *app) toolbarRemove(i int) {
	ids := slices.Clone(a.toolbarIDs())
	if i < 0 || i >= len(ids) {
		return
	}
	ids = slices.Delete(ids, i, i+1)
	a.tbSel = min(a.tbSel, len(ids)-1)
	a.setToolbar(ids)
}

// toolbarAdd appends id.
func (a *app) toolbarAdd(id string) {
	ids := append(slices.Clone(a.toolbarIDs()), id)
	a.tbSel = len(ids) - 1
	a.setToolbar(ids)
}

// toolbarReset goes back to defaultToolbar.
func (a *app) toolbarReset() {
	a.cfg.Toolbar = nil
	a.tbSel = 0
	a.saveConfig()
}

// toolbarKey is the page's keyboard: ↑↓ choose a row, ⌘↑⌘↓ move it, ⌦ and
// ⌫ remove it.
func (a *app) toolbarKey(ev ui.InputEvent) {
	if ev.Kind != ui.InputKeyDown || a.setPage != pageToolbar {
		return
	}
	n := len(a.toolbarIDs())
	switch {
	case ev.Key == ui.KeyUp && ev.Mods&ui.Cmd != 0:
		a.toolbarMove(a.tbSel, -1)
	case ev.Key == ui.KeyDown && ev.Mods&ui.Cmd != 0:
		a.toolbarMove(a.tbSel, 1)
	case ev.Key == ui.KeyUp && ev.Mods == 0:
		a.tbSel = max(a.tbSel-1, 0)
	case ev.Key == ui.KeyDown && ev.Mods == 0:
		a.tbSel = min(a.tbSel+1, n-1)
	case (ev.Key == ui.KeyDelete || ev.Key == ui.KeyBackspace) && ev.Mods == 0:
		a.toolbarRemove(a.tbSel)
	}
}

// toolbarGlyph is an item's symbol (or nothing) before its name.
func (a *app) toolbarGlyph(c *ui.Context, id string) {
	sym := actionSymbols[id]
	switch id {
	case itemGap:
		sym = "ellipsis"
	case itemFlex:
		sym = "arrow.right.to.line"
	}
	if sym != "" && sfsymbols.Available() {
		sfsymbols.Symbol(c, sym).Color(tok(c, "ink"))
	} else {
		ui.Row(c).Width(16)
	}
}

// toolbarKeysCaption is the item's first key, or "".
func (a *app) toolbarKeysCaption(id string) string {
	if keys := a.km.KeysFor(id); len(keys) > 0 {
		return keys[0].String()
	}
	return ""
}

// toolbarPage draws the page.
func (a *app) toolbarPage(c *ui.Context) {
	ids := a.toolbarIDs()
	a.tbSel = max(min(a.tbSel, len(ids)-1), 0)
	ui.Row(c).Gap(px("space-3")).Children(func() {
		ui.Column(c.Key("tb.cur")).Gap(px("space-1")).Grow(6).Basis(0).Children(func() {
			ui.Text(c, "当前工具栏").Bold()
			ui.Scroll(c).Height(toolbarListH).Background(tok(c, "bg")).Radius(px("radius-control")).Children(func() {
				ui.Column(c).Gap(toolbarRowGap).Padding(px("space-1")).Children(func() {
					if len(ids) == 0 {
						uiCaption(c, "空：工具栏已隐藏", "muted")
					}
					for i, id := range ids {
						a.toolbarRow(c, i, id, len(ids))
					}
				})
			})
		})
		ui.Column(c.Key("tb.add")).Gap(px("space-1")).Grow(5).Basis(0).Children(func() {
			ui.Text(c, "可添加").Bold()
			ui.Scroll(c).Height(toolbarListH).Background(tok(c, "bg")).Radius(px("radius-control")).Children(func() {
				ui.Column(c).Gap(toolbarRowGap).Padding(px("space-1")).Children(func() {
					for _, id := range a.toolbarChoices() {
						ui.Row(c.Key("add:"+id)).Height(toolbarButtonH+2).Gap(px("space-2")).AlignItems(ui.Center).Padding(0, px("space-1")).Children(func() {
							a.toolbarGlyph(c, id)
							ui.Text(c, a.toolbarItemTitle(id)).SingleLine().Grow(1).MinWidth(0).Justify(ui.Start)
							if ui.Button(c.Key("addbtn:"+id), "添加").Clicked() {
								a.toolbarAdd(id)
							}
						})
					}
				})
			})
		})
	})
	uiCaption(c, "↑↓ 选行，⌘↑ ⌘↓ 移动，⌫ 移除；改动立即写入 config.json", "muted")
	ui.Row(c).Gap(px("space-2")).Justify(ui.End).Children(func() {
		if ui.Button(c.Key("tb.reset"), "恢复默认").Disabled(a.cfg.Toolbar == nil).Clicked() {
			a.toolbarReset()
		}
		if ui.PrimaryButton(c.Key("tbdone"), "完成").Clicked() {
			a.dlgOpen = false
		}
	})
}

// toolbarRow is item i of the current toolbar.
func (a *app) toolbarRow(c *ui.Context, i int, id string, n int) {
	sel := i == a.tbSel
	row := ui.Row(c.Key("cur:"+strconv.Itoa(i))).Height(toolbarButtonH+2).Gap(px("space-1")).AlignItems(ui.Center).
		Padding(0, px("space-1")).Radius(px("radius-row"))
	col, capCol := "ink", "muted"
	if sel {
		capCol = "selection-ink"
		row.Background(tok(c, "selection"))
		col = "selection-ink"
	}
	row.Children(func() {
		pick := ui.ButtonBase(c.Key("pick:" + strconv.Itoa(i))).Height(toolbarButtonH).Grow(1).MinWidth(0).Justify(ui.Start).Label(a.toolbarItemTitle(id)).
			Padding(0).TextColor(tok(c, col))
		pick.Children(func() {
			ui.Row(c).Gap(px("space-2")).AlignItems(ui.Center).Children(func() {
				a.toolbarGlyph(c, id)
				ui.Text(c, a.toolbarItemTitle(id)).SingleLine().TextColor(tok(c, col))
				if k := a.toolbarKeysCaption(id); k != "" {
					uiCaption(c, k, capCol)
				}
			})
		})
		if pick.Clicked() {
			a.tbSel = i
		}
		if a.symButton(c.Key("up:"+strconv.Itoa(i)), "chevron.up", "上移", "上移  ⌘↑", false, i == 0).Clicked() {
			a.toolbarMove(i, -1)
		}
		if a.symButton(c.Key("down:"+strconv.Itoa(i)), "chevron.down", "下移", "下移  ⌘↓", false, i == n-1).Clicked() {
			a.toolbarMove(i, 1)
		}
		if a.symButton(c.Key("rm:"+strconv.Itoa(i)), "xmark", "移除", "移除  ⌫", false, false).Clicked() {
			a.toolbarRemove(i)
		}
	})
}
