package finder_test

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"testing"

	. "gofinder/internal/finder"
	"gofinder/internal/finder/findertest"
)

func TestParsePlist(t *testing.T) {
	in := map[string]any{
		"s": "plain", "u": "中文名", "n": 42, "t": true, "f": false,
		"d": []byte{1, 2, 3}, "a": []any{"x", 7, UID(3)}, "z": nil,
		"long": "a string longer than fifteen bytes",
	}
	v, err := ParsePlist(findertest.Plist(in))
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]any)
	if m["s"] != "plain" || m["u"] != "中文名" || m["n"] != int64(42) || m["t"] != true || m["f"] != false ||
		!bytes.Equal(m["d"].([]byte), []byte{1, 2, 3}) || m["z"] != nil || m["long"] != in["long"] {
		t.Fatalf("parsed %#v", m)
	}
	if a := m["a"].([]any); a[0] != "x" || a[1] != int64(7) || a[2] != UID(3) {
		t.Fatalf("array %#v", a)
	}
	for _, bad := range [][]byte{nil, []byte("bplist00"), []byte("notaplist" + string(make([]byte, 40)))} {
		if _, err := ParsePlist(bad); err == nil {
			t.Fatalf("parsed %q", bad)
		}
	}
	// Truncated data never panics.
	b := findertest.Plist(in)
	for i := range len(b) {
		ParsePlist(b[:i])
	}
}

func TestParseBookmark(t *testing.T) {
	bm, err := ParseBookmark(findertest.Bookmark("/Users/me/项目", "我的项目"))
	if err != nil || bm.Path != "/Users/me/项目" || bm.Name != "我的项目" {
		t.Fatalf("%+v %v", bm, err)
	}
	if _, err := ParseBookmark([]byte("alis")); err == nil {
		t.Fatal("not a bookmark")
	}
	b := findertest.Bookmark("/a/b", "")
	for i := range len(b) {
		ParseBookmark(b[:i]) // never panics
	}
}

