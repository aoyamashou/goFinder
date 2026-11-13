package appinstall

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func bundle(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, "Contents", "MacOS"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"Contents/Info.plist", "Contents/MacOS/Demo"} {
		if err := os.WriteFile(filepath.Join(path, file), []byte("fixture"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestEligibility(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	src := bundle(t, filepath.Join(root, "Demo.APP"))
	installed := bundle(t, filepath.Join(home, "Applications", "Tools", "Demo.app"))
	link := filepath.Join(root, "Link.app")
	if err := os.Symlink(src, link); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(root, "Empty.app")
	os.Mkdir(empty, 0755)
	file := filepath.Join(root, "File.app")
	os.WriteFile(file, nil, 0644)
	for _, c := range []struct {
		path string
		want bool
	}{{src, true}, {installed, false}, {link, false}, {empty, false}, {file, false}, {root + "/a.zip!/Demo.app", false}} {
		if got := Eligible(c.path, home); got != c.want {
			t.Errorf("%s: %v", c.path, got)
		}
	}
	alias := filepath.Join(root, "home-alias")
	os.Symlink(home, alias)
	if !Installed(filepath.Join(alias, "Applications", "Tools", "Demo.app"), home) {
		t.Fatal("aliased installed path")
	}
	sibling := bundle(t, filepath.Join(home, "Applications-other", "Demo.app"))
	if !Eligible(sibling, home) {
		t.Fatal("prefix is not an installed root")
	}
}

func TestCopyArgumentsAndConflicts(t *testing.T) {
	for _, unq := range []bool{false, true} {
		t.Run(map[bool]string{false: "preserve", true: "unquarantine"}[unq], func(t *testing.T) {
			root := t.TempDir()
			src := bundle(t, filepath.Join(root, "-中 文 ' app.app"))
			dstDir := filepath.Join(root, "Applications")
			os.Mkdir(dstDir, 0755)
			calls := 0
			run := func(name string, args ...string) error {
				calls++
				want := []string{"--rsrc", "--extattr", "--acl", "--qtn"}
				if name == "/usr/bin/xattr" {
					if !unq || len(args) != 5 || !reflect.DeepEqual(args[:4], []string{"-d", "-r", "-s", "com.apple.quarantine"}) {
						t.Fatal(args)
					}
					return nil
				}
				if name != "/usr/bin/ditto" || !reflect.DeepEqual(args[:len(want)], want) || args[len(want)] != src || len(args) != len(want)+2 {
					t.Fatalf("command %s %q", name, args)
				}
				if filepath.Dir(filepath.Dir(args[len(args)-1])) != dstDir || !strings.HasPrefix(filepath.Base(filepath.Dir(args[len(args)-1])), "."+filepath.Base(src)+".gofinder-") {
					t.Fatal(args)
				}
				os.Mkdir(args[len(args)-1], 0755)
				return os.WriteFile(filepath.Join(args[len(args)-1], "copied"), []byte("yes"), 0644)
			}
			trash := func(path string) error { return os.RemoveAll(path) }
			dst, err := Copy(src, dstDir, unq, run, trash)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = os.Stat(filepath.Join(dst, "copied")); err != nil {
				t.Fatal(err)
			}
			if _, err = Copy(src, dstDir, unq, run, trash); err == nil || calls != map[bool]int{false: 1, true: 2}[unq] {
				t.Fatalf("existing target: %v calls=%d", err, calls)
			}
		})
	}
}

func TestCopyFailureAndLateConflict(t *testing.T) {
	for _, kind := range []string{"copy", "late-conflict", "trash"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			src := bundle(t, filepath.Join(root, "Demo.app"))
			dstDir := filepath.Join(root, "Applications")
			os.Mkdir(dstDir, 0755)
			var stage, trashed string
			run := func(_ string, args ...string) error {
				stage = filepath.Dir(args[len(args)-1])
				if kind == "late-conflict" {
					return os.WriteFile(filepath.Join(dstDir, "Demo.app"), []byte("existing"), 0644)
				}
				return errors.New("copy failed")
			}
			trash := func(p string) error {
				trashed = p
				if kind == "trash" {
					return errors.New("trash failed")
				}
				return os.RemoveAll(p)
			}
			_, err := Copy(src, dstDir, false, run, trash)
			if err == nil || trashed != stage || stage == "" {
				t.Fatalf("err=%v trashed=%q stage=%q", err, trashed, stage)
			}
			if kind == "trash" && !strings.Contains(err.Error(), stage) {
				t.Fatal(err)
			}
			if kind == "late-conflict" {
				b, _ := os.ReadFile(filepath.Join(dstDir, "Demo.app"))
				if string(b) != "existing" {
					t.Fatal("overwritten")
				}
			}
		})
	}
}

// Real system commands are restricted to fixtures in TempDir, never /Applications.
func TestDittoMetadata(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS ditto")
	}
	root := t.TempDir()
	src := bundle(t, filepath.Join(root, "下载 ' Demo.app"))
	nodes := []string{src, filepath.Join(src, "Contents"), filepath.Join(src, "Contents", "Info.plist"), filepath.Join(src, "Contents", "MacOS", "Demo")}
	cmd := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("/usr/bin/xattr", args...).CombinedOutput(); err != nil {
			t.Fatalf("xattr %q: %v %s", args, err, out)
		}
	}
	for _, p := range nodes {
		cmd("-w", "com.apple.quarantine", "0081;00000000;goFinder-test;", p)
		cmd("-w", "im.ryou.gofinder.test", "retained", p)
	}
	os.Symlink("MacOS/Demo", filepath.Join(src, "Contents", "link"))
	// Record --noqtn behavior separately; the production fallback is verified below.
	probe := filepath.Join(root, "noqtn-probe.app")
	if err := Run("/usr/bin/ditto", "--rsrc", "--noqtn", src, probe); err != nil {
		t.Fatal(err)
	}
	_, probeErr := exec.Command("/usr/bin/xattr", "-p", "com.apple.quarantine", probe).Output()
	t.Logf("ditto --noqtn retained root quarantine: %v", probeErr == nil)
	for _, unq := range []bool{false, true} {
		dir := filepath.Join(root, map[bool]string{false: "preserve", true: "unquarantine"}[unq])
		os.Mkdir(dir, 0755)
		dst, err := Copy(src, dir, unq, Run, func(p string) error { return os.RemoveAll(p) })
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range nodes {
			rel, _ := filepath.Rel(src, p)
			target := filepath.Join(dst, rel)
			out, err := exec.Command("/usr/bin/xattr", "-p", "im.ryou.gofinder.test", target).CombinedOutput()
			if err != nil || strings.TrimSpace(string(out)) != "retained" {
				t.Fatalf("lost xattr %s: %v %s", target, err, out)
			}
			_, err = exec.Command("/usr/bin/xattr", "-p", "com.apple.quarantine", target).Output()
			if (err == nil) == unq {
				t.Fatalf("quarantine unq=%v path=%s err=%v", unq, target, err)
			}
		}
		if link, err := os.Readlink(filepath.Join(dst, "Contents", "link")); err != nil || link != "MacOS/Demo" {
			t.Fatalf("link %q %v", link, err)
		}
		if st, err := os.Stat(filepath.Join(dst, "Contents", "MacOS", "Demo")); err != nil || st.Mode().Perm() != 0755 {
			t.Fatalf("executable mode %v %v", st, err)
		}
	}
	for _, p := range nodes {
		if err := exec.Command("/usr/bin/xattr", "-p", "com.apple.quarantine", p).Run(); err != nil {
			t.Fatal("source changed", err)
		}
	}
}

