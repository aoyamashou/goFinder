// Package finfo reads what the inspector shows of a file or folder: its
// kind, size, times, permissions and owner, and a folder's counts.
package finfo

import (
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Info is one item's attributes.
type Info struct {
	Name, Path string
	Dir, Link  bool
	Kind       string
	Size       int64
	Created    time.Time // zero when unknown
	Modified   time.Time
	Accessed   time.Time // the last time it was opened (read)
	Mode       fs.FileMode
	Owner      string
	Group      string
	Target     string // a link's target

	Added   time.Time // when it came to its folder (Spotlight); zero when unknown
	Hidden  bool      // the hidden flag (chflags hidden)
	Locked  bool      // the user immutable flag (chflags uchg)
	HideExt bool      // Finder hides its extension
	Ext     string    // the extension, without its dot
}

// Stat reads path's attributes, of a link itself rather than its target.
func Stat(path string) (Info, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return Info{}, err
	}
	in := Info{
		Name: filepath.Base(path), Path: path, Dir: st.IsDir(),
		Size: st.Size(), Modified: st.ModTime(), Mode: st.Mode(),
	}
	if st.Mode()&fs.ModeSymlink != 0 {
		in.Link = true
		in.Target, _ = os.Readlink(path)
		if t, err := os.Stat(path); err == nil && t.IsDir() {
			in.Dir = true
		}
	}
	if sys, ok := st.Sys().(*syscall.Stat_t); ok {
		fill(&in, sys)
	}
	in.Kind = Kind(in.Name, in.Dir, in.Link)
	in.Ext = strings.TrimPrefix(filepath.Ext(in.Name), ".")
	readExtra(&in)
	return in, nil
}

// Perm is the permissions as ls shows them: "rwxr-xr-x".
func (in Info) Perm() string { return in.Mode.Perm().String()[1:] }

// Octal is the permissions as chmod takes them: "755", with a leading
// digit for setuid, setgid and sticky.
func (in Info) Octal() string {
	v := uint32(in.Mode.Perm())
	if in.Mode&fs.ModeSetuid != 0 {
		v |= 0o4000
	}
	if in.Mode&fs.ModeSetgid != 0 {
		v |= 0o2000
	}
	if in.Mode&fs.ModeSticky != 0 {
		v |= 0o1000
	}
	return fmt.Sprintf("%03o", v)
}

// Exec tells whether a file has an x permission.
func (in Info) Exec() bool { return !in.Dir && in.Mode.IsRegular() && in.Mode&0o111 != 0 }

var (
	namesMu sync.Mutex
	users   = map[uint32]string{}
	groups  = map[uint32]string{}
)

// userName and groupName look an id up once.
func userName(id uint32) string {
	namesMu.Lock()
	defer namesMu.Unlock()
	if n, ok := users[id]; ok {
		return n
	}
	n := strconv.Itoa(int(id))
	if u, err := user.LookupId(n); err == nil {
		n = u.Username
	}
	users[id] = n
	return n
}

func groupName(id uint32) string {
	namesMu.Lock()
	defer namesMu.Unlock()
	if n, ok := groups[id]; ok {
		return n
	}
	n := strconv.Itoa(int(id))
	if g, err := user.LookupGroupId(n); err == nil {
		n = g.Name
	}
	groups[id] = n
	return n
}

