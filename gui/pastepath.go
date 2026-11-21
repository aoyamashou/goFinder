package gui

import (
	"path/filepath"

	"gofinder/internal/vfs"
)

// pastePath goes to the path on the clipboard, as ⌘V or v with a path
// copied in another app: a folder is entered, a file shown in its folder.
// Items copied in goFinder are pasted instead unless the clipboard changed
// since, so a path copied before them, or with c c, does not take over.
func (a *app) pastePath(ts *tabState) bool {
	text := a.svc.ReadClipboard()
	if len(a.clip) > 0 && (text == a.clipText || text == a.copied) {
		return false
	}
	p, ok := vfs.PastedPath(text)
	if !ok {
		return false
	}
	if _, err := vfs.Stat(p); err != nil {
		if len(a.clip) > 0 {
			return false // not a place: paste the items
		}
		a.status = "找不到：" + p
		return true
	}
	a.goTyped(ts, p)
	a.feedback("前往 " + filepath.Base(p))
	return true
}
