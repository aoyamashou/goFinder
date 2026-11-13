package gui

import (
	"fmt"
	"path/filepath"
	"strings"

	"gofinder/internal/fileundo"
	"gofinder/internal/git"
	"gofinder/internal/projectinit"
	"gofinder/internal/vfs"
)

func (a *app) folderOptions() projectinit.Options {
	item, _ := a.defaultAgentItem()
	return projectinit.Options{Git: a.cfg.NewFolderGit, AgentFiles: a.cfg.NewFolderAgentFiles, Skills: a.cfg.NewFolderSkills, AgentID: item.ID}
}

func (a *app) folderSkillsCaption() string {
	item, ok := a.defaultAgentItem()
	path := projectinit.SkillsDir(item.ID) + "/"
	if !ok {
		return "创建 " + path
	}
	return fmt.Sprintf("按默认 Agent（%s）创建 %s", item.Name, path)
}

func (a *app) initializeFolder(dir string, opts projectinit.Options, remember bool) {
	changed := remember && (a.cfg.NewFolderGit != opts.Git || a.cfg.NewFolderAgentFiles != opts.AgentFiles || a.cfg.NewFolderSkills != opts.Skills)
	if remember {
		a.cfg.NewFolderGit, a.cfg.NewFolderAgentFiles, a.cfg.NewFolderSkills = opts.Git, opts.AgentFiles, opts.Skills
	}
	if !opts.Enabled() {
		if changed {
			a.saveConfig()
		}
		a.recordCreated(dir, true)
		return
	}
	initGit, post := a.initGit, a.post
	var save func() error
	if changed {
		save = a.configSave()
	}
	a.async(func() {
		results := projectinit.Init(dir, opts, initGit)
		ok := true
		for _, r := range results {
			if r.Err != nil {
				ok = false
			}
		}
		step := undoStep{unavailable: 1}
		if ok {
			if e, err := fileundo.Record(dir, ""); err == nil {
				step = undoStep{entries: []fileundo.Entry{e}}
			}
		}
		status := folderInitStatus(dir, results)
		if save != nil {
			if err := save(); err != nil {
				status += "；本次选择未能保存：" + err.Error()
			}
		}
		post(func() {
			a.status = status
			a.finishFileOp(step)
			visible := a.refreshInitializedFolder(dir)
			for _, r := range results {
				if visible && r.Name == "Git" && r.Err == nil {
					a.refreshGit(dir)
				}
			}
		})
	})
}

func folderInitStatus(dir string, results []projectinit.Result) string {
	done := 0
	var details []string
	for _, r := range results {
		switch {
		case r.Err != nil:
			details = append(details, r.Name+"："+r.Err.Error())
		case r.Skipped:
			details = append(details, r.Name+" 已存在，已跳过")
		default:
			done++
		}
	}
	prefix := "初始化完成"
	if len(details) > 0 {
		prefix = "初始化部分完成"
		if done == 0 {
			prefix = "初始化未完成"
		}
	}
	s := fmt.Sprintf("已创建 %s，%s", filepath.Base(dir), prefix)
	if len(details) > 0 {
		s += "：" + strings.Join(details, "；")
	}
	return s
}

// Refresh only listings that can display this project or its contents. Preserve
// the cursor at completion time, not the row captured before the background read.
// It never requests focus or navigation. An outstanding creation focus, if its
// ordinary read was superseded, is fulfilled just as that read would have done.
func (a *app) refreshInitializedFolder(dir string) bool {
	visible := false
	parent := filepath.Dir(dir)
	for i := 0; i < a.visiblePanes(); i++ {
		p := a.panes[i]
		ts := p.tab()
		if ts.t.Dir == dir || ts.t.Dir == parent {
			visible = true
			a.refreshInitializedListing(ts)
		}
		if p.parent.dir == dir || p.parent.dir == parent {
			visible = true
			p.parent.seq++
			p.parent.dir = ""
		}
		if p.pv.path == dir || p.pv.path == parent {
			visible = true
			p.pv.seq++
			p.pv.path = ""
		}
	}
	return visible
}

func (a *app) refreshInitializedListing(ts *tabState) {
	ts.seq++
	seq, dir, findGit, post := ts.seq, ts.t.Dir, !a.gitOff, a.post
	a.async(func() {
		es, err := vfs.ReadDir(dir)
		root := ""
		if findGit && err == nil {
			root, _ = git.FindRoot(dir)
		}
		post(func() {
			if ts.seq != seq || ts.t.Dir != dir || err != nil {
				return
			}
			keep, hasCursor := ts.cursorEntry()
			pendingFocus := ts.t.Focus
			ts.t.Focus = ""
			ts.loaded, ts.err = true, ""
			ts.t.SetEntries(es)
			if pendingFocus != "" {
				ts.cursor = ts.t.Index(pendingFocus)
				if ts.cursor >= 0 {
					ts.list.ScrollIntoView(ts.cursor)
				}
			} else if hasCursor {
				ts.cursor = ts.t.Index(keep.Name)
			}
			ts.gitRoot = root
			a.refreshGit(root)
		})
	})
}
