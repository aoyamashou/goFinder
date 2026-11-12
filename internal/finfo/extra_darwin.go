package finfo

import (
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// finderInfo is the xattr holding Finder's flags; the flag word is at
// bytes 8-9, and 0x0010 in it says the extension is hidden.
const finderInfo = "com.apple.FinderInfo"

// readExtra reads what only the system knows: when the item was added
// (Spotlight) and whether Finder hides its extension. A failure leaves
// the field empty.
func readExtra(in *Info) {
	if out, err := exec.Command("mdls", "-name", "kMDItemDateAdded", "-raw", in.Path).Output(); err == nil {
		if t, err := time.Parse("2006-01-02 15:04:05 -0700", strings.TrimSpace(string(out))); err == nil {
			in.Added = t
		}
	}
	if b, ok := readFinderInfo(in.Path); ok {
		in.HideExt = b[9]&0x10 != 0
	}
}

func readFinderInfo(path string) ([]byte, bool) {
	out, err := exec.Command("xattr", "-p", "-x", finderInfo, path).Output()
	if err != nil {
		return nil, false
	}
	b, err := hex.DecodeString(strings.Join(strings.Fields(string(out)), ""))
	if err != nil || len(b) < 10 {
		return nil, false
	}
	return b, true
}

// SetHideExt makes Finder hide path's extension, or show it, by the flag
// Finder itself sets in its FinderInfo xattr (no Automation permission).
func SetHideExt(path string, on bool) error {
	b, ok := readFinderInfo(path)
	if !ok {
		b = make([]byte, 32)
	}
	if on {
		b[9] |= 0x10
	} else {
		b[9] &^= 0x10
	}
	if out, err := exec.Command("xattr", "-w", "-x", finderInfo, hex.EncodeToString(b), path).CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// SetHidden sets or clears path's hidden flag.
func SetHidden(path string, on bool) error { return chflags(path, "hidden", "nohidden", on) }

// SetLocked sets or clears path's user immutable flag.
func SetLocked(path string, on bool) error { return chflags(path, "uchg", "nouchg", on) }

func chflags(path, set, clear string, on bool) error {
	flag := clear
	if on {
		flag = set
	}
	if out, err := exec.Command("chflags", flag, path).CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}
