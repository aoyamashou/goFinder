package main

import "fmt"

// killFinderCmd restarts Finder, which a new default file viewer needs.
const killFinderCmd = "killall Finder"

// copyKillFinder puts the restart command on the clipboard; the app never
// runs it.
func (a *app) copyKillFinder() {
	if a.hasSvc {
		a.svc.WriteClipboard(killFinderCmd)
	}
	a.status = "已复制，粘贴到终端运行"
}

// fullDiskURL is System Settings' Full Disk Access pane.
const fullDiskURL = "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"

func (a *app) openFullDiskSettings() {
	a.async(func() {
		if err := a.sys.OpenURL(fullDiskURL); err != nil {
			a.post(func() { a.status = "无法打开系统设置：" + err.Error() })
		}
	})
}

// copyText puts text on the clipboard and says what was copied, by its
// label ("已复制 路径"). It needs the services of a frame already drawn.
func (a *app) copyText(label, text string) {
	if a.hasSvc {
		a.svc.WriteClipboard(text)
	}
	a.status = fmt.Sprintf("已复制 %s", label)
}
