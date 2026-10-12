//go:build !windows

package signin

import (
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// lineSource reads what the user types at the terminal, in its normal
// (cooked, echoing) mode. A raw fd with select(2): poll(2) on macOS "does not
// support devices" (man page, BUGS) — it answers POLLNVAL for /dev/tty, and
// the typed line was never read. Measured in the parity test.
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
	var rset unix.FdSet
	rset.Zero()
	rset.Set(s.fd)
	tv := unix.NsecToTimeval(d.Nanoseconds())
	n, err := unix.Select(s.fd+1, &rset, nil, nil, &tv)
	if err != nil || n <= 0 || !rset.IsSet(s.fd) {
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
