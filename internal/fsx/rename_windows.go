//go:build windows

package fsx

import (
	"os"
	"time"
)

// rename retries briefly: on Windows replacing a file fails while another
// process has it open without delete-sharing — Claude Code reads
// settings.json on every request.
func rename(from, to string) error {
	var err error
	for i := 0; i < 20; i++ {
		if err = os.Rename(from, to); err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return err
}
