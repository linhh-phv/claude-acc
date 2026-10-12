//go:build windows

package signin

import (
	"bufio"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// lineSource reads lines typed at the console (CONIN$, line mode with
// echo). The console has no poll-with-timeout for whole lines, so one
// goroutine blocks on it and hands lines over.
type lineSource struct {
	h     windows.Handle
	lines chan string
}

func openLines() *lineSource {
	p, _ := windows.UTF16PtrFromString("CONIN$")
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return &lineSource{h: windows.InvalidHandle}
	}
	s := &lineSource{h: h, lines: make(chan string, 16)}
	go func() {
		r := bufio.NewReader(os.NewFile(uintptr(h), "CONIN$"))
		for {
			ln, err := r.ReadString('\n')
			if ln != "" {
				s.lines <- ln
			}
			if err != nil {
				return
			}
		}
	}()
	return s
}

func (s *lineSource) poll(d time.Duration) []string {
	if s.h == windows.InvalidHandle {
		time.Sleep(d)
		return nil
	}
	select {
	case ln := <-s.lines:
		return []string{ln}
	case <-time.After(d):
		return nil
	}
}

// close drops typed-but-unread input, so it does not land in the shell, and
// cancels the read still waiting in the goroutine — left there, it would eat
// the first line of any later prompt.
func (s *lineSource) close() {
	if s.h == windows.InvalidHandle {
		return
	}
	windows.CancelIoEx(s.h, nil)
	windows.FlushConsoleInputBuffer(s.h)
}
