//go:build !windows

package tty

import (
	"os"
	"os/exec"
)

func open() (*os.File, error) { return os.OpenFile("/dev/tty", os.O_RDWR, 0) }

func flush(f *os.File) { FlushFd(int(f.Fd())) }

// Sane puts the terminal back in its normal mode. `claude setup-token` is a
// full-screen program and can leave it raw, after which a prompt would not
// receive anything.
func Sane() {
	f, err := os.Open("/dev/tty")
	if err != nil {
		return
	}
	defer f.Close()
	c := exec.Command("stty", "sane")
	c.Stdin = f
	c.Run()
}
