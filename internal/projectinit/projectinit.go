// Package projectinit initializes a newly created project directory on demand.
// It never overwrites existing items or rolls back successful steps.
package projectinit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Options struct {
	Git, AgentFiles, Skills bool
	AgentID                 string
}

func (o Options) Enabled() bool { return o.Git || o.AgentFiles || o.Skills }

// SkillsDir is relative to the project. Only these two catalog IDs are Claude.
func SkillsDir(agentID string) string {
	switch agentID {
	case "claude", "claude-agent-teams":
		return ".claude/skills"
	default:
		return ".agents/skills"
	}
}

type Result struct {
	Name    string
	Skipped bool
	Err     error
}

// Init runs independent steps in order. gitInit may be injected by tests;
// nil uses the real, bounded Git command. Call off the UI thread.
func Init(dir string, opts Options, gitInit func(string) error) []Result {
	var results []Result
	add := func(name string, skipped bool, err error) {
		results = append(results, Result{Name: name, Skipped: skipped, Err: err})
	}
	// Do not follow a project root that has been replaced by a symlink.
	st, rootErr := os.Lstat(dir)
	if rootErr == nil && !st.IsDir() {
		rootErr = fmt.Errorf("目标不是文件夹：%s", dir)
	}
	file := func(name, body string) {
		if rootErr != nil {
			add(name, false, rootErr)
			return
		}
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			add(name, true, nil)
			return
		}
		if err == nil {
			_, err = f.WriteString(body)
			if closeErr := f.Close(); err == nil {
				err = closeErr
			}
		}
		add(name, false, err)
	}
	if opts.AgentFiles {
		file("AGENTS.md", "")
		file("CLAUDE.md", "@AGENTS.md\n")
	}
	if opts.Skills {
		name := SkillsDir(opts.AgentID)
		skipped, err := false, rootErr
		if err == nil {
			skipped, err = makeSkills(dir, name)
		}
		add(name+"/", skipped, err)
	}
	if opts.Git {
		err := rootErr
		skipped := false
		if err == nil {
			_, err = os.Lstat(filepath.Join(dir, ".git"))
			if err == nil {
				skipped = true
			} else if errors.Is(err, os.ErrNotExist) {
				if gitInit == nil {
					gitInit = InitGit
				}
				err = gitInit(dir)
			}
		}
		add("Git", skipped, err)
	}
	return results
}

func makeSkills(dir, name string) (bool, error) {
	// Inspect both components rather than following an existing .agents/.claude
	// symlink into another project or the user's global configuration.
	for _, part := range []string{filepath.Dir(name), "skills"} {
		dir = filepath.Join(dir, part)
		err := os.Mkdir(dir, 0o755)
		if errors.Is(err, os.ErrExist) {
			st, statErr := os.Lstat(dir)
			if statErr != nil {
				return false, statErr
			}
			if !st.IsDir() {
				return false, fmt.Errorf("已有同名项不是普通目录：%s", dir)
			}
			if part == "skills" {
				return true, nil
			}
		} else if err != nil {
			return false, err
		}
	}
	return false, nil
}
