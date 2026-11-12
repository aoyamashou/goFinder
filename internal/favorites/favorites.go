// Package favorites keeps the folders of the sidebar's Favorites section.
// Where they are stored is a Store, so a synced one (iCloud, phase 3) can
// replace the local file without the app changing.
package favorites

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// Store loads and saves the list of favorite folders, in order.
type Store interface {
	Load() ([]string, error)
	Save([]string) error
}

// File is a Store in a JSON file: {"folders": ["/a", "/b"]}.
type File struct{ Path string }

type fileData struct {
	Folders []string `json:"folders"`
}

// Load reads the file; a missing file is an empty list.
func (f File) Load() ([]string, error) {
	b, err := os.ReadFile(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var d fileData
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	return d.Folders, nil
}

// Save writes the file, through a temporary file so a crash never leaves
// half of it.
func (f File) Save(folders []string) error {
	b, err := json.MarshalIndent(fileData{Folders: folders}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
		return err
	}
	tmp := f.Path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, f.Path)
}

// Add appends dir unless it is there already; it reports a change.
func Add(list []string, dir string) ([]string, bool) {
	if slices.Contains(list, dir) {
		return list, false
	}
	return append(slices.Clip(list), dir), true
}

// Remove takes dir out; it reports a change.
func Remove(list []string, dir string) ([]string, bool) {
	i := slices.Index(list, dir)
	if i < 0 {
		return list, false
	}
	return slices.Delete(slices.Clone(list), i, i+1), true
}

// Merge combines the list on disk, changed by another Mac, with this one's,
// given base, the list both last agreed on (what was last read or saved):
//   - the order is the disk's, then folders added here since base, in
//     their order here;
//   - folders removed here since base are dropped, even if the disk still
//     has them;
//   - folders removed on the other Mac stay removed unless added here
//     since base;
//   - duplicates (by cleaned path) are kept once.
func Merge(base, local, disk []string) []string {
	inBase := map[string]bool{}
	for _, p := range base {
		inBase[filepath.Clean(p)] = true
	}
	inLocal := map[string]bool{}
	for _, p := range local {
		inLocal[filepath.Clean(p)] = true
	}
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		p = filepath.Clean(p)
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, p := range disk {
		if c := filepath.Clean(p); inBase[c] && !inLocal[c] {
			continue // removed here
		}
		add(p)
	}
	for _, p := range local {
		if !inBase[filepath.Clean(p)] {
			add(p) // added here
		}
	}
	return out
}

// ICloudFile is where synced favorites live, in the iCloud Drive folder
// under home; ok is false when iCloud Drive is not there.
func ICloudFile(home string) (path string, ok bool) {
	drive := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs")
	if st, err := os.Stat(drive); err != nil || !st.IsDir() {
		return "", false
	}
	return filepath.Join(drive, "goFinder", "favorites.json"), true
}

// Migrate moves the favorites of the local file into to, merged with what
// to has already, then renames the local file to favorites.json.migrated
// so it is not read again. Nothing happens when from is missing.
func Migrate(from, to File) error {
	if _, err := os.Stat(from.Path); err != nil {
		return nil
	}
	old, err := from.Load()
	if err != nil {
		return err
	}
	cur, err := to.Load()
	if err != nil {
		return err
	}
	if err := to.Save(Merge(nil, old, cur)); err != nil {
		return err
	}
	return os.Rename(from.Path, from.Path+".migrated")
}
