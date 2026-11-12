package finder

import (
	"encoding/binary"
	"errors"
	"strings"
)

var errBookmark = errors.New("finder: bad bookmark")

// Bookmark items read here (keys of the bookmark's table of contents).
const (
	bmPath        = 0x1004 // array of the path's components
	bmDisplayName = 0xf017 // the name Finder shows, localized
)

// Item types: the low byte is a subtype.
const (
	bmString = 0x0100
	bmArray  = 0x0600
)

// bookmark is what a bookmark ("book" data) says of its file.
type bookmark struct {
	Path string // absolute, "" for none (a device, AirDrop, a search)
	Name string // the localized display name, if the bookmark has one
}

// parseBookmark reads the path and display name of bookmark data, as
// CFURLCreateBookmarkData writes it: a header ("book", sizes, the offset
// of the data), then items addressed by tables of contents.
func parseBookmark(b []byte) (bookmark, error) {
	var bm bookmark
	if len(b) < 16 || string(b[:4]) != "book" {
		return bm, errBookmark
	}
	le := binary.LittleEndian
	hs := int(le.Uint32(b[12:]))
	if hs < 16 || hs+4 > len(b) {
		return bm, errBookmark
	}
	data := b[hs:]
	u32 := func(at int) (int, bool) {
		if at < 0 || at+4 > len(data) {
			return 0, false
		}
		return int(le.Uint32(data[at:])), true
	}
	// item reads an item's type and payload.
	item := func(off int) (int, []byte, bool) {
		n, ok1 := u32(off)
		t, ok2 := u32(off + 4)
		if !ok1 || !ok2 || n > len(data)-off-8 {
			return 0, nil, false
		}
		return t, data[off+8 : off+8+n], true
	}
	str := func(off int) (string, bool) {
		t, p, ok := item(off)
		if !ok || t&0xff00 != bmString {
			return "", false
		}
		return string(p), true
	}
	toc, ok := u32(0)
	keys := map[int]int{}
	for hops := 0; ok && toc != 0 && hops < 16; hops++ {
		magic, ok1 := u32(toc + 4)
		next, ok2 := u32(toc + 12)
		count, ok3 := u32(toc + 16)
		if !ok1 || !ok2 || !ok3 || magic != 0xfffffffe || count > len(data)/12 {
			return bm, errBookmark
		}
		for i := range count {
			k, ok1 := u32(toc + 20 + 12*i)
			off, ok2 := u32(toc + 24 + 12*i)
			if !ok1 || !ok2 {
				return bm, errBookmark
			}
			if _, dup := keys[k]; !dup {
				keys[k] = off
			}
		}
		toc = next
	}
	if off, ok := keys[bmPath]; ok {
		t, p, ok := item(off)
		if !ok || t&0xff00 != bmArray || len(p)%4 != 0 {
			return bm, errBookmark
		}
		parts := make([]string, 0, len(p)/4)
		for i := 0; i < len(p); i += 4 {
			s, ok := str(int(le.Uint32(p[i:])))
			if !ok {
				return bm, errBookmark
			}
			parts = append(parts, s)
		}
		bm.Path = "/" + strings.Join(parts, "/")
	}
	if off, ok := keys[bmDisplayName]; ok {
		bm.Name, _ = str(off)
	}
	return bm, nil
}
