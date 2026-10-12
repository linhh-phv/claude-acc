//go:build !darwin

package maincred

import "os"

// Outside macOS Claude Code keeps the credential in a file (docs:
// %USERPROFILE%\.claude\.credentials.json on Windows).
func readRaw() ([]byte, error) { return os.ReadFile(credentialsFile()) }
