package main

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"gofinder/internal/fsops"
	"gofinder/internal/shelf"
	"gofinder/internal/vfs"
	"gofinder/internal/workspace"
)

// ---- Shelf ----

// shelfState is the shelf on the right: paths set aside to copy or move
// into another folder later. It is saved to shelf.json.
type shelfState struct {
	show  bool
	items []string
	file  string // shelf.json; "" saves nothing
}

func (a *app) loadShelf() {
	f := a.sh.file
	if f == "" {
		return
	}
	a.async(func() {
		items, err := shelf.File{Path: f}.Load()
		items = shelf.Existing(items) // drop what was deleted meanwhile
		a.post(func() {
			if err != nil {
				a.status = "无法读取暂存架：" + err.Error()
				return
			}
			a.sh.items, _ = shelf.Add(items, a.sh.items...)
		})
	})
}

func (a *app) saveShelf() {
	if a.sh.file == "" {
		return
	}
	f, items := a.sh.file, slices.Clone(a.sh.items)
	a.async(func() {
		if err := (shelf.File{Path: f}).Save(items); err != nil {
			a.post(func() { a.status = "无法保存暂存架：" + err.Error() })
		}
	})
}

// shelve puts paths on the shelf and shows it.
func (a *app) shelve(ps []string) {
	var ok []string
	for _, p := range ps {
		if !vfs.InZip(p) {
			ok = append(ok, p)
		}
	}
	if len(ok) == 0 {
		if len(ps) > 0 {
			a.status = "压缩包内的项目不能加入暂存架"
		}
		return
	}
	var n int
	a.sh.items, n = shelf.Add(a.sh.items, ok...)
	a.sh.show = true
	a.status = fmt.Sprintf("已加入暂存架 %d 项，共 %d 项", n, len(a.sh.items))
	if n > 0 {
		a.saveShelf()
	}
}

func (a *app) unshelve(ps ...string) {
	var n int
	if a.sh.items, n = shelf.Remove(a.sh.items, ps...); n > 0 {
		a.saveShelf()
	}
}

// shelfTo copies or moves everything on the shelf into the tab's folder.
// Moved items leave the shelf; copied ones stay for the next folder.
func (a *app) shelfTo(ts *tabState, move bool) {
	if len(a.sh.items) == 0 {
		a.status = "暂存架是空的"
		return
	}
	if a.readOnly(ts) {
		return
	}
	srcs, dst := slices.Clone(a.sh.items), ts.t.Dir
	verb := map[bool]string{false: "拷贝", true: "移动"}[move]
	a.status = fmt.Sprintf("正在把暂存架的 %d 项%s到这里…", len(srcs), verb)
	a.async(func() {
		var out []string
		var err error
		if move {
			out, err = fsops.Move(srcs, dst, nil)
		} else {
			out, err = fsops.Copy(srcs, dst, nil)
		}
		var gone []string
		if move {
			left := shelf.Existing(srcs)
			for _, s := range srcs {
				if !slices.Contains(left, s) {
					gone = append(gone, s)
				}
			}
		}
		a.post(func() {
			if err != nil {
				a.status = verb + "失败：" + err.Error()
			} else {
				a.status = fmt.Sprintf("已从暂存架%s %d 项", verb, len(out))
			}
			a.unshelve(gone...)
			if len(out) > 0 && ts.t.Dir == dst {
				ts.t.Focus = filepath.Base(out[len(out)-1])
			}
			a.reloadVisible()
		})
	})
}

