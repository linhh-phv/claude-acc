//go:build darwin

package secret

import (
	"os/exec"
	"strings"
)

// The `security` CLI rather than the Keychain API: no cgo, and it is exactly
// what the bash version called, so the items stay byte-for-byte the same.

func get(name string) (string, error) {
	out, err := exec.Command("security", "find-generic-password", "-s", Service, "-a", name, "-w").Output()
	if err != nil {
		return "", ErrNotFound
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func set(name, token string) error {
	return exec.Command("security", "add-generic-password", "-U",
		"-s", Service, "-a", name, "-w", token,
		"-D", "Claude Code account token",
		"-j", "Token dang nhap Claude Code cho acc '"+name+"' (quan ly boi claude-acc)",
	).Run()
}

func del(name string) error {
	if err := exec.Command("security", "delete-generic-password", "-s", Service, "-a", name).Run(); err != nil {
		return ErrNotFound
	}
	return nil
}
