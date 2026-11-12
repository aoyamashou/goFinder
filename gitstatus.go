package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"

	"gofinder/internal/git"
	"gofinder/internal/vfs"
)

// gitRepo is the cached status of one repository, by its root. At most
// one git runs per repository; changes meanwhile run it once more after.
type gitRepo struct {
	st                     *git.Status
	queued, running, again bool
}

// refreshGit runs git status for root after gitDelay (at once the first
// time), folding the changes that come meanwhile into one run.
func (a *app) refreshGit(root string) {
	if root == "" || a.gitOff {
		return
	}
	g := a.git[root]
	if g == nil {
		g = &gitRepo{}
		a.git[root] = g
	}
	if g.queued {
		return
	}
	g.queued = true
	post := a.post
	run := func() {
		post(func() {
			g.queued = false
			a.runGit(root, g)
		})
	}
	if a.gitDelay > 0 && g.st != nil { // the first status comes at once
		time.AfterFunc(a.gitDelay, run)
	} else {
		run()
	}
}

func (a *app) runGit(root string, g *gitRepo) {
	if g.running {
		g.again = true
		return
	}
	g.running = true
	a.async(func() {
		st, err := git.Load(root, 10*time.Second)
		a.post(func() {
			g.running = false
			if err == git.ErrNoGit {
				a.gitOff = true // never try again this launch
			}
			if err == nil {
				g.st = st
			}
			if g.again {
				g.again = false
				a.refreshGit(root)
			}
		})
	})
}

// gitStatus is the cached status of the tab's repository, nil when none.
func (a *app) gitStatus(ts *tabState) *git.Status {
	if ts.gitRoot == "" || vfs.InZip(ts.t.Dir) {
		return nil
	}
	if g := a.git[ts.gitRoot]; g != nil {
		return g.st
	}
	return nil
}

// gitMark is the entry's git mark in the tab's repository.
func gitMark(st *git.Status, root string, e vfs.Entry) string {
	if st == nil {
		return ""
	}
	rel, err := filepath.Rel(root, e.Path)
	if err != nil {
		return ""
	}
	m := st.Mark(filepath.ToSlash(rel), e.IsDir)
	if m == "??" {
		m = "?" // one letter after the name, as the others
	}
	return m
}

func gitColor(c *ui.Context, mark string) ui.Color {
	switch mark {
	case "A":
		return tok(c, "git-added")
	case "?":
		return tok(c, "git-untracked")
	case "D", "U":
		return tok(c, "git-conflict")
	case "•":
		return tok(c, "muted")
	}
	return tok(c, "git-modified")
}

// gitBranch is the address bar's "⎇ branch · n" for the tab, "" outside
// a repository.
func (a *app) gitBranch(ts *tabState) string {
	st := a.gitStatus(ts)
	if st == nil || st.Branch == "" {
		return ""
	}
	if n := st.Len(); n > 0 {
		return fmt.Sprintf("⎇ %s · %d", st.Branch, n)
	}
	return "⎇ " + st.Branch
}
