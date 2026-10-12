//go:build darwin

package parity

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var apiSpecs = map[string]apiSpec{
	"main-ok":                        {code: 200, status: [2]string{"allowed", "allowed"}, util: [2]string{"0.42", "0.13"}},
	"main-old":                       {code: 401, reason: "OAuth token has expired"},
	"tok-work":                       {code: 200, status: [2]string{"allowed", "allowed"}, util: [2]string{"0.1", "0.55"}},
	"tok-play":                       {code: 200, status: [2]string{"allowed_warning", "allowed"}, util: [2]string{"0.81", "0.2"}},
	"tok-out":                        {code: 429, status: [2]string{"rejected", "allowed"}, util: [2]string{"1.0", "0.5"}, reason: "Rate limited"},
	"tok-week":                       {code: 429, status: [2]string{"allowed", "rejected"}, util: [2]string{"0.3", "1.02"}, reason: "Rate limited"},
	"tok-revoked":                    {code: 401, reason: "OAuth token has been revoked"},
	"tok-500":                        {code: 500},
	"sk-ant-oat01-GOODtoken_ABC-123": {code: 200, status: [2]string{"allowed", "allowed"}, util: [2]string{"0.0", "0.0"}},
}

type scenario struct {
	name  string
	lang  string // default en_US.UTF-8
	setup func(s *sandbox) (cwd string)
	args  []string
	stdin string
	env   map[string]string
	// fix adjusts the two results for a documented, deliberate difference.
	fix func(b, g *result)
}

// std is the usual world: two secondary accounts and main.
func std(s *sandbox) {
	s.accounts("work", "play")
	s.token("work", "tok-work")
	s.token("play", "tok-play")
}

func settingsFile(s *sandbox) string { return filepath.Join(s.home, ".claude", "settings.json") }

func pin(s *sandbox, dir, name, tok string) {
	s.writeAbs(filepath.Join(dir, ".claude", "settings.local.json"),
		`{"env": {"CLAUDE_CODE_OAUTH_TOKEN": "`+tok+`", "CLAUDE_ACC_NAME": "`+name+`"}}`+"\n")
}

// workspace lays out hub/<name>.code-workspace with folders a (folder [0]),
// b and c, all git repos.
func workspace(s *sandbox, name string) (a, b, c string) {
	a, b, c = s.repo("hub/a"), s.repo("hub/b"), s.repo("hub/c")
	s.file("hub/"+name+".code-workspace", `{
	// comment line, as VS Code allows
	"folders": [{"path": "a"}, {"path": "b"}, {"path": "./c"}],
	"settings": {}
}`)
	return
}

