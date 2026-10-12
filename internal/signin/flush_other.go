//go:build !darwin && !windows

package signin

import "golang.org/x/sys/unix"

// flushInput is tcflush(fd, TCIFLUSH).
func flushInput(fd int) { unix.IoctlSetInt(fd, unix.TCFLSH, unix.TCIFLUSH) }
