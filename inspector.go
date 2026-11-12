package main

import (
	"fmt"
	"io/fs"
	"os/user"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/finfo"
	"gofinder/internal/vfs"
)

// The inspector is a narrow fourth column, off by default (the action
// "preview"): the attributes of what the cursor is on, no preview, which
// the third column has.
const (
	inspMaxItems = 100000 // a folder's size is taken from at most this many items
)

// inspState is the inspector's item, read off the main thread.
type inspState struct {
	path string
	mod  time.Time
	seq  int
	busy bool
	info finfo.Info
	err  string

	// A folder's counts: counting until they come.
	stats    finfo.Stats
	counting bool
	stop     *atomic.Bool // ends the count once another item is shown

	// The folded groups, kept as items change: closed, so the zero value
	// is open.
	generalShut, permShut bool
}

// ensureInspect reads e unless it is the item shown, as it was.
func (a *app) ensureInspect(p *paneState, e vfs.Entry) {
	in := &p.insp
	if in.path == e.Path && in.mod.Equal(e.ModTime) {
		return
	}
	if in.stop != nil {
		in.stop.Store(true)
	}
	in.seq++
	seq := in.seq
	stop := new(atomic.Bool)
	*in = inspState{path: e.Path, mod: e.ModTime, seq: seq, busy: true, counting: e.IsDir && !vfs.InZip(e.Path), stop: stop,
		generalShut: in.generalShut, permShut: in.permShut}
	a.async(func() {
		var info finfo.Info
		var err error
		if vfs.InZip(e.Path) {
			info = zipInfo(e)
		} else {
			info, err = finfo.Stat(e.Path)
		}
		a.post(func() {
			if p.insp.seq != seq {
				return
			}
			p.insp.busy, p.insp.info = false, info
			if err != nil {
				p.insp.err, p.insp.counting = err.Error(), false
			}
		})
		if err != nil || !e.IsDir || vfs.InZip(e.Path) {
			return
		}
		st, err := finfo.Count(e.Path, inspMaxItems, stop.Load)
		a.post(func() {
			if p.insp.seq != seq {
				return
			}
			p.insp.counting, p.insp.stats = false, st
			if err != nil {
				p.insp.err = err.Error()
			}
		})
	})
}

// zipInfo is what an archive's listing says of an item in it.
func zipInfo(e vfs.Entry) finfo.Info {
	return finfo.Info{
		Name: e.Name, Path: e.Path, Dir: e.IsDir, Size: e.Size, Modified: e.ModTime, Mode: e.Mode,
		Kind: finfo.Kind(e.Name, e.IsDir, false),
	}
}

// gitWords say a git mark.
var gitWords = map[string]string{
	"M": "已修改", "A": "已添加", "D": "已删除", "R": "已重命名", "C": "已拷贝", "U": "有冲突", "??": "未跟踪", "•": "含改动",
}

