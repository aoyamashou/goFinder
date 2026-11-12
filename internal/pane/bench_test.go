package pane

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gofinder/internal/vfs"
)

// BenchmarkOpen10k measures what opening a folder of 10,000 files costs:
// reading it, then sorting and filtering for display.
func BenchmarkOpen10k(b *testing.B) {
	dir := b.TempDir()
	for i := range 10000 {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("file-%05d.txt", i)), nil, 0o644)
	}
	b.ResetTimer()
	for b.Loop() {
		es, err := vfs.ReadDir(dir)
		if err != nil {
			b.Fatal(err)
		}
		t := New(dir)
		t.SetEntries(es)
		if len(t.Entries()) != 10000 {
			b.Fatal(len(t.Entries()))
		}
	}
}

// BenchmarkFilterChinese10k: a filter over 10k names in Chinese, matched by
// the first letters of their readings.
func BenchmarkFilterChinese10k(b *testing.B) {
	names := make([]string, 10000)
	for i := range names {
		names[i] = fmt.Sprintf("雨夜红披风英雄冲向坠落的城市-%05d.png", i)
	}
	for b.Loop() {
		n := 0
		for _, s := range names {
			if MatchName(s, "yyhpf") {
				n++
			}
		}
	}
}
