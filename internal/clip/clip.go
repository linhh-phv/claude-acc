// Package clip reads and writes the system clipboard as text.
package clip

import "github.com/linhh-phv/claude-acc/internal/token"

// Read returns the clipboard text, or "" when it cannot.
func Read() string { return read() }

// Write replaces the clipboard text.
func Write(s string) error { return write(s) }

// Token returns the token on the clipboard, even with text around it or
// broken across lines by the terminal it was copied from.
func Token() string { return token.Extract(Read()) }
