//go:build windows

package tty

import (
	"os"

	"golang.org/x/sys/windows"
)

// open returns the console input, the Windows counterpart of /dev/tty.
func open() (*os.File, error) {
	p, _ := windows.UTF16PtrFromString("CONIN$")
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), "CONIN$"), nil
}

// Sane restores the usual line-input mode of the console.
func Sane() {
	f, err := open()
	if err != nil {
		return
	}
	defer f.Close()
	h := windows.Handle(f.Fd())
	var mode uint32
	if windows.GetConsoleMode(h, &mode) != nil {
		return
	}
	mode |= windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_INPUT
	windows.SetConsoleMode(h, mode)
}
