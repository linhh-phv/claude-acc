//go:build windows

package claudebin

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// find resolves claude on PATH. An npm install puts claude.cmd there — a
// batch file, which Windows runs through cmd.exe: arguments with %, &, ^ or
// quotes get mangled, and Ctrl+C asks "Terminate batch job (Y/N)?". The real
// program is the claude.exe the shim calls; run that instead.
func find() (string, error) {
	p, err := exec.LookPath("claude")
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(p))
	if ext != ".cmd" && ext != ".bat" {
		return p, nil
	}
	if exe := behindShim(p); exe != "" {
		return exe, nil
	}
	return p, nil
}

// realExe: npm leaves a small placeholder until its postinstall copies the
// native binary in; a placeholder is not worth running.
func realExe(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir() && st.Size() > 4096
}

var shimTarget = regexp.MustCompile(`(?i)"%(?:~?dp0)%?\\?([^"%]+\.exe)"`)

// behindShim returns the claude.exe an npm shim points to, or "".
func behindShim(shim string) string {
	dir := filepath.Dir(shim)
	// The layout npm uses for @anthropic-ai/claude-code (bin/claude.exe).
	conventional := filepath.Join(dir, "node_modules", "@anthropic-ai", "claude-code", "bin", "claude.exe")
	if realExe(conventional) {
		return conventional
	}
	// Otherwise read the shim: cmd-shim writes "%dp0%\<path>.exe" %*.
	b, err := os.ReadFile(shim)
	if err != nil {
		return ""
	}
	if m := shimTarget.FindSubmatch(b); m != nil {
		p := filepath.Join(dir, filepath.FromSlash(strings.TrimLeft(string(m[1]), `\/`)))
		if realExe(p) {
			return p
		}
	}
	return ""
}