func (a *app) shelfView(c *ui.Context) {
	t := c.Theme()
	col := ui.Column(c.Key("shelf")).Width(220).Gap(px("space-1.5")).Padding(px("space-1.5")).
		Radius(px("radius-menu")).Background(tok(c, "surface-100"))
	// Files dragged here, from Finder or the lists, join the shelf; they
	// stay where they are.
	opts := transfer.DropOptions{Formats: []transfer.Format{transfer.FileList}, Operations: transfer.Copy}
	if _, over := ui.DataDragOver(col, opts); over {
		col.Border(borderW, t.Accent)
	}
	if d, ok := ui.DropData(col, opts); ok {
		if files, _ := d.Data.Files(); len(files) > 0 {
			a.shelve(files)
		}
	}
	if files := col.DroppedFiles(); files != nil {
		a.shelve(files)
	}
	defer func() { a.dbg(c, col, "shelf") }()
	col.Children(func() {
		ui.Text(c, fmt.Sprintf("暂存架（%d）", len(a.sh.items))).Bold()
		empty := len(a.sh.items) == 0
		// The three buttons share the row and may shrink: no overflow.
		ui.Row(c).Gap(4).Children(func() {
			if faceButton(c.Key("shc"), "拷贝到此", "ink", empty).Padding(0, px("space-1")).Grow(1).Shrink(1).MinWidth(0).Tooltip(a.keyTip("shelf.copy", "拷贝到当前文件夹")).Clicked() {
				a.run("shelf.copy")
			}
			if faceButton(c.Key("shm"), "移动到此", "ink", empty).Padding(0, px("space-1")).Grow(1).Shrink(1).MinWidth(0).Tooltip(a.keyTip("shelf.move", "移动到当前文件夹")).Clicked() {
				a.run("shelf.move")
			}
			if faceButton(c.Key("shx"), "清空", "ink", empty).Padding(0, px("space-1")).Grow(1).Shrink(1).MinWidth(0).Clicked() {
				a.run("shelf.clear")
			}
		})
		if empty {
			uiCaption(c, a.keyTip("shelf.add", "加入选中项")+"，或把文件从 Finder 拖到这里", "muted")
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			for _, p := range a.sh.items {
				ui.Row(c.Key("sh:" + p)).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Text(c, midTrunc(filepath.Base(p), 20)).SingleLine().Grow(1).Shrink(1).MinWidth(0).Tooltip(p)
					if ui.Button(c.Key("x"), "×").Tooltip("从暂存架移除").Clicked() {
						a.unshelve(p)
					}
				}).ContextMenu(func(m *ui.Menu) {
					if m.Item("在 Finder 中显示").Chosen() {
						a.sys.Reveal(p)
					}
					if m.Item("从暂存架移除").Chosen() {
						a.unshelve(p)
					}
				})
			}
		})
	})
}

// keyTip is "title ⌘⌥S", with the action's first key when it has one.
func (a *app) keyTip(id, title string) string {
	if keys := a.km.KeysFor(id); len(keys) > 0 {
		return keys[0].String() + " " + title
	}
	return title
}

// ---- Workspaces ----

var layoutNames = map[int]string{layoutDual: "dual", layoutColumns: "columns", layoutRows: "rows"}

// layoutOf is the layout a saved name stands for. The single list of
// before is now one pane in three columns.
func layoutOf(name string) int {
	for l, n := range layoutNames {
		if n == name {
			return l
		}
	}
	return layoutColumns // "single", and names unknown
}

// snapshot is the arrangement on screen.
func (a *app) snapshot() workspace.State {
	s := workspace.State{
		Layout: layoutNames[a.layout], Active: a.active,
		ShowPreview: a.showPreview, ShowShelf: a.sh.show,
	}
	for _, p := range a.panes {
		wp := workspace.Pane{Cur: p.cur}
		for _, ts := range p.tabs {
			wp.Tabs = append(wp.Tabs, workspace.Tab{Dir: ts.t.Dir, Sort: ts.sort.Column, Desc: ts.sort.Descending})
		}
		s.Panes = append(s.Panes, wp)
	}
	return s
}

// fixDirs replaces folders that are gone with their nearest existing
// parent. It stats: call it off the main thread.
func fixDirs(s *workspace.State) {
	for pi := range s.Panes {
		for ti := range s.Panes[pi].Tabs {
			d := s.Panes[pi].Tabs[ti].Dir
			for {
				if e, err := vfs.Stat(d); err == nil && e.IsDir {
					break
				}
				parent := filepath.Dir(d)
				if parent == d {
					break
				}
				d = parent
			}
			s.Panes[pi].Tabs[ti].Dir = d
		}
	}
}

