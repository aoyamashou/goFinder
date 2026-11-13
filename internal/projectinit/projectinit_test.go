package projectinit

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestInitCombinations(t *testing.T) {
	for bits := 0; bits < 8; bits++ {
		t.Run(string(rune('0'+bits)), func(t *testing.T) {
			dir := t.TempDir()
			opts := Options{Git: bits&1 != 0, AgentFiles: bits&2 != 0, Skills: bits&4 != 0, AgentID: "claude"}
			calls := 0
			got := Init(dir, opts, func(path string) error {
				calls++
				if path != dir {
					t.Fatal(path)
				}
				return nil
			})
			for _, r := range got {
				if r.Err != nil || r.Skipped {
					t.Fatalf("%+v", r)
				}
			}
			for name, want := range map[string]bool{"AGENTS.md": opts.AgentFiles, "CLAUDE.md": opts.AgentFiles, ".claude/skills": opts.Skills} {
				_, err := os.Stat(filepath.Join(dir, name))
				if (err == nil) != want {
					t.Fatalf("%s: %v", name, err)
				}
			}
			if (calls == 1) != opts.Git {
				t.Fatal(calls)
			}
			if opts.AgentFiles {
				b, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
				if string(b) != "@AGENTS.md\n" {
					t.Fatal(string(b))
				}
				st, _ := os.Stat(filepath.Join(dir, "AGENTS.md"))
				if st.Size() != 0 {
					t.Fatal(st.Size())
				}
			}
			if _, err := os.Stat(filepath.Join(dir, ".agents")); !os.IsNotExist(err) {
				t.Fatal("created other skills directory")
			}
		})
	}
	for _, id := range []string{"claude", "claude-agent-teams", "codex", "openclaude", ""} {
		dir := t.TempDir()
		got := Init(dir, Options{Skills: true, AgentID: id}, nil)
		path := ".agents/skills"
		if id == "claude" || id == "claude-agent-teams" {
			path = ".claude/skills"
		}
		if got[0].Err != nil {
			t.Fatal(got)
		}
		es, err := os.ReadDir(filepath.Join(dir, path))
		if err != nil || len(es) != 0 {
			t.Fatalf("%s: %v %v", id, es, err)
		}
	}
}

func TestInitPreservesExistingAndContinues(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(outside, "rules"), []byte("personal"), 0o644)
	os.Symlink(filepath.Join(outside, "rules"), filepath.Join(dir, "AGENTS.md"))
	os.Mkdir(filepath.Join(dir, "CLAUDE.md"), 0o755)
	os.Symlink(outside, filepath.Join(dir, ".agents"))
	calls := 0
	results := Init(dir, Options{Git: true, AgentFiles: true, Skills: true}, func(string) error { calls++; return errors.New("no git") })
	if !results[0].Skipped || !results[1].Skipped || results[2].Err == nil || results[3].Err == nil || calls != 1 {
		t.Fatal(results, calls)
	}
	b, _ := os.ReadFile(filepath.Join(outside, "rules"))
	if string(b) != "personal" {
		t.Fatal("overwritten")
	}
	if _, err := os.Stat(filepath.Join(outside, "skills")); !os.IsNotExist(err) {
		t.Fatal("followed symlink")
	}
	// An existing .git may be a file, directory or dangling symlink.
	for _, kind := range []string{"file", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, ".git")
			switch kind {
			case "file":
				os.WriteFile(p, []byte("keep"), 0o644)
			case "directory":
				os.Mkdir(p, 0o755)
			case "symlink":
				os.Symlink("missing", p)
			}
			got := Init(dir, Options{Git: true}, func(string) error { t.Fatal("reinitialized"); return nil })
			if !got[0].Skipped || got[0].Err != nil {
				t.Fatal(got)
			}
		})
	}
	// Repeat never overwrites successfully created files or the skills directory.
	dir = t.TempDir()
	opts := Options{AgentFiles: true, Skills: true}
	Init(dir, opts, nil)
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("edited"), 0o644)
	for _, r := range Init(dir, opts, nil) {
		if !r.Skipped || r.Err != nil {
			t.Fatal(r)
		}
	}
	b, _ = os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if string(b) != "edited" {
		t.Fatal("lost edit")
	}
}

