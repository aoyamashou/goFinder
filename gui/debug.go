package gui

import (
	"fmt"
	"runtime/debug"
	"sort"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Debug 编号：设置 › 关于 里打开后，界面各元素左上角显示固定编号，沟通时
// 说「7 号」即可指到元素。编号表 debugIDs 是唯一来源，docs/debug-ids.md
// 列出全部编号（TestDebugIDsDoc 保证两者一致）。编号一经分配不再改动，
// 新元素只往所在分段的空号里加。

// debugIDs maps an element's name to its number. Segments: 1–19 title bar
// and toolbar, 20–29 sidebar, 30–39 address bar, 40–59 columns, inspector
// and shelf, 60–69 status bar, 70–89 dialogs and overlays.
var debugIDs = map[string]int{
	// 标题栏与工具栏
	"titlebar":     1,
	"tb.agent":     11,
	"tb.workspace": 12,

	// 侧边栏
	"sidebar":      20,
	"sidebar.favs": 21,
	"sidebar.locs": 22,
	"sidebar.mine": 23,

	// 地址栏
	"addr.capsule": 30,
	"addr.crumbs":  31,
	"addr.git":     32,
	"addr.filter":  34,

	// 三列、检查器、暂存架
	"col.parent":     40,
	"col.current":    41,
	"col.preview":    42,
	"inspector":      43,
	"inspector.info": 44,
	"inspector.perm": 45,
	"shelf":          46,

	// 状态栏
	"status.msg":    60,
	"status.hints":  61,
	"status.layout": 62,
	"status.insp":   63,
	"status.debug":  64,

	// 对话框与弹层
	"dlg.help":        70,
	"dlg.set.general": 71,
	"dlg.set.keys":    72,
	"dlg.set.agent":   73,
	"dlg.set.about":   75,
	"dlg.rename":      76,
	"dlg.newfile":     77,
	"dlg.newfolder":   78,
	"dlg.trash":       79,
	"dlg.workspace":   80,
	"menu.context":    81,
}

// debugIDNames lists the table's names by number.
func debugIDNames() []string {
	names := make([]string, 0, len(debugIDs))
	for n := range debugIDs {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return debugIDs[names[i]] < debugIDs[names[j]] })
	return names
}

// dbg puts the number of name on e's top-left corner while Debug is on,
// and returns e. An optional pane index adds the pane's number after it
// (41·2) when two panes are shown. Off, it returns e at once.
//
// The badge is built in an Overlay and attached to e (AttachTo): it leaves
// its parent's layout, so nothing moves, and it is a Text with no handler,
// so a press goes through to what is under it.
func (a *app) dbg(c *ui.Context, e ui.Element, name string, pane ...int) ui.Element {
	if !a.debugOn {
		return e
	}
	id, ok := debugIDs[name]
	if !ok || !e.Valid() {
		return e
	}
	label := fmt.Sprint(id)
	if len(pane) > 0 && a.visiblePanes() > 1 {
		label = fmt.Sprintf("%d·%d", id, pane[0]+1)
	}
	key := "dbg:" + name
	if len(pane) > 0 {
		key += fmt.Sprint(pane[0])
	}
	ui.Overlay(c, func() {
		b := ui.Text(c.Key(key), label).Font("").FontSize(11).TextColor(tok(c, "signal-text")).
			Padding(0, 4).Background(tok(c, "signal-soft")).Radius(px("radius-pill")).
			SingleLine().IgnoreInput(true).HitTest(func(float32, float32, ui.Rect) bool { return false })
		if name == "addr.crumbs" {
			// Inside the capsule, whose own badge sits at the same corner:
			// this one takes the row's right end instead.
			b.AttachTo(e, ui.AnchorTopRight, ui.AnchorBottomRight).Top(4).Left(-2)
			return
		}
		b.AttachTo(e, ui.AnchorTopLeft, ui.AnchorBottomLeft).Top(4).Left(-2)
	})
	return e
}

// The settings' 关于 page.

// mygoVersion is the MyGo module's version as this build was made with.
func mygoVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Path == "github.com/egoist/mygo" {
				if d.Replace != nil && d.Replace.Version != "" {
					return d.Replace.Version
				}
				return d.Version
			}
		}
	}
	return mygo.Version
}

// appVersion is mygo.json's version in a packaged app, else 开发版.
func appVersion() string {
	if v := mygo.App.Version(); v != "" {
		return v
	}
	return "开发版"
}

// buildInfo is the commit this build was made from and its time, as
// "b06db5b · 2026-11-12 10:52"; "" when Go recorded none. Whether the
// tree was clean is left out: untracked files make every build "modified".
func buildInfo() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	var rev, at string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.time":
			at = s.Value
		}
	}
	if rev == "" {
		return ""
	}
	out := rev[:min(7, len(rev))]
	if t, err := time.Parse(time.RFC3339, at); err == nil {
		out += " · " + t.Local().Format("2006-01-02 15:04")
	}
	return out
}
