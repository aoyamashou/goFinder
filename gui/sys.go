package gui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo"

	"gofinder/internal/appinstall"
	"gofinder/internal/macfile"
	"gofinder/internal/shellcmd"
)

// system is what the app asks of the OS, so tests can fake it.
type system interface {
	Open(path string) error
	InstallApp(path string, unquarantine bool) (string, error)
	Trash(path string) error
	TrashItems(paths []string) []trashResult
	Reveal(path string)
	OpenWith(app, path string) error
	// Watch calls changed when dir's direct contents change, until stop.
	Watch(dir string, changed func()) (stop func())
	// InstalledApps lists the names "open -a" takes (the .app bundles'
	// names), sorted. It reads the disk: call it off the main thread.
	InstalledApps() ([]string, error)
	// RunShell runs a command line with sh in dir; on failure it returns
	// the command's output too. It blocks: call it off the main thread.
	RunShell(dir, cmd string) (string, error)
	// FileViewer is the app macOS shows folders with (NSFileViewer), ""
	// for Finder; SetFileViewer sets it, "" restoring Finder.
	FileViewer() (string, error)
	SetFileViewer(bundleID string) error
	// BundleID is this app's identifier, read from its Info.plist.
	BundleID() string
	// RunsScripts tells whether the app opens .command scripts, as a
	// terminal that runs them. It reads the disk: call it off the main thread.
	RunsScripts(app string) bool
	// Thumbnail draws the file at src with QuickLook (qlmanage), px pixels
	// across, into the PNG dst, within qlTimeout. It blocks: call it off
	// the main thread.
	Thumbnail(src, dst string, px int) error
	// QuickLook opens the system's Quick Look (qlmanage -p) on paths; stop
	// ends it. It returns at once.
	QuickLook(paths []string) (stop func())
	// FullDiskAccess tells whether the app can read the TCC database, which
	// only a Full Disk Access grant allows. It reads the disk: call it
	// off the main thread.
	FullDiskAccess() bool
	// OpenURL opens a URL scheme, such as System Settings' panes.
	OpenURL(url string) error
	// Eject ejects the volume mounted at path, as Finder's eject button
	// (diskutil eject, unmount for a share). It blocks: call it off the
	// main thread.
	Eject(path string) error
}

type realSystem struct{}

type trashResult struct {
	path, trashed string
	err           error
}

func (realSystem) TrashItems(paths []string) []trashResult {
	results := make([]trashResult, 0, len(paths))
	for _, p := range paths {
		dst, err := macfile.Trash(p)
		results = append(results, trashResult{path: p, trashed: dst, err: err})
	}
	return results
}

func (realSystem) Open(p string) error  { return mygo.Shell.OpenPath(p) }
func (realSystem) Trash(p string) error { return mygo.Shell.TrashItem(p) }
func (realSystem) Reveal(p string)      { mygo.Shell.ShowItemInFolder(p) }

func (realSystem) OpenWith(app, p string) error {
	return exec.Command("open", "-a", app, p).Run()
}

func (realSystem) InstalledApps() ([]string, error) {
	apps, err := mygo.Shell.InstalledApps()
	if err != nil {
		return nil, err
	}
	// The bundle's name, not the localized one ("终端"), which open -a
	// may not find.
	var out []string
	for _, ap := range apps {
		if n := strings.TrimSuffix(filepath.Base(ap.Path), ".app"); n != "" && n != "." {
			out = append(out, n)
		}
	}
	slices.SortFunc(out, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	return slices.Compact(out), nil
}

func (realSystem) RunsScripts(app string) bool {
	if app == "Terminal" || app == "iTerm" {
		return true
	}
	apps, err := mygo.Shell.InstalledApps()
	if err != nil {
		return false
	}
	for _, ap := range apps {
		if filepath.Base(ap.Path) != app+".app" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(ap.Path, "Contents", "Info.plist"))
		if err != nil {
			return false
		}
		return bytes.Contains(b, []byte("com.apple.terminal.shell-script")) ||
			bytes.Contains(b, []byte("<string>command</string>")) || bytes.Contains(b, []byte("\x57command"))
	}
	return false
}

