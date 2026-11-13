// Package appinstall copies local application bundles without changing ordinary file operations.
package appinstall

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gofinder/internal/vfs"
)

// Bundle is deliberately a lightweight check, not a signature or plist validation.
func Bundle(path string) bool {
	if vfs.InZip(path) || !strings.EqualFold(filepath.Ext(path), ".app") {
		return false
	}
	st, err := os.Lstat(path)
	if err != nil || !st.IsDir() {
		return false
	}
	st, err = os.Stat(filepath.Join(path, "Contents", "Info.plist"))
	return err == nil && st.Mode().IsRegular()
}

func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Installed resolves symbolic links in directory paths before checking standard installation roots.
func Installed(path, home string) bool {
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	dirs := []string{"/Applications", "/System/Applications"}
	if home != "" {
		dirs = append(dirs, filepath.Join(home, "Applications"))
	}
	for _, dir := range dirs {
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			dir = real
		}
		if within(path, dir) {
			return true
		}
	}
	return false
}

func Eligible(path, home string) bool { return Bundle(path) && !Installed(path, home) }

// Runner executes a fixed executable with separate arguments; tests may replace it.
type Runner func(name string, args ...string) error

func Run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w：%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Copy stages in dstDir, then publishes the original name. It never intentionally
// overwrites an existing target. As reviewed, Lstat+Rename accepts a small race
// with another process creating an empty directory between those calls.
// trash only receives our own staging directory, never the source or target.
func Copy(src, dstDir string, unquarantine bool, run Runner, trash func(string) error) (dst string, err error) {
	src, err = filepath.Abs(src)
	if err != nil {
		return "", err
	}
	dstDir, err = filepath.Abs(dstDir)
	if err != nil {
		return "", err
	}
	if !Bundle(src) {
		return "", fmt.Errorf("不是可安装的应用包：%s", src)
	}
	name := filepath.Base(src)
	dst = filepath.Join(dstDir, name)
	check := func() error {
		if _, e := os.Lstat(dst); e == nil {
			return fmt.Errorf("%s 里已有 %s，未复制", dstDir, name)
		} else if !errors.Is(e, fs.ErrNotExist) {
			return e
		}
		return nil
	}
	if err = check(); err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(dstDir, "."+name+".gofinder-")
	if err != nil {
		return "", permissionError(dstDir, err)
	}
	// ditto does not copy root-directory xattrs onto an existing directory.
	// Reserve a private parent, and let ditto create the bundle inside it.
	payload := filepath.Join(staging, name)
	defer func() {
		if e := trash(staging); e != nil {
			detail := fmt.Sprintf("暂存目录未能移到废纸篓：%s（%v）", staging, e)
			if err != nil {
				err = fmt.Errorf("%w；%s", err, detail)
			} else {
				err = errors.New(detail)
			}
		}
	}()
	args := []string{"--rsrc", "--extattr", "--acl", "--qtn"}
	args = append(args, src, payload)
	if err = run("/usr/bin/ditto", args...); err != nil {
		return "", permissionError(dstDir, err)
	}
	if unquarantine {
		// On the verified macOS fixture --noqtn retained quarantine. Remove
		// only this attribute on the staged copy, without following links.
		if err = run("/usr/bin/xattr", "-d", "-r", "-s", "com.apple.quarantine", payload); err != nil {
			return "", err
		}
	}
	if err = check(); err != nil {
		return "", err
	}
	if err = os.Rename(payload, dst); err != nil {
		return "", permissionError(dstDir, err)
	}
	return dst, nil
}

func permissionError(dir string, err error) error {
	if errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("没有权限写入 %s：%w", dir, err)
	}
	return err
}
