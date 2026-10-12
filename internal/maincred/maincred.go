// Package maincred reads the "main" account — the one Claude Code itself
// signed in with `claude auth login` — and gets Claude Code to refresh it.
//
// Its access token lives a few hours and only Claude Code holds the refresh
// token, so a 401 on main usually just means "expired": run one cheap request
// through `claude`, which renews and stores it, then read it again.
package maincred

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/linhh-phv/claude-acc/internal/pyjson"
)

// ErrNoCred means the main credential could not be read.
var ErrNoCred = errors.New("main credential not readable")

// AccessToken returns main's current OAuth access token.
func AccessToken() (string, error) {
	raw, err := readRaw()
	if err != nil {
		return "", ErrNoCred
	}
	d, err := pyjson.ParseObject(raw)
	if err != nil {
		return "", ErrNoCred
	}
	v, _ := d.Get("claudeAiOauth")
	o, ok := v.(*pyjson.Object)
	if !ok {
		return "", ErrNoCred
	}
	tok := o.String("accessToken")
	if tok == "" {
		return "", ErrNoCred
	}
	return tok, nil
}

// credentialsFile is where Claude Code keeps the credential outside macOS:
// <CLAUDE_CONFIG_DIR or ~/.claude>/.credentials.json.
func credentialsFile() string {
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".claude")
	}
	return filepath.Join(dir, ".credentials.json")
}

// Refresh runs one tiny request through `claude` in a throwaway directory
// whose settings force the stored credential (an empty token at directory
// level overrides any inherited one), so Claude Code renews it.
func Refresh() {
	d, err := os.MkdirTemp("", "claude-acc-refresh.")
	if err != nil {
		return
	}
	defer os.RemoveAll(d)
	os.Chmod(d, 0o700)
	if err := os.MkdirAll(filepath.Join(d, ".claude"), 0o755); err != nil {
		return
	}
	f := filepath.Join(d, ".claude", "settings.local.json")
	if err := os.WriteFile(f, []byte(`{"env":{"CLAUDE_CODE_OAUTH_TOKEN":""}}`), 0o600); err != nil {
		return
	}
	cmd := exec.Command("claude", "-p", "hi", "--model", "haiku")
	cmd.Dir = d
	cmd.Env = WithoutToken(os.Environ())
	cmd.Run()
}

// WithoutToken returns env minus CLAUDE_CODE_OAUTH_TOKEN (`env -u`).
func WithoutToken(env []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if len(kv) >= 24 && kv[:24] == "CLAUDE_CODE_OAUTH_TOKEN=" {
			continue
		}
		out = append(out, kv)
	}
	return out
}
