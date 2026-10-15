//go:build darwin

package parity

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestHomeOfOneWorkspace is a deliberate difference from 1.x. svc is folder
// [0] of its own workspace and an extra folder of a bigger one, app. 1.x
// called that "several workspaces" and refused to pin; besides, it took the
// first workspace by file name for --all and ls, so app's. 2.x pins svc for
// its own workspace and leaves app's alone: app's chat tabs run in app,
// which a pin in svc never reaches anyway. The other way round, pinning app
// leaves svc's pin alone too — 1.x moved it along with app's.
func TestHomeOfOneWorkspace(t *testing.T) {
	goBin, bashBin := binaries(t)
	s := newSandbox(t, apiSpecs)
	s.extraEnv = map[string]string{}
	lang := "en_US.UTF-8"

	var app, svc string
	setup := func() {
		s.reset()
		std(s)
		app, svc = s.repo("hub/app"), s.repo("hub/svc")
		s.repo("hub/svc-wt")
		s.repo("hub/lib")
		s.file("hub/app.code-workspace", `{"folders": [{"path": "app"}, {"path": "lib"}, {"path": "svc"}]}`)
		s.file("hub/svc.code-workspace", `{"folders": [{"path": "svc"}, {"path": "svc-wt"}]}`)
		pin(s, app, "play", "tok-play")
		s.registry(app + "\tplay")
	}
	args := []string{"here", "work", "--all", "--no-check"}

	setup()
	if r := s.run([]string{"bash", bashBin}, svc, args, "", lang); r.code == 0 || !strings.Contains(r.stderr, "SEVERAL different workspaces") {
		t.Fatalf("1.x was expected to refuse; exit %d\n%s", r.code, r.stderr)
	}

	setup()
	r := s.run([]string{goBin}, svc, args, "", lang)
	if r.code != 0 {
		t.Fatalf("here exited %d\n%s", r.code, r.stderr)
	}
	for _, want := range []string{"svc.code-workspace (2 folder)", "Also a folder of app.code-workspace"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("here does not say %q:\n%s", want, r.stderr)
		}
	}
	f := r.files
	if got := f["w/hub/svc/.claude/settings.local.json"]; !strings.Contains(got, `"CLAUDE_ACC_NAME": "work"`) {
		t.Errorf("svc not pinned to work: %q", got)
	}
	if got, want := f["w/hub/svc-wt/.claude/settings.local.json"], "symlink -> "+filepath.Join(svc, ".claude", "settings.local.json"); got != want {
		t.Errorf("svc-wt: %q, want %q", got, want)
	}
	if got := f["w/hub/app/.claude/settings.local.json"]; !strings.Contains(got, `"CLAUDE_ACC_NAME": "play"`) {
		t.Errorf("app's own pin was touched: %q", got)
	}
	if got, ok := f["w/hub/lib/.claude/settings.local.json"]; ok {
		t.Errorf("lib (app's folder) got a pin: %q", got)
	}

	// ls from svc: a pin of its own, not one folder of app's workspace.
	r = s.run([]string{goBin}, svc, []string{"ls"}, "", lang)
	if r.code != 0 {
		t.Fatalf("ls exited %d\n%s", r.code, r.stderr)
	}
	for _, want := range []string{"* work     " + svc, "  play     " + app, "HERE      work"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("ls lacks %q:\n%s", want, r.stdout)
		}
	}

	// Pinning app — with or without --all — does not move svc's pin.
	svcPin := r.files["w/hub/svc/.claude/settings.local.json"]
	for _, a := range [][]string{{"here", "main", "--no-check"}, {"here", "play", "--all", "--no-check"}} {
		r = s.run([]string{goBin}, app, a, "", lang)
		if r.code != 0 {
			t.Fatalf("%v from app exited %d\n%s", a, r.code, r.stderr)
		}
		if !strings.Contains(r.stderr, "skipped "+svc+" — it is folder [0] of svc.code-workspace") {
			t.Errorf("%v does not say svc was skipped:\n%s", a, r.stderr)
		}
		if got := r.files["w/hub/svc/.claude/settings.local.json"]; got != svcPin {
			t.Errorf("%v moved svc's pin: %q", a, got)
		}
	}
	if got, want := r.files["w/hub/lib/.claude/settings.local.json"], "symlink -> "+filepath.Join(app, ".claude", "settings.local.json"); got != want {
		t.Errorf("lib: %q, want %q", got, want)
	}
	r = s.run([]string{goBin}, app, []string{"ls"}, "", lang)
	if !strings.Contains(r.stdout, "whole workspace: 3 folders (1 shared, 0 keep their own config)") {
		t.Errorf("app's summary counts svc:\n%s", r.stdout)
	}
}
