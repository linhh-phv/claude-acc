//go:build !darwin && !windows

package maincred

import "os"

// Elsewhere (Linux) Claude Code keeps the credential in a file.
func readRaw() ([]byte, error) { return os.ReadFile(credentialsFile()) }

func where() string { return "file " + credentialsFile() }