func (realSystem) Thumbnail(src, dst string, px int) error {
	// qlmanage names its PNG after the file: draw into a folder of our
	// own, then move it in place.
	tmp, err := os.MkdirTemp(filepath.Dir(dst), "ql-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	ctx, cancel := context.WithTimeout(context.Background(), qlTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "qlmanage", "-t", "-s", strconv.Itoa(px), "-o", tmp, src).CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("QuickLook 超时")
	}
	png := filepath.Join(tmp, filepath.Base(src)+".png")
	if _, serr := os.Stat(png); serr != nil {
		if err == nil {
			err = fmt.Errorf("QuickLook 没有生成缩略图：%s", strings.TrimSpace(string(out)))
		}
		return err
	}
	return os.Rename(png, dst)
}

func (realSystem) QuickLook(paths []string) func() {
	cmd := exec.Command("qlmanage", append([]string{"-p"}, paths...)...)
	cmd.Stdout, cmd.Stderr = nil, nil
	if err := cmd.Start(); err != nil {
		return func() {}
	}
	go cmd.Wait() // reap it, whenever it ends
	return func() { cmd.Process.Kill() }
}

func (realSystem) FullDiskAccess() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	f, err := os.Open(filepath.Join(home, "Library", "Application Support", "com.apple.TCC", "TCC.db"))
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func (realSystem) OpenURL(u string) error { return exec.Command("open", u).Run() }

func (realSystem) Eject(p string) error {
	out, err := exec.Command("diskutil", "eject", p).CombinedOutput()
	if err == nil {
		return nil
	}
	if out2, err2 := exec.Command("diskutil", "unmount", p).CombinedOutput(); err2 == nil {
		return nil // a network share, which only unmounts
	} else if len(out2) > 0 {
		out = out2
	}
	if msg := strings.TrimSpace(string(out)); msg != "" {
		return errors.New(msg)
	}
	return err
}

func (realSystem) RunShell(dir, cmd string) (string, error) { return shellcmd.Run(dir, cmd) }

func (realSystem) FileViewer() (string, error) {
	out, err := exec.Command("defaults", "read", "-g", "NSFileViewer").Output()
	if err != nil {
		return "", nil // not set: Finder
	}
	return strings.TrimSpace(string(out)), nil
}

func (realSystem) SetFileViewer(id string) error {
	var c *exec.Cmd
	if id == "" {
		c = exec.Command("defaults", "delete", "-g", "NSFileViewer")
	} else {
		c = exec.Command("defaults", "write", "-g", "NSFileViewer", "-string", id)
	}
	out, err := c.CombinedOutput()
	if err == nil && id != "" {
		registerBundle()
	}
	if err != nil && id == "" && strings.Contains(string(out), "does not exist") {
		return nil // already Finder
	}
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// lsregister is LaunchServices' registration tool.
const lsregister = "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"

// registerBundle re-registers the running .app with LaunchServices, so the
// bundle ID resolves to it, not to a path a rename or a deleted build left
// behind. Best effort: outside an .app there is nothing to register.
func registerBundle() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	app := filepath.Dir(filepath.Dir(filepath.Dir(exe))) // X.app/Contents/MacOS/x
	if !strings.HasSuffix(app, ".app") {
		return
	}
	exec.Command(lsregister, "-f", app).Run()
}

// fallbackBundleID is mygo.json's identifier, for a run outside an app.
const fallbackBundleID = "io.github.aoyamashou.gofinder"

var bundleIDRe = regexp.MustCompile(`<key>CFBundleIdentifier</key>\s*<string>([^<]+)</string>`)

func (realSystem) BundleID() string {
	exe, err := os.Executable()
	if err != nil {
		return fallbackBundleID
	}
	// .../X.app/Contents/MacOS/x → .../X.app/Contents/Info.plist
	b, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(exe)), "Info.plist"))
	if m := bundleIDRe.FindSubmatch(b); err == nil && m != nil {
		return string(m[1])
	}
	return fallbackBundleID
}

func (realSystem) Watch(dir string, changed func()) func() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// HintsOnly keeps no snapshot and stats nothing: the cheapest watch.
		w, err := mygo.WatchFiles(ctx, dir, mygo.FileWatchOptions{HintsOnly: true})
		if err != nil {
			return
		}
		defer w.Close()
		for {
			if _, err := w.Next(ctx); err != nil {
				return
			}
			changed()
		}
	}()
	return cancel
}

