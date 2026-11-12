package finder

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
)

// Kind is what an item is, for its icon.
type Kind string

const (
	Home         Kind = "home"
	Desktop      Kind = "desktop"
	Downloads    Kind = "downloads"
	Documents    Kind = "documents"
	Pictures     Kind = "pictures"
	Music        Kind = "music"
	Movies       Kind = "movies"
	Applications Kind = "applications"
	ICloud       Kind = "icloud"
	Cloud        Kind = "cloud" // a cloud drive under ~/Library/CloudStorage
	Volume       Kind = "volume"
	Folder       Kind = "folder"
)

// Item is a place in the sidebar.
type Item struct {
	Name string
	Path string
	Kind Kind
}

// ListDir is the folder of Finder's shared file lists, the sidebar's.
func ListDir(home string) string {
	return filepath.Join(home, "Library", "Application Support", "com.apple.sharedfilelist")
}

// ICloudDrive is the iCloud Drive folder.
func ICloudDrive(home string) string {
	return filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs")
}

var errNoList = errors.New("finder: no sidebar list")

// Favorites is Finder's sidebar section of favorites, in Finder's order,
// with the iCloud section's folders that are not in it after them. It
// leaves out what Finder hides, what has no folder (AirDrop, Recents,
// devices) and folders that are gone. Names are Finder's: a custom one,
// else the bookmark's localized name.
func Favorites(home string) ([]Item, error) {
	dir := ListDir(home)
	items, err := readList(dir, "com.apple.LSSharedFileList.FavoriteItems")
	if err != nil {
		return nil, err
	}
	if more, err := readList(dir, "com.apple.LSSharedFileList.iCloudItems"); err == nil {
		items = append(items, more...)
	}
	var out []Item
	for _, it := range items {
		if it.Path == "" || strings.HasSuffix(it.Path, ".cannedSearch") ||
			slices.ContainsFunc(out, func(o Item) bool { return o.Path == it.Path }) {
			continue
		}
		if st, err := os.Stat(it.Path); err != nil || !st.IsDir() {
			continue
		}
		it.Kind = kindOf(home, it.Path)
		if it.Name == "" {
			it.Name = defaultName(home, it.Path, it.Kind)
		}
		out = append(out, it)
	}
	return out, nil
}