// inspectorView is the inspector of the active pane's cursor, or of its
// chosen items when there are several: the name and kind on top, then the
// 通用 and 权限 groups, which fold.
func (a *app) inspectorView(c *ui.Context) {
	p := a.pane()
	ts := p.tab()
	insp := ui.Scroll(c.Key("inspector")).Width(px("inspector-width")).Shrink(0).Background(tok(c, "bg"))
	defer func() { a.dbg(c, insp, "inspector") }()
	insp.Children(func() {
		ui.Column(c).Padding(px("space-3")).Gap(px("space-3")).Children(func() {
			defer a.inspShelfButtons(c)
			if ts.sel.Len() > 1 {
				var size int64
				n := 0
				for _, e := range ts.chosen() {
					n++
					if !e.IsDir {
						size += e.Size
					}
				}
				ui.Text(c, fmt.Sprintf("已选 %d 项", n)).FontSize(fontTitle).FontWeight(weightTitle)
				a.inspRow(c, "大小", "共 "+finfo.Size(size)+"（文件夹不计）")
				return
			}
			e, ok := ts.cursorEntry()
			if !ok {
				uiCaption(c, "没有选中项", "muted")
				return
			}
			a.ensureInspect(p, e)
			in := &p.insp
			if in.busy {
				ui.Text(c, e.Name).FontSize(fontTitle).FontWeight(weightTitle)
				uiCaption(c, "读取中…", "muted")
				return
			}
			if in.err != "" {
				uiCaption(c, in.err, "danger")
			}
			info := in.info
			isDir := info.Dir && !info.Link
			sub := info.Kind
			if !isDir {
				sub += " - " + finfo.Size(info.Size)
			} else if !in.counting && !vfs.InZip(info.Path) {
				sub += " - " + finfo.Size(in.stats.Size)
			}
			ui.Text(c, e.Name).FontSize(fontTitle).FontWeight(weightTitle).Selectable()
			uiCaption(c, sub, "muted")

			editable := !vfs.InZip(info.Path) && !info.Link && ownsFile(info)
			a.inspGroup(c, "通用", &in.generalShut, func() {
				a.inspRow(c, "路径", info.Path)
				if isDir {
					if !vfs.InZip(info.Path) {
						if in.counting {
							a.inspRow(c, "子项", "统计中…")
						} else {
							s := in.stats
							a.inspRow(c, "子项", fmt.Sprintf("%d 项（文件 %d · 文件夹 %d）", s.Items, s.Files, s.Dirs))
							size := finfo.Size(s.Size)
							if s.Capped {
								size = "≥ " + size + fmt.Sprintf("（只统计了 %d 项）", s.Scanned)
							}
							a.inspRow(c, "总大小", size)
						}
					}
				} else {
					a.inspRow(c, "大小", finfo.SizeLong(info.Size))
				}
				a.inspTime(c, "修改时间", info.Modified)
				a.inspTime(c, "创建时间", info.Created)
				a.inspTime(c, "添加时间", info.Added)
				if info.Ext != "" {
					a.inspRow(c, "扩展名", info.Ext)
				}
				if info.Link {
					a.inspRow(c, "链接到", info.Target)
				}
				if !isDir {
					a.inspCheck(c, "隐藏扩展名", info.HideExt, !editable, func(v bool) {
						in.info.HideExt = v
						a.inspWrite(p, info.Path, func() error { return finfo.SetHideExt(info.Path, v) })
					})
				}
				a.inspCheck(c, "隐藏", info.Hidden, !editable, func(v bool) {
					in.info.Hidden = v
					a.inspWrite(p, info.Path, func() error { return finfo.SetHidden(info.Path, v) })
				})
				if m := gitMark(a.gitStatus(ts), ts.gitRoot, e); m != "" {
					a.inspRow(c, "Git", m+" "+gitWords[m])
				}
			})
			a.inspGroup(c, "权限", &in.permShut, func() {
				a.inspPerm(c, p, info, !editable)
				if info.Owner != "" {
					a.inspRow(c, "所有者", info.Owner)
					a.inspRow(c, "组", info.Group)
				}
				a.inspCheck(c, "锁定", info.Locked, !editable, func(v bool) {
					in.info.Locked = v
					a.inspWrite(p, info.Path, func() error { return finfo.SetLocked(info.Path, v) })
				})
			})
		})
	})
}

// me is the user running goFinder, whose files the checkboxes can change.
var me = sync.OnceValue(func() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return ""
})

// ownsFile tells whether the checkboxes can change info's item: it is the
// user's own (root's too).
func ownsFile(info finfo.Info) bool {
	return info.Owner != "" && (info.Owner == me() || me() == "root")
}

// inspGroup is a group's title with a chevron on the right, folding its
// rows, which are drawn while open.
func (a *app) inspGroup(c *ui.Context, title string, shut *bool, rows func()) {
	g := ui.Column(c.Key("ig:" + title)).Gap(0)
	defer func() { a.dbg(c, g, map[string]string{"通用": "inspector.info", "权限": "inspector.perm"}[title]) }()
	g.Children(func() {
		h := ui.Row(c.Key("igh:"+title)).AlignItems(ui.Center).Padding(px("space-0.5"), px("space-1")).Cursor(ui.CursorPointer).
			Radius(px("radius-row")).Label(title)
		if h.Hovered() {
			h.Background(tok(c, "surface-100"))
		}
		h.Children(func() {
			ui.Text(c, title).FontSize(fontBody).FontWeight(weightTitle).Grow(1)
			chev := "▾"
			if *shut {
				chev = "▸"
			}
			ui.Text(c, chev).FontSize(fontBody).TextColor(tok(c, "muted"))
		})
		if h.Clicked() {
			*shut = !*shut
		}
		if !*shut {
			rows()
		}
	})
}

// inspLine is the 1px line between two rows.
func (a *app) inspLine(c *ui.Context) {
	ui.Column(c).Height(1).Shrink(0).Background(tok(c, "separator"))
}

// inspRow is a label in muted, its value on the right; a click copies
// the value as shown, and the hovered row sits on surface-100.
func (a *app) inspRow(c *ui.Context, label, value string) {
	a.inspLine(c)
	row := ui.Row(c.Key("ic:"+label)).Gap(px("space-2")).AlignItems(ui.Center).Padding(px("space-1"), px("space-1")).
		Radius(px("radius-row")).Cursor(ui.CursorPointer).Tooltip("点击复制")
	if row.Hovered() {
		row.Background(tok(c, "surface-100"))
	}
	row.Children(func() {
		ui.Text(c, label).FontSize(fontBody).TextColor(tok(c, "muted")).SingleLine().Shrink(0)
		inspValue(c, value).Grow(1).Shrink(1).TextAlign(ui.End)
	})
	if row.Clicked() {
		a.copyText(label, value)
	}
}

