//go:build darwin

package clip

import (
	"os/exec"
	"strings"
)

func read() string {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func write(s string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}
