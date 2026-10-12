//go:build !windows

package claudebin

import "os/exec"

func find() (string, error) { return exec.LookPath("claude") }
