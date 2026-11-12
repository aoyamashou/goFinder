// Package shellcmd expands the user's shell commands. {path} is the
// current folder and {selection} the chosen items, each quoted for sh, so
// that no file name can run code: a name like "a'; rm -rf ~; '" stays one
// argument.
package shellcmd

import (
	"os/exec"
	"strings"
)

// Quote makes s one sh word: single quotes, with each ' as '\”.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Expand fills the placeholders of tmpl. A placeholder the user put in
// quotes ("{path}" or '{path}') is replaced with its quotes, which would
// otherwise let a name close them.
func Expand(tmpl, dir string, sel []string) string {
	qs := make([]string, len(sel))
	for i, s := range sel {
		qs[i] = Quote(s)
	}
	vals := map[string]string{"{path}": Quote(dir), "{selection}": strings.Join(qs, " ")}
	var b strings.Builder
	for i := 0; i < len(tmpl); {
		matched := false
		for ph, v := range vals {
			for _, q := range []string{`"`, `'`, ""} {
				w := q + ph + q
				if strings.HasPrefix(tmpl[i:], w) {
					b.WriteString(v)
					i += len(w)
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			b.WriteByte(tmpl[i])
			i++
		}
	}
	return b.String()
}

// Run runs an expanded command with sh in dir, returning its combined
// output when it fails.
func Run(dir, cmd string) (string, error) {
	c := exec.Command("/bin/sh", "-c", cmd)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), err
	}
	return "", nil
}
