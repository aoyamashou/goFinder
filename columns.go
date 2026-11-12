package main

import (
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/icons"
	"gofinder/internal/keymap"
	"gofinder/internal/pane"
	"gofinder/internal/vfs"
)

// The lists are set in a monospaced font, as Yazi in a terminal: 14pt by
// default (config.json's listFontSize), lines 1.25 high.
const (
	defaultListSize = 14
	listLineHeight  = 1.25
	glyphExtra      = 3   // the glyph is this much bigger than the name
	markW           = 14  // room for a git mark
	minColW         = 120 // the narrowest a column of names gets
	maxColW         = 300 // the widest: longer names end in "…"
)

// listSizes are the sizes the settings offer.
var listSizes = []int{13, 14, 15, 16, 18}

// listSize is the lists' font size: config.json's, 14 when missing or
// not one of listSizes.
func (a *app) listSize() int {
	if slices.Contains(listSizes, a.cfg.ListFontSize) {
		return a.cfg.ListFontSize
	}
	return defaultListSize
}

// rowHeight is a row's height at size: 1.25 lines, at least row-height.
func rowHeight(size int) float32 {
	return max(float32(math.Round(float64(size)*listLineHeight)), px("row-height"))
}

// glyphSize is the font size of a row's glyph.
func glyphSize(size int) int { return size + glyphExtra }

// iconW is the glyph's column at size.
func iconW(size int) float32 { return float32(math.Round(float64(size) * 1.45)) }

// Row metrics come from the design tokens (docs/design-system.md §5).
var (
	rowPadX = px("space-1.5")   // either side of a row
	rowGap  = px("space-1.5")   // between the glyph, the name and the git mark
	borderW = px("focus-width") // the focus border, clear when not focused
)

// listFont is the lists' font: JetBrainsMono Nerd Font Mono, else the
// same font with wider glyphs, else the system's monospaced font.
func (a *app) listFont() string {
	return icons.Font + ", " + icons.FontAlt + ", monospace"
}

// hasFont tells whether text can use the font family. Asking the system
// takes a while the first time: call it off the main thread.
func hasFont(family string) bool {
	for _, f := range ui.FontFamilies() {
		if strings.EqualFold(f.Name, family) {
			return true
		}
	}
	return false
}

