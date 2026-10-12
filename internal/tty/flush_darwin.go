//go:build darwin

package tty

import "golang.org/x/sys/unix"

// FlushFd drops what was typed but not yet read: tcflush(fd, TCIFLUSH),
// i.e. TIOCFLUSH with FREAD.
func FlushFd(fd int) { unix.IoctlSetPointerInt(fd, unix.TIOCFLUSH, 1) }