// config is the user's settings, config.json in the app's data folder.
type config struct {
	Terminal    string `json:"terminal"`
	Editor      string `json:"editor"`
	ShowHidden  bool   `json:"showHidden"`
	ShowSidebar bool   `json:"showSidebar"`
	VimKeys     bool   `json:"vimKeys"` // optional h/j/k/l; arrows always work
	// Optional initialization after creating a folder; all default to false.
	NewFolderGit        bool `json:"newFolderGit"`
	NewFolderAgentFiles bool `json:"newFolderAgentFiles"`
	NewFolderSkills     bool `json:"newFolderSkills"`
	// ContextMenu lists action IDs in order, "-" a separator.
	ContextMenu []string  `json:"contextMenu"`
	Commands    []command `json:"commands"`
	// BlankMenu is the menu of a column's blank space, as ContextMenu;
	// nil is defaultBlankMenu. AgentCommand runs in the terminal for
	// "agent"; "" is claude.
	BlankMenu []string `json:"blankMenu"`
	// AgentCommand is from before the Agent page, kept for migrateAgents.
	AgentCommand string `json:"agentCommand"`
	// Agents is each agent's settings by ID, DefaultAgent the one the a
	// key opens ("" the first enabled).
	Agents       map[string]agentCfg `json:"agents,omitempty"`
	DefaultAgent string              `json:"defaultAgent,omitempty"`
	// ConfirmTrash asks before d and ⌘⌫ move items to the Trash; the
	// confirmation's 不再提醒 turns it off. Missing is on.
	ConfirmTrash bool `json:"confirmTrash"`
	// Appearance is "dark" (missing), "light" or "system": the window is
	// drawn so, whatever macOS's appearance, its own title bar included.
	Appearance string `json:"appearance,omitempty"`
	// ListFontSize is the lists' font size, one of listSizes; 0 is 14.
	ListFontSize int `json:"listFontSize,omitempty"`
	// StartDir is the folder the window opens in, set by a folder's
	// 启动时从这里开始; "" (or a folder gone) is Finder's new-window place.
	StartDir string `json:"startDir,omitempty"`
	// StartDir2 is the second pane's (dual and rows layouts): each pane
	// keeps its own, for 启动时从这里开始 and ` alike.
	StartDir2 string `json:"startDir2,omitempty"`
	// Workspaces is the order of the saved workspaces (1–9, [ ], the
	// title bar); those not named here follow by name. WorkspacesSeeded is
	// set once the three of the first launch (Home, Download, Documents)
	// were made, so ones deleted stay deleted.
	Workspaces       []string `json:"workspaces,omitempty"`
	WorkspacesSeeded bool     `json:"workspacesSeeded,omitempty"`
	// DefaultWorkspace is the workspace that cannot be deleted, only
	// renamed (the first demo one). "" with WorkspacesSeeded is a config
	// of the English demo names, which migrateDemoNames renames.
	DefaultWorkspace string `json:"defaultWorkspace,omitempty"`
	// Version is the config's format.
	Version int `json:"version"`
}

const configVersion = 7

// command is a shell command of the user's, the action "cmd.<id>".
// {path} is the current folder and {selection} the chosen items, each
// quoted for sh.
type command struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Command string   `json:"command"`
	Keys    []string `json:"keys,omitempty"`
}

func defaultConfig() config {
	return config{
		Terminal: "Terminal", Editor: "Visual Studio Code", ShowSidebar: true,
		ContextMenu: slices.Clone(defaultMenu), Commands: []command{},
		BlankMenu: slices.Clone(defaultBlankMenu), AgentCommand: defaultAgent, ConfirmTrash: true, Version: configVersion,
	}
}

// loadConfig reads dir/config.json, writing the defaults there when it is
// missing so the user has a file to edit.
func loadConfig(dir string) config {
	cfg := defaultConfig()
	p := filepath.Join(dir, "config.json")
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			if b, err := json.MarshalIndent(cfg, "", "  "); err == nil {
				os.MkdirAll(dir, 0o755)
				os.WriteFile(p, b, 0o644)
			}
		}
		return cfg
	}
	cfg.Version = 0
	json.Unmarshal(data, &cfg)
	migrateAgents(&cfg)
	if cfg.Version < configVersion {
		cfg.Version = configVersion
	}
	return cfg
}

// saveConfig writes cfg to dir/config.json.
func saveConfig(dir string, cfg config) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	p := filepath.Join(dir, "config.json")
	if err := os.WriteFile(p+".tmp", b, 0o644); err != nil {
		return err
	}
	return os.Rename(p+".tmp", p)
}

func (s realSystem) InstallApp(path string, unquarantine bool) (string, error) {
	return appinstall.Copy(path, "/Applications", unquarantine, appinstall.Run, s.Trash)
}
