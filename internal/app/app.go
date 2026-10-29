// Package app is claude-acc's commands.
package app

import (
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/linhh-phv/claude-acc/internal/claudebin"
	"github.com/linhh-phv/claude-acc/internal/i18n"
	"github.com/linhh-phv/claude-acc/internal/ui"
)

// Version of claude-acc.
const Version = "2.0.3"

// Run executes one command line and returns the exit code.
func Run(args []string) int {
	setupPaths()
	i18n.Lang = i18n.Detect(langFile)
	ui.Init()

	cmd := "help"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	// help and version answer even before Claude Code is installed.
	switch cmd {
	case "help", "-h", "--help":
		io.WriteString(ui.Stdout, i18n.Help(ui.Bold, ui.Reset, Version))
		return 0
	case "version", "-v", "--version":
		ui.Out("claude-acc " + Version)
		return 0
	}

	if !supported() {
		ui.Die(i18n.T("err_macos"))
	}
	if _, err := claudebin.Path(); err != nil {
		ui.Die(i18n.T("err_no_claude"))
	}
	ensureFiles()

	switch cmd {
	case "add":
		return cmdAdd(args)
	case "here", "pin":
		return cmdHere(args)
	// `trigger`/`warm` used to be commands of their own, but all they did is
	// what `usage` does anyway: one request, which opens the 5-hour window.
	case "usage", "quota", "trigger", "warm":
		return cmdUsage(args)
	case "lang", "language":
		return cmdLang(args)
	case "reassign":
		return cmdReassign(args)
	case "sync":
		return cmdSync(args)
	case "switch", "use":
		// `switch <name> --here` is another spelling of `here <name>`.
		rest, here := without(args, "--here")
		if here {
			return cmdHere(rest)
		}
		return cmdSwitch(args)
	case "login":
		return cmdLogin(args)
	case "rm", "remove", "del":
		return cmdRm(args)
	case "ls", "list":
		return cmdLs(args)
	case "run", "exec":
		return cmdRun(args)
	case "env":
		return cmdEnv(args)
	case "check", "status":
		return cmdCheck(args)
	case "uninstall":
		return cmdUninstall(args)
	}
	ui.Die(i18n.Tp("err_unknown_cmd", cmd))
	return 1
}

func supported() bool { return runtime.GOOS == "darwin" || runtime.GOOS == "windows" }

// arg returns args[i] or "".
func arg(args []string, i int) string {
	if i < len(args) {
		return args[i]
	}
	return ""
}

// without returns args minus every occurrence of flag, and whether it was there.
func without(args []string, flag string) ([]string, bool) {
	var out []string
	found := false
	for _, a := range args {
		if a == flag {
			found = true
			continue
		}
		out = append(out, a)
	}
	return out, found
}

// toStderr runs f with everything it prints to stdout — its own lines and
// those of the programs it starts — sent to stderr instead.
func toStderr(f func() int) int {
	old := ui.Stdout
	ui.Stdout = ui.Stderr
	defer func() { ui.Stdout = old }()
	return f()
}

// claudeCmd prepares `claude args...` wired to the terminal (stdout follows
// ui.Stdout).
func claudeCmd(env []string, args ...string) *exec.Cmd {
	path, _ := claudebin.Path()
	c := exec.Command(path, args...)
	c.Env = env
	c.Stdin = os.Stdin
	c.Stdout = ui.Stdout
	c.Stderr = os.Stderr
	return c
}
