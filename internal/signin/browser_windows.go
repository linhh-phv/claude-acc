//go:build windows

package signin

import (
	"os"
	"os/exec"
	"strings"

	"github.com/linhh-phv/claude-acc/internal/clip"
	"golang.org/x/sys/windows/registry"
)

// wrapper points BROWSER at claude-acc.exe itself: Node refuses to spawn a
// .cmd/.bat without a shell, so a script cannot serve as BROWSER here. The
// job travels in two environment variables; see ServeAsBrowser.
//
// Read from Claude Code's Windows build (2.1.296): when BROWSER is set it
// spawns it with the URL as the only argument, directly (execFile, no shell,
// environment inherited) — so a path with spaces is fine.
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
		defaultBrowser(url)
	default:
		if exe := AppPath(mode); exe != "" {
			if exec.Command(exe, url).Start() == nil {
				return
			}
		}
		defaultBrowser(url) // an unknown name still gets the user a browser
	}
}

// defaultBrowser opens url the way Claude Code itself does without BROWSER.
func defaultBrowser(url string) { exec.Command("rundll32", "url.dll,OpenURL", url).Run() }

// AppPath resolves a browser name ("msedge", "chrome", "firefox", "brave")
// to its .exe through the App Paths registry key, or returns a path given
// directly. "" when neither works.
func AppPath(name string) string {
	if st, err := os.Stat(name); err == nil && !st.IsDir() {
		return name
	}
	exe := name
	if !strings.HasSuffix(strings.ToLower(exe), ".exe") {
		exe += ".exe"
	}
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		k, err := registry.OpenKey(root, `Software\Microsoft\Windows\CurrentVersion\App Paths\`+exe, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := k.GetStringValue("")
		k.Close()
		if err != nil {
			continue
		}
		if x, err := registry.ExpandString(v); err == nil {
			v = x
		}
		v = strings.Trim(v, `"`)
		if st, err := os.Stat(v); err == nil && !st.IsDir() {
			return v
		}
	}
	return ""
}

// BrowserMissing reports whether --browser names nothing this machine can
// open.
func BrowserMissing(name string) bool { return AppPath(name) == "" }