func TestFavorites(t *testing.T) {
	home := t.TempDir()
	for _, d := range []string{"Desktop", "Downloads", "dev", "Documents"} {
		os.Mkdir(filepath.Join(home, d), 0o755)
	}
	dir := ListDir(home)
	os.MkdirAll(dir, 0o755)
	if _, err := Favorites(home); err == nil {
		t.Fatal("no list: want an error, for the fallback")
	}
	os.WriteFile(filepath.Join(dir, "com.apple.LSSharedFileList.FavoriteItems.sfl4"), findertest.List([]findertest.Entry{
		{Path: filepath.Join(home, "dev")},
		{Path: home},
		{Path: filepath.Join(home, "Desktop"), BookmarkName: "桌面"},
		{Path: filepath.Join(home, "Downloads"), Hidden: true},
		{Path: "/nowhere/gone"},
		{BookmarkName: "AirDrop"}, // no file
		{Path: filepath.Join(home, "Documents"), Name: "我的文稿"},
	}), 0o644)
	os.WriteFile(filepath.Join(dir, "com.apple.LSSharedFileList.iCloudItems.sfl3"), findertest.List([]findertest.Entry{
		{Path: filepath.Join(home, "Desktop")},
		{Path: filepath.Join(home, "Downloads")},
		{Path: "/System/Library/x.cannedSearch"},
	}), 0o644)
	got, err := Favorites(home)
	if err != nil {
		t.Fatal(err)
	}
	want := []Item{
		{"dev", filepath.Join(home, "dev"), Folder},
		{filepath.Base(home), home, Home},
		{"桌面", filepath.Join(home, "Desktop"), Desktop},
		{"我的文稿", filepath.Join(home, "Documents"), Documents},
		{"下载", filepath.Join(home, "Downloads"), Downloads}, // from iCloud's list
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
	// A broken file is an error too.
	os.WriteFile(filepath.Join(dir, "com.apple.LSSharedFileList.FavoriteItems.sfl4"), []byte("bplist00junk"), 0o644)
	if _, err := Favorites(home); err == nil {
		t.Fatal("broken list: want an error")
	}
}

func TestLocations(t *testing.T) {
	home, vols := t.TempDir(), t.TempDir()
	os.MkdirAll(ICloudDrive(home), 0o755)
	cs := filepath.Join(home, "Library", "CloudStorage")
	for _, d := range []string{"GoogleDrive-me@example.com", "Dropbox", ".hidden"} {
		os.MkdirAll(filepath.Join(cs, d), 0o755)
	}
	for _, d := range []string{"USBDISK", ".Trashes", "77421E38-6F22-4598-BE16-E79E7AAA6B1F"} {
		os.Mkdir(filepath.Join(vols, d), 0o755)
	}
	os.Symlink("/", filepath.Join(vols, "Macintosh HD"))
	got := Locations(home, vols)
	want := []Item{
		{"iCloud云盘", ICloudDrive(home), ICloud},
		{"Dropbox", filepath.Join(cs, "Dropbox"), Cloud},
		{"Google Drive", filepath.Join(cs, "GoogleDrive-me@example.com"), Cloud},
		{"USBDISK", filepath.Join(vols, "USBDISK"), Volume},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
	// Two accounts of one provider show their folders' names.
	os.Mkdir(filepath.Join(cs, "GoogleDrive-you@example.com"), 0o755)
	got = Locations(home, vols)
	if got[2].Name != "GoogleDrive-me@example.com" || got[3].Name != "GoogleDrive-you@example.com" {
		t.Fatalf("two accounts %v", got)
	}
	if got := Locations("", vols); len(got) != 1 {
		t.Fatalf("no home: %v", got)
	}
}

func TestNewWindowPath(t *testing.T) {
	home := "/Users/me"
	for _, c := range []struct{ target, path, want string }{
		{"PfHm", "", home},
		{"PfDe", "", "/Users/me/Desktop"},
		{"PfDo", "", "/Users/me/Documents"},
		{"PfID", "", ICloudDrive(home)},
		{"PfCm", "", "/"},
		{"PfVo", "", "/"},
		{"PfLo", "file:///Users/me/My%20Work/", "/Users/me/My Work"},
		{"PfLo", "/opt/x", "/opt/x"},
		{"PfLo", "", home},
		{"PfAF", "", home}, // Recents: no folder
		{"", "", home},
	} {
		if got := NewWindowPath(home, c.target, c.path); got != c.want {
			t.Errorf("%s %q: %q, want %q", c.target, c.path, got, c.want)
		}
	}
}

func TestNewWindowDir(t *testing.T) {
	home := t.TempDir()
	if got := NewWindowDir(home); got != home {
		t.Fatalf("no prefs: %q", got)
	}
	prefs := filepath.Join(home, "Library", "Preferences")
	os.MkdirAll(prefs, 0o755)
	write := func(m map[string]any) {
		os.WriteFile(filepath.Join(prefs, "com.apple.finder.plist"), findertest.Plist(m), 0o644)
	}
	write(map[string]any{"NewWindowTarget": "PfDe"})
	if got := NewWindowDir(home); got != home {
		t.Fatalf("missing Desktop: %q, want home", got)
	}
	os.Mkdir(filepath.Join(home, "Desktop"), 0o755)
	if got := NewWindowDir(home); got != filepath.Join(home, "Desktop") {
		t.Fatalf("PfDe: %q", got)
	}
	work := filepath.Join(home, "工作 区")
	os.Mkdir(work, 0o755)
	write(map[string]any{"NewWindowTarget": "PfLo", "NewWindowTargetPath": (&url.URL{Scheme: "file", Path: work + "/"}).String()})
	if got := NewWindowDir(home); got != work {
		t.Fatalf("PfLo: %q", got)
	}
}
