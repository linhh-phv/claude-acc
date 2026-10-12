package wintest

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/linhh-phv/claude-acc/internal/clip"
	"golang.org/x/sys/windows"
)

var (
	binFlag  = flag.String("bin", "", "path to the claude-acc.exe to test (default: next to this .exe)")
	skipReal = flag.Bool("no-real", false, "skip the checks that run the real claude.exe")
)

// Shared across the run.
var (
	accBin      string // claude-acc.exe under test
	fakeBinDir  string // holds claude.exe = a copy of this test binary
	realClaude  string // the real claude.exe, if installed
	origPath    string
	testService = "claude-acc-selftest-" + strconv.Itoa(os.Getpid())
	api         *httptest.Server
	usedNames   = map[string]bool{}
	usedMu      sync.Mutex
)

const fakeEnv = "CLAUDE_ACC_FAKE_CLAUDE"

func TestMain(m *testing.M) {
	// Started as the fake claude.exe by claude-acc under test.
	if os.Getenv(fakeEnv) == "1" {
		os.Exit(fakeClaude(os.Args[1:]))
	}

	flag.Parse()
	if f := flag.Lookup("test.v"); f != nil && f.Value.String() == "false" {
		flag.Set("test.v", "true")
	}
	if f := flag.Lookup("test.timeout"); f != nil && f.Value.String() == "10m0s" {
		flag.Set("test.timeout", "30m")
	}

	origPath = os.Getenv("PATH")
	if p, err := exec.LookPath("claude"); err == nil {
		realClaude = p
	}
	accBin = findAccBin()
	if accBin == "" {
		fmt.Println("claude-acc.exe not found. Put it next to this file or pass -bin <path>.")
		os.Exit(2)
	}

	dir, err := os.MkdirTemp("", "claude-acc-selftest-bin.")
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	fakeBinDir = dir
	self, _ := os.Executable()
	if err := copyFile(self, filepath.Join(dir, "claude.exe")); err != nil {
		fmt.Println("could not prepare the fake claude.exe:", err)
		os.Exit(2)
	}

	api = httptest.NewServer(http.HandlerFunc(fakeAPI))
	savedClip := clip.Read()

	fmt.Printf("claude-acc under test: %s\nreal claude.exe: %s\n\n", accBin, orNone(realClaude))
	code := m.Run()

	api.Close()
	if savedClip != "" {
		clip.Write(savedClip)
	}
	cleanupCredentials()
	os.RemoveAll(fakeBinDir)
	os.Exit(code)
}

func orNone(s string) string {
	if s == "" {
		return "(not installed — the real-claude checks will be skipped)"
	}
	return s
}

func findAccBin() string {
	if *binFlag != "" {
		p, _ := filepath.Abs(*binFlag)
		return p
	}
	self, _ := os.Executable()
	here := filepath.Dir(self)
	for _, n := range []string{"claude-acc.exe", "claude-acc-windows-amd64.exe", "claude-acc-windows-arm64.exe"} {
		if p := filepath.Join(here, n); fileExists(p) {
			return p
		}
	}
	if p, err := exec.LookPath("claude-acc"); err == nil {
		return p
	}
	return ""
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o755)
}

// ---------- fake API ----------------------------------------------------------

type spec struct {
	code           int
	s5, s7, u5, u7 string
	reason         string
}

var specs = map[string]spec{
	"main-ok":                       {200, "allowed", "allowed", "0.42", "0.13", ""},
	"main-old":                      {401, "", "", "", "", "OAuth token has expired"},
	"main-credman":                  {200, "allowed", "allowed", "0.05", "0.06", ""},
	"sk-ant-oat01-SELFtest_WORK":    {200, "allowed", "allowed", "0.1", "0.55", ""},
	"sk-ant-oat01-SELFtest_PLAY":    {200, "allowed_warning", "allowed", "0.81", "0.2", ""},
	"sk-ant-oat01-SELFtest_TYPED":   {200, "allowed", "allowed", "0.3", "0.3", ""},
	"sk-ant-oat01-SELFtest_PRINTED": {200, "allowed", "allowed", "0.2", "0.2", ""},
}

func fakeAPI(w http.ResponseWriter, r *http.Request) {
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	sp, ok := specs[tok]
	if !ok {
		sp = spec{code: 401, reason: "Invalid bearer token"}
	}
	if sp.s5 != "" {
		h := w.Header()
		now := time.Now()
		h.Set("anthropic-ratelimit-unified-5h-utilization", sp.u5)
		h.Set("anthropic-ratelimit-unified-5h-reset", strconv.FormatInt(now.Add(3*time.Hour).Unix(), 10))
		h.Set("anthropic-ratelimit-unified-5h-status", sp.s5)
		h.Set("anthropic-ratelimit-unified-7d-utilization", sp.u7)
		h.Set("anthropic-ratelimit-unified-7d-reset", strconv.FormatInt(now.Add(80*time.Hour).Unix(), 10))
		h.Set("anthropic-ratelimit-unified-7d-status", sp.s7)
	}
	w.WriteHeader(sp.code)
	if sp.reason != "" {
		fmt.Fprintf(w, `{"type":"error","error":{"message":%q}}`, sp.reason)
	}
}

