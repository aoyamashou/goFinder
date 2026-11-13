package gui

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/finfo"
	"gofinder/internal/vfs"
)

const (
	pvNone = iota
	pvText
	pvImage
	pvDir
	pvBinary
)

const (
	textPreviewMax  = 32 << 10
	imagePreviewMax = 40 << 20
	previewMaxItems = 2000
)

// QuickLook thumbnails: qlmanage runs once the cursor has rested on a file
// qlDelay, for at most qlTimeout, drawing qlPx pixels across.
const (
	qlDelay   = 150 * time.Millisecond
	qlTimeout = 3 * time.Second
	qlPx      = 800
	qlMaxAge  = 30 * 24 * time.Hour // older thumbnails are cleared at start
)

// previewState is the preview of one item, loaded off the main thread.
type previewState struct {
	path string
	mod  time.Time
	seq  int
	kind int
	text string
	bmp  *ui.Bitmap
	info string
	busy bool

	entries []vfs.Entry // a folder's items
	hidden  bool        // the folder's hidden items are among them

	// A file the system draws a thumbnail of: ql waits for one, from the
	// frame at (when the preview was first shown) on; qlRun has started
	// qlmanage; thumb says bmp is its thumbnail.
	ql    bool
	qlRun bool
	at    time.Time
	thumb bool
}

var imageExts = map[string]bool{"png": true, "jpg": true, "jpeg": true, "gif": true, "webp": true, "bmp": true}

// qlExts are the kinds the system's QuickLook draws, which the app
// cannot itself: documents, videos, other images, fonts. Others are not
// asked for, as qlmanage takes its whole timeout on what it cannot draw.
var qlExts = map[string]bool{}

func init() {
	for _, e := range []string{
		"pdf", "html", "htm", "xhtml", "svg", "rtf", "rtfd", "doc", "docx", "odt", "pages",
		"xls", "xlsx", "numbers", "ods", "ppt", "pptx", "key", "odp", "epub",
		"mov", "mp4", "m4v", "avi", "mkv", "webm", "3gp", "mpg",
		"heic", "heif", "tif", "tiff", "avif", "psd", "ai", "eps", "icns", "ico", "raw", "cr2", "nef", "dng",
		"ttf", "otf", "ttc", "usdz", "sketch",
	} {
		qlExts[e] = true
	}
}

// ensurePreview loads the preview of e unless it is the one shown, as it
// was when read, with the pane's tab's hidden items setting.
func (a *app) ensurePreview(p *paneState, e vfs.Entry) {
	hidden := p.tab().t.ShowHidden
	if p.pv.path != e.Path || !p.pv.mod.Equal(e.ModTime) || p.pv.hidden != hidden {
		a.loadPreview(p, e, hidden)
	}
}

func previewInfo(e vfs.Entry) string {
	s := e.ModTime.Format("2006-01-02 15:04")
	if !e.IsDir {
		s = humanSize(e.Size) + " · " + s
	}
	return s + " · " + e.Mode.String()
}

func (a *app) loadPreview(p *paneState, e vfs.Entry, hidden bool) {
	pv := &p.pv
	pv.seq++
	seq := pv.seq
	*pv = previewState{path: e.Path, mod: e.ModTime, seq: seq, busy: true, hidden: hidden}
	thumb := a.qlFile(e)
	a.async(func() {
		res := buildPreview(e, hidden)
		if res.ql {
			if bmp := readThumb(thumb); bmp != nil { // drawn before
				res.kind, res.bmp, res.thumb, res.ql = pvImage, bmp, true, false
			}
		}
		a.post(func() {
			if p.pv.seq != seq {
				return
			}
			res.path, res.mod, res.seq, res.hidden = e.Path, e.ModTime, seq, hidden
			p.pv = res
		})
	})
}

func buildPreview(e vfs.Entry, hidden bool) previewState {
	dir := e.Path
	switch {
	case vfs.IsPackage(e) || vfs.IsArchive(e):
		// Shown as one file, as Finder does; QuickLook is not asked, as
		// qlmanage hangs on them.
		// Named by its kind, a package as its file kind would be.
		kind := finfo.Kind(e.Name, false, e.IsLink)
		if e.IsDir && strings.HasSuffix(kind, " 文件") {
			kind = strings.TrimSuffix(kind, " 文件") + " 包"
		}
		return previewState{kind: pvBinary, info: kind}
	case e.IsDir:
		if _, err := vfs.Stat(dir); err != nil {
			return previewState{kind: pvNone, info: err.Error()}
		}
		es := readListing(dir, hidden)
		info := fmt.Sprintf("%d 项", len(es))
		if len(es) > previewMaxItems {
			es = es[:previewMaxItems:previewMaxItems]
			info += fmt.Sprintf("，只列出前 %d 项", previewMaxItems)
		}
		return previewState{kind: pvDir, entries: es, info: info}
	case qlExts[e.Ext()]:
		return previewState{kind: pvBinary, ql: true}
	case imageExts[e.Ext()] && e.Size <= imagePreviewMax:
		data, err := readAll(e.Path)
		if err != nil {
			return previewState{info: err.Error()}
		}
		bmp, err := ui.DecodeBitmap(data)
		if err != nil {
			return previewState{kind: pvBinary, ql: true} // the system may read it
		}
		w, h := ui.ImageSize(bmp)
		return previewState{kind: pvImage, bmp: bmp, info: fmt.Sprintf("%.0f × %.0f", w, h)}
	default:
		f, err := vfs.Open(e.Path)
		if err != nil {
			return previewState{info: err.Error()}
		}
		defer f.Close()
		buf := make([]byte, textPreviewMax)
		n, _ := io.ReadFull(f, buf)
		buf = buf[:n]
		if bytes.IndexByte(buf, 0) >= 0 {
			return previewState{kind: pvBinary, info: "二进制文件"}
		}
		// Drop a rune cut at the end of the buffer.
		for len(buf) > 0 && !utf8.Valid(buf) {
			buf = buf[:len(buf)-1]
		}
		info := ""
		if e.Size > int64(n) {
			info = fmt.Sprintf("只显示前 %d KB", textPreviewMax>>10)
		}
		return previewState{kind: pvText, text: string(buf), info: info}
	}
}