// readList reads the shared file list name.sfl4, or name.sfl3 on older
// systems.
func readList(dir, name string) ([]Item, error) {
	var b []byte
	var err error
	for _, ext := range []string{".sfl4", ".sfl3"} {
		if b, err = os.ReadFile(filepath.Join(dir, name+ext)); err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	return parseList(b)
}

// parseList reads a shared file list: an NSKeyedArchiver archive whose
// root holds "items", dictionaries of a "Bookmark" and properties.
func parseList(b []byte) ([]Item, error) {
	v, err := parsePlist(b)
	if err != nil {
		return nil, err
	}
	root, ok := unarchive(v).(map[string]any)
	if !ok {
		return nil, errNoList
	}
	list, ok := root["items"].([]any)
	if !ok {
		return nil, errNoList
	}
	var items []Item
	for _, x := range list {
		d, ok := x.(map[string]any)
		if !ok {
			continue
		}
		props, _ := d["CustomItemProperties"].(map[string]any)
		if hidden, _ := props["com.apple.LSSharedFileList.ItemIsHidden"].(bool); hidden {
			continue
		}
		data, _ := d["Bookmark"].([]byte)
		bm, err := parseBookmark(data)
		if err != nil || bm.Path == "" {
			continue
		}
		name, _ := d["Name"].(string)
		if name == "" {
			name = bm.Name
		}
		items = append(items, Item{Name: name, Path: bm.Path})
	}
	return items, nil
}

// unarchive decodes an NSKeyedArchiver archive's root: dictionaries,
// arrays, strings and data become map[string]any, []any, string and
// []byte; other objects keep their keys.
func unarchive(v any) any {
	top, _ := v.(map[string]any)
	objs, _ := top["$objects"].([]any)
	t, _ := top["$top"].(map[string]any)
	uid, ok := t["root"].(UID)
	if !ok {
		return nil
	}
	var dec func(x any, depth int) any
	dec = func(x any, depth int) any {
		if depth > 32 {
			return nil
		}
		if u, ok := x.(UID); ok {
			if int(u) >= len(objs) {
				return nil
			}
			x = objs[u]
		}
		switch o := x.(type) {
		case string:
			if o == "$null" {
				return nil
			}
			return o
		case map[string]any:
			if ks, ok := o["NS.keys"].([]any); ok {
				vs, _ := o["NS.objects"].([]any)
				m := make(map[string]any, len(ks))
				for i, k := range ks {
					if s, ok := dec(k, depth+1).(string); ok && i < len(vs) {
						m[s] = dec(vs[i], depth+1)
					}
				}
				return m
			}
			if vs, ok := o["NS.objects"].([]any); ok {
				arr := make([]any, len(vs))
				for i, e := range vs {
					arr[i] = dec(e, depth+1)
				}
				return arr
			}
			if s, ok := o["NS.string"]; ok {
				return dec(s, depth+1)
			}
			if d, ok := o["NS.data"]; ok {
				return dec(d, depth+1)
			}
			m := make(map[string]any, len(o))
			for k, e := range o {
				if k != "$class" {
					m[k] = dec(e, depth+1)
				}
			}
			return m
		}
		return x
	}
	return dec(uid, 0)
}

func kindOf(home, p string) Kind {
	switch p {
	case home:
		return Home
	case filepath.Join(home, "Desktop"):
		return Desktop
	case filepath.Join(home, "Downloads"):
		return Downloads
	case filepath.Join(home, "Documents"):
		return Documents
	case filepath.Join(home, "Pictures"):
		return Pictures
	case filepath.Join(home, "Music"):
		return Music
	case filepath.Join(home, "Movies"):
		return Movies
	case "/Applications":
		return Applications
	case ICloudDrive(home):
		return ICloud
	}
	if filepath.Dir(p) == filepath.Join(home, "Library", "CloudStorage") {
		return Cloud
	}
	if filepath.Dir(p) == "/Volumes" {
		return Volume
	}
	return Folder
}

var names = map[Kind]string{
	Desktop: "桌面", Downloads: "下载", Documents: "文稿", Pictures: "图片",
	Music: "音乐", Movies: "影片", Applications: "应用程序", ICloud: "iCloud云盘",
}

func defaultName(home, p string, k Kind) string {
	if n, ok := names[k]; ok {
		return n
	}
	if k == Cloud {
		return cloudName(filepath.Base(p))
	}
	return filepath.Base(p)
}

// cloudName is a CloudStorage folder's provider, as Finder shows it:
// "GoogleDrive-me@example.com" is "Google Drive".
func cloudName(dir string) string {
	prov, _, _ := strings.Cut(dir, "-")
	switch prov {
	case "GoogleDrive":
		return "Google Drive"
	case "":
		return dir
	}
	return prov
}

// Locations is the sidebar's section of locations: iCloud Drive, the
// cloud drives in ~/Library/CloudStorage and the volumes in volumesDir
// the user sees in Finder (Volumes).
func Locations(home, volumesDir string) []Item {
	var out []Item
	if home != "" {
		if st, err := os.Stat(ICloudDrive(home)); err == nil && st.IsDir() {
			out = append(out, Item{Name: names[ICloud], Path: ICloudDrive(home), Kind: ICloud})
		}
		cs := filepath.Join(home, "Library", "CloudStorage")
		des, _ := os.ReadDir(cs)
		var clouds []Item
		for _, de := range des {
			if !de.IsDir() || strings.HasPrefix(de.Name(), ".") {
				continue
			}
			clouds = append(clouds, Item{Name: cloudName(de.Name()), Path: filepath.Join(cs, de.Name()), Kind: Cloud})
		}
		count := map[string]int{}
		for _, c := range clouds {
			count[c.Name]++
		}
		for i, c := range clouds {
			if count[c.Name] > 1 { // two accounts of one provider
				clouds[i].Name = filepath.Base(c.Path)
			}
		}
		out = append(out, clouds...)
	}
	for _, v := range Volumes(volumesDir) {
		out = append(out, Item{Name: filepath.Base(v), Path: v, Kind: Volume})
	}
	return out
}

var uuidName = regexp.MustCompile(`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`)

// mntDontBrowse marks a volume that Finder does not show (MNT_DONTBROWSE):
// Recovery, Preboot, VM and the like.
const mntDontBrowse = 0x00100000

// Volumes lists the volumes mounted in dir that Finder shows: not the
// startup disk (a link to "/"), not hidden ones, not system volumes
// (MNT_DONTBROWSE, or named by a UUID).
func Volumes(dir string) []string {
	des, _ := os.ReadDir(dir)
	var vols []string
	for _, de := range des {
		name := de.Name()
		p := filepath.Join(dir, name)
		if strings.HasPrefix(name, ".") || uuidName.MatchString(name) {
			continue
		}
		if t, err := filepath.EvalSymlinks(p); err != nil || t == "/" {
			continue
		}
		if hiddenMount(p) {
			continue
		}
		vols = append(vols, p)
	}
	return vols
}

// hiddenMount tells whether p is a mount point Finder doesn't show. A
// folder that is not a mount point has its volume's flags (the hidden
// Data volume's, for one under /Volumes), so only mount points count.
func hiddenMount(p string) bool {
	var st syscall.Statfs_t
	if syscall.Statfs(p, &st) != nil || st.Flags&mntDontBrowse == 0 {
		return false
	}
	var on []byte
	for _, c := range st.Mntonname {
		if c == 0 {
			break
		}
		on = append(on, byte(c))
	}
	return string(on) == p
}

// NewWindowDir is where Finder opens a new window (its NewWindowTarget
// preference), home when that can't be read or is gone.
func NewWindowDir(home string) string {
	b, err := os.ReadFile(filepath.Join(home, "Library", "Preferences", "com.apple.finder.plist"))
	if err != nil {
		return home
	}
	v, err := parsePlist(b)
	if err != nil {
		return home
	}
	d, _ := v.(map[string]any)
	target, _ := d["NewWindowTarget"].(string)
	path, _ := d["NewWindowTargetPath"].(string)
	p := newWindowPath(home, target, path)
	if st, err := os.Stat(p); err != nil || !st.IsDir() {
		return home
	}
	return p
}

// newWindowPath maps a NewWindowTarget, and its NewWindowTargetPath URL
// for "PfLo", to a folder.
func newWindowPath(home, target, path string) string {
	switch target {
	case "PfDe":
		return filepath.Join(home, "Desktop")
	case "PfDo":
		return filepath.Join(home, "Documents")
	case "PfID":
		return ICloudDrive(home)
	case "PfCm", "PfVo":
		return "/"
	case "PfLo":
		if u, err := url.Parse(path); err == nil && u.Scheme == "file" && u.Path != "" {
			return filepath.Clean(u.Path)
		}
		if strings.HasPrefix(path, "/") {
			return filepath.Clean(path)
		}
	}
	return home
}
