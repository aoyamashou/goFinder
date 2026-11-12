package shellcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var nasty = []string{
	"plain.txt",
	"with space.md",
	"a'; touch PWNED; '",
	`b"; touch PWNED; "`,
	"$(touch PWNED)",
	"`touch PWNED`",
	"semi;touch PWNED",
	"new\nline",
	"-n",
	"*",
}

// Each name reaches the command as exactly one argument, unchanged, and
// nothing in a name runs — also when the user quoted the placeholder.
func TestExpandIsSafe(t *testing.T) {
	for _, tmpl := range []string{
		`printf '%s\0' {selection}`,
		`printf '%s\0' "{selection}"`,
		`printf '%s\0' '{selection}'`,
	} {
		dir := t.TempDir()
		cmd := Expand(tmpl, dir, nasty)
		c := exec.Command("/bin/sh", "-c", cmd)
		c.Dir = dir
		out, err := c.Output()
		if err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
		got := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
		if strings.Join(got, "|") != strings.Join(nasty, "|") {
			t.Errorf("%s:\n got %q\nwant %q", tmpl, got, nasty)
		}
		if _, err := os.Stat(filepath.Join(dir, "PWNED")); err == nil {
			t.Fatalf("%s: a file name ran a command", tmpl)
		}
	}
}

func TestExpandPath(t *testing.T) {
	got := Expand(`cd {path} && ls`, "/tmp/it's here", nil)
	if got != `cd '/tmp/it'\''s here' && ls` {
		t.Fatalf("got %s", got)
	}
	if got := Expand("echo {selection}", "/", nil); got != "echo " {
		t.Fatalf("empty selection: %q", got)
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	if _, err := Run(dir, "touch "+Quote("made it")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "made it")); err != nil {
		t.Fatal("command did not run in dir")
	}
	out, err := Run(dir, "echo oops >&2; exit 3")
	if err == nil || out != "oops" {
		t.Fatalf("failure: %q %v", out, err)
	}
}
