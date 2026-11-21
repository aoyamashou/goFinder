package vfs

import (
	"net/url"
	"path/filepath"
	"strings"
)

// PastedPath reads clipboard text as one macOS path: an absolute path, one
// under "~", or a file:// URL; quoted, or with the backslash escapes a
// Terminal gives a dropped file. Other text, or more than one line, is not
// a path. It does not look at the disk.
func PastedPath(text string) (string, bool) {
	s := strings.TrimSpace(text)
	if s == "" || len(s) > 4096 || strings.ContainsAny(s, "\n\r\x00") {
		return "", false
	}
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		s = s[1 : len(s)-1]
	} else if strings.HasPrefix(s, "file://") {
		u, err := url.Parse(s)
		if err != nil || u.Host != "" && u.Host != "localhost" {
			return "", false
		}
		s = u.Path
	} else {
		s = unescapeShell(s)
	}
	s = ExpandHome(s)
	if !filepath.IsAbs(s) {
		return "", false
	}
	return filepath.Clean(s), true
}

// unescapeShell drops the backslashes before characters, as in
// "/Users/me/My\ Files".
func unescapeShell(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
