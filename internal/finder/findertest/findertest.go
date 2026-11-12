// Package findertest writes Finder's files for tests: binary plists,
// bookmarks and sidebar lists made from scratch, so no fixture holds a
// real Mac's paths.
package findertest

import (
	"bytes"
	"encoding/binary"
	"slices"
	"unicode/utf16"

	"gofinder/internal/finder"
)

// Plist writes v (nil, bool, int, string, []byte, finder.UID, []any,
// map[string]any) as a binary plist.
func Plist(v any) []byte {
	var objs [][]byte
	var add func(v any) int
	marker := func(hi byte, n int) []byte {
		if n < 15 {
			return []byte{hi<<4 | byte(n)}
		}
		b := []byte{hi<<4 | 0xf, 0x12}
		return binary.BigEndian.AppendUint32(b, uint32(n))
	}
	ref := func(i int) []byte { return binary.BigEndian.AppendUint16(nil, uint16(i)) }
	add = func(v any) int {
		i := len(objs)
		objs = append(objs, nil)
		var b []byte
		switch x := v.(type) {
		case nil:
			b = []byte{0}
		case bool:
			b = []byte{0x08}
			if x {
				b = []byte{0x09}
			}
		case int:
			b = binary.BigEndian.AppendUint64([]byte{0x13}, uint64(x))
		case string:
			ascii := true
			for _, r := range x {
				ascii = ascii && r < 0x80
			}
			if ascii {
				b = append(marker(5, len(x)), x...)
			} else {
				u := utf16.Encode([]rune(x))
				b = marker(6, len(u))
				for _, c := range u {
					b = binary.BigEndian.AppendUint16(b, c)
				}
			}
		case []byte:
			b = append(marker(4, len(x)), x...)
		case finder.UID:
			b = binary.BigEndian.AppendUint16([]byte{0x81}, uint16(x))
		case []any:
			b = marker(0xa, len(x))
			for _, e := range x {
				b = append(b, ref(add(e))...)
			}
		case map[string]any:
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			b = marker(0xd, len(keys))
			for _, k := range keys {
				b = append(b, ref(add(k))...)
			}
			for _, k := range keys {
				b = append(b, ref(add(x[k]))...)
			}
		default:
			panic("encodePlist")
		}
		objs[i] = b
		return i
	}
	add(v)
	out := []byte("bplist00")
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = len(out)
		out = append(out, o...)
	}
	table := len(out)
	for _, o := range offs {
		out = binary.BigEndian.AppendUint32(out, uint32(o))
	}
	tr := make([]byte, 32)
	tr[6], tr[7] = 4, 2
	binary.BigEndian.PutUint64(tr[8:], uint64(len(objs)))
	binary.BigEndian.PutUint64(tr[24:], uint64(table))
	return append(out, tr...)
}

// Bookmark writes bookmark data for path ("" for none), with a display
// name if name is not "".
func Bookmark(path, name string) []byte {
	le := binary.LittleEndian
	var data []byte
	item := func(typ int, p []byte) int {
		off := len(data)
		data = le.AppendUint32(data, uint32(len(p)))
		data = le.AppendUint32(data, uint32(typ))
		data = append(data, p...)
		for len(data)%4 != 0 {
			data = append(data, 0)
		}
		return off
	}
	data = make([]byte, 4) // the offset of the first TOC, set below
	keys := [][2]int{{0x2002, item(0x0101, []byte("/"))}}
	if path != "" { // "": no file, as AirDrop's
		var arr []byte
		for _, part := range bytes.Split([]byte(path[1:]), []byte("/")) {
			arr = le.AppendUint32(arr, uint32(item(0x0101, part)))
		}
		keys = append(keys, [2]int{0x1004, item(0x0601, arr)})
	}
	if name != "" {
		keys = append(keys, [2]int{0xf017, item(0x0101, []byte(name))})
	}
	toc := len(data)
	le.PutUint32(data, uint32(toc))
	data = le.AppendUint32(data, uint32(20+12*len(keys)))
	data = le.AppendUint32(data, 0xfffffffe)
	data = le.AppendUint32(data, 1)
	data = le.AppendUint32(data, 0) // no next TOC
	data = le.AppendUint32(data, uint32(len(keys)))
	for _, k := range keys {
		data = le.AppendUint32(data, uint32(k[0]))
		data = le.AppendUint32(data, uint32(k[1]))
		data = le.AppendUint32(data, 0)
	}
	head := []byte("book")
	head = le.AppendUint32(head, uint32(48+len(data)))
	head = le.AppendUint32(head, 0x10040000)
	head = le.AppendUint32(head, 48)
	head = append(head, make([]byte, 48-16)...)
	return append(head, data...)
}

// Entry is an item of a list: its folder, its custom name, the
// bookmark's display name, and whether Finder hides it.
type Entry struct {
	Path, Name, BookmarkName string
	Hidden                   bool
}

// List writes a shared file list (sfl4) as Finder does: an
// NSKeyedArchiver archive of {items: [{Bookmark, CustomItemProperties,
// Name?}], properties}.
func List(items []Entry) []byte {
	objs := []any{"$null"}
	obj := func(v any) finder.UID { objs = append(objs, v); return finder.UID(len(objs) - 1) }
	dictClass := obj(map[string]any{"$classname": "NSDictionary", "$classes": []any{"NSDictionary", "NSObject"}})
	arrClass := obj(map[string]any{"$classname": "NSArray", "$classes": []any{"NSArray", "NSObject"}})
	dict := func(m map[string]finder.UID) finder.UID {
		var ks, vs []any
		for _, k := range slices.Sorted(func(yield func(string) bool) {
			for k := range m {
				if !yield(k) {
					return
				}
			}
		}) {
			ks, vs = append(ks, obj(k)), append(vs, m[k])
		}
		return obj(map[string]any{"$class": dictClass, "NS.keys": ks, "NS.objects": vs})
	}
	var refs []any
	for _, it := range items {
		props := map[string]finder.UID{}
		if it.Hidden {
			props["com.apple.LSSharedFileList.ItemIsHidden"] = obj(true)
		}
		bm := Bookmark(it.Path, it.BookmarkName)
		m := map[string]finder.UID{
			"Bookmark":             obj(bm),
			"CustomItemProperties": dict(props),
			"uuid":                 obj("00000000-0000-0000-0000-000000000000"),
			"visibility":           obj(0),
		}
		if it.Name != "" {
			m["Name"] = obj(it.Name)
		}
		refs = append(refs, dict(m))
	}
	list := obj(map[string]any{"$class": arrClass, "NS.objects": refs})
	root := dict(map[string]finder.UID{"items": list, "properties": dict(nil)})
	return Plist(map[string]any{
		"$archiver": "NSKeyedArchiver",
		"$version":  100000,
		"$top":      map[string]any{"root": root},
		"$objects":  objs,
	})
}
