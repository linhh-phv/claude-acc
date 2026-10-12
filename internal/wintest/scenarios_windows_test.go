package wintest

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/linhh-phv/claude-acc/internal/clip"
	"github.com/linhh-phv/claude-acc/internal/maincred"
	"github.com/linhh-phv/claude-acc/internal/secret"
	"golang.org/x/sys/windows"
)

const (
	tokWork    = "sk-ant-oat01-SELFtest_WORK"
	tokPlay    = "sk-ant-oat01-SELFtest_PLAY"
	tokTyped   = "sk-ant-oat01-SELFtest_TYPED"
	tokPrinted = "sk-ant-oat01-SELFtest_PRINTED"
)

func init() { os.Setenv("CLAUDE_ACC_SECRET_SERVICE", testService) }

func needGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed — the repo/workspace checks need it")
	}
}

func gitInit(t *testing.T, dir string) {
	os.MkdirAll(dir, 0o755)
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}

func stored(t *testing.T, name string) string {
	tok, err := secret.Get(name)
	if err != nil {
		t.Fatalf("no Credential Manager entry for %q: %v", name, err)
	}
	return tok
}

// ---------- A. commands, non-interactive ----------------------------------------

func TestVersionAndHelp(t *testing.T) {
	s := newSandbox(t)
	want(t, s.ok(s.w, "", "version").stdout, "claude-acc 2.0.0")
	h := s.ok(s.w, "", "help").stdout
	want(t, h, "Credential Manager")
	want(t, h, "Invoke-Expression")
	wantNot(t, h, "Keychain")
	s.env["LANG"] = "vi_VN.UTF-8"
	want(t, s.ok(s.w, "", "help").stdout, "Windows Credential Manager")
}

func TestAddListSwitchCheck(t *testing.T) {
	s := newSandbox(t)
	s.addAccount("work", tokWork)
	if got := stored(t, "work"); got != tokWork {
		t.Fatalf("stored token %q", got)
	}
	ls := s.ok(s.w, "", "ls").stdout
	want(t, ls, "✓ work")
	want(t, ls, "main@example.com")
	want(t, ls, "Claude Code's own credential")

	r := s.ok(s.w, "", "switch", "work")
	want(t, r.stdout, "✓ work works")
	want(t, s.settings(), `"CLAUDE_ACC_NAME": "work"`)
	want(t, s.settings(), tokWork)
	want(t, s.ok(s.w, "", "ls").stdout, "GLOBAL    work")

	s.ok(s.w, "", "switch", "main")
	wantNot(t, s.settings(), "CLAUDE_ACC_NAME")
	want(t, s.ok(s.w, "", "check", "work").stdout, "✓ work works")
}

func TestMainCredentialFile(t *testing.T) {
	s := newSandbox(t)
	want(t, s.ok(s.w, "", "check", "main").stdout, "✓ main works")

	// an expired access token: claude-acc lets Claude Code renew it
	writeMainCred(filepath.Join(s.home, ".claude"), "main-old")
	s.env["FAKE_REFRESH_TO"] = "main-ok"
	want(t, s.ok(s.w, "", "check", "main").stdout, "✓ main works")
	want(t, s.claudeLog(), "-p hi --model haiku")
}

func TestMainCredentialManager(t *testing.T) {
	s := newSandbox(t)
	cfg := filepath.Join(s.root, "cfg")
	os.MkdirAll(cfg, 0o755)
	s.env["CLAUDE_CONFIG_DIR"] = cfg
	s.env["CLAUDE_CODE_FORCE_WINDOWS_CREDMAN"] = "1"
	base := credmanService(cfg) + "/claude-code-user"
	targets := []string{base, base + "#m", base + "#0", base + "#1", base + "#2", base + "#3", base + "#p"}
	defer func() {
		for _, x := range targets {
			deleteTarget(x)
		}
	}()

	cred := `{"claudeAiOauth":{"accessToken":"main-credman","refreshToken":"r","expiresAt":1}}`
	if err := secret.WriteTarget(base, cred); err != nil {
		t.Fatalf("writing a test entry to Credential Manager: %v", err)
	}
	want(t, s.ok(s.w, "", "check", "main").stdout, "✓ main works")

	// past 2400 bytes Claude Code splits it into base64 chunks
	deleteTarget(base)
	big := strings.Replace(cred, `"expiresAt"`, `"pad":"`+strings.Repeat("x", 5000)+`","expiresAt"`, 1)
	b64 := base64.StdEncoding.EncodeToString([]byte(big))
	n := (len(b64) + 2399) / 2400
	for i := 0; i < n; i++ {
		end := min((i+1)*2400, len(b64))
		if err := secret.WriteTarget(base+"#"+strconv.Itoa(i), b64[i*2400:end]); err != nil {
			t.Fatal(err)
		}
	}
	secret.WriteTarget(base+"#m", `{"n":`+strconv.Itoa(n)+`,"l":`+strconv.Itoa(len(b64))+`}`)
	want(t, s.ok(s.w, "", "check", "main").stdout, "✓ main works")
	u := s.ok(s.w, "", "usage", "main").stdout
	want(t, u, "main")
	want(t, u, "5%")
}

