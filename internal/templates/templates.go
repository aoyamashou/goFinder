// Package templates lists the files new files can start from: each regular
// file in the templates folder is one, named as the new file will be.
package templates

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// samples are written on first launch, so there is something to start from
// and a folder to add one's own to.
var samples = map[string]string{
	"Untitled.txt": "",
	"Untitled.md":  "# 标题\n\n",
	"main.go":      "package main\n\nfunc main() {\n}\n",
}

// Ensure creates dir with the samples when it does not exist yet. A folder
// the user emptied stays empty.
func Ensure(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, body := range samples {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// List returns the template names in dir, sorted; dot files and folders
// are left out.
func List(dir string) []string {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, de := range des {
		if de.Type()&fs.ModeType != 0 || strings.HasPrefix(de.Name(), ".") {
			continue
		}
		out = append(out, de.Name())
	}
	slices.SortFunc(out, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	return out
}

// Read returns the template's content.
func Read(dir, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(dir, filepath.Base(name)))
}
