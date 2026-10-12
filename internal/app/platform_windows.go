//go:build windows

package app

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/linhh-phv/claude-acc/internal/i18n"
	"github.com/linhh-phv/claude-acc/internal/ui"
)

// useSymlinks is off on Windows: creating one needs Developer Mode or admin
// rights, and whether Claude Code follows it there is unmeasured. Every
// workspace folder gets its own registered copy instead.
const useSymlinks = false

// execClaude runs claude and hands back its exit code — Windows has no exec.
func execClaude(env []string, args []string) int {
	c := exec.Command("claude", args...)
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
