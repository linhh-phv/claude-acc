//go:build !darwin && !windows

package clip

import "errors"

func read() string         { return "" }
func write(s string) error { return errors.New("clipboard not supported on this platform") }
