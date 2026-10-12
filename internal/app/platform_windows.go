//go:build windows

package app

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/linhh-phv/claude-acc/internal/claudebin"
	"github.com/linhh-phv/claude-acc/internal/fsx"
	"github.com/linhh-phv/claude-acc/internal/i18n"
	"github.com/linhh-phv/claude-acc/internal/ui"
)

// useSymlinks is off on Windows: creating one needs Developer Mode or admin
// rights, and whether Claude Code follows it there is unmeasured. Every
// workspace folder gets its own registered copy instead.
const useSymlinks = false

// execClaude runs claude and hands back its exit code — Windows has no exec.
// Ctrl+C reaches every process on the console; claude handles it (it
// interrupts a reply rather than quitting), so claude-acc must not die from
// it and leave claude fighting the shell for the console.
func execClaude(env []string, args []string) int {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	path, err := claudebin.Path()
	if err != nil {
		ui.Die(i18n.T("err_no_claude"))
	}
	c := exec.Command(path, args...)
	c.Env = env
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		ui.Die(i18n.T("err_no_claude"))
	}
	return 0
}

// psQuote quotes s as a PowerShell single-quoted string.
func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// PowerShell syntax: `claude-acc env work | Invoke-Expression`.
func unsetLine() string { return "Remove-Item Env:" + envToken + " -ErrorAction SilentlyContinue" }

func exportLine(tok string) string { return "$env:" + envToken + " = " + psQuote(tok) }

// settingsLabel is how `ls` names the global settings file.
func settingsLabel() string { return `%USERPROFILE%\.claude\settings.json` }

// homeLabel shortens a path under the profile folder to ~ (PowerShell reads
// it), matching case-insensitively and only at a folder boundary.
func homeLabel(p, home string) string {
	if home != "" && fsx.Within(p, home) && !fsx.SamePath(p, home) {
		return "~" + p[len(home):]
	}
	return p
}