func TestDanglingTargetAndUnquarantineFailure(t *testing.T) {
	root := t.TempDir()
	src := bundle(t, filepath.Join(root, "Demo.app"))
	dstDir := filepath.Join(root, "Applications")
	os.Mkdir(dstDir, 0755)
	dst := filepath.Join(dstDir, "Demo.app")
	os.Symlink(filepath.Join(root, "missing"), dst)
	called := false
	run := func(_ string, _ ...string) error { called = true; return nil }
	if _, err := Copy(src, dstDir, false, run, os.RemoveAll); err == nil || called {
		t.Fatal("dangling target must block copy")
	}
	// A second destination isolates the unquarantine failure without removing the existing link.
	dstDir = filepath.Join(root, "second")
	os.Mkdir(dstDir, 0755)
	var trashed string
	run = func(name string, args ...string) error {
		if name == "/usr/bin/ditto" {
			return os.Mkdir(args[len(args)-1], 0755)
		}
		return errors.New("quarantine removal failed")
	}
	if _, err := Copy(src, dstDir, true, run, func(p string) error { trashed = p; return os.RemoveAll(p) }); err == nil || trashed == "" {
		t.Fatal("cleanup missing", err)
	}
	if _, err := os.Lstat(filepath.Join(dstDir, "Demo.app")); !os.IsNotExist(err) {
		t.Fatal("failed copy published")
	}
}

func TestUnquarantineWithoutAttributeAndExternalLink(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS commands")
	}
	root := t.TempDir()
	src := bundle(t, filepath.Join(root, "Demo.app"))
	dstDir := filepath.Join(root, "Applications")
	os.Mkdir(dstDir, 0755)
	external := filepath.Join(root, "outside")
	os.WriteFile(external, []byte("external"), 0644)
	if out, err := exec.Command("/usr/bin/xattr", "-w", "com.apple.quarantine", "0081;00000000;test;", external).CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	if err := os.Symlink(external, filepath.Join(src, "Contents", "outside")); err != nil {
		t.Fatal(err)
	}
	dst, err := Copy(src, dstDir, true, Run, os.RemoveAll)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "Contents", "Info.plist")); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("/usr/bin/xattr", "-p", "com.apple.quarantine", external).Run(); err != nil {
		t.Fatal("external link target changed", err)
	}
}
