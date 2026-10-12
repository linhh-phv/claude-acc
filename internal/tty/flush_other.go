//go:build !darwin && !windows

package tty

import "golang.org/x/sys/unix"

// FlushFd drops what was typed but not yet read: tcflush(fd, TCIFLUSH).
func FlushFd(fd int) { unix.IoctlSetInt(fd, unix.TCFLSH, unix.TCIFLUSH) }
