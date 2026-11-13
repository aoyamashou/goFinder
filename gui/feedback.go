package gui

import (
	"fmt"
	"os"
	"time"

	"github.com/egoist/mygo/ui"
)

// feedback records one message for the next UI build; the window owns timing.
func (a *app) feedback(message string) { a.toast = message }

func (a *app) feedbackView(c *ui.Context) {
	if a.toast != "" {
		c.AddToast(ui.Toast{ID: "shortcut-feedback", Title: a.toast, Timeout: 1500 * time.Millisecond})
		a.toast = ""
	}
	wasShowing := a.feedbackShowing
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		a.feedbackShowing = len(toasts) > 0
		viewport.AlignItems(ui.Center).Padding(0, 0, 3*px("statusbar-row")+px("space-2")+px("space-0.5"), 0)
		for _, t := range toasts {
			toast := ui.ToastBase(c, t)
			toast.Root.Row().PassThrough().Padding(px("space-1.5"), px("space-3")).Radius(px("radius-control")).Background(tok(c, "signal"))
			toast.Root.Children(func() { uiCaption(c, t.Title, "signal-ink").SingleLine().PassThrough() })
		}
	})
	if wasShowing != a.feedbackShowing {
		c.After(0) // refresh the status dot when the toast appears or closes
	}
}

func (a *app) setSort(ts *tabState, col string, descending bool) {
	if ts.sort.Column == col && ts.sort.Descending == descending {
		return
	}
	ts.sort.Column, ts.sort.Descending = col, descending
	title := col
	for _, s := range sortOrder {
		if s.col == col {
			title = s.title
			break
		}
	}
	direction := "升序"
	if descending {
		direction = "降序"
	}
	a.feedback("按" + title + direction)
}

func (a *app) goStart(ts *tabState) {
	target := *a.startDirOf(a.active)
	fallback := false
	if target != "" {
		if fi, err := os.Stat(target); err != nil || !fi.IsDir() {
			target = ""
			fallback = true
		}
	}
	if target == "" {
		var err error
		target, err = os.UserHomeDir()
		if err != nil || target == "" {
			a.feedback("无法回到起点")
			return
		}
	}
	switch {
	case fallback:
		a.feedback("起点不可用，已回到主目录")
	case ts.t.Dir == target:
		a.feedback("已在起点")
	default:
		a.feedback("已回到起点")
	}
	a.navigate(ts, target)
}

func (a *app) copyFeedback(ts *tabState, cut bool) {
	if cut && a.readOnly(ts) {
		a.feedback("压缩包内不能剪切")
		return
	}
	chosen := paths(ts.chosen())
	if len(chosen) == 0 {
		a.feedback("没有选中的项目")
		return
	}
	a.clip, a.clipCut = chosen, cut
	verb := "已复制"
	if cut {
		verb = "已剪切"
	}
	a.status = fmt.Sprintf("%s %d 项，到目标文件夹按 %s 粘贴", verb, len(chosen), a.firstKey("paste"))
	a.feedback(fmt.Sprintf("%s %d 项", verb, len(chosen)))
}
