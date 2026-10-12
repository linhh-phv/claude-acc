//go:build !windows

package signin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/linhh-phv/claude-acc/internal/clip"
)

// shq quotes s for /bin/sh.
func shq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// wrapper writes a tiny /bin/sh script — the form proven on macOS.
func wrapper(mode, urlFile string) ([]string, func(), error) {
	f, err := os.CreateTemp("", "claude-acc-browser.")
	if err != nil {
		return nil, func() {}, err
	}
	var act string
	switch mode {
	case ModeClip:
		act = `printf "%s" "$1" | pbcopy 2>/dev/null`
	case ModeDefault:
		if b := os.Getenv("BROWSER"); b != "" {
			act = shq(b) + ` "$1"`
		} else {
			act = `open "$1"`
		}
	default:
		act = "open -a " + shq(mode) + ` "$1"`
	}
	script := "#!/bin/sh\n"
	if urlFile != "" {
		script += fmt.Sprintf("printf \"%%s\\n\" \"$1\" > %s\n", shq(urlFile))
	}
	script += act + "\n"
	f.WriteString(script)
	f.Close()
	os.Chmod(f.Name(), 0o700)
	name := f.Name()
	return []string{"BROWSER=" + name}, func() { os.Remove(name) }, nil
}

// openURL is used only when claude-acc itself serves as BROWSER (not the
// macOS path, kept for completeness).
func openURL(mode, url string) {
	switch mode {
	case ModeClip:
		clip.Write(url)
	case ModeDefault:
		exec.Command("open", url).Run()
	default:
		exec.Command("open", "-a", mode, url).Run()
	}
}

// BrowserMissing reports whether --browser names an app that is not in
// /Applications (macOS).
func BrowserMissing(name string) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	st, err := os.Stat(filepath.Join("/Applications", name+".app"))
	return err != nil || !st.IsDir()
}
