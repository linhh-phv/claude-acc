//go:build darwin

package parity

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// openPty returns a pseudo-terminal pair (macOS ioctls), like openpty(3).
func openPty() (master, slave *os.File, err error) {
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	fd := m.Fd()
	if err := unix.IoctlSetInt(int(fd), unix.TIOCPTYGRANT, 0); err != nil {
		m.Close()
		return nil, nil, err
	}
	if err := unix.IoctlSetInt(int(fd), unix.TIOCPTYUNLK, 0); err != nil {
		m.Close()
		return nil, nil, err
	}
	var name [128]byte
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, fd, unix.TIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0]))); e != 0 {
		m.Close()
		return nil, nil, e
	}
	n := strings.TrimRight(string(name[:]), "\x00")
	s, err := os.OpenFile(n, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		m.Close()
		return nil, nil, err
	}
	unix.IoctlSetWinsize(int(s.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 40, Col: 200})
	return m, s, nil
}

// step of an interactive session: wait for text matching expect, then type
// send. An empty expect types immediately.
type step struct {
	expect string
	send   string
}

// runPty runs bin in a terminal, plays steps, and returns the transcript.
func (s *sandbox) runPty(bin []string, cwd string, args []string, lang string, steps []step) result {
	m, sl, err := openPty()
	if err != nil {
		s.t.Fatal(err)
	}
	defer m.Close()
	cmd := exec.Command(bin[0], append(bin[1:], args...)...)
	cmd.Dir = cwd
	env := []string{
		"PATH=" + s.pathEnv, "HOME=" + s.home, "LANG=" + lang, "TMPDIR=" + s.tmp + "/",
		"PWD=" + cwd, "TERM=xterm-256color", "USER=t",
		"FAKE_KC=" + s.kc, "FAKE_LOG=" + s.tmp + "/claude.log", "FAKE_CLIP=" + s.tmp + "/clip",
		"FAKE_OPEN_LOG=" + s.tmp + "/open.log", "CLAUDE_ACC_API_URL=" + s.api.URL,
	}
	for k, v := range s.extraEnv {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = sl, sl, sl
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		s.t.Fatal(err)
	}
	sl.Close()

	var mu sync.Mutex
	var buf []byte
	done := make(chan struct{})
	go func() {
		b := make([]byte, 4096)
		for {
			n, err := m.Read(b)
			mu.Lock()
			buf = append(buf, b[:n]...)
			mu.Unlock()
			if err != nil {
				close(done)
				return
			}
		}
	}()
	seen := 0
	for _, st := range steps {
		if st.expect != "" {
			re := regexp.MustCompile(st.expect)
			deadline := time.Now().Add(15 * time.Second)
			for {
				mu.Lock()
				loc := re.FindIndex(buf[seen:])
				mu.Unlock()
				if loc != nil {
					seen += loc[1]
					break
				}
				if time.Now().After(deadline) {
					ps, _ := exec.Command("ps", "-o", "pid,stat,tty,command", "-g", strconv.Itoa(cmd.Process.Pid)).CombinedOutput()
					mu.Lock()
					s.t.Fatalf("timed out waiting for %q; transcript:\n%s\nprocesses:\n%s", st.expect, buf, ps)
				}
				time.Sleep(20 * time.Millisecond)
			}
		}
		if st.send != "" {
			time.Sleep(100 * time.Millisecond)
			m.Write([]byte(st.send))
		}
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	var werr error
	select {
	case werr = <-waited:
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		s.t.Fatalf("did not finish; transcript:\n%s", buf)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	code := 0
	if ee, ok := werr.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	mu.Lock()
	out := strings.ReplaceAll(string(buf), "\r\n", "\n")
	mu.Unlock()
	logb, _ := os.ReadFile(s.tmp + "/claude.log")
	openb, _ := os.ReadFile(s.tmp + "/open.log")
	clipb, _ := os.ReadFile(s.tmp + "/clip")
	return result{stdout: out, code: code, files: s.snapshot(),
		claudeLog: string(logb) + "\n--open--\n" + string(openb) + "\n--clip--\n" + string(clipb)}
}

// More fakes for the sign-in flows.
const fakeOpen = `#!/bin/sh
printf '%s\n' "$*" >> "$FAKE_OPEN_LOG"
`
const fakePbcopy = `#!/bin/sh
cat > "$FAKE_CLIP"
`
const fakePbpaste = `#!/bin/sh
cat "$FAKE_CLIP" 2>/dev/null
`

// fakeClaudeSignin adds setup-token and auth login to the fake claude.
const fakeClaudeSignin = `#!/bin/sh
printf '%s | tok=%s\n' "$*" "${CLAUDE_CODE_OAUTH_TOKEN-UNSET}" >> "$FAKE_LOG"
URL='https://claude.com/cai/oauth/authorize?code=true&client_id=x&state=ST4TE_abc-123'
case "$1 $2" in
  "setup-token ")
    [ -n "$BROWSER" ] && "$BROWSER" "$URL"
    case "$FAKE_SETUP" in
      plain)   printf '\n  Your OAuth token (valid for 1 year):\n\n  sk-ant-oat01-GOODtoken_ABC-123\n\n  Store this token securely.\n' ;;
      wrapped) printf '\n  Your OAuth token:\n\nsk-ant-oat01-GOOD\ntoken_ABC-1\n23\n\n  Store this token securely.\n' ;;
      none)    printf '\n  Sign-in cancelled.\n' ;;
    esac
    exit 0 ;;
  "auth login")
    printf 'Opening browser to sign in…\nIf the browser didn'"'"'t open, visit: %s\nPaste code here if prompted > ' "$URL"
    [ -n "$BROWSER" ] && "$BROWSER" "$URL"
    if [ "$FAKE_LOGIN_COPY" = 1 ]; then (sleep 1; printf 'C0DE_xyz#ST4TE_abc-123' > "$FAKE_CLIP") & fi
    while IFS= read -r line; do
      if [ "$line" = 'C0DE_xyz#ST4TE_abc-123' ]; then echo "Login successful."; exit 0; fi
      echo "Invalid code. Please make sure the full code was copied."
    done
    echo "Login failed: no code"; exit 1 ;;
  "auth status")
    if [ "$3" = "--json" ]; then echo '{"email": "main@example.com"}'; else echo "Email: main@example.com"; fi
    exit 0 ;;
esac
exit 0
`

func (s *sandbox) signinFakes() {
	for name, body := range map[string]string{"open": fakeOpen, "pbcopy": fakePbcopy, "pbpaste": fakePbpaste, "claude": fakeClaudeSignin} {
		must(s.t, os.WriteFile(s.bin+"/"+name, []byte(body), 0o755))
	}
}

type ptyScenario struct {
	name  string
	lang  string
	env   map[string]string
	setup func(s *sandbox) string
	args  []string
	steps []step
	check func(t *testing.T, r result) // extra assertions on the Go run
}

var ptyScenarios = []ptyScenario{
	{name: "add-capture-plain", env: map[string]string{"FAKE_SETUP": "plain"}, args: []string{"add", "newacc"},
		setup: func(s *sandbox) string { std(s); return s.w },
		check: func(t *testing.T, r result) {
			if !strings.Contains(r.files["kc/claude-code-token@newacc"], "sk-ant-oat01-GOODtoken_ABC-123") {
				t.Errorf("token not stored: %q", r.files["kc/claude-code-token@newacc"])
			}
		}},
	{name: "add-capture-wrapped", lang: "vi_VN.UTF-8", env: map[string]string{"FAKE_SETUP": "wrapped"}, args: []string{"add", "work"},
		setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "add-browser-app", env: map[string]string{"FAKE_SETUP": "plain"}, args: []string{"add", "newacc", "--browser", "Safari"},
		setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "add-url-only", env: map[string]string{"FAKE_SETUP": "plain"}, args: []string{"add", "newacc", "--url-only"},
		setup: func(s *sandbox) string { std(s); return s.w }},
	{name: "add-no-capture-typed", env: map[string]string{"FAKE_SETUP": "none"}, args: []string{"add", "newacc"},
		setup: func(s *sandbox) string { std(s); return s.w },
		steps: []step{{expect: `\n> `, send: "  sk-ant-oat01-GOODtoken_ABC-123  \r"}}},
	{name: "add-no-capture-clipboard", env: map[string]string{"FAKE_SETUP": "none"}, args: []string{"add", "newacc"},
		setup: func(s *sandbox) string {
			std(s)
			s.writeAbs(s.tmp+"/clip", "here: sk-ant-oat01-GOODtoken_ABC-123 (copied)")
			return s.w
		},
		steps: []step{{expect: `\n> `, send: "\r"}}},
	{name: "add-clip-flag", env: map[string]string{"FAKE_SETUP": "none"}, args: []string{"add", "newacc", "--clip"},
		setup: func(s *sandbox) string {
			std(s)
			s.writeAbs(s.tmp+"/clip", "sk-ant-oat01-GOODtoken_ABC-123")
			return s.w
		}},
	{name: "uninstall-yes", args: []string{"uninstall"},
		setup: func(s *sandbox) string {
			std(s)
			r := s.repo("r1")
			pin(s, r, "work", "tok-work")
			s.registry(r + "\twork")
			s.writeAbs(settingsFile(s), `{"env": {"CLAUDE_CODE_OAUTH_TOKEN": "tok-play", "CLAUDE_ACC_NAME": "play"}, "model": "x"}`)
			s.writeAbs(settingsFile(s)+".bak-claude-acc", `{}`)
			return s.w
		},
		steps: []step{{expect: `\[y/N\] `, send: "y\r"}}},
	{name: "uninstall-no", lang: "vi_VN.UTF-8", args: []string{"uninstall"},
		setup: func(s *sandbox) string { std(s); return s.w },
		steps: []step{{expect: `\[y/N\] `, send: "\r"}}},
	{name: "login-typed", args: []string{"login"},
		setup: func(s *sandbox) string { std(s); return s.w },
		steps: []step{{expect: `prompted > `, send: "wrong\r"}, {expect: `Invalid code`, send: "C0DE_xyz#ST4TE_abc-123\r"}}},
	{name: "login-clipboard", args: []string{"login"}, env: map[string]string{"FAKE_LOGIN_COPY": "1"},
		setup: func(s *sandbox) string {
			std(s)
			s.writeAbs(s.tmp+"/clip", "OLDcode#SOMEotherSTATE") // a stale code: must not be sent
			return s.w
		}},
	{name: "login-browser", lang: "vi_VN.UTF-8", args: []string{"login", "--browser", "Safari"}, env: map[string]string{"FAKE_LOGIN_COPY": "1"},
		setup: func(s *sandbox) string { std(s); return s.w }},
}

func TestParityInteractive(t *testing.T) {
	goBin, bashBin := binaries(t)
	s := newSandbox(t, apiSpecs)
	for _, sc := range ptyScenarios {
		t.Run(sc.name, func(t *testing.T) {
			s.t = t
			lang := sc.lang
			if lang == "" {
				lang = "en_US.UTF-8"
			}
			s.extraEnv = sc.env
			run := func(bin []string) result {
				s.reset()
				s.signinFakes()
				cwd := sc.setup(s)
				return s.runPty(bin, cwd, sc.args, lang, sc.steps)
			}
			b := run([]string{"bash", bashBin})
			g := run([]string{goBin})
			b.stdout = traceback.ReplaceAllString(b.stdout, "")
			// uninstall names its own binary, which differs by construction
			b.stdout = strings.ReplaceAll(b.stdout, "claude-acc-bash", "claude-acc-BIN")
			g.stdout = strings.ReplaceAll(g.stdout, "claude-acc-go", "claude-acc-BIN")
			if b.code != g.code {
				t.Errorf("exit code: bash %d, go %d", b.code, g.code)
			}
			if b.stdout != g.stdout {
				t.Errorf("terminal differs\n--- bash ---\n%s\n--- go ---\n%s", b.stdout, g.stdout)
			}
			if b.claudeLog != g.claudeLog {
				t.Errorf("calls differ\n--- bash ---\n%s\n--- go ---\n%s", b.claudeLog, g.claudeLog)
			}
			if d := diffMaps(normalize(b.files), normalize(g.files)); d != "" {
				t.Errorf("files differ:\n%s", d)
			}
			if sc.check != nil {
				sc.check(t, g)
			}
		})
	}
}