func TestRepoPin(t *testing.T) {
	needGit(t)
	s := newSandbox(t)
	s.addAccount("work", tokWork)
	r1 := filepath.Join(s.w, "r1")
	gitInit(t, r1)

	r := s.ok(r1, "", "switch", "work", "--here", "--no-check")
	want(t, r.stderr, `Pinned "work"`)
	pin := s.read(filepath.Join(r1, ".claude", "settings.local.json"))
	want(t, pin, tokWork)
	want(t, s.read(filepath.Join(r1, ".git", "info", "exclude")), ".claude/settings.local.json")
	want(t, s.ok(r1, "", "ls").stdout, "HERE      work")

	gs, _ := exec.Command("git", "-C", r1, "status", "--porcelain").Output()
	wantNot(t, string(gs), "settings.local.json")

	s.ok(r1, "", "here", "off")
	wantNot(t, s.read(filepath.Join(r1, ".claude", "settings.local.json")), "CLAUDE_ACC_NAME")
}

func TestWorkspaceAll(t *testing.T) {
	needGit(t)
	s := newSandbox(t)
	s.addAccount("work", tokWork)
	s.addAccount("play", tokPlay)
	hub := filepath.Join(s.w, "hub")
	a, b, c := filepath.Join(hub, "a"), filepath.Join(hub, "b"), filepath.Join(hub, "c")
	for _, d := range []string{a, b, c} {
		gitInit(t, d)
	}
	os.WriteFile(filepath.Join(hub, "proj.code-workspace"), []byte(`{
	// comment line, as VS Code allows
	"folders": [{"path": "a"}, {"path": "b"}, {"path": "./c"}],
	"settings": {}
}`), 0o644)

	r := s.ok(b, "", "here", "play", "--all", "--no-check")
	want(t, r.stderr, "proj.code-workspace (3 folder)")
	want(t, r.stderr, "pinned 2 other folders of the workspace too")
	for _, d := range []string{a, b, c} {
		f := filepath.Join(d, ".claude", "settings.local.json")
		if fi, err := os.Lstat(f); err != nil || fi.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("%s: want a real file, got %v %v", f, fi, err)
		}
		want(t, s.read(f), tokPlay)
	}
	want(t, s.ok(a, "", "ls").stdout, "with their own copy of the pin")

	// a later --here from another folder moves every copy
	s.ok(c, "", "here", "work", "--no-check")
	for _, d := range []string{a, b, c} {
		want(t, s.read(filepath.Join(d, ".claude", "settings.local.json")), tokWork)
	}

	r = s.ok(s.w, "", "reassign", "work", "play")
	want(t, r.stderr, `Moved 3 directories from "work" to "play"`)
	want(t, s.ok(s.w, "", "sync").stderr, "Synced 3 directories")
}

func TestUsageAndRm(t *testing.T) {
	s := newSandbox(t)
	s.addAccount("work", tokWork)
	s.addAccount("play", tokPlay)
	u := s.ok(s.w, "", "usage").stdout
	for _, x := range []string{"main", "work", "play", "Most headroom:"} {
		want(t, u, x)
	}
	want(t, s.read(filepath.Join(s.appdata, "claude-acc", "usage-cache")), "work\t")
	want(t, s.ok(s.w, "", "ls").stdout, "5h 10%")

	s.ok(s.w, "", "switch", "play")
	r := s.ok(s.w, "", "rm", "play")
	want(t, r.stderr, `removed "play"`)
	if _, err := secret.Get("play"); err == nil {
		t.Fatal("token still in Credential Manager after rm")
	}
	wantNot(t, s.settings(), "CLAUDE_ACC_NAME")
}

