package favorites

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestFileStoreAddRemove(t *testing.T) {
	s := File{Path: filepath.Join(t.TempDir(), "sub", "favorites.json")}
	if l, err := s.Load(); err != nil || l != nil {
		t.Fatalf("missing file: %v %v", l, err)
	}
	l, ok := Add(nil, "/a")
	l, _ = Add(l, "/b")
	if _, again := Add(l, "/a"); !ok || again {
		t.Fatal("Add reported wrong change")
	}
	if err := s.Save(l); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil || !slices.Equal(got, []string{"/a", "/b"}) {
		t.Fatalf("Load = %v %v", got, err)
	}
	got, ok = Remove(got, "/a")
	if !ok || !slices.Equal(got, []string{"/b"}) {
		t.Fatalf("Remove = %v", got)
	}
	if _, ok := Remove(got, "/zzz"); ok {
		t.Fatal("Remove of missing reported a change")
	}
}

func TestMerge(t *testing.T) {
	cases := []struct {
		name                    string
		base, local, disk, want []string
	}{
		{"other Mac added", []string{"/a"}, []string{"/a"}, []string{"/a", "/b"}, []string{"/a", "/b"}},
		{"other Mac removed", []string{"/a", "/b"}, []string{"/a", "/b"}, []string{"/b"}, []string{"/b"}},
		{"both added", []string{"/a"}, []string{"/a", "/c"}, []string{"/a", "/b"}, []string{"/a", "/b", "/c"}},
		{"removed here, kept there", []string{"/a", "/b"}, []string{"/a"}, []string{"/a", "/b", "/d"}, []string{"/a", "/d"}},
		{"same add both sides, dedupe", []string{}, []string{"/x/"}, []string{"/x"}, []string{"/x"}},
		{"disk order wins", []string{"/a", "/b"}, []string{"/a", "/b"}, []string{"/b", "/a"}, []string{"/b", "/a"}},
		{"duplicates on disk", nil, nil, []string{"/a", "/a/"}, []string{"/a"}},
	}
	for _, c := range cases {
		if got := Merge(c.base, c.local, c.disk); !slices.Equal(got, c.want) {
			t.Errorf("%s: Merge = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestICloudAndMigrate(t *testing.T) {
	home := t.TempDir()
	if _, ok := ICloudFile(home); ok {
		t.Fatal("iCloud Drive found in an empty home")
	}
	os.MkdirAll(filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs"), 0o755)
	p, ok := ICloudFile(home)
	if !ok || !strings.HasSuffix(p, "com~apple~CloudDocs/goFinder/favorites.json") {
		t.Fatalf("ICloudFile = %q %v", p, ok)
	}
	local := File{Path: filepath.Join(t.TempDir(), "favorites.json")}
	cloud := File{Path: p}
	if err := Migrate(local, cloud); err != nil { // nothing to migrate
		t.Fatal(err)
	}
	local.Save([]string{"/a", "/b"})
	cloud.Save([]string{"/b", "/c"})
	if err := Migrate(local, cloud); err != nil {
		t.Fatal(err)
	}
	got, _ := cloud.Load()
	if !slices.Equal(got, []string{"/b", "/c", "/a"}) {
		t.Fatalf("migrated = %v", got)
	}
	if _, err := os.Stat(local.Path); !os.IsNotExist(err) {
		t.Fatal("local file still there")
	}
	if _, err := os.Stat(local.Path + ".migrated"); err != nil {
		t.Fatal(err)
	}
}
