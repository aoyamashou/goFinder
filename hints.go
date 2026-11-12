package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/icons"
	"gofinder/internal/keymap"
)

// hintTitles are shorter titles for the status bar's hints.
var hintTitles = map[string]string{
	"filter": "过滤", "copy": "拷贝", "copypath": "拷贝路径", "cut": "剪切", "paste": "粘贴",
	"rename": "重命名", "trash": "删除", "newfolder": "新文件夹", "newfile": "新文件",
	"quicklook": "预览", "help": "帮助", "hidden": "隐藏文件", "sortmenu": "排序", "addr": "前往", "settings": "设置",
	"agent": "启动智能体", "go.start": "回到起点",
}

// hintsMax is how many hints a held modifier shows before "… ? 全部".
const hintsMax = 14

// keycapFont is the keycap's family: the lists' font, as the tokens' mono.
const keycapFont = icons.Font + ", " + icons.FontAlt + ", monospace"

// keycap draws one key as a small rounded cap. It is in the system font,
// whose ⇧⌘⌥⌃ have their own widths: the mono font squeezes each into one
// cell, and "⇧⌘H" ran together.
func keycap(c *ui.Context, key string) ui.Element {
	return ui.Text(c, key).FontSize(fontKeycap).FontWeight(weightKeycap).LetterSpacing(1).SingleLine().
		Padding(px("space-0.5"), px("space-1.5")).Radius(px("radius-pill")).
		Background(tok(c, "surface-200")).TextColor(tok(c, "ink"))
}

// keycaps draws a binding's caps: "n n" is two caps, "⇧⌘H" one.
func keycaps(c *ui.Context, keys string) {
	for _, k := range strings.Fields(keys) {
		keycap(c, k)
	}
}

// keyGroups draws an action's bindings: the caps of one binding ("n n")
// close together, the bindings apart with a muted "/" between them, so
// "c" and "⌘C" read as two ways to press, not three keys.
func keyGroups(c *ui.Context, chords []keymap.Chord) {
	ui.Row(c).Gap(px("space-3")).AlignItems(ui.Center).Shrink(0).Children(func() {
		for i, ch := range chords {
			if i > 0 {
				ui.Text(c, "/").FontSize(fontCaption).SingleLine().TextColor(tok(c, "muted"))
			}
			ui.Row(c).Gap(px("space-0.5")).AlignItems(ui.Center).Shrink(0).Children(func() {
				keycaps(c, ch.String())
			})
		}
	})
}

// hintItem is a binding's caps and what it does.
func hintItem(c *ui.Context, keys, title string) {
	ui.Row(c.Key("hint:" + keys + title)).Gap(px("space-1")).AlignItems(ui.Center).Shrink(0).Children(func() {
		keycaps(c, keys)
		uiCaption(c, title, "muted").SingleLine()
	})
}

// statusHints is the middle of the status bar: the common keys; while a
// modifier is held, every key with exactly it, by group, cut short where
// there is no room; while the filter types, its query and keys.
func (a *app) statusHints(c *ui.Context) {
	ui.Row(c).Grow(1).Shrink(1).Gap(px("space-3")).AlignItems(ui.Center).Clip().Children(func() {
		if a.filterOn {
			uiCaption(c, fmt.Sprintf("过滤「%s」：%d 项", a.filterText, len(a.tab().t.Entries())), "ink").SingleLine().Shrink(0)
			hintItem(c, "↩", "保留")
			hintItem(c, "Esc", "清除")
			return
		}
		held := fromUIMods(c.Modifiers())
		group := ""
		for i, h := range a.km.Hints(held) {
			if held != 0 && i == hintsMax {
				hintItem(c, "?", "… 全部")
				break
			}
			if held != 0 && h.Group != group {
				if group != "" {
					ui.Row(c).Width(1).Height(px("space-3")).Background(tok(c, "separator")).Shrink(0)
				}
				group = h.Group
				uiCaption(c, group, "muted").Bold().SingleLine().Shrink(0)
			}
			title := h.Title
			if s, ok := hintTitles[h.ID]; ok {
				title = s
			}
			hintItem(c, h.Keys, title)
		}
	})
}
