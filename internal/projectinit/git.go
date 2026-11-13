package projectinit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const gitTimeout = 5 * time.Second

type gitExecutor struct {
	lookPath func(string) (string, error)
	output   func(context.Context, string, string, ...string) ([]byte, error)
}

// InitGit respects the user's Git templates and default branch. All processes
// share one deadline, including the macOS developer-tools preflight.
func InitGit(dir string) error {
	return initGit(dir, gitExecutor{exec.LookPath, commandOutput})
}

func initGit(dir string, executor gitExecutor) error {
	bin, err := executor.lookPath("git")
	if err != nil {
		return fmt.Errorf("未找到 Git，已跳过 Git 初始化")
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	if bin == "/usr/bin/git" {
		if _, err := executor.output(ctx, dir, "/usr/bin/xcode-select", "-p"); err != nil {
			return fmt.Errorf("未安装命令行工具，已跳过 Git 初始化")
		}
	}
	out, err := executor.output(ctx, dir, bin, "init")
	if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("Git 初始化超时（5 秒）")
	}
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("Git 初始化失败：%s", detail)
	}
	return nil
}

func commandOutput(ctx context.Context, dir, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = gitEnvironment(os.Environ())
	cmd.WaitDelay = 100 * time.Millisecond
	return cmd.CombinedOutput()
}

func gitEnvironment(env []string) []string {
	out := make([]string, 0, len(env))
	for _, item := range env {
		key, _, _ := strings.Cut(item, "=")
		switch key {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE",
			"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_PREFIX",
			"GIT_IMPLICIT_WORK_TREE", "GIT_CEILING_DIRECTORIES", "GIT_DISCOVERY_ACROSS_FILESYSTEM",
			"GIT_TERMINAL_PROMPT":
			continue
		}
		out = append(out, item)
	}
	return append(out, "GIT_TERMINAL_PROMPT=0")
}
