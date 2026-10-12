// Package ui prints: colours, progress notes on stderr, fatal errors.
package ui

import (
	"fmt"
	"io"
	"os"

	"github.com/linhh-phv/claude-acc/internal/i18n"
	"golang.org/x/term"
)

// Colour codes. Empty when stdout is not a terminal, so `claude-acc ls > file`
// does not fill the file with ANSI codes — the same switch drives the notes on
// stderr, as it always has.
var Dim, Bold, Red, Green, Yellow, Reset string

// Stdout and Stderr are where output goes.
var Stdout io.Writer = os.Stdout
var Stderr io.Writer = os.Stderr

// Exit ends the program. A variable so a caller can run cleanup first.
var Exit = os.Exit

// Init turns colours on when stdout is a terminal that can show them.
func Init() {
	if !term.IsTerminal(int(os.Stdout.Fd())) || !enableVT() {
		return
	}
	Dim, Bold, Red, Green, Yellow, Reset = "\033[2m", "\033[1m", "\033[31m", "\033[32m", "\033[33m", "\033[0m"
}

// Info prints one line to stderr.
func Info(s string) { fmt.Fprintln(Stderr, s) }

// Out prints one line to stdout.
func Out(s string) { fmt.Fprintln(Stdout, s) }

// Die prints "error: msg" and exits 1.
func Die(msg string) {
	fmt.Fprintf(Stderr, "%s%s%s %s\n", Red, i18n.T("err_prefix"), Reset, msg)
	Exit(1)
}
