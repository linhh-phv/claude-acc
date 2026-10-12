package signin

import (
	"os"
	"os/exec"
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
func plainSetupToken(env []string) {
	c := exec.Command("claude", "setup-token")
	c.Env = env
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.Run()
}
