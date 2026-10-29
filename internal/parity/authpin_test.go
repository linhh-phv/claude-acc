//go:build darwin

package parity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linhh-phv/claude-acc/internal/i18n"
)

// TestPinForNewClaudeCode: Claude Code 2.1.312 ignores CLAUDE_CODE_OAUTH_TOKEN
// in repo settings, so a token pin also carries ANTHROPIC_AUTH_TOKEN; a main
// pin cannot override a token global account there, and says so; a pin
// written before (1.x, older 2.x) asks for `sync`, which fixes it.
func TestPinForNewClaudeCode(t *testing.T) {
	goBin, _ := binaries(t)
	s := newSandbox(t, apiSpecs)
	s.reset()
	std(s)
	lang := "en_US.UTF-8"
	i18n.Lang = "en"
	run := func(dir string, args ...string) result { return s.run([]string{goBin}, dir, args, "", lang) }
	read := func(dir string) string {
		b, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
		return string(b)
	}
	outdated := i18n.T("pin_outdated")

	r1 := s.repo("r1")
	s.writeAbs(filepath.Join(r1, ".claude", "settings.local.json"),
		`{"env": {"FOO": "1"}, "permissions": {"allow": []}}`+"\n")

	// A token pin: both keys, the same token; everything else kept.
	run(r1, "here", "work", "--no-check")
	want := `{
  "env": {
    "FOO": "1",
    "CLAUDE_CODE_OAUTH_TOKEN": "tok-work",
    "CLAUDE_ACC_NAME": "work",
    "ANTHROPIC_AUTH_TOKEN": "tok-work"
  },
  "permissions": {
    "allow": []
  }
}
`
	if got := read(r1); got != want {
		t.Errorf("token pin:\n%s\nwant\n%s", got, want)
	}
	if l := run(r1, "ls"); strings.Contains(l.stdout, "! ") {
		t.Errorf("warning under a current pin:\n%s", l.stdout)
	}

	// reassign moves both keys.
	run(s.w, "reassign", "work", "play")
	if got := read(r1); !strings.Contains(got, `"CLAUDE_CODE_OAUTH_TOKEN": "tok-play"`) || !strings.Contains(got, `"ANTHROPIC_AUTH_TOKEN": "tok-play"`) {
		t.Errorf("after reassign:\n%s", got)
	}

	// main: no ANTHROPIC_AUTH_TOKEN — and fine while main is the global account.
	if h := run(r1, "here", "main", "--no-check"); strings.Contains(h.stderr, "! ") {
		t.Errorf("warning for main under a main global:\n%s", h.stderr)
	}
	if got := read(r1); strings.Contains(got, "ANTHROPIC_AUTH_TOKEN") || !strings.Contains(got, `"CLAUDE_CODE_OAUTH_TOKEN": ""`) {
		t.Errorf("main pin:\n%s", got)
	}

	// ...but not under a token global account: here, ls and switch say so.
	sw := run(s.w, "switch", "play")
	if w := i18n.Tp("sw_main_pins", "1", "play"); !strings.Contains(sw.stderr, w+"\n") {
		t.Errorf("switch does not warn about the main pin:\n%s", sw.stderr)
	}
	mainIgnored := i18n.Tp("pin_main_ignored", "play", "play")
	if h := run(r1, "here", "main", "--no-check"); !strings.Contains(h.stderr, mainIgnored+"\n") {
		t.Errorf("here main under a token global:\n%s", h.stderr)
	}
	if l := run(r1, "ls"); !strings.Contains(l.stdout, "HERE      main  (pin of "+r1+")\n"+mainIgnored+"\n") {
		t.Errorf("ls under a main pin with a token global:\n%s", l.stdout)
	}

	// off takes both keys out, and leaves the rest.
	run(r1, "here", "off")
	if got := read(r1); strings.Contains(got, "TOKEN") || !strings.Contains(got, `"FOO": "1"`) {
		t.Errorf("after off:\n%s", got)
	}

	// A pin from before: ls asks for sync, sync fixes it.
	r2 := s.repo("r2")
	pin(s, r2, "work", "tok-work")
	s.registry(r2 + "\twork")
	if l := run(r2, "ls"); !strings.Contains(l.stdout, "HERE      work  (pin of "+r2+")\n"+outdated+"\n") {
		t.Errorf("ls under an old pin:\n%s", l.stdout)
	}
	if u := run(r2, "usage", "work"); !strings.Contains(u.stdout, outdated+"\n") {
		t.Errorf("usage under an old pin:\n%s", u.stdout)
	}
	run(s.w, "sync")
	if got := read(r2); !strings.Contains(got, `"ANTHROPIC_AUTH_TOKEN": "tok-work"`) {
		t.Errorf("sync did not add the key:\n%s", got)
	}
	if l := run(r2, "ls"); strings.Contains(l.stdout, outdated) {
		t.Errorf("still asks for sync after it:\n%s", l.stdout)
	}
}
