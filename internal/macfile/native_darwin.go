//go:build darwin

// Package macfile provides the native, non-overwriting operations needed by undo.
package macfile

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var native struct {
	sync.Once
	err    error
	rename func(string, string, uint32) int32
}

func load() error {
	native.Do(func() {
		_, native.err = purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if native.err != nil {
			return
		}
		lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			native.err = err
			return
		}
		fn, err := purego.Dlsym(lib, "renamex_np")
		if err != nil {
			native.err = err
			return
		}
		purego.RegisterFunc(&native.rename, fn)
	})
	return native.err
}

// Restore atomically refuses to replace anything at dst, including a symlink.
// A failure preserves src. Cross-volume restores are deliberately refused.
func Restore(src, dst string) error {
	if strings.ContainsRune(src, 0) || strings.ContainsRune(dst, 0) {
		return errors.New("路径含空字符")
	}
	if err := load(); err != nil {
		return err
	}
	if native.rename(src, dst, 0x4) != 0 {
		return fmt.Errorf("无法恢复到 %s：目标已存在、目录不可用或不在同一卷", dst)
	}
	return nil
}

// Trash returns the actual URL assigned by NSFileManager, without a subprocess
// or Finder automation. The autorelease pool stays on its owning OS thread.
func Trash(path string) (string, error) {
	if !strings.HasPrefix(path, "/") || strings.ContainsRune(path, 0) {
		return "", errors.New("废纸篓需要绝对路径")
	}
	if err := load(); err != nil {
		return "", err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	send := func(id objc.ID, sel string, args ...any) objc.ID { return id.Send(objc.RegisterName(sel), args...) }
	pool := send(objc.ID(objc.GetClass("NSAutoreleasePool")), "new")
	defer send(pool, "drain")
	str := send(objc.ID(objc.GetClass("NSString")), "stringWithUTF8String:", path)
	u := send(objc.ID(objc.GetClass("NSURL")), "fileURLWithPath:", str)
	fm := send(objc.ID(objc.GetClass("NSFileManager")), "defaultManager")
	var result, failure objc.ID
	ok := objc.Send[bool](fm, objc.RegisterName("trashItemAtURL:resultingItemURL:error:"), u, &result, &failure)
	if !ok {
		if failure != 0 {
			return "", errors.New(objc.Send[string](send(failure, "localizedDescription"), objc.RegisterName("UTF8String")))
		}
		return "", errors.New("无法移到废纸篓")
	}
	if result == 0 {
		return "", nil
	} // already moved, but cannot safely undo
	p := objc.Send[string](send(result, "path"), objc.RegisterName("UTF8String"))
	return p, nil
}

// Identity identifies an object independently of timestamps changed by renames.
func Identity(info os.FileInfo) string { return identity(info) }
