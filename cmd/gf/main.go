// Command gf is the standalone terminal UI companion for goFinder.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"gofinder/tui"
)

func main() {
	var (
		vimFlag    bool
		hiddenFlag bool
		noNerdFlag bool
		cwdFile    string
		keymapPath string
		configDir  string
		shellInit  string
	)

	flag.BoolVar(&vimFlag, "vim", false, "启用 Vim 方向键 (h/j/k/l)")
	flag.BoolVar(&hiddenFlag, "hidden", false, "初始显示隐藏文件")
	flag.BoolVar(&noNerdFlag, "no-nerd", false, "关闭 Nerd Font 图标，使用纯文字符号")
	flag.StringVar(&cwdFile, "cwd-file", "", "退出时将最终所在目录写入该文件（供 Shell wrapper 切目录）")
	flag.StringVar(&keymapPath, "keymap", "", "自定义快捷键配置文件路径（默认 ~/.config/gf/keymap.json）")
	flag.StringVar(&configDir, "config-dir", "", "配置目录（默认 $XDG_CONFIG_HOME/gf 或 ~/.config/gf）")
	flag.StringVar(&shellInit, "shell-init", "", "输出退出自动切换目录的 Shell 包装函数（zsh / bash / fish）")

	// Allow flags after a positional directory argument (e.g. "gf ~/dir --vim").
	var flagArgs, posArgs []string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			posArgs = append(posArgs, args[i+1:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			flagArgs = append(flagArgs, a)
			name := a[1:]
			if len(name) > 0 && name[0] == '-' {
				name = name[1:]
			}
			// Value-taking flags without "=" consume the next argument.
			switch name {
			case "cwd-file", "keymap", "config-dir", "shell-init":
				if i+1 < len(args) {
					i++
					flagArgs = append(flagArgs, args[i])
				}
			}
			continue
		}
		posArgs = append(posArgs, a)
	}
	_ = flag.CommandLine.Parse(append(flagArgs, posArgs...))

	if shellInit != "" {
		if err := printShellInit(shellInit); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}

	var opts tui.Options
	opts.ConfigDir = configDir
	opts.KeymapPath = keymapPath
	if flag.NArg() > 0 {
		opts.StartDir = flag.Arg(0)
	}
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "vim":
			v := vimFlag
			opts.VimOverride = &v
		case "hidden":
			v := hiddenFlag
			opts.Hidden = &v
		case "no-nerd":
			v := !noNerdFlag
			opts.NerdOverride = &v
		}
	})

	m := tui.New(opts)
	defer m.Close()

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gf:", err)
		os.Exit(1)
	}
	if fm, ok := finalModel.(*tui.Model); ok && cwdFile != "" {
		if dir := fm.FinalDir(); dir != "" {
			_ = os.WriteFile(cwdFile, []byte(dir), 0o600)
		}
	}
}

func printShellInit(shell string) error {
	switch shell {
	case "zsh", "bash":
		fmt.Print(`unalias gf 2>/dev/null
function gf() {
	local tmp="$(mktemp -t "gf-cwd.XXXXXX")" cwd
	command gf --cwd-file="$tmp" "$@"
	if cwd="$(command cat -- "$tmp")" && [ -n "$cwd" ] && [ "$cwd" != "$PWD" ]; then
		builtin cd -- "$cwd"
	fi
	rm -f -- "$tmp"
}
`)
		return nil
	case "fish":
		fmt.Print(`function gf
	set tmp (mktemp -t "gf-cwd.XXXXXX")
	command gf --cwd-file="$tmp" $argv
	if set cwd (command cat -- "$tmp"); and [ -n "$cwd" ]; and [ "$cwd" != "$PWD" ]
		builtin cd -- "$cwd"
	end
	rm -f -- "$tmp"
end
`)
		return nil
	default:
		return fmt.Errorf("不支持的 shell %q（可选：zsh、bash、fish）", shell)
	}
}