// ---------- fake claude --------------------------------------------------------

const (
	loginCode = "C0DE_xyz#ST4TE_abc-123"
	loginURL  = "https://claude.com/cai/oauth/authorize?code=true&client_id=x&state=ST4TE_abc-123"
)

func fakeClaude(args []string) int {
	tok, ok := os.LookupEnv("CLAUDE_CODE_OAUTH_TOKEN")
	if !ok {
		tok = "UNSET"
	}
	wd, _ := os.Getwd()
	if f, err := os.OpenFile(os.Getenv("FAKE_LOG"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		fmt.Fprintf(f, "%s | tok=%s | cwd=%s\n", strings.Join(args, " "), tok, wd)
		f.Close()
	}
	arg := func(i int) string {
		if i < len(args) {
			return args[i]
		}
		return ""
	}
	browser := func(url string) {
		if b := os.Getenv("BROWSER"); b != "" {
			exec.Command(b, url).Run() // execFile, no shell — what Claude Code does
		}
	}
	switch {
	case arg(0) == "auth" && arg(1) == "status":
		if arg(2) == "--json" {
			fmt.Println(`{"loggedIn": true, "authMethod": "claude.ai", "account": {"emailAddress": "main@example.com"}}`)
		} else {
			fmt.Println("Email: main@example.com")
		}
		return 0
	case arg(0) == "-p":
		if to := os.Getenv("FAKE_REFRESH_TO"); to != "" {
			writeMainCred(filepath.Join(os.Getenv("USERPROFILE"), ".claude"), to)
		}
		fmt.Println("ok")
	case arg(0) == "setup-token":
		browser(loginURL)
		if t := os.Getenv("FAKE_SETUP_TOKEN"); t != "" {
			fmt.Printf("\n  Your OAuth token (valid for 1 year):\n\n  %s\n\n  Store this token securely.\n", t)
			switch os.Getenv("FAKE_SETUP_COPY") {
			case "1":
				clip.Write(t)
			case "wrap":
				// what a copy of a token Ink wrapped in a narrow window holds
				clip.Write(" " + t[:20] + "\r\n " + t[20:] + "\r\n")
			}
		}
		return 0
	case arg(0) == "auth" && arg(1) == "login":
		fmt.Printf("Opening browser to sign in…\nIf the browser didn't open, visit: %s\nPaste code here if prompted > ", loginURL)
		browser(loginURL)
		if os.Getenv("FAKE_LOGIN_COPY") == "1" {
			go func() { time.Sleep(time.Second); clip.Write(loginCode) }()
		}
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			if strings.TrimSpace(sc.Text()) == loginCode {
				fmt.Println("Login successful.")
				return 0
			}
			fmt.Fprintln(os.Stderr, "Invalid code. Please make sure the full code was copied.")
		}
		fmt.Println("Login failed: no code")
		return 1
	case arg(0) == "interactive":
		// Stands in for an interactive claude: survives Ctrl+C, quits on "bye".
		sig := make(chan os.Signal, 4)
		signal.Notify(sig, os.Interrupt)
		go func() {
			for range sig {
				fmt.Println("got Ctrl+C")
			}
		}()
		fmt.Println("READY")
		r := bufio.NewReader(os.Stdin)
		for deadline := time.Now().Add(60 * time.Second); time.Now().Before(deadline); {
			line, err := r.ReadString('\n')
			if strings.TrimSpace(line) == "bye" {
				fmt.Println("BYE")
				return 0
			}
			if err != nil {
				// Ctrl+C can abort a console read; read again.
				time.Sleep(100 * time.Millisecond)
				r = bufio.NewReader(os.Stdin)
			}
		}
		return 3
	}
	if c := os.Getenv("FAKE_EXIT"); c != "" {
		n, _ := strconv.Atoi(c)
		return n
	}
	return 0
}

func writeMainCred(dir, tok string) {
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, ".credentials.json"),
		[]byte(`{"claudeAiOauth":{"accessToken":"`+tok+`","refreshToken":"r","expiresAt":1}}`), 0o600)
}

// ---------- the sandbox --------------------------------------------------------

type sandbox struct {
	t                           *testing.T
	root, home, appdata, tmp, w string
	env                         map[string]string // extra variables for this test
}

func newSandbox(t *testing.T) *sandbox {
	root := t.TempDir()
	s := &sandbox{t: t, root: root, env: map[string]string{}}
	s.home = filepath.Join(root, "home")
	s.appdata = filepath.Join(root, "appdata")
	s.tmp = filepath.Join(root, "tmp")
	s.w = filepath.Join(root, "w")
	for _, d := range []string{s.home, filepath.Join(s.home, ".claude"), s.appdata, s.tmp, s.w} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeMainCred(filepath.Join(s.home, ".claude"), "main-ok")
	return s
}

