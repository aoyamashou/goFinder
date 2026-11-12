package templates

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestEnsureListRead(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "templates")
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	if got := List(dir); !slices.Equal(got, []string{"main.go", "Untitled.md", "Untitled.txt"}) {
		t.Fatalf("List = %v", got)
	}
	b, err := Read(dir, "Untitled.md")
	if err != nil || string(b) != "# 标题\n\n" {
		t.Fatalf("Read = %q %v", b, err)
	}
	// The user's choices stay: Ensure does not refill an existing folder.
	os.Remove(filepath.Join(dir, "main.go"))
	os.Mkdir(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, ".DS_Store"), nil, 0o644)
	Ensure(dir)
	if got := List(dir); !slices.Equal(got, []string{"Untitled.md", "Untitled.txt"}) {
		t.Fatalf("after edits List = %v", got)
	}
}
