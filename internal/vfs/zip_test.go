package vfs

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// MakeZip-like helper: names ending in "/" are folders.
func mkZip(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	p := filepath.Join(dir, "a.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for _, name := range []string{"docs/", "docs/readme.md", "docs/sub/deep.txt", "top.txt", "../evil.txt", "__MACOSX/._top.txt"} {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if body, ok := files[name]; ok {
			io.WriteString(fw, body)
		} else if name[len(name)-1] != '/' {
			io.WriteString(fw, "body of "+name)
		}
	}
	w.Close()
	f.Close()
	return p
}

func entryNames(es []Entry) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.Name)
	}
	slices.Sort(out)
	return out
}

func TestSplitZip(t *testing.T) {
	for _, c := range []struct {
		in, a, inner string
		ok           bool
	}{
		{"/x/a.zip!", "/x/a.zip", "", true},
		{"/x/a.zip!/sub/dir", "/x/a.zip", "sub/dir", true},
		{"/x/A.ZIP!/s", "/x/A.ZIP", "s", true},
		{"/x/a.zip", "", "", false},
		{"/x/a.zip!x/b.zip!/c", "/x/a.zip!x/b.zip", "c", true},
		{"/x/folder!", "", "", false},
	} {
		a, inner, ok := SplitZip(c.in)
		if a != c.a || inner != c.inner || ok != c.ok {
			t.Errorf("SplitZip(%q) = %q %q %v", c.in, a, inner, ok)
		}
	}
	if RealDir("/x/a.zip!/s") != "/x" || RealDir("/y") != "/y" {
		t.Error("RealDir")
	}
}

func TestZipReadDirStatOpen(t *testing.T) {
	d := t.TempDir()
	z := mkZip(t, d, map[string]string{"top.txt": "hello top"})
	root := ZipRoot(z)
	es, err := ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := entryNames(es); !slices.Equal(got, []string{"docs", "top.txt"}) {
		t.Fatalf("root: %v", got) // ../evil.txt and __MACOSX are skipped
	}
	for _, e := range es {
		if e.Name == "docs" && (!e.IsDir || e.Path != root+"/docs") {
			t.Fatalf("docs entry %+v", e)
		}
	}
	// "docs/sub" has no entry of its own: it is implied by deep.txt.
	es, _ = ReadDir(root + "/docs")
	if got := entryNames(es); !slices.Equal(got, []string{"readme.md", "sub"}) {
		t.Fatalf("docs: %v", got)
	}
	if subs := Subdirs(root+"/docs", false); !slices.Equal(subs, []string{"sub"}) {
		t.Fatalf("Subdirs: %v", subs)
	}
	if _, err := ReadDir(root + "/nope"); err == nil {
		t.Fatal("missing dir listed")
	}
	if e, err := Stat(root + "/docs/sub"); err != nil || !e.IsDir {
		t.Fatalf("Stat sub: %+v %v", e, err)
	}
	if e, err := Stat(root); err != nil || !e.IsDir {
		t.Fatalf("Stat root: %+v %v", e, err)
	}
	rc, err := Open(root + "/top.txt")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	if string(b) != "hello top" {
		t.Fatalf("Open read %q", b)
	}
}

func TestZipExtract(t *testing.T) {
	d := t.TempDir()
	root := ZipRoot(mkZip(t, d, nil))
	out := filepath.Join(d, "out")
	if err := Extract(root+"/docs", out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "sub", "deep.txt"))
	if err != nil || string(b) != "body of docs/sub/deep.txt" {
		t.Fatalf("deep.txt: %q %v", b, err)
	}
	f := filepath.Join(d, "one.txt")
	if err := Extract(root+"/top.txt", f); err != nil {
		t.Fatal(err)
	}
	if err := Extract(root+"/top.txt", f); err == nil {
		t.Fatal("Extract overwrote an existing file")
	}
	if _, err := os.Stat(filepath.Join(d, "evil.txt")); err == nil {
		t.Fatal("zip slip")
	}
}
