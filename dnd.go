package main

import (
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"slices"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"gofinder/internal/fsops"
	"gofinder/internal/icons"
	"gofinder/internal/vfs"
)

// Files drag as Finder's do: out of any of the lists to Finder, another
// app or a folder here, and into a folder's row, a column's blank space or
// a name in the path bar. The app never deletes what it drags out: the
// receiver copies or moves, and the lists read their folders again.

// dndState is the drag and drop of files.
type dndState struct {
	// hot is the row under the pointer, in the list hotList: the one a
	// drag starts from.
	hot     string
	hotList *ui.ListState
	// src is what this window drags, while it does.
	src []string
	// offer is the drag over a target, as last seen: the modifier keys
	// held and the operations its source allows.
	offer transfer.Offer
	// The hover's outline: the focus border's accent, or Danger where
	// the files cannot go.
	accent, danger ui.Color
	// preview is the image under the pointer while files drag: their
	// glyph and name, drawn as the drag starts.
	preview *image.RGBA
}

// dragPreview is the drag's image, read as the drag starts, after the
// files it shows are known: DragOptions are given before.
type dragPreview struct{ d *dndState }

func (p dragPreview) ColorModel() color.Model { return color.RGBAModel }
func (p dragPreview) Bounds() image.Rectangle {
	if p.d.preview == nil {
		return image.Rect(0, 0, 1, 1)
	}
	return p.d.preview.Bounds()
}
func (p dragPreview) At(x, y int) color.Color {
	if p.d.preview == nil {
		return color.RGBA{}
	}
	return p.d.preview.At(x, y)
}

// renderPreview draws the first file's glyph and name, and how many more
// there are, as a chosen row.
func (a *app) renderPreview(srcs []string) *image.RGBA {
	name := filepath.Base(srcs[0])
	if n := len(srcs); n > 1 {
		name += fmt.Sprintf(" 等 %d 项", n)
	}
	name = midTrunc(name, 40)
	st, err := os.Stat(srcs[0])
	isDir := err == nil && st.IsDir()
	ic := icons.For(filepath.Base(srcs[0]), isDir, false)
	glyph := ic.Glyph
	if !a.nerd {
		glyph = icons.Fallback(isDir, false)
	}
	font, cw, size := a.listFont(), a.charW, a.listSize()
	if cw <= 0 {
		cw = 8
	}
	w := int(float32(nameCols(name)+2)*cw + 2*rowPadX + iconW(size))
	return ui.Render(func(c *ui.Context) {
		t := c.Theme()
		ui.Row(c).Fill().Gap(rowGap).Padding(0, rowPadX).AlignItems(ui.Center).Background(t.Accent).
			Font(font).FontSize(float32(size)).TextColor(t.AccentText).Children(func() {
			ui.Text(c, glyph).Width(iconW(size)).FontSize(float32(glyphSize(size))).SingleLine()
			ui.Text(c, name).SingleLine()
		})
	}, w, int(rowHeight(size)), 1)
}

// fileDrop takes file lists, to copy or to move.
var fileDrop = transfer.DropOptions{Formats: []transfer.Format{transfer.FileList}, Operations: transfer.Copy | transfer.Move}

// acceptDrop makes e take files dropped on it into dir, outlined while
// they are over it. Any element can be a target, as a sidebar's item:
//
//	a.acceptDrop(item, path)
func (a *app) acceptDrop(e ui.Element, dir string) ui.Element {
	opts := fileDrop
	if a.dnd.src != nil {
		// Our own drag: the pointer shows the copy or the move to come.
		if dropOp(a.dnd.src, dir, a.dnd.offer) == fsops.OpMove {
			opts.Operations = transfer.Move
		} else {
			opts.Operations = transfer.Copy
		}
	}
	if offer, over := ui.DataDragOver(e, opts); over {
		a.dnd.offer = offer
		color := a.dnd.accent
		if vfs.InZip(dir) || a.dnd.src != nil && fsops.CheckDrop(a.dnd.src, dir) != nil {
			color = a.dnd.danger
		}
		e.Border(borderW, color).FocusRing(false)
	}
	if d, ok := ui.DropData(e, opts); ok {
		files, _ := d.Data.Files()
		a.dropFiles(files, dir, a.dnd.offer)
		a.dnd.offer = transfer.Offer{}
	}
	// Files a drop of the older kind brings, as ui.Tester's DropFiles.
	if files := e.DroppedFiles(); files != nil {
		a.dropFiles(files, dir, transfer.Offer{})
	}
	return e
}

// dropOp is what files dropped in dir do, by Finder's rule, the modifier
// keys held, and what the drag's source allows.
func dropOp(srcs []string, dir string, offer transfer.Offer) fsops.Op {
	mod := fsops.DropAuto
	switch offer.Suggested {
	case transfer.Copy:
		mod = fsops.DropCopy
	case transfer.Move:
		mod = fsops.DropMove
	}
	op := fsops.DropOp(srcs, dir, mod)
	if ops := offer.Operations; ops != 0 {
		if op == fsops.OpMove && ops&transfer.Move == 0 {
			op = fsops.OpCopy
		} else if op == fsops.OpCopy && ops&transfer.Copy == 0 && ops&transfer.Move != 0 {
			op = fsops.OpMove
		}
	}
	return op
}

