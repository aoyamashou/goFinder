package tui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

	"gofinder/internal/finfo"
	"gofinder/internal/pane"
	"gofinder/internal/vfs"
)

type previewKind int

const (
	pvNone previewKind = iota
	pvDir
	pvText
	pvImage
	pvMeta
	pvError
)

const (
	textPreviewMax  = 64 << 10
	imagePreviewMax = 20 << 20
	previewMaxItems = 2000
)

var imageExts = map[string]bool{
	"png":  true,
	"jpg":  true,
	"jpeg": true,
	"gif":  true,
	"webp": true,
	"bmp":  true,
}

type previewState struct {
	path      string
	mod       time.Time
	seq       int
	width     int
	height    int
	hidden    bool
	loading   bool
	kind      previewKind
	header    string
	lines     []string
	entries   []vfs.Entry
	scroll    int
	truncated bool
}

func buildPreview(e vfs.Entry, hidden bool, w, h int, th theme) previewState {
	if e.Path == "" {
		return previewState{kind: pvNone}
	}
	if e.IsDir {
		return buildDirPreview(e, hidden)
	}
	if imageExts[e.Ext()] && e.Size > 0 && e.Size <= imagePreviewMax {
		if pv, ok := buildImagePreview(e, w, h, th); ok {
			return pv
		}
	}
	return buildFilePreview(e, th)
}

func buildDirPreview(e vfs.Entry, hidden bool) previewState {
	raw, err := vfs.ReadDir(e.Path)
	if err != nil {
		return previewState{kind: pvError, header: "目录无法读取", lines: []string{err.Error()}}
	}
	out := make([]vfs.Entry, 0, len(raw))
	for _, item := range raw {
		if !hidden && item.Hidden() {
			continue
		}
		out = append(out, item)
	}
	pane.SortEntries(out, pane.ByName, false)
	header := fmt.Sprintf("目录 · %d 项", len(out))
	if len(out) > previewMaxItems {
		out = out[:previewMaxItems:previewMaxItems]
		header = fmt.Sprintf("目录 · 仅显示前 %d 项", previewMaxItems)
	}
	return previewState{
		kind:    pvDir,
		header:  header,
		entries: out,
	}
}

func buildImagePreview(e vfs.Entry, w, h int, th theme) (previewState, bool) {
	data, err := os.ReadFile(e.Path)
	if err != nil {
		return previewState{}, false
	}
	// Reserve 2 rows: 1 for header, 1 for visual breathing room.
	imgRows := max(h-2, 2)
	imgCols := max(w-2, 4)
	lines, origW, origH, format, err := RenderHalfBlock(data, imgCols, imgRows, th.imgBg)
	if err != nil {
		return previewState{}, false
	}
	hdr := fmt.Sprintf("%s · %d×%d · %s", strings.ToUpper(format), origW, origH, humanSize(e.Size))
	return previewState{
		kind:   pvImage,
		header: hdr,
		lines:  lines,
	}, true
}

func buildFilePreview(e vfs.Entry, th theme) previewState {
	f, err := os.Open(e.Path)
	if err != nil {
		return previewState{kind: pvError, header: "无法打开文件", lines: []string{err.Error()}}
	}
	defer f.Close()

	buf := make([]byte, textPreviewMax)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return previewState{kind: pvError, header: "读取失败", lines: []string{err.Error()}}
	}
	buf = buf[:n]

	// Detect binary content by NUL byte in the prefix.
	sample := buf
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	if bytes.IndexByte(sample, 0) >= 0 {
		return buildMetaPreview(e, th)
	}

	// Trim incomplete UTF-8 sequence at the end of the buffer.
	for len(buf) > 0 && !utf8.Valid(buf) {
		buf = buf[:len(buf)-1]
	}
	if len(buf) > 0 && !utf8.Valid(buf) {
		return buildMetaPreview(e, th)
	}

	text := strings.ReplaceAll(string(buf), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\t", "    ")

	highlighted := highlightText(e.Name, text, th.dark)
	lines := strings.Split(highlighted, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	truncated := e.Size > int64(n)
	hdr := fmt.Sprintf("%s · %d 行", humanSize(e.Size), len(lines))
	if truncated {
		hdr += fmt.Sprintf("（前 %d KB）", textPreviewMax>>10)
	}
	return previewState{
		kind:      pvText,
		header:    hdr,
		lines:     lines,
		truncated: truncated,
	}
}

func highlightText(filename, text string, dark bool) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	lexer := lexers.Match(filename)
	if lexer == nil {
		// Only analyse files with a known shebang to avoid misclassifying prose.
		if strings.HasPrefix(text, "#!") {
			lexer = lexers.Analyse(text)
		}
	}
	if lexer == nil {
		return text
	}
	styleName := "monokai"
	if !dark {
		styleName = "github"
	}
	st := styles.Get(styleName)
	if st == nil {
		st = styles.Fallback
	}
	fm := formatters.Get("terminal16m")
	if fm == nil {
		return text
	}
	it, err := lexer.Tokenise(nil, text)
	if err != nil {
		return text
	}
	var out bytes.Buffer
	if err := fm.Format(&out, st, it); err != nil {
		return text
	}
	return out.String()
}

func buildMetaPreview(e vfs.Entry, th theme) previewState {
	info, err := finfo.Stat(e.Path)
	if err != nil {
		return previewState{
			kind:   pvMeta,
			header: "文件信息",
			lines: []string{
				metaLine(th, "名称", e.Name),
				metaLine(th, "大小", humanSize(e.Size)),
				metaLine(th, "修改", e.ModTime.Format("2006-01-02 15:04:05")),
				metaLine(th, "权限", e.Mode.String()),
			},
		}
	}

	lines := []string{
		metaLine(th, "名称", info.Name),
		metaLine(th, "种类", info.Kind),
		metaLine(th, "大小", fmt.Sprintf("%s (%d B)", humanSize(info.Size), info.Size)),
		metaLine(th, "权限", fmt.Sprintf("%s (%s)", info.Perm(), info.Octal())),
	}
	if info.Owner != "" || info.Group != "" {
		lines = append(lines, metaLine(th, "所有", fmt.Sprintf("%s:%s", info.Owner, info.Group)))
	}
	if !info.Created.IsZero() {
		lines = append(lines, metaLine(th, "创建", info.Created.Format("2006-01-02 15:04:05")))
	}
	if !info.Modified.IsZero() {
		lines = append(lines, metaLine(th, "修改", info.Modified.Format("2006-01-02 15:04:05")))
	}
	if !info.Accessed.IsZero() {
		lines = append(lines, metaLine(th, "访问", info.Accessed.Format("2006-01-02 15:04:05")))
	}
	if info.Target != "" {
		lines = append(lines, metaLine(th, "指向", info.Target))
	}
	lines = append(lines, metaLine(th, "路径", filepath.Dir(info.Path)))
	return previewState{
		kind:   pvMeta,
		header: info.Kind,
		lines:  lines,
	}
}

func metaLine(th theme, k, v string) string {
	return th.metaKey.Render(k+"  ") + th.metaVal.Render(v)
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
