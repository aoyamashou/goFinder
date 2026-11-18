package gui

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
	_ "golang.org/x/image/bmp"  // thumbnails
	_ "golang.org/x/image/webp" // thumbnails

	"gofinder/internal/vfs"
)

// The gallery (Q) takes the place of the preview column's single preview:
// the folder's images as thumbnails alone, chosen as the list's rows are.
const (
	galCell    = 112 // a cell's least width, and its height
	galPx      = 224 // a thumbnail's longer side, in pixels (2x the cell)
	galMax     = 240 // thumbnails kept; the least recently shown go first
	galLoaders = 3   // thumbnails decoded at once
)

// galleryState is the thumbnails of the gallery: kept by path and
// modification time, so a changed file is drawn again.
type galleryState struct {
	bmp   map[string]*ui.Bitmap
	order []string // by last use, oldest first
	busy  map[string]bool
	fail  map[string]bool
	sem   chan struct{}
}

func thumbKey(e vfs.Entry) string { return e.Path + "\x00" + e.ModTime.UTC().Format(time.RFC3339Nano) }

// qlImageExts are the images the app cannot decode itself but the system's
// QuickLook draws: the gallery asks it for their thumbnails.
var qlImageExts = map[string]bool{
	"heic": true, "heif": true, "tif": true, "tiff": true, "avif": true,
	"psd": true, "ico": true, "icns": true, "raw": true, "cr2": true, "nef": true, "dng": true,
}

// galleryEntries are the images among es, in order.
func galleryEntries(es []vfs.Entry) []vfs.Entry {
	var out []vfs.Entry
	for _, e := range es {
		if e.IsDir || e.Size > imagePreviewMax {
			continue
		}
		if imageExts[e.Ext()] || qlImageExts[e.Ext()] && !vfs.InZip(e.Path) {
			out = append(out, e)
		}
	}
	return out
}

func (g *galleryState) init() {
	if g.bmp == nil {
		g.bmp, g.busy, g.fail = map[string]*ui.Bitmap{}, map[string]bool{}, map[string]bool{}
		g.sem = make(chan struct{}, galLoaders)
	}
}

// drop frees every thumbnail, as the gallery is left.
func (g *galleryState) drop() {
	g.bmp, g.order, g.busy, g.fail = nil, nil, nil, nil
}

// thumb is the thumbnail of e, nil until it is drawn, which it starts to.
func (a *app) thumb(e vfs.Entry) *ui.Bitmap {
	g := &a.gal
	g.init()
	k := thumbKey(e)
	if b, ok := g.bmp[k]; ok {
		if i := slices.Index(g.order, k); i >= 0 && i != len(g.order)-1 {
			g.order = append(slices.Delete(g.order, i, i+1), k)
		}
		return b
	}
	if g.busy[k] || g.fail[k] {
		return nil
	}
	g.busy[k] = true
	a.async(func() {
		g.sem <- struct{}{}
		var small image.Image
		if qlImageExts[e.Ext()] {
			small = a.makeQLThumb(e)
		} else {
			small = makeThumb(e.Path)
		}
		<-g.sem
		a.post(func() {
			if g.busy == nil { // the gallery was left meanwhile
				return
			}
			delete(g.busy, k)
			if small == nil {
				g.fail[k] = true
				return
			}
			g.bmp[k] = ui.NewBitmap(small)
			g.order = append(g.order, k)
			for len(g.order) > galMax {
				delete(g.bmp, g.order[0])
				g.order = g.order[1:]
			}
		})
	})
	return nil
}

// makeThumb decodes the image at path, shrunk to galPx on its longer side
// and upright; nil when it cannot be read. It reads the disk: call it off
// the main thread.
func makeThumb(path string) image.Image {
	data, err := readAll(path)
	if err != nil {
		return nil
	}
	return thumbOf(data)
}

// makeQLThumb is makeThumb for a kind the system draws: its QuickLook
// thumbnail, kept in the cache as the preview's are, then shrunk.
func (a *app) makeQLThumb(e vfs.Entry) image.Image {
	dst := a.qlFile(e)
	if _, err := os.Stat(dst); err != nil {
		if os.MkdirAll(filepath.Dir(dst), 0o755) != nil || a.sys.Thumbnail(e.Path, dst, qlPx) != nil {
			return nil
		}
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		return nil
	}
	return thumbOf(data)
}

func thumbOf(data []byte) image.Image {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	small := shrink(img, galPx)
	if o := jpegOrientation(data); o > 1 {
		small = orient(small, o)
	}
	return small
}

// shrink scales img to at most px on its longer side, averaging the
// pixels each destination pixel covers; smaller images stay as they are.
func shrink(img image.Image, px int) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= px && h <= px {
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
		return dst
	}
	dw, dh := px, px
	if w >= h {
		dh = max(1, h*px/w)
	} else {
		dw = max(1, w*px/h)
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		y0, y1 := y*h/dh, max(y*h/dh+1, (y+1)*h/dh)
		for x := 0; x < dw; x++ {
			x0, x1 := x*w/dw, max(x*w/dw+1, (x+1)*w/dw)
			// Sampled on a grid of at most 4x4, not every pixel: a
			// thumbnail does not need more.
			sx, sy := max(1, (x1-x0)/4), max(1, (y1-y0)/4)
			var r, g, bl, al, n uint32
			for yy := y0; yy < y1; yy += sy {
				for xx := x0; xx < x1; xx += sx {
					cr, cg, cb, ca := img.At(b.Min.X+xx, b.Min.Y+yy).RGBA()
					r, g, bl, al, n = r+cr, g+cg, bl+cb, al+ca, n+1
				}
			}
			i := dst.PixOffset(x, y)
			dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] =
				byte(r/n>>8), byte(g/n>>8), byte(bl/n>>8), byte(al/n>>8)
		}
	}
	return dst
}