func (a *app) inspTime(c *ui.Context, label string, tm time.Time) {
	if !tm.IsZero() {
		a.inspRow(c, label, tm.Format("2006-01-02 15:04:05"))
	}
}

// inspValue is a row's value: body, digits of one width.
func inspValue(c *ui.Context, value string) ui.Element {
	return ui.Text(c, value).FontSize(fontBody).FontFeatures("tnum").TextColor(tok(c, "ink"))
}

// inspCheck is a label with a check box on the right; fn gets the new
// value. Disabled, it is a plain mark.
func (a *app) inspCheck(c *ui.Context, label string, on, off bool, fn func(bool)) {
	a.inspLine(c)
	ui.Row(c.Key("ik:"+label)).Gap(px("space-2")).AlignItems(ui.Center).Padding(px("space-1"), px("space-1")).Children(func() {
		ui.Text(c, label).FontSize(fontBody).TextColor(tok(c, "muted")).SingleLine().Grow(1)
		v := on
		if ui.Checkbox(c.Key("ikb:"+label), &v, "").Label(label + " 开关").Disabled(off).Changed() {
			fn(v)
		}
	})
}

// permColW is the width of one of the R, W, X columns.
const permColW = 36

// inspPerm is the 所有者/组/其他 × R/W/X grid of check boxes, then the
// 八进制 row: the bits as ls shows them and as chmod takes them.
func (a *app) inspPerm(c *ui.Context, p *paneState, info finfo.Info, off bool) {
	a.inspLine(c)
	ui.Row(c).Padding(px("space-1"), px("space-1")).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1)
		for _, h := range []string{"R", "W", "X"} {
			ui.Text(c, h).FontSize(fontBody).TextColor(tok(c, "muted")).Width(permColW).TextAlign(ui.Center)
		}
	})
	for i, who := range []string{"所有者", "组", "其他"} {
		shift := uint(6 - 3*i)
		a.inspLine(c)
		ui.Row(c.Key("ip:"+who)).Padding(px("space-1"), px("space-1")).AlignItems(ui.Center).Children(func() {
			ui.Text(c, who).FontSize(fontBody).TextColor(tok(c, "muted")).Grow(1)
			for j, bit := range []fs.FileMode{4, 2, 1} {
				mask := bit << shift
				v := info.Mode&mask != 0
				ui.Row(c).Width(permColW).Justify(ui.Center).Children(func() {
					name := who + " " + "RWX"[j:j+1]
					if ui.Checkbox(c.Key("ipb:"+name), &v, "").Label(name).Disabled(off).Changed() {
						mode := info.Mode.Perm() &^ mask
						if v {
							mode |= mask
						}
						p.insp.info.Mode = p.insp.info.Mode&^fs.ModePerm | mode
						a.inspWrite(p, info.Path, func() error { return finfo.SetPerm(info.Path, mode) })
					}
				})
			}
		})
	}
	a.inspLine(c)
	row := ui.Row(c.Key("ic:八进制")).Gap(px("space-2")).AlignItems(ui.Center).Padding(px("space-1"), px("space-1")).
		Radius(px("radius-row")).Cursor(ui.CursorPointer).Tooltip("点击复制")
	row.Children(func() {
		ui.Text(c, "八进制").FontSize(fontBody).TextColor(tok(c, "muted")).Grow(1)
		inspValue(c, info.Perm())
		inspValue(c, info.Octal()).Padding(px("space-0.5"), px("space-2")).Background(tok(c, "surface-100")).Radius(px("radius-row"))
	})
	if row.Clicked() {
		a.copyText("八进制", info.Octal())
	}
}

// inspWrite runs a change of path off the main thread; whatever came of
// it, the attributes are read again, and a failure shows in the status.
func (a *app) inspWrite(p *paneState, path string, op func() error) {
	a.async(func() {
		err := op()
		info, serr := finfo.Stat(path)
		a.post(func() {
			if err != nil {
				a.status = "无法修改：" + err.Error()
			}
			if serr == nil && p.insp.path == path {
				p.insp.info = info
			}
		})
	})
}

// inspShelfButtons are the two shelf buttons at the foot, stacked so they
// fit the narrow column.
func (a *app) inspShelfButtons(c *ui.Context) {
	ui.Column(c).Gap(px("space-1")).Children(func() {
		if faceButton(c.Key("insp-shelf-add"), "加入暂存架", "ink", false).Tooltip(a.keyTip("shelf.add", "加入暂存架")).Clicked() {
			a.run("shelf.add")
		}
		label := "打开暂存架"
		if a.sh.show {
			label = "关闭暂存架"
		}
		if faceButton(c.Key("insp-shelf-toggle"), label, "ink", false).Tooltip(a.keyTip("shelf", "显示/隐藏暂存架")).Clicked() {
			a.run("shelf")
		}
	})
}
