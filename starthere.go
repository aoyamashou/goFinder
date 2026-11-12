package main

import (
	"path/filepath"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/vfs"
)

// startHereItem is a folder's 启动时从这里开始, ticked on the folder
// chosen: choosing it sets config.json's startDir to the folder, and
// choosing it again on that folder clears it (startDir).
func (a *app) startHereItem(m *ui.Menu, e vfs.Entry) {
	if !e.IsDir || vfs.InZip(e.Path) {
		return
	}
	on := a.cfg.StartDir == e.Path
	m.Separator()
	if m.Item("启动时从这里开始").Checked(on).Chosen() {
		a.setStartDir(e.Path, !on)
	}
}

// setStartDir makes dir the folder the window opens in, or, with on
// false, goes back to Finder's new-window place; it saves config.json.
func (a *app) setStartDir(dir string, on bool) {
	if on {
		a.cfg.StartDir = dir
		a.status = "启动时将打开 " + filepath.Base(dir)
	} else {
		a.cfg.StartDir = ""
		a.status = "已取消启动文件夹"
	}
	a.saveConfig()
}
