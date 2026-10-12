//go:build !windows

package fsx

import "os"

func rename(from, to string) error { return os.Rename(from, to) }

// protect: the 0600 mode given at creation already keeps others out.
func protect(string) {}
