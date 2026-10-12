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

func where() string {
	if credmanEnabled() {
		if _, err := credmanRead(secret.ReadTarget); err == nil {
			return "credential-manager " + credmanService() + "/" + credmanName
		}
		return "file (Credential Manager is in use but holds no entry) " + credentialsFile()
	}
	return "file " + credentialsFile()
}
