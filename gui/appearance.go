package gui

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

// appearanceModes are config.json's appearance values, in the settings'
// order; the first is the default.
var appearanceModes = []string{"dark", "light", "system"}

// appearanceIndex is the settings' segment of a config.json appearance
// value ("" and unknown words are dark).
func appearanceIndex(mode string) int {
	for i, m := range appearanceModes {
		if m == mode {
			return i
		}
	}
	return 0
}

// appearanceSettings are the settings' 外观 and 工具栏 rows on the 通用
// page. Both take effect at once and are saved straight away.
func (a *app) appearanceSettings(c *ui.Context) {
	ui.Row(c).Gap(px("space-2")).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "外观").Width(56)
		sel := appearanceIndex(a.cfg.Appearance)
		if i := segmented(c.Key("set.appearance"), &sel, "外观", "深色", "浅色", "跟随系统"); i >= 0 {
			a.cfg.Appearance = appearanceModes[i]
			a.saveConfig()
		}
	})
	ui.Row(c).Gap(px("space-2")).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "列表字号").Width(56)
		sel := slices.Index(listSizes, a.listSize())
		if i := segmented(c.Key("set.listsize"), &sel, "列表字号", "13", "14", "15", "16", "18"); i >= 0 {
			a.cfg.ListFontSize = listSizes[i]
			a.saveConfig()
		}
	})
}
