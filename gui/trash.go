package gui

import (
	"fmt"
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

// trashListMax is how many names the trash confirmation lists.
const trashListMax = 5

// askTrash moves the chosen items to the Trash, first asking when
// config.json's confirmTrash is on (the default). Only d and ⌘⌫ ask:
// dragging to the Trash does not.
func (a *app) askTrash(ts *tabState) {
	if !a.cfg.ConfirmTrash {
		a.trash(ts)
		return
	}
	ps := paths(ts.chosen())
	if len(ps) == 0 {
		return
	}
	a.openDialog(dlgTrash, ts, "", "")
	a.trashPaths, a.trashNoAsk = ps, false
}

// confirmTrash is the confirmation's ↩: it trashes, and with 不再提醒
// ticked turns confirmTrash off in config.json.
func (a *app) confirmTrash() {
	if !a.dlgOpen || a.dlg != dlgTrash {
		return
	}
	a.dlgOpen, a.wantFocus = false, true
	if a.trashNoAsk {
		a.cfg.ConfirmTrash = false
		a.saveConfig()
	}
	if ts := a.dlgTab; ts != nil {
		a.trash(ts)
	}
}

// trashView asks "将 N 个项目移到废纸篓？", listing the first names; ↩
// confirms and Esc cancels.
func (a *app) trashView(c *ui.Context) {
	confirm := false
	dialogModal(c, &a.dlgOpen, func() {
		tc := ui.Column(c).Width(dialogW).Gap(px("space-3"))
		defer func() { a.dbg(c, tc, "dlg.trash") }()
		tc.Children(func() {
			ui.Text(c, fmt.Sprintf("将 %d 个项目移到废纸篓？", len(a.trashPaths))).
				FontSize(fontTitle).FontWeight(weightTitle).TextColor(tok(c, "ink"))
			ui.Column(c).Gap(px("space-0.5")).Children(func() {
				for i, p := range a.trashPaths {
					if i == trashListMax {
						uiCaption(c, fmt.Sprintf("…还有 %d 项", len(a.trashPaths)-trashListMax), "muted")
						break
					}
					ui.Text(c.Key("trash:"+p), filepath.Base(p)).Font(keycapFont).FontSize(fontBody).SingleLine().TextColor(tok(c, "muted"))
				}
			})
			uiCaption(c, "可以在废纸篓中找回", "muted")
			ui.Row(c).Gap(px("space-2")).AlignItems(ui.Center).Children(func() {
				ui.Checkbox(c.Key("trash.noask"), &a.trashNoAsk, "不再提醒").Grow(1)
				if faceButton(c.Key("trash.cancel"), "取消", "ink", false).Clicked() {
					a.dlgOpen = false
				}
				if faceButton(c.Key("trash.ok"), "移到废纸篓", "danger", false).AutoFocus().Clicked() {
					confirm = true
				}
			})
			if c.Shortcut(0, ui.KeyEnter) {
				confirm = true
			}
		})
	})
	if confirm {
		a.confirmTrash()
	}
	if !a.dlgOpen {
		a.wantFocus = true
	}
}
