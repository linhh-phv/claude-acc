//go:build !windows

package signin

import (
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// lineSource reads what the user types at the terminal, in its normal
// (cooked, echoing) mode. A raw fd with poll rather than os.File: the Go
// runtime poller and a tty do not mix well on macOS.
type lineSource struct{ fd int }

func openLines() *lineSource {
	fd, err := unix.Open("/dev/tty", unix.O_RDONLY, 0)
	if err != nil {
		return &lineSource{fd: -1}
	}
	return &lineSource{fd: fd}
}

// poll waits up to d for a line and returns the lines read (canonical mode
// delivers whole lines).
func (s *lineSource) poll(d time.Duration) []string {
	if s.fd < 0 {
		time.Sleep(d)
		return nil
	}
	fds := []unix.PollFd{{Fd: int32(s.fd), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, int(d/time.Millisecond))
	if err != nil || n == 0 || fds[0].Revents&unix.POLLIN == 0 {
		if err == nil && n > 0 {
			time.Sleep(d) // hang-up or error: do not spin
		}
		return nil
	}
	buf := make([]byte, 4096)
	m, err := unix.Read(s.fd, buf)
	if err != nil || m <= 0 {
		return nil // Ctrl+D on an empty line
	}
	return strings.Split(strings.TrimRight(string(buf[:m]), "\r\n"), "\n")
}

// close drops whatever was typed but not read — e.g. a code pasted just as
// claude finished signing in on its own — or the shell would run it as a
// command.
func (s *lineSource) close() {
	if s.fd < 0 {
		return
	}
	flushInput(s.fd)
	unix.Close(s.fd)
}
