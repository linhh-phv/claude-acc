//go:build darwin

package maincred

import "os/exec"

// On macOS Claude Code keeps the credential in the Keychain.
func readRaw() ([]byte, error) {
	return exec.Command("security", "find-generic-password", "-s", "Claude Code-credentials", "-w").Output()
}

func where() string { return "keychain" }
