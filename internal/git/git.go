// Package git reads a repository's branch and changed files with
// `git status --porcelain=v2 --branch -z`. It only reads: no add, commit
// or push.
package git

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// FindRoot walks up from dir to the folder holding .git (a folder, or a
// file in worktrees and submodules). It only stats.
func FindRoot(dir string) (string, bool) {
	dir = filepath.Clean(dir)
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// Status is a repository's branch and changes.
type Status struct {
	Branch string // "" when unknown; "(detached)" off any branch
	// files maps a changed path, relative to the root with "/", to its
	// mark: "M", "A", "D", "R", "C", "T", "U" or "??".
	files map[string]string
	// dirs holds the folders with changes somewhere inside.
	dirs map[string]bool
	// untracked holds folders git reports whole as untracked ("new/").
	untracked map[string]bool
}

// Len is how many paths have changes.
func (s *Status) Len() int { return len(s.files) + len(s.untracked) }

// Mark is the mark for rel (relative to the root, "/"-separated): a
// file's change, "??" inside an untracked folder, "•" for a folder with
// changes inside, "" for none.
func (s *Status) Mark(rel string, isDir bool) string {
	if s == nil || rel == "" || rel == "." {
		return ""
	}
	if m, ok := s.files[rel]; ok {
		return m
	}
	for p := rel; p != "." && p != "/" && p != ""; p = path.Dir(p) {
		if s.untracked[p] {
			return "??"
		}
	}
	if isDir && s.dirs[rel] {
		return "•"
	}
	return ""
}

// Parse reads the output of `git status --porcelain=v2 --branch -z`.
func Parse(out []byte) *Status {
	s := &Status{files: map[string]string{}, dirs: map[string]bool{}, untracked: map[string]bool{}}
	fields := bytes.Split(out, []byte{0})
	for i := 0; i < len(fields); i++ {
		line := string(fields[i])
		if line == "" {
			continue
		}
		switch line[0] {
		case '#':
			if b, ok := strings.CutPrefix(line, "# branch.head "); ok {
				s.Branch = b
			}
		case '1', '2', 'u':
			// 1 XY sub mH mI mW hH hI path
			// 2 XY sub mH mI mW hH hI Xscore path, then origPath
			// u XY sub m1 m2 m3 mW h1 h2 h3 path
			n := map[byte]int{'1': 9, '2': 10, 'u': 11}[line[0]]
			parts := strings.SplitN(line, " ", n)
			if len(parts) < n {
				continue
			}
			mark := "U"
			if line[0] != 'u' {
				mark = xyMark(parts[1])
			}
			s.add(parts[n-1], mark)
			if line[0] == '2' {
				i++ // the original path
			}
		case '?':
			p := strings.TrimPrefix(line, "? ")
			if d, ok := strings.CutSuffix(p, "/"); ok {
				s.untracked[d] = true
				s.addDirs(d)
			} else {
				s.add(p, "??")
			}
		}
	}
	return s
}

// xyMark picks one letter from the index (X) and worktree (Y) columns:
// the staged change first, as in "AM" → "A".
func xyMark(xy string) string {
	for _, c := range xy {
		if c != '.' {
			return string(c)
		}
	}
	return "M"
}

func (s *Status) add(p, mark string) {
	p = strings.TrimSuffix(p, "/")
	s.files[p] = mark
	s.addDirs(p)
}

func (s *Status) addDirs(p string) {
	for d := path.Dir(p); d != "." && d != "/"; d = path.Dir(d) {
		s.dirs[d] = true
	}
}

// ErrNoGit is returned when no git command is installed.
var ErrNoGit = errors.New("git not found")

// Load runs git status in root. It takes the time git takes: call it off
// the main thread. It gives up after timeout.
func Load(root string, timeout time.Duration) (*Status, error) {
	bin, err := exec.LookPath("git")
	if err != nil {
		return nil, ErrNoGit
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--no-optional-locks", "-C", root,
		"status", "--porcelain=v2", "--branch", "-z")
	// No pager, no prompts, no locale-dependent output.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return Parse(out), nil
}
