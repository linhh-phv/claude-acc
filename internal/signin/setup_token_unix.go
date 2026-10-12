//go:build !windows

package signin

import (
	"os"
	"os/exec"
)

// setupToken goes through script(1). Unlike capturing stdout (tried before:
// stdout became a pipe and the full-screen UI hung), `script` gives claude a
// real pty — it still sees a normal terminal — while recording what it
// displays, from which the token is read back.
func setupToken(env []string) string {
	if _, err := exec.LookPath("script"); err != nil {
		plainSetupToken(env)
		return ""
	}
	f, err := os.CreateTemp("", "claude-acc-typescript.")
	if err != nil {
		plainSetupToken(env)
		return ""
	}
	log := f.Name()
	f.Close()
	defer os.Remove(log)

	c := exec.Command("script", "-q", log, "claude", "setup-token")
	c.Env = env
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.Run()

	raw, err := os.ReadFile(log)
	if err != nil {
		return ""
	}
	return TokenFromTranscript(string(raw))
}
