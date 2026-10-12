// Command claude-acc runs several Claude Code accounts on one machine
// without logging in again.
package main

import (
	"os"

	"github.com/linhh-phv/claude-acc/internal/app"
	"github.com/linhh-phv/claude-acc/internal/signin"
)

func main() {
	// Started by claude as its BROWSER (Windows): open/copy the URL, quietly.
	if signin.ServeAsBrowser(os.Args[1:]) {
		return
	}
	os.Exit(app.Run(os.Args[1:]))
}
