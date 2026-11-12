package finfo

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"
)

func TestStatFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "run.sh")
	os.WriteFile(p, []byte("echo hi\n"), 0o755)
	os.Chmod(p, 0o755)
	in, err := Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if in.Name != "run.sh" || in.Dir || in.Link || in.Size != 8 || in.Kind != "Shell 脚本" {
		t.Fatalf("info %+v", in)
	}
	if in.Perm() != "rwxr-xr-x" || in.Octal() != "755" || !in.Exec() {
		t.Fatalf("perm %s %s exec %v", in.Perm(), in.Octal(), in.Exec())
	}
	if in.Created.IsZero() || in.Accessed.IsZero() || time.Since(in.Modified) > time.Minute {
		t.Fatalf("times %v %v %v", in.Created, in.Modified, in.Accessed)
	}
	if u, err := user.Current(); err == nil && in.Owner != u.Username {
		t.Fatalf("owner %q, want %q", in.Owner, u.Username)
	}
	if in.Group == "" {
		t.Fatal("no group")
	}
}

func TestStatLinkAndDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.Mkdir(sub, 0o700)
	ln := filepath.Join(dir, "ln")
	os.Symlink(sub, ln)
	in, err := Stat(ln)
	if err != nil {
		t.Fatal(err)
	}
	if !in.Link || !in.Dir || in.Target != sub || in.Kind != "替身（符号链接）" {
		t.Fatalf("link %+v", in)
	}
	d, _ := Stat(sub)
	if !d.Dir || d.Kind != "文件夹" || d.Perm() != "rwx------" || d.Octal() != "700" || d.Exec() {
		t.Fatalf("dir %+v %s", d, d.Perm())
	}
	os.Chmod(sub, os.ModeSticky|0o777)
	if d, _ = Stat(sub); d.Octal() != "1777" {
		t.Fatalf("sticky: %s", d.Octal())
	}
}

func TestKind(t *testing.T) {
	cases := []struct {
		name      string
		dir, link bool
		want      string
	}{
		{"a.PDF", false, false, "PDF 文稿"},
		{"x.qqq", false, false, "QQQ 文件"},
		{"Makefile", false, false, "文稿"},
		{".zshrc", false, false, "文稿"},
		{"Safari.app", true, false, "应用程序"},
		{"photos.v1", true, false, "文件夹"},
	}
	for _, c := range cases {
		if got := Kind(c.name, c.dir, c.link); got != c.want {
			t.Errorf("Kind(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestCount(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755)
	os.WriteFile(filepath.Join(dir, "x"), make([]byte, 100), 0o644)
	os.WriteFile(filepath.Join(dir, "a", "y"), make([]byte, 20), 0o644)
	os.WriteFile(filepath.Join(dir, "a", "b", "z"), make([]byte, 3), 0o644)
	s, err := Count(dir, 100000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Items != 2 || s.Files != 1 || s.Dirs != 1 || s.Size != 123 || s.Capped || s.Scanned != 5 {
		t.Fatalf("stats %+v", s)
	}
	s, _ = Count(dir, 2, nil)
	if !s.Capped || s.Scanned != 2 || s.Items != 2 {
		t.Fatalf("capped %+v", s)
	}
	s, _ = Count(dir, 100000, func() bool { return true })
	if !s.Capped {
		t.Fatalf("stopped %+v", s)
	}
	if _, err := Count(filepath.Join(dir, "none"), 10, nil); err == nil {
		t.Fatal("no error for a missing folder")
	}
}

func TestSizes(t *testing.T) {
	if Size(999) != "999 B" || Size(1500) != "1.5 KB" || Size(2_000_000) != "2.0 MB" {
		t.Fatal(Size(999), Size(1500), Size(2_000_000))
	}
	if Bytes(1234567) != "1,234,567 字节" || Bytes(12) != "12 字节" || Bytes(123456) != "123,456 字节" {
		t.Fatal(Bytes(1234567), Bytes(12), Bytes(123456))
	}
}

func TestExtraFlags(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.JPG")
	os.WriteFile(p, []byte("x"), 0o644)
	in, _ := Stat(p)
	if in.Ext != "JPG" || in.Hidden || in.Locked || in.HideExt {
		t.Fatalf("fresh %+v", in)
	}
	if in.Added.IsZero() {
		t.Log("Added unknown (Spotlight does not index this folder)")
	}
	if err := SetHidden(p, true); err != nil {
		t.Skip("chflags:", err)
	}
	if err := SetHideExt(p, true); err != nil {
		t.Fatal(err)
	}
	if err := SetLocked(p, true); err != nil {
		t.Fatal(err)
	}
	in, _ = Stat(p)
	if !in.Hidden || !in.Locked || !in.HideExt {
		t.Fatalf("set %+v", in)
	}
	SetLocked(p, false) // a locked item takes no xattr
	SetHidden(p, false)
	SetHideExt(p, false)
	in, _ = Stat(p)
	if in.Hidden || in.Locked || in.HideExt {
		t.Fatalf("cleared %+v", in)
	}
}

func TestSetPermAndSizeLong(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	os.WriteFile(p, nil, 0o644)
	if err := SetPerm(p, 0o640); err != nil {
		t.Fatal(err)
	}
	in, _ := Stat(p)
	if in.Perm() != "rw-r-----" || in.Octal() != "640" {
		t.Fatalf("%s %s", in.Perm(), in.Octal())
	}
	if got := SizeLong(1469302); got != "1.5 MB（1 469 302 字节）" {
		t.Fatal(got)
	}
	if got := SizeLong(12); got != "12 B（12 字节）" {
		t.Fatal(got)
	}
}
