package signin

import (
	"os"
	"strings"
)

// Browser modes for Wrapper.
const (
	ModeDefault = "__default__" // whatever would open without a wrapper
	ModeClip    = "__clip__"    // copy the URL, open nothing
)

// Claude Code calls $BROWSER with the URL as its only argument (verified on
// macOS for setup-token and auth login). claude-acc sets it to a wrapper
// that can record the URL — the login flow needs the `state` in it — and
// open a chosen app, or only copy the URL.
//
// IMPORTANT: the wrapper must never print to the terminal. setup-token draws
// a full-screen UI while it waits; text written at that moment (measured:
// any) corrupted it until it stopped taking keys. claude prints its own
// fallback URL anyway.

// Wrapper prepares the BROWSER for one sign-in. mode is ModeDefault,
// ModeClip or an app name; urlFile, if set, receives the URL. It returns the
// environment entries to give claude and a cleanup function.
func Wrapper(mode, urlFile string) (env []string, cleanup func(), err error) {
	return wrapper(mode, urlFile)
}

// envBrowserMode / envURLFile carry the wrapper's job when claude-acc itself
// is the BROWSER (Windows, where a script cannot be one).
const (
	envBrowserMode = "CLAUDE_ACC_BROWSER"
	envURLFile     = "CLAUDE_ACC_URL_FILE"
)

// ServeAsBrowser handles the case where this process was started by claude
// as its BROWSER. It reports whether it did; the caller then exits.
func ServeAsBrowser(args []string) bool {
	mode := os.Getenv(envBrowserMode)
	if mode == "" || len(args) != 1 || !strings.Contains(args[0], "://") {
		return false
	}
	url := args[0]
	if f := os.Getenv(envURLFile); f != "" {
		os.WriteFile(f, []byte(url+"\n"), 0o600)
	}
	openURL(mode, url)
	return true
}
