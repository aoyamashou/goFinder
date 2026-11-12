package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/vfs"
)

// startHereItem is a folder's 启动时从这里开始, ticked on the folder
// chosen: choosing it sets the pane's start folder (config.json's
// startDir, startDir2 for the second pane), and choosing it again on that
// folder clears it.
func (a *app) startHereItem(m *ui.Menu, pi int, e vfs.Entry) {
	if !e.IsDir || vfs.InZip(e.Path) {
		return
	}
	on := *a.startDirOf(pi) == e.Path
	m.Separator()
	if m.Item("设为起点").Checked(on).Chosen() {
		a.setStartDir(pi, e.Path, !on)
	}
}

// startDirOf is pane pi's start folder in the config: the panes of the
// dual and rows layouts each keep their own.
func (a *app) startDirOf(pi int) *string {
	if pi == 1 {
		return &a.cfg.StartDir2
	}
	return &a.cfg.StartDir
}

// setStartDir makes dir the folder pane pi opens in, or, with on false,
// goes back to Finder's new-window place; it saves config.json.
func (a *app) setStartDir(pi int, dir string, on bool) {
	if on {
		*a.startDirOf(pi) = dir
		a.status = "已把 " + filepath.Base(dir) + " 设为起点"
	} else {
		*a.startDirOf(pi) = ""
		a.status = "已取消起点"
	}
	a.saveConfig()
}

// isDir reports whether d names a folder that is there.
func isDir(d string) bool {
	if d == "" {
		return false
	}
	st, err := os.Stat(d)
	return err == nil && st.IsDir()
}

// startLabel is where ` goes, at the status bar's end: "起点：~", or each
// pane's, "左：… 右：…" side by side and "上：… 下：…" one above the
// other. A pane with none set starts at home.
func (a *app) startLabel() string {
	home := a.sb.home
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	short := func(pi int) string {
		d := *a.startDirOf(pi)
		switch {
		case d == "" || d == home:
			return "~"
		case home != "" && strings.HasPrefix(d, home+"/"):
			return "~" + d[len(home):]
		}
		return d
	}
	switch a.layout {
	case layoutDual:
		return "左：" + short(0) + "　右：" + short(1)
	case layoutRows:
		return "上：" + short(0) + "　下：" + short(1)
	}
	return "起点：" + short(0)
}