// dropped: variables of the real machine that must not leak into a run.
var dropped = []string{"USERPROFILE", "HOME", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "BROWSER", "LANG", "LC_ALL",
	"LC_MESSAGES", "PATH", "TEMP", "TMP"}

func (s *sandbox) environ(fake bool) []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		ku := strings.ToUpper(k)
		skip := strings.HasPrefix(ku, "CLAUDE_") || strings.HasPrefix(ku, "FAKE_")
		for _, d := range dropped {
			if ku == d {
				skip = true
			}
		}
		if !skip {
			env = append(env, kv)
		}
	}
	// set replaces a variable whatever its case; an empty value removes it.
	set := func(k, v string) {
		out := env[:0]
		for _, kv := range env {
			if name, _, _ := strings.Cut(kv, "="); !strings.EqualFold(name, k) {
				out = append(out, kv)
			}
		}
		env = out
		if v != "" {
			env = append(env, k+"="+v)
		}
	}
	path := origPath
	if fake {
		path = fakeBinDir + ";" + origPath
		set(fakeEnv, "1")
	}
	for _, kv := range [][2]string{
		{"PATH", path}, {"USERPROFILE", s.home}, {"HOME", s.home}, {"APPDATA", s.appdata}, {"LOCALAPPDATA", s.appdata},
		{"TEMP", s.tmp}, {"TMP", s.tmp}, {"LANG", "en_US.UTF-8"},
		{"CLAUDE_ACC_API_URL", api.URL}, {"CLAUDE_ACC_SECRET_SERVICE", testService},
		{"FAKE_LOG", filepath.Join(s.tmp, "claude.log")},
	} {
		set(kv[0], kv[1])
	}
	keys := make([]string, 0, len(s.env))
	for k := range s.env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		set(k, s.env[k])
	}
	return env
}

type result struct {
	stdout, stderr string
	code           int
}

func (r result) all() string { return r.stdout + r.stderr }

// run executes claude-acc with pipes, in a hidden console of its own, so
// nothing can read from or write to the console the test was started in.
func (s *sandbox) run(dir string, stdin string, args ...string) result {
	s.t.Helper()
	for i, a := range args {
		if a == "add" && i+1 < len(args) {
			markUsed(args[i+1])
		}
	}
	cmd := exec.Command(accBin, args...)
	cmd.Dir = dir
	cmd.Env = s.environ(true)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}
	if err := cmd.Start(); err != nil {
		s.t.Fatalf("start claude-acc %v: %v", args, err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var err error
	select {
	case err = <-done:
	case <-time.After(90 * time.Second):
		exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
		<-done
		s.t.Fatalf("claude-acc %v did not finish in 90s\nstdout:\n%s\nstderr:\n%s", args, out.String(), errb.String())
	}
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	return result{out.String(), errb.String(), code}
}

// ok runs and fails the test unless claude-acc exits 0.
func (s *sandbox) ok(dir, stdin string, args ...string) result {
	s.t.Helper()
	r := s.run(dir, stdin, args...)
	if r.code != 0 {
		s.t.Fatalf("claude-acc %v exited %d\nstdout:\n%s\nstderr:\n%s", args, r.code, r.stdout, r.stderr)
	}
	return r
}

func (s *sandbox) read(p string) string {
	b, _ := os.ReadFile(p)
	return string(b)
}

func (s *sandbox) claudeLog() string { return s.read(filepath.Join(s.tmp, "claude.log")) }

func (s *sandbox) settings() string { return s.read(filepath.Join(s.home, ".claude", "settings.json")) }

func (s *sandbox) addAccount(name, tok string) {
	s.t.Helper()
	r := s.ok(s.w, tok+"\n", "add", name, "--no-setup", "--stdin")
	want(s.t, r.stderr, `"`+name+`" is ready`)
}

func want(t *testing.T, got, sub string) {
	t.Helper()
	if !strings.Contains(got, sub) {
		t.Fatalf("expected %q in:\n%s", sub, got)
	}
}

func wantNot(t *testing.T, got, sub string) {
	t.Helper()
	if strings.Contains(got, sub) {
		t.Fatalf("did not expect %q in:\n%s", sub, got)
	}
}

func markUsed(name string) {
	usedMu.Lock()
	usedNames[name] = true
	usedMu.Unlock()
}

// cleanupCredentials removes every Credential Manager entry the run made.
func cleanupCredentials() {
	usedMu.Lock()
	defer usedMu.Unlock()
	for n := range usedNames {
		deleteTarget(testService + ":" + n)
	}
}

// credmanService is the Claude Code credential name for a config dir.
func credmanService(cfgDir string) string {
	h := sha256.Sum256([]byte(cfgDir))
	return "Claude Code-credentials-" + hex.EncodeToString(h[:])[:8]
}
