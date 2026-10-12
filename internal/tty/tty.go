// Package tty reads from the user's terminal directly — not stdin, which may
// be a pipe — and restores it after a full-screen program.
package tty

import (
	"bufio"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// ReadLine prints prompt to stderr and reads one line typed at the terminal.
func ReadLine(prompt string) (string, error) {
	os.Stderr.WriteString(prompt)
	f, err := open()
	if err != nil {
		return "", err
	}
	defer f.Close()
	line, err := bufio.NewReader(f).ReadString('\n')
	if err != nil && (err != io.EOF || line == "") {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// ReadSecret reads one line with echo off. Line editing still works, so a
// long pasted token can be fixed before Enter.
func ReadSecret() (string, error) {
	f, err := open()
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := term.ReadPassword(int(f.Fd()))
	// A paste of several lines returns at the first line break; the rest
	// would stay queued and the shell would run it — a piece of a token in
	// the command history. Drop it.
	flush(f)
	return string(b), err
}
