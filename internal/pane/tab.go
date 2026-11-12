// Package pane holds the browsing state of a tab: where it is, its history,
// and the listing sorted and filtered for display. It does no I/O; the view
// reads directories and hands the entries in.
package pane

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	"gofinder/internal/pinyin"
	"gofinder/internal/vfs"
)

// SortKey is the column a listing is sorted by.
type SortKey int

const (
	ByName SortKey = iota
	BySize
	ByDate
	ByKind
)

// Tab is one tab of a pane.
type Tab struct {
	Dir        string
	Sort       SortKey
	Desc       bool
	ShowHidden bool
	// Filter keeps only the entries whose names contain it (Yazi's
	// "filter --smart"): see MatchName. Going to another folder clears it.
	Filter string

	back, fwd []string
	all       []vfs.Entry
	visible   []vfs.Entry
	// Focus is the name to put the cursor on once the listing arrives,
	// as the folder we came up from.
	Focus string
}

// New returns a tab at dir.
func New(dir string) *Tab { return &Tab{Dir: filepath.Clean(dir)} }

// Go moves to dir, recording history. It returns false when already there.
func (t *Tab) Go(dir string) bool {
	dir = filepath.Clean(dir)
	if dir == t.Dir {
		return false
	}
	t.back = append(t.back, t.Dir)
	t.fwd = nil
	t.Dir, t.Focus, t.Filter = dir, "", ""
	return true
}

// Up goes to the parent, focusing the folder left.
func (t *Tab) Up() bool {
	parent := filepath.Dir(t.Dir)
	if parent == t.Dir {
		return false
	}
	name := baseName(t.Dir)
	t.Go(parent)
	t.Focus = name
	return true
}

// Back goes back in history.
func (t *Tab) Back() bool {
	if len(t.back) == 0 {
		return false
	}
	t.fwd = append(t.fwd, t.Dir)
	from := t.Dir
	t.Dir, t.back = t.back[len(t.back)-1], t.back[:len(t.back)-1]
	t.Focus, t.Filter = childOf(t.Dir, from), ""
	return true
}

// Forward goes forward in history.
func (t *Tab) Forward() bool {
	if len(t.fwd) == 0 {
		return false
	}
	t.back = append(t.back, t.Dir)
	t.Dir, t.fwd = t.fwd[len(t.fwd)-1], t.fwd[:len(t.fwd)-1]
	t.Focus, t.Filter = "", ""
	return true
}

// CanBack and CanForward report history.
func (t *Tab) CanBack() bool    { return len(t.back) > 0 }
func (t *Tab) CanForward() bool { return len(t.fwd) > 0 }

// childOf returns the name of the child of dir on the way to p, or "".
func childOf(dir, p string) string {
	rel, err := filepath.Rel(dir, p)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	return baseName(filepath.Join(dir, strings.SplitN(rel, string(filepath.Separator), 2)[0]))
}

// baseName is p's last element, where an archive's root "a.zip!" is the
// archive "a.zip", the item to focus on coming out of it.
func baseName(p string) string {
	if a, inner, ok := vfs.SplitZip(p); ok && inner == "" {
		return filepath.Base(a)
	}
	return filepath.Base(p)
}

// Title is the tab's label.
func (t *Tab) Title() string {
	if t.Dir == "/" {
		return "/"
	}
	return baseName(t.Dir)
}

// SetEntries replaces the listing.
func (t *Tab) SetEntries(es []vfs.Entry) {
	t.all = es
	t.refresh()
}

// SetSort changes the order.
func (t *Tab) SetSort(k SortKey, desc bool) {
	t.Sort, t.Desc = k, desc
	t.refresh()
}

// ToggleHidden shows or hides dot files.
func (t *Tab) ToggleHidden() {
	t.ShowHidden = !t.ShowHidden
	t.refresh()
}

// SetFilter changes the filter.
func (t *Tab) SetFilter(q string) {
	t.Filter = q
	t.refresh()
}

// Entries is the listing as shown.
func (t *Tab) Entries() []vfs.Entry { return t.visible }

// Index returns the row of the entry named name, or -1.
func (t *Tab) Index(name string) int {
	return slices.IndexFunc(t.visible, func(e vfs.Entry) bool { return e.Name == name })
}

func (t *Tab) refresh() {
	v := t.visible[:0:0]
	for _, e := range t.all {
		if !t.ShowHidden && e.Hidden() {
			continue
		}
		if !MatchName(e.Name, t.Filter) {
			continue
		}
		v = append(v, e)
	}
	SortEntries(v, t.Sort, t.Desc)
	t.visible = v
}

// SortEntries sorts folders first, then by key; names break ties
// (case-insensitive, natural-ish via lowercase compare).
func SortEntries(es []vfs.Entry, k SortKey, desc bool) {
	slices.SortStableFunc(es, func(a, b vfs.Entry) int {
		if a.IsDir != b.IsDir {
			if a.IsDir {
				return -1
			}
			return 1
		}
		var r int
		switch k {
		case BySize:
			r = cmp.Compare(a.Size, b.Size)
		case ByDate:
			r = a.ModTime.Compare(b.ModTime)
		case ByKind:
			r = cmp.Compare(a.Ext(), b.Ext())
		}
		if r == 0 {
			r = cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		}
		if desc {
			r = -r
		}
		return r
	})
}

// MatchName tells whether name contains q, as Yazi's smart case: with no
// capital in q the match ignores case, with one it is exact; with no
// capital, a name in Chinese matches by the first letters of its readings
// too. An empty q matches everything.
func MatchName(name, q string) bool {
	if q == "" {
		return true
	}
	lower := q == strings.ToLower(q)
	if lower {
		name = strings.ToLower(name)
	}
	if strings.Contains(name, q) {
		return true
	}
	// A name in Chinese is found by the first letters of its readings too:
	// "模型查询.txt" by "mxcx".
	return lower && isASCII(q) && pinyin.HasHan(name) && strings.Contains(pinyin.Initials(name), q)
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
