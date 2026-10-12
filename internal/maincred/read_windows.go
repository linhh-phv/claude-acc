//go:build windows

package maincred

import (
	"os"

	"github.com/linhh-phv/claude-acc/internal/secret"
)

// readRaw follows Claude Code's own order on Windows: Credential Manager
// when it uses it, then the file.
func readRaw() ([]byte, error) {
	if credmanEnabled() {
		if v, err := credmanRead(secret.ReadTarget); err == nil {
			return []byte(v), nil
		}
	}
	return os.ReadFile(credentialsFile())
}
