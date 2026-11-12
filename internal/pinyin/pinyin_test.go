package pinyin

import "testing"

func TestInitials(t *testing.T) {
	if len(table) != last-first+1 {
		t.Fatalf("table has %d, want %d", len(table), last-first+1)
	}
	for in, want := range map[string]string{
		"模型查询.txt":  "mxcx.txt",
		"2026年报告":   "2026nbg",
		"Readme.MD": "readme.md",
		"瞳瞳在留卡":     "ttzlk",
	} {
		if got := Initials(in); got != want {
			t.Errorf("Initials(%q) = %q, want %q", in, got, want)
		}
	}
	if !HasHan("a模b") || HasHan("abc") || HasHan("café") {
		t.Fatal("HasHan")
	}
}

func BenchmarkInitials(b *testing.B) {
	for b.Loop() {
		Initials("雨夜红披风英雄冲向坠落的城市.png")
	}
}