// readAll reads a file, inside an archive or not.
func readAll(p string) ([]byte, error) {
	f, err := vfs.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, imagePreviewMax))
}

// qlKey names the thumbnail of the file at path, as it was at mod, px
// pixels across: another version or size is another thumbnail.
func qlKey(path string, mod time.Time, px int) string {
	h := fnv.New64a()
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write([]byte(mod.UTC().Format(time.RFC3339Nano)))
	h.Write([]byte{0})
	h.Write([]byte(strconv.Itoa(px)))
	return fmt.Sprintf("%016x.png", h.Sum64())
}

// qlDir is where thumbnails are kept: Caches/<id>/ql.
func (a *app) qlDir() string {
	base := a.cacheDir
	if base == "" {
		base = filepath.Join(os.TempDir(), "goFinder")
	}
	return filepath.Join(base, "ql")
}

// qlFile is the thumbnail of e, once drawn.
func (a *app) qlFile(e vfs.Entry) string {
	return filepath.Join(a.qlDir(), qlKey(e.Path, e.ModTime, qlPx))
}

// readThumb decodes a thumbnail drawn before, nil when there is none.
func readThumb(p string) *ui.Bitmap {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	bmp, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil
	}
	return bmp
}

// waitThumb starts qlmanage on e once the cursor has rested on it qlDelay,
// asking for a frame then until it has. A file it failed on is not asked
// for again while the app runs: it shows its icon.
func (a *app) waitThumb(c *ui.Context, p *paneState, e vfs.Entry) {
	pv := &p.pv
	if !pv.ql || pv.qlRun {
		return
	}
	dst := a.qlFile(e)
	if a.qlFailed[dst] {
		pv.ql = false
		return
	}
	now := c.Now()
	if pv.at.IsZero() {
		pv.at = now
	}
	if wait := qlDelay - now.Sub(pv.at); wait > 0 {
		c.After(wait)
		return
	}
	pv.qlRun = true
	seq := pv.seq
	src := e.Path
	zipped := vfs.InZip(e.Path)
	if zipped {
		src = a.zipCacheFile(e.Path)
	}
	a.async(func() {
		err := os.MkdirAll(filepath.Dir(dst), 0o755)
		if err == nil && zipped {
			os.RemoveAll(src)
			if err = os.MkdirAll(filepath.Dir(src), 0o755); err == nil {
				err = vfs.Extract(e.Path, src)
			}
		}
		if err == nil {
			err = a.sys.Thumbnail(src, dst, qlPx)
		}
		var bmp *ui.Bitmap
		if err == nil {
			if bmp = readThumb(dst); bmp == nil {
				err = fmt.Errorf("缩略图无法解码")
				os.Remove(dst)
			}
		}
		a.post(func() {
			if err != nil {
				a.qlFailed[dst] = true
			}
			if p.pv.seq != seq {
				return
			}
			p.pv.ql = false
			if err == nil {
				p.pv.kind, p.pv.bmp, p.pv.thumb = pvImage, bmp, true
			}
		})
	})
}

// zipCacheFile is where a file inside an archive is extracted to: the
// archive's own folder of the cache, as opening it uses.
func (a *app) zipCacheFile(p string) string {
	archive, inner, _ := vfs.SplitZip(p)
	base := a.cacheDir
	if base == "" {
		base = os.TempDir()
	}
	h := fnv.New64a()
	h.Write([]byte(archive))
	return filepath.Join(base, "zip", fmt.Sprintf("%x", h.Sum64()), filepath.FromSlash(inner))
}

// pruneThumbs removes thumbnails not drawn for qlMaxAge. It reads the
// disk: call it off the main thread.
func pruneThumbs(dir string, now time.Time) {
	es, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range es {
		if info, err := e.Info(); err == nil && now.Sub(info.ModTime()) > qlMaxAge {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
