package main

import (
	"os"
	"path/filepath"
)

// oldName is the app's name before goFinder: its data folders are moved
// to the new name's at start (moveOld).
const oldName = "MygoFinder"

// moveOld moves the folder old to dir, for a folder named after the app
// that the rename left behind. It does so only while dir is missing or
// empty (MyGo makes the data folder on first use); otherwise, or when
// old is not there, it leaves both alone.
func moveOld(old, dir string) error {
	if st, err := os.Stat(old); err != nil || !st.IsDir() {
		return nil
	}
	if es, err := os.ReadDir(dir); err == nil {
		if len(es) > 0 {
			return nil
		}
		if err := os.Remove(dir); err != nil {
			return err
		}
	}
	return os.Rename(old, dir)
}

// besideOld is dir's sibling with the old name.
func besideOld(dir string) string { return filepath.Join(filepath.Dir(dir), oldName) }
