//go:build !windows

package app

import (
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"github.com/linhh-phv/claude-acc/internal/i18n"
	"github.com/linhh-phv/claude-acc/internal/ui"
)

// useSymlinks: one pin file shared through links across a workspace.
const useSymlinks = true

// execClaude replaces this process with claude, like `exec` in a shell, so
// signals and the terminal belong to claude alone.
func execClaude(env []string, args []string) int {
	path, err := exec.LookPath("claude")
	if err != nil {
		ui.Die(i18n.T("err_no_claude"))
	}
	err = syscall.Exec(path, append([]string{"claude"}, args...), env)
	ui.Die(err.Error())
	return 1
}

var shSafe = regexp.MustCompile(`^[A-Za-z0-9._/:@%+=,-]+$`)

// shQuote quotes s for a POSIX shell (what bash's printf %q was for).
func shQuote(s string) string {
	if shSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func unsetLine() string { return "unset " + envToken }

func exportLine(tok string) string { return "export " + envToken + "=" + shQuote(tok) }

// settingsLabel is how `ls` names the global settings file.
func settingsLabel() string { return "~/.claude/settings.json" }