var scenarios = []scenario{
	{name: "help-en", args: []string{"help"}, setup: func(s *sandbox) string { return s.w }, fix: noConfigForHelp},
	{name: "help-vi", lang: "vi_VN.UTF-8", args: []string{"--help"}, setup: func(s *sandbox) string { return s.w }, fix: noConfigForHelp},
	{name: "no-args", args: nil, setup: func(s *sandbox) string { return s.w }, fix: noConfigForHelp},
	{name: "unknown-cmd", args: []string{"frobnicate"}, setup: func(s *sandbox) string { return s.w }},

	// ---- ls
	{name: "ls-empty", args: []string{"ls"}, setup: func(s *sandbox) string { return s.w }},
	{name: "ls-full", args: []string{"ls"}, setup: func(s *sandbox) string {
		std(s)
		s.accounts("work", "play", "gone")
		now := s.now.Unix()
		ts := func(d int64) string { return itoa(now + d) }
		s.usageCache(
			"main\t"+ts(-5*3600)+"\t0.42\t"+ts(3600)+"\tallowed\t0.13\t"+ts(86400*3)+"\tallowed",
			"play\t"+ts(-3000)+"\t0.815\t"+ts(-10)+"\tallowed\t0.205\t"+ts(86400)+"\tallowed",
			"work\t"+ts(-3*86400)+"\tx\t"+ts(600)+"\tallowed\t1\t"+ts(600)+"\tallowed")
		r := s.repo("r1")
		pin(s, r, "work", "tok-work")
		a, b, _ := workspace(s, "ws")
		pin(s, a, "play", "tok-play")
		os.MkdirAll(filepath.Join(b, ".claude"), 0o755)
		os.Symlink(filepath.Join(a, ".claude", "settings.local.json"), filepath.Join(b, ".claude", "settings.local.json"))
		s.registry(a+"\tplay", r+"\twork", filepath.Join(s.w, "vanished")+"\twork")
		s.writeAbs(settingsFile(s), `{"env": {"CLAUDE_CODE_OAUTH_TOKEN": "tok-work", "CLAUDE_ACC_NAME": "work"}}`)
		return b
	}},
	{name: "ls-in-repo-pin", lang: "vi_VN.UTF-8", args: []string{"list"}, setup: func(s *sandbox) string {
		std(s)
		r := s.repo("r1")
		pin(s, r, "play", "tok-play")
		s.registry(r + "\tplay")
		return r
	}},

	// ---- switch
	{name: "switch-acc", args: []string{"switch", "work"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "switch-acc-vi", lang: "vi_VN.UTF-8", args: []string{"use", "play"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "switch-main", args: []string{"switch", "main"}, setup: func(s *sandbox) string {
		std(s)
		s.writeAbs(settingsFile(s), `{"model": "opus", "env": {"FOO": "bar", "CLAUDE_CODE_OAUTH_TOKEN": "tok-work", "CLAUDE_ACC_NAME": "work"}, "permissions": {"allow": ["Bash(npm run *)"], "deny": []}}`)
		return s.w
	}},
	{name: "switch-main-only-pin", args: []string{"switch", "main"}, setup: func(s *sandbox) string {
		std(s)
		s.writeAbs(settingsFile(s), `{"env": {"CLAUDE_CODE_OAUTH_TOKEN": "tok-work", "CLAUDE_ACC_NAME": "work"}}`)
		return s.w
	}},
	{name: "switch-unicode-settings", args: []string{"switch", "work"}, setup: func(s *sandbox) string {
		std(s)
		s.writeAbs(settingsFile(s), "{\n  \"statusLine\": {\"type\": \"command\", \"command\": \"echo 'tiếng Việt — ✓' \\\"q\\\" \\t\"},\n  \"n\": [1, 2.50, 1e-7, true, null]\n}\n")
		return s.w
	}},
	{name: "switch-broken-settings", args: []string{"switch", "work"}, setup: func(s *sandbox) string {
		std(s)
		s.writeAbs(settingsFile(s), `{"env": {`)
		return s.w
	}},
	{name: "switch-no-token", args: []string{"switch", "nope"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "switch-bad-name", args: []string{"switch", "a b"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "switch-no-arg", args: []string{"switch"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "switch-while-pinned-here", args: []string{"switch", "work"}, setup: func(s *sandbox) string {
		std(s)
		r := s.repo("r1")
		pin(s, r, "play", "tok-play")
		return s.dir("r1/sub")
	}},
	{name: "switch-revoked", args: []string{"switch", "work"}, setup: func(s *sandbox) string {
		std(s)
		s.token("work", "tok-revoked")
		return s.w
	}},

	// ---- here
	{name: "here-repo-nocheck", args: []string{"switch", "work", "--here", "--no-check"}, setup: func(s *sandbox) string {
		std(s)
		return s.repo("r1")
	}},
	{name: "here-repo-check", args: []string{"here", "work"}, setup: func(s *sandbox) string {
		std(s)
		r := s.repo("r1")
		s.file("r1/.claude/settings.local.json", `{"permissions": {"allow": ["Read"]}, "env": {"X": "1"}}`)
		return filepath.Join(r)
	}},
	{name: "here-subdir", lang: "vi_VN.UTF-8", args: []string{"pin", "play", "--no-check"}, setup: func(s *sandbox) string {
		std(s)
		s.repo("r1")
		return s.dir("r1/deep/er")
	}},
	{name: "here-same-again", args: []string{"here", "work", "--no-check"}, setup: func(s *sandbox) string {
		std(s)
		r := s.repo("r1")
		pin(s, r, "work", "tok-old")
		s.registry(r + "\twork")
		return r
	}},
	{name: "here-main", args: []string{"here", "main"}, setup: func(s *sandbox) string { std(s); return s.repo("r1") }},
	{name: "here-main-nocred", args: []string{"here", "main"}, setup: func(s *sandbox) string {
		std(s)
		os.Remove(filepath.Join(s.kc, "Claude_Code-credentials@"))
		return s.repo("r1")
	}},
	{name: "here-off", args: []string{"here", "off"}, setup: func(s *sandbox) string {
		std(s)
		r := s.repo("r1")
		pin(s, r, "work", "tok-work")
		s.registry(r + "\twork")
		return r
	}},
	{name: "here-off-keeps-other-settings", args: []string{"here", "off"}, setup: func(s *sandbox) string {
		std(s)
		r := s.repo("r1")
		s.file("r1/.claude/settings.local.json", `{"env": {"CLAUDE_CODE_OAUTH_TOKEN": "tok-work", "CLAUDE_ACC_NAME": "work", "KEEP": "1"}, "hooks": {}}`)
		s.registry(r + "\twork")
		return r
	}},
	{name: "here-off-not-pinned", args: []string{"here", "off"}, setup: func(s *sandbox) string { std(s); return s.repo("r1") }},
	{name: "here-no-git", args: []string{"here", "play", "--no-check"}, setup: func(s *sandbox) string { std(s); return s.dir("plain") }},
	{name: "here-at", args: []string{"here", "work", "--no-check", "--at", "../other"}, setup: func(s *sandbox) string {
		std(s)
		s.dir("other")
		return s.dir("here")
	}},
	{name: "here-at-eq", args: []string{"here", "--at=nowhere", "work"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "here-tracked", args: []string{"here", "work"}, setup: func(s *sandbox) string {
		std(s)
		r := s.repo("r1")
		s.file("r1/.claude/settings.local.json", "{}\n")
		s.git(r, "add", "-f", ".claude/settings.local.json")
		return r
	}},
	{name: "here-ignored-already", args: []string{"here", "work", "--no-check"}, setup: func(s *sandbox) string {
		std(s)
		r := s.repo("r1")
		s.file("r1/.gitignore", ".claude/\n")
		return r
	}},
	{name: "here-revoked", args: []string{"here", "work"}, setup: func(s *sandbox) string {
		std(s)
		s.token("work", "tok-revoked")
		return s.repo("r1")
	}},
	{name: "here-two-names", args: []string{"here", "work", "play"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "here-bad-flag", args: []string{"here", "work", "--nope"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "here-ws-member", args: []string{"here", "work", "--no-check"}, setup: func(s *sandbox) string {
		std(s)
		_, b, _ := workspace(s, "proj")
		return b
	}},
	{name: "here-ws-all", args: []string{"here", "work", "--all", "--no-check"}, setup: func(s *sandbox) string {
		std(s)
		_, b, c := workspace(s, "proj")
		// b keeps settings of its own: merged, not replaced
		s.writeAbs(filepath.Join(b, ".claude", "settings.local.json"), `{"permissions": {"allow": ["Bash"]}}`)
		// c holds only an old pin: replaced by the shared link
		pin(s, c, "play", "tok-play")
		return s.dir("hub/a/src")
	}},
	{name: "here-ws-resync-members", args: []string{"here", "play", "--no-check"}, setup: func(s *sandbox) string {
		std(s)
		a, b, _ := workspace(s, "proj")
		pin(s, a, "work", "tok-work")
		pin(s, b, "work", "tok-work")
		s.registry(a+"\twork", b+"\twork")
		return a
	}},
	{name: "here-ws-ambiguous", args: []string{"here", "work"}, setup: func(s *sandbox) string {
		std(s)
		workspace(s, "one")
		s.file("hub/two.code-workspace", `{"folders": [{"path": "b"}, {"path": "a"}]}`)
		return filepath.Join(s.w, "hub", "a")
	}},

	// ---- reassign / sync / rm
	{name: "reassign", args: []string{"reassign", "work", "play"}, setup: func(s *sandbox) string {
		std(s)
		r1, r2 := s.repo("r1"), s.repo("r2")
		pin(s, r1, "work", "tok-work")
		pin(s, r2, "work", "tok-work")
		s.registry(r1+"\twork", r2+"\twork", filepath.Join(s.w, "gone")+"\twork", filepath.Join(s.w, "x")+"\tplay")
		return s.w
	}},
	{name: "reassign-to-main", lang: "vi_VN.UTF-8", args: []string{"reassign", "work", "main"}, setup: func(s *sandbox) string {
		std(s)
		r1 := s.repo("r1")
		pin(s, r1, "work", "tok-work")
		s.registry(r1 + "\twork")
		return s.w
	}},
	{name: "reassign-none", args: []string{"reassign", "nobody", "play"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "sync", args: []string{"sync"}, setup: func(s *sandbox) string {
		std(s)
		s.accounts("work", "play", "lost")
		r1, r2, r3 := s.repo("r1"), s.repo("r2"), s.repo("r3")
		pin(s, r1, "work", "tok-OLD")
		pin(s, r2, "play", "tok-play")
		s.writeAbs(filepath.Join(r3, ".claude", "settings.local.json"), `{"env": {"CLAUDE_CODE_OAUTH_TOKEN": ""}}`)
		s.registry(r1+"\twork", r2+"\tplay", r3+"\tmain", filepath.Join(s.w, "gone")+"\twork", s.dir("lostdir")+"\tlost")
		s.writeAbs(settingsFile(s), `{"env": {"CLAUDE_CODE_OAUTH_TOKEN": "tok-OLD", "CLAUDE_ACC_NAME": "work"}}`)
		return s.w
	}},
	{name: "sync-nothing-gone", args: []string{"sync"}, setup: func(s *sandbox) string {
		std(s)
		r1 := s.repo("r1")
		pin(s, r1, "play", "tok-play")
		s.registry(r1 + "\tplay")
		return s.w
	}, fix: func(b, g *result) {
		// bash exits 1 here: its last line is `[[ $gone -gt 0 ]] && ...`.
		if b.code == 1 {
			b.code = 0
		}
	}},
	{name: "rm-pinned", args: []string{"rm", "play"}, setup: func(s *sandbox) string {
		std(s)
		s.writeAbs(settingsFile(s), `{"env": {"CLAUDE_CODE_OAUTH_TOKEN": "tok-play", "CLAUDE_ACC_NAME": "play"}}`)
		return s.w
	}},
	{name: "rm-missing", lang: "vi_VN.UTF-8", args: []string{"remove", "ghost"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "rm-main", args: []string{"rm", "main"}, setup: func(s *sandbox) string { std(s); return s.w }},

	// ---- env / run
	{name: "env-acc", args: []string{"env", "work"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "env-main", args: []string{"env", "main"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "env-none", args: []string{"env", "nope"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "run-acc", args: []string{"run", "work", "--", "-p", "hello"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "run-main", args: []string{"exec", "main", "--version"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "run-dirpin", args: []string{"run", "work"}, setup: func(s *sandbox) string {
		std(s)
		r := s.repo("r1")
		pin(s, r, "play", "tok-play")
		return r
	}},
	{name: "run-globalpin", args: []string{"run", "main"}, setup: func(s *sandbox) string {
		std(s)
		s.writeAbs(settingsFile(s), `{"env": {"CLAUDE_CODE_OAUTH_TOKEN": "tok-work", "CLAUDE_ACC_NAME": "work"}}`)
		return s.w
	}},
	{name: "run-globalpin-acc", lang: "vi_VN.UTF-8", args: []string{"run", "play"}, setup: func(s *sandbox) string {
		std(s)
		s.writeAbs(settingsFile(s), `{"env": {"CLAUDE_CODE_OAUTH_TOKEN": "tok-work", "CLAUDE_ACC_NAME": "work"}}`)
		return s.w
	}},

	// ---- check
	{name: "check-acc", args: []string{"check", "work"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "check-revoked", args: []string{"status", "work"}, setup: func(s *sandbox) string {
		std(s)
		s.token("work", "tok-revoked")
		return s.w
	}},
	{name: "check-ratelimited", lang: "vi_VN.UTF-8", args: []string{"check", "work"}, setup: func(s *sandbox) string {
		std(s)
		s.token("work", "tok-out")
		return s.w
	}},
	{name: "check-500", args: []string{"check", "work"}, setup: func(s *sandbox) string {
		std(s)
		s.token("work", "tok-500")
		return s.w
	}},
	{name: "check-main", args: []string{"check", "main"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "check-main-refresh", args: []string{"check", "main"}, env: map[string]string{"FAKE_REFRESH_TO": "main-ok"},
		setup: func(s *sandbox) string { std(s); s.mainCred("main-old"); return s.w }},
	{name: "check-main-dead", args: []string{"check", "main"}, setup: func(s *sandbox) string {
		std(s)
		s.mainCred("main-old")
		return s.w
	}},
	{name: "check-no-token", args: []string{"check", "nope"}, setup: func(s *sandbox) string { std(s); return s.w }},

	// ---- usage
	{name: "usage-all", args: []string{"usage"}, env: map[string]string{"FAKE_REFRESH_TO": "main-ok"}, setup: func(s *sandbox) string {
		std(s)
		s.mainCred("main-old")
		s.accounts("work", "play", "out", "week", "revoked", "lost", "err")
		s.token("out", "tok-out")
		s.token("week", "tok-week")
		s.token("revoked", "tok-revoked")
		s.token("err", "tok-500")
		s.usageCache("zed\t1\t0.1\t1\tallowed\t0.1\t1\tallowed", "work\t1\t0.9\t1\tallowed\t0.9\t1\tallowed")
		return s.w
	}},
	{name: "usage-vi", lang: "vi_VN.UTF-8", args: []string{"quota", "play", "work"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "usage-one", args: []string{"usage", "work"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "usage-flag", args: []string{"usage", "-x"}, setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "usage-unknown", args: []string{"usage", "ghost"}, setup: func(s *sandbox) string { std(s); return s.w }},

	// ---- lang
	{name: "lang-show", args: []string{"lang"}, setup: func(s *sandbox) string { return s.w }},
	{name: "lang-set-vi", args: []string{"lang", "vi"}, setup: func(s *sandbox) string { return s.w }},
	{name: "lang-set-en", lang: "vi_VN.UTF-8", args: []string{"language", "en"}, setup: func(s *sandbox) string { return s.w }},
	{name: "lang-auto", lang: "vi_VN.UTF-8", args: []string{"lang", "auto"}, setup: func(s *sandbox) string {
		s.writeAbs(filepath.Join(s.home, ".config/claude-acc/lang"), "en\n")
		return s.w
	}},
	{name: "lang-file-wins", lang: "vi_VN.UTF-8", args: []string{"lang"}, setup: func(s *sandbox) string {
		s.writeAbs(filepath.Join(s.home, ".config/claude-acc/lang"), " en \n")
		return s.w
	}},
	{name: "lang-bad", args: []string{"lang", "fr"}, setup: func(s *sandbox) string { return s.w }},

	// ---- add (non-interactive paths)
	{name: "add-stdin", args: []string{"add", "work", "--no-setup", "--stdin"}, stdin: "  sk-ant-oat01-GOODtoken_ABC-123 \n",
		setup: func(s *sandbox) string {
			std(s)
			r := s.repo("r1")
			pin(s, r, "work", "tok-work")
			s.registry(r + "\twork")
			s.writeAbs(settingsFile(s), `{"env": {"CLAUDE_CODE_OAUTH_TOKEN": "tok-work", "CLAUDE_ACC_NAME": "work"}}`)
			return s.w
		}},
	{name: "add-stdin-new", lang: "vi_VN.UTF-8", args: []string{"add", "new-acc", "--stdin", "--no-setup"}, stdin: "sk-ant-oat01-GOODtoken_ABC-123\n",
		setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "add-stdin-bad-token", args: []string{"add", "x", "--no-setup", "--stdin"}, stdin: "sk-ant-oat01-NOPE\n",
		setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "add-not-a-token", args: []string{"add", "x", "--no-setup", "--stdin"}, stdin: "abc#def\n",
		setup: func(s *sandbox) string { std(s); return s.w },
		fix: func(b, g *result) {
			// The message is translated now; bash hardcoded unaccented Vietnamese.
			if strings.Contains(b.stderr, "Day khong phai token") && strings.Contains(g.stderr, "This is not a token.") {
				b.stderr, g.stderr = "", ""
			}
		}},
	{name: "add-empty", args: []string{"add", "x", "--no-setup", "--stdin"}, stdin: "\n", setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "add-bad-name", args: []string{"add", "../x"}, setup: func(s *sandbox) string { return s.w }},
	{name: "add-main", args: []string{"add", "main"}, setup: func(s *sandbox) string { return s.w }},
	{name: "add-no-name", args: []string{"add", "--no-setup"}, setup: func(s *sandbox) string { return s.w }},
	{name: "add-two-names", args: []string{"add", "a", "b"}, setup: func(s *sandbox) string { return s.w }},
	{name: "add-browser-missing", args: []string{"add", "a", "--browser"}, setup: func(s *sandbox) string { return s.w }},
	{name: "add-browser-flag", args: []string{"add", "a", "--browser", "--clip"}, setup: func(s *sandbox) string { return s.w },
		fix: func(b, g *result) {
			// bash: the message starts with "--browser", so its printf took it
			// for an option and printed nothing useful.
			if strings.Contains(b.stderr, "printf: --: invalid option") && strings.Contains(g.stderr, `not "--clip"`) {
				b.stderr, g.stderr = "", ""
			}
		}},
	{name: "switch-no-claude-dir", args: []string{"switch", "work"}, setup: func(s *sandbox) string {
		std(s)
		os.RemoveAll(filepath.Join(s.home, ".claude"))
		return s.w
	}, fix: func(b, g *result) {
		// bash died (could not create settings.json.tmp); Go creates ~/.claude.
		if b.code == 1 && g.code == 0 && strings.Contains(g.files["home/.claude/settings.json"], "tok-work") {
			*b = *g
		}
	}},
	{name: "login-bad-arg", args: []string{"login", "--what"}, setup: func(s *sandbox) string { return s.w }},
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

var (
	tmpNames = regexp.MustCompile(`claude-acc-refresh\.[A-Za-z0-9]+`)
	// The bash version let Python tracebacks through before its own error
	// line; the Go version prints only that line.
	traceback = regexp.MustCompile(`(?s)Traceback \(most recent call last\):\n.*?\n[A-Za-z_][\w.]*(Error|Exception)[^\n]*\n`)
)

// noConfigForHelp: help no longer creates the (empty) config files.
func noConfigForHelp(b, g *result) {
	for _, f := range []string{"home/.config/claude-acc/accounts", "home/.config/claude-acc/repos"} {
		if b.files[f] == "[-rw-r--r--] " && g.files[f] == "" {
			delete(b.files, f)
		}
	}
}

func TestParity(t *testing.T) {
	goBin, bashBin := binaries(t)
	s := newSandbox(t, apiSpecs)
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			s.t = t
			lang := sc.lang
			if lang == "" {
				lang = "en_US.UTF-8"
			}
			s.extraEnv = sc.env
			if s.extraEnv == nil {
				s.extraEnv = map[string]string{}
			}

			s.reset()
			cwd := sc.setup(s)
			b := s.run([]string{"bash", bashBin}, cwd, sc.args, sc.stdin, lang)

			s.reset()
			cwd = sc.setup(s)
			g := s.run([]string{goBin}, cwd, sc.args, sc.stdin, lang)

			// The version is the one difference everywhere.
			b.stdout = strings.ReplaceAll(b.stdout, "1.0.0", "2.0.0")
			for _, r := range []*result{&b, &g} {
				r.claudeLog = tmpNames.ReplaceAllString(r.claudeLog, "claude-acc-refresh.X")
			}
			b.stderr = traceback.ReplaceAllString(b.stderr, "")
			if sc.fix != nil {
				sc.fix(&b, &g)
			}

			if b.code != g.code {
				t.Errorf("exit code: bash %d, go %d", b.code, g.code)
			}
			if b.stdout != g.stdout {
				t.Errorf("stdout differs\n--- bash ---\n%s\n--- go ---\n%s", b.stdout, g.stdout)
			}
			if b.stderr != g.stderr {
				t.Errorf("stderr differs\n--- bash ---\n%s\n--- go ---\n%s", b.stderr, g.stderr)
			}
			if b.claudeLog != g.claudeLog {
				t.Errorf("claude calls differ\n--- bash ---\n%s\n--- go ---\n%s", b.claudeLog, g.claudeLog)
			}
			if d := diffMaps(normalize(b.files), normalize(g.files)); d != "" {
				t.Errorf("files differ:\n%s", d)
			}
		})
	}
}
