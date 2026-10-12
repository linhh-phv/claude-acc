// Package secret stores the long-lived token of each secondary account in the
// OS credential store: the Keychain on macOS, Credential Manager on Windows.
package secret

import (
	"errors"
	"os"
)

// Service is the Keychain service (macOS) / target prefix (Windows) every
// token is filed under. Unchanged from the bash version, so existing
// accounts are found.
const Service = "claude-code-token"

// service is Service, or CLAUDE_ACC_SECRET_SERVICE: the Windows self-test
// files its tokens under a name of its own, never touching real accounts.
func service() string {
	if s := os.Getenv("CLAUDE_ACC_SECRET_SERVICE"); s != "" {
		return s
	}
	return Service
}

// ErrNotFound means no token is stored for that account.
var ErrNotFound = errors.New("no token stored")

// Get returns the token of account name.
func Get(name string) (string, error) { return get(name) }

// Has reports whether a token is stored for name.
func Has(name string) bool {
	_, err := get(name)
	return err == nil
}

// Set stores (or replaces) the token of account name.
func Set(name, token string) error { return set(name, token) }

// Delete removes the token of account name.
func Delete(name string) error { return del(name) }
