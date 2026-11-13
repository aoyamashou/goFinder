package gui

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/appinstall"
)

const installTitle = "复制到「应用程序」"
const installUnquarantineTitle = "复制到「应用程序」并解除隔离"

func (a *app) canInstall(path string) bool {
	home := a.sb.home
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return appinstall.Eligible(path, home)
}

func (a *app) installItems(m *ui.Menu, path string) {
	for _, item := range []struct {
		title        string
		unquarantine bool
	}{
		{installTitle, false}, {installUnquarantineTitle, true},
	} {
		if m.Item(item.title).Chosen() {
			a.dropHot()
			a.installApp(path, item.unquarantine)
		}
	}
}

func (a *app) installApp(path string, unquarantine bool) {
	if !a.canInstall(path) {
		a.status = "无法安装：不是待安装的本地应用包"
		return
	}
	if a.installing[path] {
		return
	}
	if a.installing == nil {
		a.installing = make(map[string]bool)
	}
	a.installing[path] = true
	a.status = "正在复制 " + filepath.Base(path) + "…"
	a.async(func() {
		dst, err := a.sys.InstallApp(path, unquarantine)
		a.post(func() {
			delete(a.installing, path)
			if err != nil && dst != "" {
				a.status = "已复制到 " + dst + "；" + err.Error()
			} else if err != nil {
				a.status = "无法安装：" + err.Error()
			} else {
				a.status = "已复制到 " + dst
			}
		})
	})
}
