//go:build windows

package signin

import (
	"os"

	"github.com/linhh-phv/claude-acc/internal/clip"
	"golang.org/x/sys/windows"
)

// wrapper points BROWSER at claude-acc.exe itself: Node refuses to spawn a
// .cmd/.bat without a shell, so a script cannot serve as BROWSER here. The
// job travels in two environment variables; see ServeAsBrowser.
//
// UNMEASURED: whether Claude Code on Windows honours BROWSER at all. If it
// does not, it opens the default browser itself and prints the URL; --browser
// and --url-only then have no effect and the login code is not auto-filled,
// but pasting it by hand still works.
func wrapper(mode, urlFile string) ([]string, func(), error) {
	self, err := os.Executable()
	if err != nil {
		return nil, func() {}, err
	}
	env := []string{"BROWSER=" + self, envBrowserMode + "=" + mode}
	if urlFile != "" {
		env = append(env, envURLFile+"="+urlFile)
	}
	return env, func() {}, nil
}

func openURL(mode, url string) {
	switch mode {
	case ModeClip:
		clip.Write(url)
	case ModeDefault:
		shellOpen(url, "")
	default:
		shellOpen(mode, url) // e.g. "chrome", "msedge", "firefox"
	}
}

func shellOpen(file, params string) {
	verb, _ := windows.UTF16PtrFromString("open")
	f, _ := windows.UTF16PtrFromString(file)
	var p *uint16
	if params != "" {
		p, _ = windows.UTF16PtrFromString(params)
	}
	windows.ShellExecute(0, verb, f, p, nil, windows.SW_SHOWNORMAL)
}