// jpegOrientation is the EXIF orientation (1-8) of a JPEG, 1 for none.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			return 1
		}
		m, n := data[i+1], int(binary.BigEndian.Uint16(data[i+2:]))
		if m == 0xDA || n < 2 {
			return 1
		}
		if m == 0xE1 && i+2+n <= len(data) {
			return exifOrient(data[i+4 : i+2+n])
		}
		i += 2 + n
	}
	return 1
}

func exifOrient(seg []byte) int {
	if len(seg) < 14 || string(seg[:6]) != "Exif\x00\x00" {
		return 1
	}
	t := seg[6:]
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:]))
	if off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off:]))
	for i := 0; i < n; i++ {
		p := off + 2 + 12*i
		if p+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[p:]) == 0x0112 {
			if o := int(bo.Uint16(t[p+8:])); o >= 1 && o <= 8 {
				return o
			}
			return 1
		}
	}
	return 1
}

// orient turns img upright from EXIF orientation o (2-8).
func orient(img *image.RGBA, o int) *image.RGBA {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var nx, ny int
			switch o {
			case 2:
				nx, ny = w-1-x, y
			case 3:
				nx, ny = w-1-x, h-1-y
			case 4:
				nx, ny = x, h-1-y
			case 5:
				nx, ny = y, x
			case 6:
				nx, ny = h-1-y, x
			case 7:
				nx, ny = h-1-y, w-1-x
			default:
				nx, ny = y, w-1-x
			}
			si, di := img.PixOffset(x, y), dst.PixOffset(nx, ny)
			copy(dst.Pix[di:di+4], img.Pix[si:si+4])
		}
	}
	return dst
}

// toggleGallery switches the preview between one item and the gallery. The
// gallery lives in the preview column, so it brings the three columns.
func (a *app) toggleGallery() {
	a.gallery = !a.gallery
	if a.gallery {
		a.setLayout(layoutColumns)
	} else {
		a.gal.drop()
	}
	a.wantFocus = true
}

// galleryView is the preview column as a gallery of the folder's images.
// A click chooses one (the cursor goes to it), Cmd or Shift with it
// several, a double click opens it.
func (a *app) galleryView(c *ui.Context, pi int) {
	p := a.panes[pi]
	ts := p.tab()
	imgs := galleryEntries(ts.t.Entries())
	if len(imgs) == 0 {
		ui.Text(c, "无法启用图库视图，当前文件夹下没有图片，按 "+a.firstKey("gallery")+" 返回文件列表").TextColor(tok(c, "muted")).Padding(4, rowPadX).Grow(1)
		return
	}
	// The cursor's image is the one the grid has, while the list moves it.
	p.galSel = -1
	if e, ok := ts.cursorEntry(); ok {
		p.galSel = slices.IndexFunc(imgs, func(x vfs.Entry) bool { return x.Path == e.Path })
	}
	before := p.galSel
	st := &p.gal
	st.Selected, st.Selection = &p.galSel, &ts.sel
	st.Key = func(i int) any { return imgs[i].Path }
	st.Label = func(i int) string { return imgs[i].Name }
	g := ui.GridView(c.Key("gallery"), st, len(imgs), galCell, galCell, func(i int) {
		if b := a.thumb(imgs[i]); b != nil {
			ui.Image(c, b).Fit(ui.Contain).Grow(1).Padding(2)
		} else {
			ui.Column(c).Grow(1)
		}
	}).Grow(1).Padding(4).Gap(4).Label("图库")
	if g.Changed() && p.galSel >= 0 && p.galSel != before {
		a.active = pi
		es := ts.t.Entries()
		if i := slices.IndexFunc(es, func(x vfs.Entry) bool { return x.Path == imgs[p.galSel].Path }); i >= 0 {
			ts.cursor = i
		}
	}
	if g.Submitted() && p.galSel >= 0 && p.galSel < len(imgs) {
		a.active = pi
		a.openEntry(ts, imgs[p.galSel])
	}
}

// openInFinder opens the cursor's item with Finder: a folder in a Finder
// window of its own; a file, package or archive, which Finder would open
// with another app, shown selected in its folder's window; with no item,
// the pane's folder.
func (a *app) openInFinder(ts *tabState) {
	e, ok := ts.cursorEntry()
	if !ok {
		a.openDirInFinder(ts.t.Dir)
		return
	}
	if e.IsDir && !vfs.IsPackage(e) && !vfs.IsArchive(e) {
		a.openDirInFinder(e.Path)
		return
	}
	p := e.Path
	if archive, _, in := vfs.SplitZip(p); in {
		p = archive // Finder cannot look inside: show the archive
	}
	a.finderDo(p, true)
}

// openDirInFinder opens dir in a Finder window; one inside an archive
// opens the folder holding the archive.
func (a *app) openDirInFinder(dir string) { a.finderDo(vfs.RealDir(dir), false) }

// finderDo has the system Finder itself show p, as goFinder may be the
// default file viewer, which opening it by app name would come back to.
func (a *app) finderDo(p string, reveal bool) {
	a.async(func() {
		if err := a.sys.FinderOpen(p, reveal); err != nil {
			a.post(func() { a.status = "无法用 Finder 打开：" + err.Error() })
		}
	})
}
