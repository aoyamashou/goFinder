package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	out := "# branch.oid 1234\x00# branch.head main\x00" +
		"1 .M N... 100644 100644 100644 aaa bbb src/a.go\x00" +
		"1 A. N... 000000 100644 100644 000 bbb new file.txt\x00" +
		"1 D. N... 100644 000000 000000 aaa 000 gone.txt\x00" +
		"2 R. N... 100644 100644 100644 aaa bbb R100 docs/new.md\x00docs/old.md\x00" +
		"u UU N... 100644 100644 100644 100644 a b c conflict.txt\x00" +
		"? scratch.txt\x00? tmp/\x00"
	s := Parse([]byte(out))
	if s.Branch != "main" {
		t.Fatalf("branch %q", s.Branch)
	}
	for rel, want := range map[string]string{
		"src/a.go": "M", "new file.txt": "A", "gone.txt": "D", "docs/new.md": "R",
		"docs/old.md": "", "conflict.txt": "U", "scratch.txt": "??", "tmp": "??",
		"tmp/deep/x": "??", "clean.txt": "",
	} {
		if got := s.Mark(rel, false); got != want {
			t.Errorf("Mark(%q) = %q, want %q", rel, got, want)
		}
	}
	if s.Mark("src", true) != "•" || s.Mark("docs", true) != "•" || s.Mark("other", true) != "" {
		t.Errorf("folder marks: src %q docs %q", s.Mark("src", true), s.Mark("docs", true))
	}
	if s.Len() != 7 {
		t.Errorf("Len = %d", s.Len())
	}
}

// TestRealRepo needs git on the machine.
func TestRealRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "trunk")
	os.MkdirAll(filepath.Join(root, "sub"), 0o755)
	os.WriteFile(filepath.Join(root, "sub", "kept.txt"), []byte("1"), 0o644)
	os.WriteFile(filepath.Join(root, "staged.txt"), []byte("1"), 0o644)
	run("add", "sub/kept.txt")
	run("commit", "-q", "-m", "init")
	os.WriteFile(filepath.Join(root, "sub", "kept.txt"), []byte("2"), 0o644)
	run("add", "staged.txt")
	os.WriteFile(filepath.Join(root, "loose.txt"), []byte("1"), 0o644)

	got, ok := FindRoot(filepath.Join(root, "sub"))
	if !ok || got != root {
		t.Fatalf("FindRoot = %q %v", got, ok)
	}
	if _, ok := FindRoot(t.TempDir()); ok {
		t.Fatal("FindRoot found a repo in a plain temp dir")
	}
	s, err := Load(root, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if s.Branch != "trunk" || s.Mark("sub/kept.txt", false) != "M" || s.Mark("staged.txt", false) != "A" ||
		s.Mark("loose.txt", false) != "??" || s.Mark("sub", true) != "•" {
		t.Fatalf("status: branch %q kept %q staged %q loose %q sub %q", s.Branch,
			s.Mark("sub/kept.txt", false), s.Mark("staged.txt", false), s.Mark("loose.txt", false), s.Mark("sub", true))
	}
}
