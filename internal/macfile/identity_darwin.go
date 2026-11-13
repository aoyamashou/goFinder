package macfile

import (
	"fmt"
	"os"
	"syscall"
)

func identity(info os.FileInfo) string {
	s := info.Sys().(*syscall.Stat_t)
	return fmt.Sprintf("%d/%d/%d/%d", s.Dev, s.Ino, s.Ctimespec.Sec, s.Ctimespec.Nsec)
}