// nameCols is how many monospaced columns name takes: wide (CJK) runes
// take two.
func nameCols(name string) int {
	n := 0
	for _, r := range name {
		if r >= 0x1100 && (unicode.Is(unicode.Han, r) || unicode.In(r, unicode.Hangul, unicode.Hiragana, unicode.Katakana) || r >= 0xFF00 && r <= 0xFF60) {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// fitWidth is a column's width for names of at most cols columns of charW
// DIPs: the name, its glyph, a git mark and the padding, within
// minColW..maxColW.
func fitWidth(cols int, charW float32, size int) float32 {
	w := rowChrome(size) + float32(cols)*charW
	return max(minColW, min(maxColW, w))
}

// rowChrome is what a row takes besides the name: borders, padding, the
// glyph and room for a git mark.
func rowChrome(size int) float32 {
	return 2*borderW + 2*rowPadX + iconW(size) + 2*rowGap + markW
}

// nameRoom is how many columns of charW DIPs a name gets in a list w DIPs
// wide; 0 while the list has no width yet, for no limit.
func nameRoom(w, charW float32, size int) int {
	if w <= 0 || charW <= 0 {
		return 0
	}
	return max(1, int((w-rowChrome(size))/charW+0.01))
}

// midTrunc fits name in room columns as Yazi does: a long name keeps its
// extension and loses the middle ("build.gra….kts"); one with no short
// extension loses its end. room 0 is no limit.
func midTrunc(name string, room int) string {
	if room <= 0 || nameCols(name) <= room {
		return name
	}
	if room == 1 {
		return "…"
	}
	tail := ""
	if i := strings.LastIndexByte(name, '.'); i > 0 && len(name)-i <= 12 {
		tail = name[i:]
	}
	if nameCols(tail)+2 > room {
		tail = "" // no room for the extension and a start: cut the end
	}
	budget := room - 1 - nameCols(tail)
	var b strings.Builder
	n := 0
	for _, r := range name {
		w := nameCols(string(r))
		if n+w > budget {
			break
		}
		b.WriteRune(r)
		n += w
	}
	return b.String() + "…" + tail
}

// widthCache keeps the widest name of a listing until it changes.
type widthCache struct {
	first *vfs.Entry
	n     int
	cols  int
}

func (w *widthCache) maxCols(es []vfs.Entry) int {
	var first *vfs.Entry
	if len(es) > 0 {
		first = &es[0]
	}
	if first != w.first || len(es) != w.n {
		w.first, w.n, w.cols = first, len(es), 0
		for _, e := range es {
			w.cols = max(w.cols, nameCols(e.Name)+linkCols(e))
		}
	}
	return w.cols
}

func linkCols(e vfs.Entry) int {
	if e.IsLink {
		return 2 // " ↗"
	}
	return 0
}

// charWidth is the width of a column of the lists' font, measured once.
func (a *app) charWidth(c *ui.Context) float32 {
	key := fmt.Sprintf("%s@%d", a.listFont(), a.listSize())
	if a.charW == 0 || a.charWFont != key {
		w, _ := c.MeasureText(0, ui.Span{Text: "0000000000", Font: a.listFont(), Size: float32(a.listSize())})
		a.charW, a.charWFont = w/10, key
		if a.charW <= 0 {
			a.charW = 8
		}
	}
	return a.charW
}

// kindToken is the type-* color token of a glyph's category.
func kindToken(k icons.Kind) string {
	switch k {
	case icons.KindFolder:
		return "type-folder"
	case icons.KindCode:
		return "type-code"
	case icons.KindImage:
		return "type-image"
	case icons.KindMedia:
		return "type-media"
	case icons.KindArchive:
		return "type-archive"
	case icons.KindExec:
		return "type-exec"
	case icons.KindConfig:
		return "type-config"
	}
	return "type-doc"
}

// fileList is a list in the lists' font, outlined in the accent color
// while it has the keyboard, and the name columns its rows have room for,
// as wide as it was last laid out.
func (a *app) fileList(c *ui.Context, st *ui.ListState, es []vfs.Entry, focused bool) (ui.Element, int) {
	room := nameRoom(st.Bounds(c).W, a.charWidth(c), a.listSize())
	return a.focusBorder(c, ui.List(c, st, len(es)).Grow(1).Padding(2, 0).
		Font(a.listFont()).FontSize(float32(a.listSize())).LineHeight(listLineHeight), focused), room
}

// focusBorder outlines the column that has the keyboard; the others get
// a clear border of the same width, so nothing moves as the focus does.
func (a *app) focusBorder(c *ui.Context, e ui.Element, focused bool) ui.Element {
	e = e.Radius(px("radius-row"))
	switch {
	case focused && !c.WindowActive():
		// goFinder is in the background: the border is gray, as the rest.
		return e.Border(borderW, tok(c, "separator")).FocusRing(false)
	case focused:
		return e.Border(borderW, tok(c, "focus")).FocusRing(false)
	}
	return e.Border(borderW, ui.Color{}).FocusRing(false)
}

// rowLook is how a row is highlighted.
type rowLook int

const (
	lookPlain  rowLook = iota
	lookAccent         // in the accent color, text in AccentText
	lookGray           // in gray, text in its own colors
)

// lookOf highlights the cursor's row and the chosen ones: in the accent
// color while the list has the keyboard in the active window, else in
// gray. The list paints only chosen rows itself, and the cursor's row is
// not chosen after a folder loads, so the row paints its own highlight.
func lookOf(cursor, chosen, focused bool) rowLook {
	switch {
	case !cursor && !chosen:
		return lookPlain
	case focused:
		return lookAccent
	}
	return lookGray
}

// rowColors are the colors of a row's glyph, name and git mark: in the
// accent over the accent, else the type color, ink (faint for a hidden
// name) and the git color. The name is never tinted by its kind.
func rowColors(c *ui.Context, look rowLook, kind icons.Kind, hidden bool, mark string) (glyph, name, git ui.Color) {
	glyph, name, git = tok(c, kindToken(kind)), tok(c, "ink"), gitColor(c, mark)
	if hidden {
		name = tok(c, "faint")
	}
	if look == lookAccent {
		glyph, name, git = tok(c, "selection-ink"), tok(c, "selection-ink"), tok(c, "selection-ink")
	}
	return
}

// rowFill is a row's background: selection while its list has the
// keyboard, selection-inactive when it does not.
func rowFill(c *ui.Context, look rowLook) (ui.Color, bool) {
	switch look {
	case lookAccent:
		return tok(c, "selection"), true
	case lookGray:
		return tok(c, "selection-inactive"), true
	}
	return ui.Color{}, false
}

// fileRow is an entry as Yazi shows it: its glyph, its name, and its git
// mark. A folder under the cursor shows as open. A long name is cut in
// the middle to room columns (midTrunc), 0 for no limit.
func (a *app) fileRow(c *ui.Context, e vfs.Entry, look rowLook, open bool, mark string, room int) ui.Element {
	ic := icons.Of(e.Name, icons.Attr{
		Dir: e.IsDir, Open: open, Link: e.IsLink,
		Exec: e.Mode.IsRegular() && e.Mode&0o111 != 0, Repo: e.IsDir && a.git[e.Path] != nil,
	})
	glyph := ic.Glyph
	gc, text, mc := rowColors(c, look, ic.Kind, ic.Dim, mark)
	if !a.nerd {
		glyph = icons.Fallback(e.IsDir, open)
		if look != lookAccent {
			gc = tok(c, "muted")
		}
	}
	name := e.Name
	if e.IsLink {
		name = midTrunc(name, max(0, room-2)) + " ↗"
	} else {
		name = midTrunc(name, room)
	}
	row := ui.Row(c).Gap(rowGap).Padding(0, rowPadX).Height(rowHeight(a.listSize())).AlignItems(ui.Center).Radius(px("radius-row"))
	if bg, ok := rowFill(c, look); ok {
		row.Background(bg)
	}
	return row.Children(func() {
		ui.Text(c, glyph).TextColor(gc).Width(iconW(a.listSize())).FontSize(float32(glyphSize(a.listSize()))).SingleLine()
		ui.Text(c, name).TextColor(text).SingleLine().Shrink(1)
		if mark != "" {
			ui.Text(c, mark).Bold().TextColor(mc).SingleLine()
		}
	})
}

// columnsView is pane pi in the Yazi layout: the parent folder, the
// current one, and what the cursor is on, under the pane's address bar.
// Every layout shows its panes so. The side columns shrink first when the
// pane is narrow, as each of two panes is.
func (a *app) columnsView(c *ui.Context, pi int) {
	p := a.panes[pi]
	ts := p.tab()
	cw := a.charWidth(c)
	ui.Column(c.Key(p)).Fill().Children(func() {
		a.paneHead(c, pi)
		ui.Row(c).Grow(1).Gap(0).AlignItems(ui.Stretch).Children(func() {
			a.parentView(c, pi, cw)
			a.dbg(c, ui.Column(c).Width(fitWidth(ts.width.maxCols(ts.t.Entries()), cw, a.listSize())).Shrink(1).Children(func() {
				a.paneList(c, pi)
			}), "col.current", pi)
			a.dbg(c, ui.Column(c).Grow(1).Shrink(1).Children(func() { a.fourthView(c, pi) }), "col.preview", pi)
		})
	})
}

// separator is the 1px line between two columns, the only line there is.
func (a *app) separator(c *ui.Context) {
	ui.Column(c).Width(1).Shrink(0).Background(tok(c, "separator"))
}

// parentState is the parent folder listing of the column layout.
type parentState struct {
	dir     string
	hidden  bool
	entries []vfs.Entry
	list    ui.ListState
	sel     int
	seq     int
	width   widthCache
}

func (a *app) parentView(c *ui.Context, pi int, cw float32) {
	p := a.panes[pi]
	ts := p.tab()
	dir := filepath.Dir(ts.t.Dir)
	ps := &p.parent
	if dir == ts.t.Dir {
		a.dbg(c, ui.Text(c, "/").TextColor(tok(c, "muted")).Padding(4, rowPadX).Width(minColW), "col.parent", pi)
		return
	}
	if ps.dir != dir || ps.hidden != ts.t.ShowHidden {
		ps.dir, ps.hidden, ps.entries = dir, ts.t.ShowHidden, nil
		ps.seq++
		seq := ps.seq
		hidden := ts.t.ShowHidden
		a.async(func() {
			es := readListing(dir, hidden)
			a.post(func() {
				if ps.seq == seq {
					ps.entries = es
				}
			})
		})
	}
	ps.list.Selected = &ps.sel
	ps.sel = -1
	base := filepath.Base(ts.t.Dir)
	for i, e := range ps.entries {
		if e.Name == base {
			ps.sel = i
			break
		}
	}
	es := ps.entries
	pw := fitWidth(ps.width.maxCols(es), cw, a.listSize())
	room := nameRoom(pw, cw, a.listSize())
	l := ui.List(c.Key("parent"), &ps.list, len(es)).Width(pw).Shrink(2).
		Padding(2, 0).Font(a.listFont()).FontSize(float32(a.listSize())).LineHeight(listLineHeight).
		Border(borderW, ui.Color{}).FocusRing(false).Rows(func(r ui.ListRow) {
		e := es[r.Index]
		look := a.hotLook(e.Path, lookOf(false, r.Selected(), r.ListFocused() && c.WindowActive()))
		row := a.fileRow(r.Context, e, look, r.Index == ps.sel && e.IsDir, "", room)
		a.dropRow(row, &ps.list, e)
		// A right-click acts on its row, as Finder's does: a command chosen
		// goes up to dir with the cursor on it.
		row.ContextMenu(func(m *ui.Menu) {
			a.hotMenu(e.Path)
			a.contextMenuPre(m, pi, ts, e, func() { a.pvEnterNow(pi, ts, dir, e.Name, es) })
		})
	}).Label("上级文件夹").ContextMenu(func(m *ui.Menu) { a.blankMenu(m, pi, dir) })
	a.acceptDrop(a.dragFiles(l, &ps.list, dir, func(hot string) []string { return []string{hot} }, true), dir)
	a.dbg(c, l, "col.parent", pi)
	if l.Changed() && ps.sel >= 0 && ps.sel < len(es) {
		a.active = pi
		if e := es[ps.sel]; e.IsDir {
			a.navigate(ts, e.Path)
		} else if ts.t.Go(dir) {
			ts.t.Focus = e.Name
			a.afterNav(ts)
		}
	}
}

// readListing reads dir as the side columns show it: folders first, by
// name, hidden items only when asked. It reads the disk.
func readListing(dir string, hidden bool) []vfs.Entry {
	es, _ := vfs.ReadDir(dir)
	out := es[:0]
	for _, e := range es {
		if hidden || !e.Hidden() {
			out = append(out, e)
		}
	}
	pane.SortEntries(out, pane.ByName, false)
	return out
}

// fourthView is the column right of the current one: the folder's items
// under the cursor, the file's preview when it has one (its own, or the
// system's QuickLook thumbnail), else its icon and details.
func (a *app) fourthView(c *ui.Context, pi int) {
	p := a.panes[pi]
	if e, ok := p.tab().cursorEntry(); ok {
		a.previewBody(c, p, e)
	}
}

// previewBody shows the preview of e: a folder's items, a text, an image
// or a thumbnail; else e's icon, name and details.
func (a *app) previewBody(c *ui.Context, p *paneState, e vfs.Entry) {
	muted := tok(c, "muted")
	a.ensurePreview(p, e)
	a.waitThumb(c, p, e)
	pi := slices.Index(a.panes[:], p)
	switch pv := &p.pv; {
	case pv.busy:
	case pv.kind == pvDir:
		es := pv.entries
		if len(es) == 0 {
			a.acceptDrop(ui.Text(c, "空文件夹").TextColor(muted).Padding(4, rowPadX).Grow(1).
				ContextMenu(func(m *ui.Menu) { a.blankMenu(m, pi, e.Path) }), e.Path)
			return
		}
		st := &p.pvList
		room := nameRoom(st.Bounds(c).W, a.charWidth(c), a.listSize())
		st.Selected = &p.pvSel
		p.pvSel = -1
		ts := p.tab()
		l := ui.List(c.Key("children"), st, len(es)).Grow(1).Padding(2, 0).
			Font(a.listFont()).FontSize(float32(a.listSize())).LineHeight(listLineHeight).
			Border(borderW, ui.Color{}).FocusRing(false).Rows(func(r ui.ListRow) {
			row := a.fileRow(r.Context, es[r.Index], a.hotLook(es[r.Index].Path, lookPlain), false, "", room)
			a.dropRow(row, st, es[r.Index])
			ce := es[r.Index]
			row.ContextMenu(func(m *ui.Menu) {
				// A command chosen enters the folder with the cursor on the
				// item, so it acts on it as in the current column.
				a.hotMenu(ce.Path)
				a.contextMenuPre(m, pi, ts, ce, func() { a.pvEnterNow(pi, ts, e.Path, ce.Name, es) })
			})
		}).Label("子项").ContextMenu(func(m *ui.Menu) { a.blankMenu(m, pi, e.Path) })
		a.acceptDrop(a.dragFiles(l, st, e.Path, func(hot string) []string { return []string{hot} }, true), e.Path)
		if l.Changed() && p.pvSel >= 0 {
			p.pvLast = p.pvSel
		}
		switch {
		case l.Submitted():
			p.pvSubAt = time.Now()
			// A double click: its first click is still pending.
			p.pvSeq++
			if i := p.pvLast; i >= 0 && i < len(es) {
				a.active = pi
				a.openEntry(ts, es[i])
			}
		case l.Changed() && time.Since(p.pvSubAt) > 200*time.Millisecond:
			// A click: enter the folder shown, the cursor on the item,
			// unless a second click makes it a double click first.
			if i := p.pvSel; i >= 0 && i < len(es) {
				p.pvSeq++
				seq, dir, name := p.pvSeq, e.Path, es[i].Name
				pvAfter(pvClickDelay, func() {
					a.post(func() {
						if p.pvSeq == seq && ts.t.Dir != dir {
							a.pvEnter(pi, ts, dir, name)
						}
					})
				})
			}
		}
	case pv.kind == pvText:
		ui.Scroll(c.Key("pvtext")).Grow(1).Padding(4, 10).Children(func() {
			ui.Text(c, pv.text).Font(a.listFont()).FontSize(12).LineHeight(listLineHeight)
		})
	case pv.kind == pvImage && pv.bmp != nil:
		ui.Image(c, pv.bmp).Fit(ui.ScaleDown).Grow(1).MaxWidthPercent(100).Padding(6)
	default:
		// No preview, or not yet: the icon, large, and the details.
		ic := icons.Of(e.Name, icons.Attr{Dir: e.IsDir, Link: e.IsLink, Exec: e.Mode.IsRegular() && e.Mode&0o111 != 0})
		ui.Column(c).Grow(1).Padding(16, 10).Gap(6).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			if a.nerd {
				ui.Text(c, ic.Glyph).Font(a.listFont()).FontSize(64).TextColor(tok(c, kindToken(ic.Kind)))
			}
			ui.Text(c, e.Name).Bold()
			ui.Text(c, previewInfo(e)).FontSize(12).TextColor(muted)
			if pv.ql {
				ui.Text(c, "正在生成预览…").FontSize(12).TextColor(muted)
			} else if pv.info != "" {
				ui.Text(c, pv.info).FontSize(12).TextColor(muted)
			}
		})
	}
}

// pvClickDelay is how long a click in the third column waits for a second
// one; pvAfter runs f after it (tests run it by hand).
var pvClickDelay = 250 * time.Millisecond
var pvAfter = func(d time.Duration, f func()) { time.AfterFunc(d, f) }

// pvEnter makes dir the pane's folder, the cursor on its item name.
func (a *app) pvEnter(pi int, ts *tabState, dir, name string) {
	a.active = pi
	if ts.t.Go(dir) {
		ts.t.Focus = name
		a.afterNav(ts)
	}
}

// pvEnterNow is pvEnter with the folder's items at hand, so that the
// cursor is on name at once, for a command about to act on it.
func (a *app) pvEnterNow(pi int, ts *tabState, dir, name string, es []vfs.Entry) {
	if ts.t.Dir == dir {
		ts.cursor = ts.t.Index(name)
		return
	}
	a.active = pi
	if !ts.t.Go(dir) {
		return
	}
	ts.t.SetEntries(slices.Clone(es))
	ts.cursor, ts.loaded = ts.t.Index(name), true
	ts.sel.Clear()
	a.filterOn, a.filterText = false, ""
	a.load(ts)
	a.wantFocus = true
}

// updateFocusZone notes which column has the keyboard: "sidebar", or
// "pane0"/"pane1" for the file list of that pane.
func (a *app) updateFocusZone(c *ui.Context) {
	switch {
	case a.wantSidebar:
		a.focusZone = "sidebar"
	case !a.wantFocus && a.sb.show && a.sb.h.FocusWithin(c):
		a.focusZone = "sidebar"
	default:
		a.focusZone = fmt.Sprintf("pane%d", a.active)
	}
}

// hintLine is the status bar's hints; grouped, as those of a held
// modifier are, each group is led by its name.
func hintLine(hs []keymap.Hint, grouped bool) string {
	var b strings.Builder
	group := ""
	for i, h := range hs {
		switch {
		case grouped && h.Group != group:
			if i > 0 {
				b.WriteString("  ｜ ")
			}
			b.WriteString(h.Group + "：")
			group = h.Group
		case i > 0:
			b.WriteString("   ")
		}
		b.WriteString(h.Keys + " " + h.Title)
	}
	return b.String()
}
