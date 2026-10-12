//go:build windows

package signin

// setupToken runs claude directly. Reading the token back would need a
// pseudo console (ConPTY) between claude and the terminal — not built yet,
// so the user copies the token and `add` takes it from the clipboard.
func setupToken(env []string) string {
	plainSetupToken(env)
	return ""
}
