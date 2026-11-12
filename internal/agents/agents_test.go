package agents

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func fake(t *testing.T, dir string, names ...string) {
	t.Helper()
	os.MkdirAll(dir, 0o755)
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func ids(f []Found) []string {
	var s []string
	for _, x := range f {
		s = append(s, x.Entry.ID)
	}
	return s
}

func TestDetect(t *testing.T) {
	a, b := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b")
	fake(t, a, "claude", "orca", "pi")
	fake(t, b, "codex", "cbc", "claude")
	os.WriteFile(filepath.Join(b, "agy"), []byte("x"), 0o644) // not executable
	got := Detect([]string{a, "", b}, IsExecutable)
	want := []string{"claude", "claude-agent-teams", "codebuddy", "codex", "pi"}
	if !slices.Equal(ids(got), want) {
		t.Fatalf("got %v want %v", ids(got), want)
	}
	if got[0].Path != filepath.Join(a, "claude") { // first directory wins
		t.Fatalf("path %s", got[0].Path)
	}
	if got[2].Path != filepath.Join(b, "cbc") { // alias
		t.Fatalf("alias path %s", got[2].Path)
	}
}

func TestDetectRequires(t *testing.T) {
	d := t.TempDir()
	fake(t, d, "orca")
	if got := Detect([]string{d}, IsExecutable); len(got) != 0 {
		t.Fatalf("orca alone: %v", ids(got))
	}
	// the requirement may live in another directory
	e := t.TempDir()
	fake(t, e, "claude")
	if got := ids(Detect([]string{d, e}, IsExecutable)); !slices.Equal(got, []string{"claude", "claude-agent-teams"}) {
		t.Fatalf("got %v", got)
	}
}

func TestDefaultCommand(t *testing.T) {
	for id, want := range map[string]string{
		"claude": "claude --dangerously-skip-permissions", "codex": "codex --dangerously-bypass-approvals-and-sandbox",
		"pi": "pi", "claude-agent-teams": "orca claude-teams --dangerously-skip-permissions",
		"antigravity": "agy --dangerously-skip-permissions",
	} {
		e, ok := ByID(id)
		if !ok || DefaultCommand(e) != want {
			t.Errorf("%s: %q, want %q", id, DefaultCommand(e), want)
		}
	}
}

func TestCatalogUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range Catalog {
		if seen[e.ID] || len(e.Detect) == 0 || e.Launch == "" {
			t.Errorf("bad entry %+v", e)
		}
		seen[e.ID] = true
	}
}

func TestSearchDirs(t *testing.T) {
	d := SearchDirs("/a:/usr/local/bin::/b", "/h")
	if d[0] != "/a" || d[1] != "/usr/local/bin" || d[2] != "/b" || d[3] != "/h/.local/bin" {
		t.Fatalf("%v", d)
	}
	if n := 0; true {
		for _, x := range d {
			if x == "/usr/local/bin" {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("repeated: %v", d)
		}
	}
}
