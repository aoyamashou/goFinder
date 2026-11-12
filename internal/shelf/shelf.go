// Package shelf keeps the shelf: paths set aside to copy or move later, in
// the order they were added, saved as a JSON file in the data folder.
package shelf

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// Add appends the paths not on the shelf yet, cleaned, keeping order. It
// returns the new list and how many were added.
func Add(list []string, paths ...string) ([]string, int) {
	out := slices.Clip(list)
	n := 0
	for _, p := range paths {
		if p == "" {
			continue
		}
		p = filepath.Clean(p)
		if slices.Contains(out, p) {
			continue
		}
		out = append(out, p)
		n++
	}
	return out, n
}

// Remove takes the paths off the shelf; it returns how many were there.
func Remove(list []string, paths ...string) ([]string, int) {
	out := slices.Clone(list)
	n := 0
	for _, p := range paths {
		if i := slices.Index(out, filepath.Clean(p)); i >= 0 {
			out = slices.Delete(out, i, i+1)
			n++
		}
	}
	return out, n
}

// Existing is the paths that still exist. It stats each one: call it off
// the main thread.
func Existing(list []string) []string {
	var out []string
	for _, p := range list {
		if _, err := os.Lstat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// File stores the shelf as {"items": ["/a", "/b"]}.
type File struct{ Path string }

type fileData struct {
	Items []string `json:"items"`
}

// Load reads the file; a missing file is an empty shelf.
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
	out, _ := Add(nil, d.Items...)
	return out, nil
}

// Save writes the file through a temporary one.
func (f File) Save(items []string) error {
	b, err := json.MarshalIndent(fileData{Items: items}, "", "  ")
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
