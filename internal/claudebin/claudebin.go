// Package claudebin finds the claude executable to run.
package claudebin

import "sync"

var (
	once sync.Once
	path string
	err  error
)

// Path returns the claude executable: the one on PATH, except that on
// Windows an npm shim (claude.cmd) is followed to the claude.exe behind it.
func Path() (string, error) {
	once.Do(func() { path, err = find() })
	return path, err
}
