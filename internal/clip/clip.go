// Package clip reads and writes the system clipboard as text.
package clip

import "regexp"

// tokenRe is the exact shape `claude setup-token` prints. A looser "sk-ant-"
// risked picking up an unrelated key that happened to be copied.
var tokenRe = regexp.MustCompile(`sk-ant-oat01-[A-Za-z0-9._-]*`)

// Read returns the clipboard text, or "" when it cannot.
func Read() string { return read() }

// Write replaces the clipboard text.
func Write(s string) error { return write(s) }

// Token returns the first "sk-ant-oat01-…" on the clipboard, even with text
// around it.
func Token() string { return tokenRe.FindString(Read()) }

// FindToken returns the first token in s.
func FindToken(s string) string { return tokenRe.FindString(s) }