// kinds name the common extensions, as Finder's 种类 column does.
var kinds = map[string]string{
	"pdf": "PDF 文稿", "txt": "纯文本文稿", "md": "Markdown 文稿", "rtf": "RTF 文稿",
	"html": "HTML 文本", "htm": "HTML 文本", "css": "CSS 样式表", "json": "JSON", "xml": "XML 文本",
	"yaml": "YAML", "yml": "YAML", "toml": "TOML", "csv": "逗号分隔值", "log": "日志文件",
	"doc": "Word 文稿", "docx": "Word 文稿", "xls": "Excel 工作簿", "xlsx": "Excel 工作簿",
	"ppt": "PowerPoint 演示文稿", "pptx": "PowerPoint 演示文稿",
	"pages": "Pages 文稿", "numbers": "Numbers 电子表格", "key": "Keynote 演示文稿",
	"png": "PNG 图像", "jpg": "JPEG 图像", "jpeg": "JPEG 图像", "gif": "GIF 图像", "heic": "HEIC 图像",
	"webp": "WebP 图像", "svg": "SVG 图像", "tiff": "TIFF 图像", "bmp": "BMP 图像", "ico": "图标",
	"mp4": "MPEG-4 影片", "mov": "QuickTime 影片", "mkv": "Matroska 影片", "avi": "AVI 影片",
	"mp3": "MP3 音频", "m4a": "Apple MPEG-4 音频", "wav": "WAV 音频", "flac": "FLAC 音频",
	"zip": "ZIP 归档", "gz": "gzip 压缩归档", "tar": "tar 归档", "7z": "7-Zip 归档", "rar": "RAR 归档",
	"dmg": "磁盘映像", "iso": "光盘映像", "pkg": "安装器软件包",
	"ttf": "TrueType 字体", "otf": "OpenType 字体", "ttc": "TrueType 字体集",
	"pem": "PEM 证书", "cer": "证书", "crt": "证书", "p12": "个人信息交换",
	"db": "数据库", "sqlite": "SQLite 数据库", "sql": "SQL 脚本",
	"go": "Go 源代码", "py": "Python 脚本", "js": "JavaScript 脚本", "ts": "TypeScript 源代码",
	"swift": "Swift 源代码", "c": "C 源代码", "h": "C 头文件", "cpp": "C++ 源代码", "rs": "Rust 源代码",
	"java": "Java 源代码", "kt": "Kotlin 源代码", "rb": "Ruby 脚本", "sh": "Shell 脚本", "zsh": "Shell 脚本",
	"plist": "属性列表", "app": "应用程序", "framework": "框架", "bundle": "包",
}

// Kind names the kind of an item named name: Finder's name of its
// extension, else the extension's.
func Kind(name string, dir, link bool) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	if strings.HasPrefix(name, ".") && strings.Count(name, ".") == 1 {
		ext = "" // ".zshrc"
	}
	k, known := kinds[ext]
	switch {
	case link:
		return "替身（符号链接）"
	case dir && known && (ext == "app" || ext == "framework" || ext == "bundle"):
		return k
	case dir:
		return "文件夹"
	case known:
		return k
	case ext != "":
		return strings.ToUpper(ext) + " 文件"
	}
	return "文稿"
}

// Stats counts a folder: its direct items, of them the files and the
// folders, and the size of everything in it.
type Stats struct {
	Items, Files, Dirs int
	Size               int64 // of every file under it, at least, when Capped
	Scanned            int   // items the size was taken from
	Capped             bool  // stopped at the limit: Size is a lower bound
}

// Count counts dir, taking the size of at most limit items under it.
// Links are counted but not followed. stop, when it returns true, ends
// the count early, as Capped.
func Count(dir string, limit int, stop func() bool) (Stats, error) {
	es, err := os.ReadDir(dir)
	if err != nil {
		return Stats{}, err
	}
	var s Stats
	s.Items = len(es)
	for _, e := range es {
		if e.IsDir() {
			s.Dirs++
		} else {
			s.Files++
		}
	}
	errStop := fs.SkipAll
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if p == dir {
			return err
		}
		if err != nil {
			return nil // unreadable: skip it
		}
		if s.Scanned >= limit || (stop != nil && s.Scanned%512 == 0 && stop()) {
			s.Capped = true
			return errStop
		}
		s.Scanned++
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				s.Size += info.Size()
			}
		}
		return nil
	})
	if err == errStop {
		err = nil
	}
	return s, err
}

// Size says n bytes as Finder does, in thousands: "1.2 MB".
func Size(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	v, i := float64(n), 0
	for v >= unit && i < 4 {
		v /= unit
		i++
	}
	return fmt.Sprintf("%.1f %cB", v, "KMGT"[i-1])
}

// Bytes says n with thousands separated: "1,234,567 字节".
func Bytes(n int64) string {
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String() + " 字节"
}

// SetPerm sets the nine permission bits of path, keeping setuid, setgid
// and sticky as they are.
func SetPerm(path string, perm fs.FileMode) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	keep := st.Mode() & (fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky)
	return os.Chmod(path, perm.Perm()|keep)
}

// SizeLong says n as Finder's 大小 does: "1.5 MB（1 469 302 字节）", the
// digits grouped by thousands with a space.
func SizeLong(n int64) string {
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return Size(n) + "（" + b.String() + " 字节）"
}