// applyState arranges the window as s says. It does not read the
// folders: loadAll does.
func (a *app) applyState(s workspace.State) {
	if !s.Valid() {
		return
	}
	for _, p := range a.panes {
		for _, ts := range p.tabs {
			if ts.stop != nil {
				ts.stop()
			}
		}
	}
	hidden := a.cfg.ShowHidden
	for i := range a.panes {
		wp := s.Panes[min(i, len(s.Panes)-1)]
		p := &paneState{cur: wp.Cur}
		for _, wt := range wp.Tabs {
			ts := newTabState(wt.Dir, hidden)
			if _, ok := sortKeys[wt.Sort]; ok {
				ts.sort = ui.SortOrder{Column: wt.Sort, Descending: wt.Desc}
				ts.t.SetSort(sortKeys[wt.Sort], wt.Desc)
			}
			p.tabs = append(p.tabs, ts)
		}
		a.panes[i] = p
	}
	a.layout = layoutOf(s.Layout)
	a.active = s.Active
	if a.active >= a.visiblePanes() {
		a.active = 0
	}
	a.showPreview, a.sh.show = s.ShowPreview, s.ShowShelf
	a.filterOn = false
	a.wantFocus = true
}

func (a *app) loadAll() {
	for _, p := range a.panes {
		for _, ts := range p.tabs {
			a.load(ts)
		}
	}
}

func (a *app) wsPath(name string) string {
	return workspace.Path(filepath.Join(a.dataDir, "workspaces"), name)
}

// workspaceDialog lists the saved workspaces off the main thread.
func (a *app) workspaceDialog() {
	a.openDialog(dlgWorkspace, nil, "", "")
	a.wsNames, a.wsPick = nil, ""
	if a.dataDir == "" {
		return
	}
	dir := filepath.Join(a.dataDir, "workspaces")
	a.async(func() {
		names := workspace.List(dir)
		a.post(func() {
			a.wsNames = names
			if len(names) > 0 && a.wsPick == "" {
				a.wsPick = names[0]
			}
		})
	})
}

func (a *app) saveWorkspace(name string) {
	name = workspace.CleanName(name)
	if name == "" || a.dataDir == "" {
		a.status = "请输入 Workspace 名称"
		return
	}
	p, s := a.wsPath(name), a.snapshot()
	a.async(func() {
		err := workspace.Write(p, s)
		a.post(func() {
			if err != nil {
				a.status = "无法保存 Workspace：" + err.Error()
			} else {
				a.status = "已保存 Workspace「" + name + "」"
			}
		})
	})
}

func (a *app) restoreWorkspace(name string) {
	if name == "" || a.dataDir == "" {
		return
	}
	p := a.wsPath(name)
	a.async(func() {
		s, err := workspace.Read(p)
		if err == nil {
			fixDirs(&s)
		}
		a.post(func() {
			if err != nil {
				a.status = "无法恢复 Workspace：" + err.Error()
				return
			}
			a.applyState(s)
			a.loadAll()
			a.status = "已恢复 Workspace「" + name + "」"
		})
	})
}

func (a *app) workspaceView(c *ui.Context) {
	t := c.Theme()
	ui.Modal(c, &a.dlgOpen, func() {
		wc := ui.Column(c).Width(380).Gap(10)
		defer func() { a.dbg(c, wc, "dlg.workspace") }()
		wc.Children(func() {
			ui.Text(c, "Workspace").Bold()
			if len(a.wsNames) == 0 {
				ui.Text(c, "还没有保存的 Workspace").FontSize(12).TextColor(t.TextMuted)
			} else {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Select(c.Key("wspick"), &a.wsPick, a.wsNames).Label("Workspace").Grow(1)
					if ui.PrimaryButton(c.Key("wsrestore"), "恢复").Clicked() {
						a.dlgOpen = false
						a.restoreWorkspace(a.wsPick)
					}
				})
			}
			ui.Text(c, "把当前布局和标签页保存为：").FontSize(12)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				in := ui.TextInput(c.Key("wsname"), &a.dlgText).Label("名称").Placeholder("名称").AutoFocus().Grow(1)
				if ui.Button(c.Key("wssave"), "保存").Clicked() || in.Submitted() {
					a.dlgOpen = false
					a.saveWorkspace(a.dlgText)
				}
			})
			ui.Row(c).Justify(ui.End).Children(func() {
				if ui.Button(c.Key("wsclose"), "关闭").Clicked() {
					a.dlgOpen = false
				}
			})
		})
	})
	if !a.dlgOpen {
		a.wantFocus = true
	}
}
