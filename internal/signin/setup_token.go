// Package signin drives Claude Code's two browser sign-in flows:
// `claude setup-token` (mints a secondary account's token) and
// `claude auth login` (signs in main).
package signin

import (
	"os"
	"os/exec"
	"os/signal"

	"github.com/linhh-phv/claude-acc/internal/claudebin"
)

// SetupToken runs `claude setup-token` in the user's terminal and returns
// the token it printed, or "" when it could not be read back (then the
// caller asks for it by hand — this step must never be the reason `add`
// cannot finish).
//
// stdin is deliberately NOT reopened from /dev/tty: measured, its
// "Paste code here" prompt then took no keys and no paste.
func SetupToken(env []string) string {
	return setupToken(env)
}

// plainSetupToken runs it with nothing in between and captures nothing.
// Ctrl+C there is for claude; claude-acc stays up to ask for the token.
func plainSetupToken(env []string) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	path, _ := claudebin.Path()
	c := exec.Command(path, "setup-token")
	c.Env = env
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.Run()
}
