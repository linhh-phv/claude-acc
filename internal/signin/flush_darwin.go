//go:build darwin

package signin

import "golang.org/x/sys/unix"

// flushInput is tcflush(fd, TCIFLUSH): TIOCFLUSH with FREAD.
func flushInput(fd int) { unix.IoctlSetPointerInt(fd, unix.TIOCFLUSH, 1) }
