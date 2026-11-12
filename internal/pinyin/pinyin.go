// Package pinyin gives the first letters of the Mandarin readings of a
// name, so that "模型查询.txt" is found by "mxcx", from a table made with
// macOS's own transform (gen/gen.swift); a character with more readings
// has the one the system gives first.
package pinyin

//go:generate sh -c "swift gen/gen.swift > table.go"

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const first, last = 0x4E00, 0x9FFF

// HasHan tells whether s has a character the table covers.
func HasHan(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			for _, r := range s[i:] {
				if r >= first && r <= last {
					return true
				}
			}
			return false
		}
	}
	return false
}

// Initials is s in lowercase with each character the table covers made
// the first letter of its reading: "模型查询.txt" is "mxcx.txt".
func Initials(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= first && r <= last {
			if c := table[r-first]; c != '0' {
				b.WriteByte(c)
				continue
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
