package macfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreRefusesExisting(t *testing.T) {
	d := t.TempDir()
	a, b := filepath.Join(d, "a"), filepath.Join(d, "b")
	if err := os.WriteFile(a, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Restore(a, b); err == nil {
		t.Fatal("overwrote existing target")
	}
	data, err := os.ReadFile(b)
	if err != nil || string(data) != "keep" {
		t.Fatal(string(data), err)
	}
	c := filepath.Join(d, "restored")
	if err := Restore(a, c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(a); !os.IsNotExist(err) {
		t.Fatal("source remained", err)
	}
}

// Opt-in only: every successfully trashed probe is restored immediately.
func TestNativeTrashRoundTrip(t *testing.T) {
	if os.Getenv("GOFINDER_NATIVE_TRASH_TEST") != "1" {
		t.Skip("real trash test is opt-in")
	}
	d := t.TempDir()
	var destinations []string
	for i := 0; i < 2; i++ {
		p := filepath.Join(d, "goFinder-undo-probe \"'$\n.txt")
		if err := os.WriteFile(p, []byte("probe"), 0600); err != nil {
			t.Fatal(err)
		}
		before, _ := os.Lstat(p)
		got, err := Trash(p)
		if err != nil || got == "" {
			t.Fatalf("Trash: %q %v", got, err)
		}
		destinations = append(destinations, got)
		after, err := os.Lstat(got)
		if err != nil || !os.SameFile(before, after) {
			t.Fatal("wrong trash identity", err)
		}
	}
	if destinations[0] == destinations[1] {
		t.Fatal("trash name collision")
	}
	for i, got := range destinations {
		if err := Restore(got, filepath.Join(d, []string{"restore-a", "restore-b"}[i])); err != nil {
			t.Fatal(err)
		}
	}
}
