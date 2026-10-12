//go:build windows

package ui

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVT asks the console to interpret ANSI colour codes. Old consoles that
// refuse get no colours instead of raw escape codes.
func enableVT() bool {
	ok := true
	for _, f := range []*os.File{os.Stdout, os.Stderr} {
		h := windows.Handle(f.Fd())
		var mode uint32
		if windows.GetConsoleMode(h, &mode) != nil {
			continue // redirected; nothing to enable
		}
		if windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) != nil {
			ok = false
		}
	}
	return ok
}