// dropFiles copies or moves srcs into dir, off the main thread. The
// keyboard's focus and the cursors stay where they are.
func (a *app) dropFiles(srcs []string, dir string, offer transfer.Offer) {
	if len(srcs) == 0 {
		return
	}
	if err := fsops.CheckDrop(srcs, dir); err != nil {
		if errors.Is(err, vfs.ErrReadOnly) {
			a.status = "压缩包内只读：不能拖进去"
		} else {
			a.status = err.Error()
		}
		return
	}
	move := dropOp(srcs, dir, offer) == fsops.OpMove
	if move && !slices.ContainsFunc(srcs, func(s string) bool { return filepath.Dir(s) != filepath.Clean(dir) }) {
		return // already there
	}
	verb := "复制"
	if move {
		verb = "移动"
	}
	a.status = fmt.Sprintf("正在%s %d 项…", verb, len(srcs))
	a.async(func() {
		prog := func(done, total int, _ string) {
			a.post(func() { a.status = fmt.Sprintf("正在%s %d/%d", verb, done, total) })
		}
		var out []string
		var err error
		if move {
			out, err = fsops.Move(srcs, dir, prog)
		} else {
			out, err = fsops.Copy(srcs, dir, prog)
		}
		a.post(func() {
			if err != nil {
				a.status = verb + "失败：" + err.Error()
			} else {
				a.status = fmt.Sprintf("已%s %d 项到 %s", verb, len(out), filepath.Base(dir))
			}
			a.refreshLists()
		})
	})
}

// refreshLists reads the folders on screen again: the panes, the parent
// column and the preview.
func (a *app) refreshLists() {
	a.reloadVisible()
	for _, p := range a.panes {
		p.parent.dir = ""
		p.pv.path = ""
	}
}

// hotRow notes the row r under the pointer, of path in the list st: a
// drag from st starts from it.
func (a *app) hotRow(r ui.Element, st *ui.ListState, path string) {
	if r.Hovered() {
		a.dnd.hot, a.dnd.hotList = path, st
	}
}

// dragFiles makes the list l, showing dir, drag files out, from the row
// pressed (hotRow): pick gives what that row drags, as the chosen rows
// when it is one of them. Inside an archive the items extract to the
// cache, and only copy. refocus gives the keyboard back to the files
// after the drag, for a side column the press took it to.
func (a *app) dragFiles(l ui.Element, st *ui.ListState, dir string, pick func(hot string) []string, refocus bool) ui.Element {
	ops := transfer.Copy | transfer.Move
	if vfs.InZip(dir) {
		ops = transfer.Copy
	}
	return l.DragDataFrom(func() transfer.Data {
		if a.dnd.hotList != st || a.dnd.hot == "" {
			return transfer.Data{}
		}
		srcs := pick(a.dnd.hot)
		if ops == transfer.Copy {
			out, err := a.extractForDrag(srcs)
			if err != nil {
				a.status = "无法解压：" + err.Error()
				return transfer.Data{}
			}
			srcs = out
		}
		data, err := transfer.FileData(srcs...)
		if err != nil {
			return transfer.Data{}
		}
		a.dnd.src = srcs
		a.dnd.preview = a.renderPreview(srcs)
		return data
	}, transfer.DragOptions{
		Operations: ops,
		Preview:    dragPreview{&a.dnd},
		Hotspot:    image.Pt(int(rowPadX+iconW(a.listSize())/2), int(rowHeight(a.listSize())/2)),
		Done: func(r transfer.Result) {
			a.dnd.src, a.dnd.offer, a.dnd.preview = nil, transfer.Offer{}, nil
			if refocus {
				a.wantFocus = true
			}
			if r.Operation != transfer.None {
				a.refreshLists()
			}
		},
	})
}

// extractForDrag extracts archive items to the cache, each archive to a
// folder of its own, afresh, and returns their paths there. It runs as
// the drag starts, on the main thread: the receiver needs the files then.
func (a *app) extractForDrag(srcs []string) ([]string, error) {
	base := a.cacheDir
	if base == "" {
		base = os.TempDir()
	}
	out := make([]string, 0, len(srcs))
	for _, s := range srcs {
		archive, inner, ok := vfs.SplitZip(s)
		if !ok {
			out = append(out, s)
			continue
		}
		h := fnv.New64a()
		h.Write([]byte(archive))
		dst := filepath.Join(base, "drag", fmt.Sprintf("%x", h.Sum64()), filepath.FromSlash(inner))
		os.RemoveAll(dst)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
		if err := vfs.Extract(s, dst); err != nil {
			return nil, err
		}
		out = append(out, dst)
	}
	return out, nil
}

// dropRow makes a list's row of e the start of drags from the list st,
// and, for a folder, a target for files.
func (a *app) dropRow(r ui.Element, st *ui.ListState, e vfs.Entry) {
	a.hotRow(r, st, e.Path)
	if e.IsDir {
		a.acceptDrop(r, e.Path)
	}
}