func TestEnvAndRun(t *testing.T) {
	s := newSandbox(t)
	s.addAccount("work", tokWork)
	want(t, s.ok(s.w, "", "env", "work").stdout, "$env:CLAUDE_CODE_OAUTH_TOKEN = '"+tokWork+"'")
	want(t, s.ok(s.w, "", "env", "main").stdout, "Remove-Item Env:CLAUDE_CODE_OAUTH_TOKEN")

	s.ok(s.w, "", "run", "work", "--", "-p", "hello")
	want(t, s.claudeLog(), "-p hello | tok="+tokWork)
	s.ok(s.w, "", "run", "main", "--", "-p", "again")
	want(t, s.claudeLog(), "-p again | tok=UNSET")

	s.env["FAKE_EXIT"] = "7"
	if r := s.run(s.w, "", "run", "work", "--", "whatever"); r.code != 7 {
		t.Fatalf("run should pass claude's exit code through; got %d", r.code)
	}
}

func TestSettingsKeptIntact(t *testing.T) {
	s := newSandbox(t)
	s.addAccount("work", tokWork)
	f := filepath.Join(s.home, ".claude", "settings.json")
	os.WriteFile(f, []byte("\xef\xbb\xbf{\n  \"statusLine\": {\"type\": \"command\", \"command\": \"echo tiếng Việt ✓\"},\n  \"permissions\": {\"allow\": [\"Bash(npm run *)\"]}\n}\n"), 0o644)
	s.ok(s.w, "", "switch", "work")
	got := s.settings()
	if strings.HasPrefix(got, "\xef\xbb\xbf") {
		t.Fatal("BOM written back")
	}
	for _, x := range []string{"echo tiếng Việt ✓", `"Bash(npm run *)"`, `"CLAUDE_ACC_NAME": "work"`} {
		want(t, got, x)
	}
	if !strings.HasPrefix(got, "{\n  \"statusLine\"") {
		t.Fatalf("key order or format changed:\n%s", got)
	}
}

func TestLanguage(t *testing.T) {
	s := newSandbox(t)
	s.ok(s.w, "", "lang", "vi")
	want(t, s.ok(s.w, "", "lang").stderr, "Ngôn ngữ hiện tại: vi")
	s.ok(s.w, "", "lang", "auto")
	s.env["LANG"] = ""
	t.Logf("with no LANG set this machine picks: %s", strings.TrimSpace(s.ok(s.w, "", "lang").stderr))
}

// ---------- B. interactive, in a pseudo-console -------------------------------

func TestInteractiveAddTypedToken(t *testing.T) {
	s := newSandbox(t)
	s.env["FAKE_SETUP_TOKEN"] = tokPrinted
	c := s.console(s.w, true, "add", "typed")
	c.expect(`take it from the clipboard:`, 60*time.Second)
	c.send(tokTyped + "\r")
	c.expect(`"typed" is ready`, 60*time.Second)
	if code := c.wait(30 * time.Second); code != 0 {
		t.Fatalf("exit %d; screen:\n%s", code, c.plain())
	}
	wantNot(t, c.plain(), "SELFtest_TYPED") // the hidden input never reached the screen
	if got := stored(t, "typed"); got != tokTyped {
		t.Fatalf("stored %q", got)
	}
}

func TestInteractiveAddFromClipboard(t *testing.T) {
	s := newSandbox(t)
	s.env["FAKE_SETUP_TOKEN"] = tokPrinted
	s.env["FAKE_SETUP_COPY"] = "1"
	c := s.console(s.w, true, "add", "printed")
	c.expect(`take it from the clipboard:`, 60*time.Second)
	c.send("\r")
	c.expect(`got: sk-ant-oat01-S`, 30*time.Second)
	c.expect(`"printed" is ready`, 60*time.Second)
	c.wait(30 * time.Second)
	if got := stored(t, "printed"); got != tokPrinted {
		t.Fatalf("stored %q", got)
	}
}

func TestInteractiveUninstall(t *testing.T) {
	s := newSandbox(t)
	s.addAccount("work", tokWork)
	c := s.console(s.w, true, "uninstall")
	c.expect(`\[y/N\]`, 30*time.Second)
	c.send("y\r")
	c.expect(`done\.`, 60*time.Second)
	if code := c.wait(30 * time.Second); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if _, err := secret.Get("work"); err == nil {
		t.Fatal("token still in Credential Manager after uninstall")
	}
	if _, err := os.Stat(filepath.Join(s.appdata, "claude-acc")); err == nil {
		t.Fatal("config dir still there after uninstall")
	}
}

