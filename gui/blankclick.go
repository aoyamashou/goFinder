package gui

import "github.com/egoist/mygo/ui"

// pointerDown is a press in the file list: on the blank space below the
// rows it chooses nothing, as Finder's does; the cursor stays for the keys.
func (a *app) pointerDown(ts *tabState, ev ui.InputEvent) {
	if ev.Button == 0 && ts.hitAt(ev.X, ev.Y) == "" {
		ts.sel.Clear()
	}
}

// rowHit is where a row was in the last frame.
type rowHit struct {
	path string
	r    ui.Rect
}

// hitAt is the path of the row at (x, y) in the list's box, "" for none.
func (ts *tabState) hitAt(x, y float32) string {
	x, y = x+ts.box.X, y+ts.box.Y
	for _, h := range ts.hits {
		if x >= h.r.X && x < h.r.X+h.r.W && y >= h.r.Y && y < h.r.Y+h.r.H {
			return h.path
		}
	}
	return ""
}