func TestInitRootAndWriteFailures(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "gone")
	for _, path := range []string{missing, filepath.Join(dir, "link")} {
		if strings.HasSuffix(path, "link") {
			os.Symlink(t.TempDir(), path)
		}
		for _, r := range Init(path, Options{Git: true, AgentFiles: true, Skills: true}, func(string) error { t.Fatal("invalid root"); return nil }) {
			if r.Err == nil {
				t.Fatal(r)
			}
		}
	}
	// A read-only directory fails on non-root systems, including this macOS host.
	if os.Geteuid() != 0 {
		os.Chmod(dir, 0o555)
		t.Cleanup(func() { os.Chmod(dir, 0o755) })
		calls := 0
		got := Init(dir, Options{Git: true, AgentFiles: true, Skills: true}, func(string) error { calls++; return nil })
		if got[0].Err == nil || got[1].Err == nil || got[2].Err == nil || got[3].Err != nil || calls != 1 {
			t.Fatal(got, calls)
		}
	}
}

func TestGitPreflightAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name, bin                 string
		lookErr, checkErr, gitErr error
		out                       string
		want                      string
		calls                     int
	}{
		{name: "missing", lookErr: exec.ErrNotFound, want: "未找到 Git", calls: 0},
		{name: "no tools", bin: "/usr/bin/git", checkErr: errors.New("missing"), want: "未安装命令行工具", calls: 1},
		{name: "apple git", bin: "/usr/bin/git", calls: 2},
		{name: "homebrew", bin: "/opt/homebrew/bin/git", calls: 1},
		{name: "failure", bin: "/opt/homebrew/bin/git", gitErr: errors.New("exit"), out: "permission denied", want: "permission denied", calls: 1},
		{name: "timeout", bin: "/opt/homebrew/bin/git", gitErr: context.DeadlineExceeded, want: "超时", calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			err := initGit("/tmp/project ' 中文", gitExecutor{
				lookPath: func(name string) (string, error) {
					if name != "git" {
						t.Fatal(name)
					}
					return tc.bin, tc.lookErr
				},
				output: func(ctx context.Context, dir, bin string, args ...string) ([]byte, error) {
					calls++
					deadline, ok := ctx.Deadline()
					if !ok || time.Until(deadline) > gitTimeout {
						t.Fatal("missing timeout")
					}
					if dir != "/tmp/project ' 中文" {
						t.Fatal(dir)
					}
					if bin == "/usr/bin/xcode-select" {
						if !reflect.DeepEqual(args, []string{"-p"}) {
							t.Fatal(args)
						}
						return nil, tc.checkErr
					}
					if bin != tc.bin || !reflect.DeepEqual(args, []string{"init"}) {
						t.Fatal(bin, args)
					}
					return []byte(tc.out), tc.gitErr
				},
			})
			if calls != tc.calls {
				t.Fatal(calls)
			}
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestGitEnvironment(t *testing.T) {
	env := []string{"HOME=/test", "GIT_CONFIG_GLOBAL=/test/config", "GIT_DIR=/parent", "GIT_WORK_TREE=/parent", "GIT_COMMON_DIR=/parent", "GIT_INDEX_FILE=/parent/index", "GIT_OBJECT_DIRECTORY=/parent/objects", "GIT_TERMINAL_PROMPT=1"}
	want := []string{"HOME=/test", "GIT_CONFIG_GLOBAL=/test/config", "GIT_TERMINAL_PROMPT=0"}
	if got := gitEnvironment(env); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
}

func TestRealGitNestedProject(t *testing.T) {
	// All configuration and writes stay in this test's temporary tree.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig"))
	template := filepath.Join(home, "template")
	os.Mkdir(template, 0o755)
	os.WriteFile(filepath.Join(template, "marker"), []byte("template"), 0o644)
	config := "[init]\n defaultBranch = project-test\n templateDir = " + template + "\n"
	os.WriteFile(filepath.Join(home, "gitconfig"), []byte(config), 0o644)
	parent := filepath.Join(home, "parent")
	os.Mkdir(parent, 0o755)
	if err := InitGit(parent); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(parent, "项目 ' $(touch nope)")
	os.Mkdir(dir, 0o755)
	t.Setenv("GIT_DIR", filepath.Join(parent, ".git"))
	t.Setenv("GIT_WORK_TREE", parent)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(parent, "index-should-not-exist"))
	for _, r := range Init(dir, Options{Git: true, AgentFiles: true, Skills: true}, nil) {
		if r.Err != nil {
			t.Fatal(r)
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
	if err != nil || string(b) != "ref: refs/heads/project-test\n" {
		t.Fatal(string(b), err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "marker")); err != nil {
		t.Fatal("template ignored", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "index-should-not-exist")); !os.IsNotExist(err) {
		t.Fatal("used inherited index")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "refs", "heads", "project-test")); !os.IsNotExist(err) {
		t.Fatal("created commit")
	}
}
