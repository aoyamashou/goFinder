//go:build !darwin

package macfile

import (
	"errors"
	"os"
)

func Restore(src, dst string) error     { return errors.New("原子恢复仅支持 macOS") }
func Trash(path string) (string, error) { return "", errors.New("废纸篓仅支持 macOS") }
func Identity(info os.FileInfo) string  { return "" }
