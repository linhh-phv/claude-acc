//go:build darwin

package parity

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/linhh-phv/claude-acc/internal/i18n"
)

// secondsAgo hides the age of a usage reading in seconds: the two `ls` runs
// compared can straddle a second.
var secondsAgo = regexp.MustCompile(`\d+s (trước|ago)`)

func sameLs(t *testing.T, when string, b, g result) {
	t.Helper()
	if secondsAgo.ReplaceAllString(b.stdout, "Ns $1") != secondsAgo.ReplaceAllString(g.stdout, "Ns $1") {
		t.Fatalf("ls %s differs\n--- bash ---\n%s\n--- go ---\n%s", when, b.stdout, g.stdout)
	}
}

// TestUpgradeKeepsAccounts is the promise of the rewrite: everything set up
// with the bash version (1.x) keeps working under the Go version (2.x) with
// nothing added again — and the bash version still reads what Go wrote, so
// going back is safe too.
func TestUpgradeKeepsAccounts(t *testing.T) {
	goBin, bashBin := binaries(t)
	specs := map[string]apiSpec{}
	for k, v := range apiSpecs {
		specs[k] = v
	}
	specs["sk-ant-oat01-PLAYtoken_XYZ-789"] = apiSpec{code: 200, status: [2]string{"allowed", "allowed"}, util: [2]string{"0.25", "0.4"}}
	s := newSandbox(t, specs)
	s.extraEnv = map[string]string{}
	s.reset()

	bash := []string{"bash", bashBin}
	gobin := []string{goBin}
	lang := "en_US.UTF-8"
	step := func(who []string, cwd, stdin string, args ...string) result {
		t.Helper()
		r := s.run(who, cwd, args, stdin, lang)
		if r.code != 0 {
			t.Fatalf("%s %v exited %d\nstdout:\n%s\nstderr:\n%s", filepath.Base(who[len(who)-1]), args, r.code, r.stdout, r.stderr)
		}
		return r
	}

	// ---- 1. the old version sets everything up
	r1 := s.repo("r1")
	a, b, c := workspace(s, "proj")
	step(bash, s.w, "sk-ant-oat01-GOODtoken_ABC-123\n", "add", "work", "--no-setup", "--stdin")
	step(bash, s.w, "sk-ant-oat01-PLAYtoken_XYZ-789\n", "add", "play", "--no-setup", "--stdin")
	step(bash, s.w, "", "switch", "work")
	step(bash, r1, "", "here", "play", "--no-check")
	step(bash, b, "", "here", "work", "--all", "--no-check")
	step(bash, s.w, "", "usage")
	step(bash, s.w, "", "lang", "vi")
	lang = "vi_VN.UTF-8"

	// ---- 2. the new version sees exactly the same world
	lsB := step(bash, r1, "", "ls")
	lsG := step(gobin, r1, "", "ls")
	// 1.x pins lack ANTHROPIC_AUTH_TOKEN: 2.x says to run sync, then is the same.
	i18n.Lang = "vi"
	if !strings.Contains(lsG.stdout, "TẠI ĐÂY   play  (ghim của "+r1+")\n"+i18n.T("pin_outdated")+"\n") {
		t.Errorf("go ls does not ask for sync under a 1.x pin:\n%s", lsG.stdout)
	}
	dropPinWarnings(&lsG)
	sameLs(t, "after upgrade", lsB, lsG)
	for _, want := range []string{"✓ work", "✓ play", "TẠI ĐÂY   play", "TOÀN CỤC  work", "~"} {
		if !strings.Contains(lsG.stdout, want) {
			t.Errorf("go ls lacks %q:\n%s", want, lsG.stdout)
		}
	}

	// ...and keeps working with it, nothing re-added
	step(gobin, s.w, "", "check", "work")
	step(gobin, s.w, "", "check", "play")
	step(gobin, s.w, "", "switch", "play")
	step(gobin, r1, "", "here", "off")
	step(gobin, c, "", "here", "play", "--no-check") // a linked member: lands on folder [0]
	step(gobin, s.w, "", "sync")
	step(gobin, s.w, "", "usage")
	if got := s.snapshot()["w/hub/b/.claude/settings.local.json"]; got != "symlink -> "+filepath.Join(a, ".claude", "settings.local.json") {
		t.Errorf("workspace link was not kept: %q", got)
	}
	if got := s.snapshot()["w/hub/a/.claude/settings.local.json"]; !strings.Contains(got, `"ANTHROPIC_AUTH_TOKEN": "sk-ant-oat01-PLAYtoken_XYZ-789"`) {
		t.Errorf("folder [0] did not move to play under both keys: %q", got)
	}

	// ---- 3. and the old version still reads what the new one wrote
	lsG2 := step(gobin, c, "", "ls")
	lsB2 := step(bash, c, "", "ls")
	sameLs(t, "after going back", lsB2, lsG2)
	step(bash, s.w, "", "check", "play")
	step(bash, s.w, "", "switch", "main")
	step(bash, s.w, "", "rm", "work")
}