func TestInteractiveLoginTypedCode(t *testing.T) {
	s := newSandbox(t)
	clip.Write("")
	c := s.console(s.w, true, "login", "--url-only")
	c.expect(`prompted > `, 60*time.Second)
	waitClip(t, "state=ST4TE_abc-123", 15*time.Second, "claude-acc did not serve as BROWSER (the login URL never reached the clipboard)")
	c.send("wrong\r")
	c.expect(`Invalid code`, 30*time.Second)
	want(t, c.plain(), "wrong") // typing is visible
	c.send(loginCode + "\r")
	c.expect(`Login successful`, 30*time.Second)
	c.expect(`main works`, 60*time.Second)
	if code := c.wait(30 * time.Second); code != 0 {
		t.Fatalf("exit %d", code)
	}
}

func TestInteractiveLoginFromClipboard(t *testing.T) {
	s := newSandbox(t)
	s.env["FAKE_LOGIN_COPY"] = "1"
	c := s.console(s.w, true, "login", "--url-only")
	c.expect(`picked up the code from the clipboard`, 60*time.Second)
	c.expect(`Login successful`, 30*time.Second)
	c.expect(`main works`, 60*time.Second)
	c.wait(30 * time.Second)
}

func TestRunSurvivesCtrlC(t *testing.T) {
	s := newSandbox(t)
	s.addAccount("work", tokWork)
	c := s.console(s.w, true, "run", "work", "--", "interactive")
	c.expect(`READY`, 60*time.Second)
	c.send("\x03")
	c.expect(`got Ctrl\+C`, 15*time.Second)
	c.send("bye\r")
	c.expect(`BYE`, 15*time.Second)
	if code := c.wait(30 * time.Second); code != 0 {
		t.Fatalf("claude-acc should outlive Ctrl+C and return claude's exit code 0; got %#x", code)
	}
}

func waitClip(t *testing.T, sub string, d time.Duration, why string) {
	t.Helper()
	deadline := time.Now().Add(d)
	for !strings.Contains(clip.Read(), sub) {
		if time.Now().After(deadline) {
			t.Fatalf("%s", why)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// ---------- C. the real Claude Code, without signing in -------------------------

func needRealClaude(t *testing.T) {
	if realClaude == "" {
		t.Skip("claude.exe is not installed on this machine")
	}
	if *skipReal {
		t.Skip("-no-real")
	}
}

// Claude Code calls BROWSER, and takes the code from a pipe, on Windows too.
// Runs with a temporary profile: nothing of the real login is read or
// changed, and the login is abandoned at the code prompt.
func TestRealClaudeLoginPrompt(t *testing.T) {
	needRealClaude(t)
	s := newSandbox(t)
	clip.Write("")
	c := s.console(s.w, false, "login", "--url-only")
	c.expect(`Paste code here if prompted`, 180*time.Second)
	waitClip(t, "state=", 20*time.Second, "Claude Code did not call BROWSER: --browser, --url-only and the code auto-fill cannot work")
	c.send("abc\r")
	c.expect(`Invalid code`, 30*time.Second)
	want(t, c.plain(), "abc") // the typed code is visible
	c.kill()
}

func TestRealClaudeSetupTokenBrowser(t *testing.T) {
	needRealClaude(t)
	s := newSandbox(t)
	clip.Write("")
	c := s.console(s.w, false, "add", "zz-selftest-probe", "--url-only")
	waitClip(t, "state=", 180*time.Second, "claude setup-token did not call BROWSER")
	c.kill()
}

// ---------- D. this machine, read-only -----------------------------------------

func TestThisMachine(t *testing.T) {
	v := windows.RtlGetVersion()
	t.Logf("Windows %d.%d build %d", v.MajorVersion, v.MinorVersion, v.BuildNumber)
	if realClaude == "" {
		t.Log("claude.exe: not installed")
		return
	}
	out, _ := exec.Command(realClaude, "--version").Output()
	t.Logf("claude --version: %s", strings.TrimSpace(string(out)))

	// Can claude-acc read main's credential where Claude Code keeps it?
	// The token itself is never printed.
	_, err := maincred.AccessToken()
	t.Logf("main credential: %s — readable: %v", maincred.Where(), err == nil)

	raw, _ := exec.Command(realClaude, "auth", "status", "--json").Output()
	var st struct {
		LoggedIn   bool   `json:"loggedIn"`
		AuthMethod string `json:"authMethod"`
	}
	json.Unmarshal(raw, &st)
	t.Logf("claude auth status: loggedIn=%v authMethod=%q", st.LoggedIn, st.AuthMethod)
	if st.LoggedIn && st.AuthMethod == "claude.ai" && err != nil {
		t.Errorf("Claude Code is signed in, but claude-acc cannot read its credential (%s)", maincred.Where())
	}
}
