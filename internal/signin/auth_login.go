package signin

import (
	"errors"
	"io"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"github.com/linhh-phv/claude-acc/internal/claudebin"
	"github.com/linhh-phv/claude-acc/internal/clip"
)

// codeRe is the code the browser's callback page shows: "<code>#<state>".
var codeRe = regexp.MustCompile(`^[^\s#]+#([^\s#]+)$`)

var stateRe = regexp.MustCompile(`[?&]state=([^&\s]+)`)

// AuthLogin runs `claude auth login` so that typing or pasting the code is
// visible, and fills the code in from the clipboard when the user clicks
// Copy on the browser's code page.
//
// Unlike setup-token (a full-screen UI), `claude auth login` reads the code
// with readline on stdin — and, measured in a pty (Claude Code 2.1.260), puts
// the terminal in raw mode with echo OFF the moment it prints "Paste code
// here if prompted >". Keys and pastes still reach it, Enter submits, but
// nothing shows, so it looks like the terminal stopped taking input.
//
// Given a PIPE for stdin, claude leaves the terminal alone (measured: still
// ECHO/ICANON/ISIG) and reads lines from the pipe as usual. So:
//   - claude: stdin = pipe; stdout/stderr straight to the terminal;
//   - what the user types is read from the terminal in its normal mode —
//     echoed — and forwarded into the pipe;
//   - the clipboard is watched for "<code>#<state>". The state comes from the
//     URL claude hands to BROWSER (the same URL it prints), so only a code of
//     THIS sign-in is ever filled in; an old code left on the clipboard never
//     gets sent.
//
// notice formats the line shown when a code is taken from the clipboard.
func AuthLogin(env []string, urlFile string, notice func(code string) string) int {
	path, _ := claudebin.Path()
	c := exec.Command(path, "auth", "login")
	c.Env = env
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	stdin, err := c.StdinPipe()
	if err != nil {
		return 1
	}
	src := openLines()
	defer src.close()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	if err := c.Start(); err != nil {
		return 1
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()

	// sent: every line sent; auto: codes taken from the clipboard. The same
	// code arriving twice (auto-filled, then pasted by hand too) goes once,
	// while a mistyped line typed again is still sent, so claude can say so.
	sent, auto := map[string]bool{}, map[string]bool{}
	send := func(line string) {
		sent[line] = true
		io.WriteString(stdin, line+"\n")
	}

	var nextClip time.Time
	for {
		select {
		case err := <-done:
			stdin.Close()
			return exitCode(err)
		case <-sig:
			// The terminal is in its normal mode, so Ctrl+C reached claude too
			// (same process group) and it quits on its own. Only wait for it,
			// so it is not left behind.
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				c.Process.Kill()
				<-done
			}
			stdin.Close()
			return 130
		default:
		}

		for _, ln := range src.poll(250 * time.Millisecond) {
			if ln = strings.TrimSpace(ln); ln != "" && !auto[ln] {
				send(ln)
			}
		}

		if time.Now().Before(nextClip) {
			continue
		}
		nextClip = time.Now().Add(500 * time.Millisecond)
		// No state yet, no auto-fill: claude prints the prompt BEFORE it
		// calls BROWSER (~50ms later, measured), and every code looks alike.
		st := wantState(urlFile)
		if st == "" {
			continue
		}
		code := strings.TrimSpace(clip.Read())
		m := codeRe.FindStringSubmatch(code)
		if m == nil || m[1] != st || sent[code] {
			continue
		}
		os.Stderr.WriteString("\n" + notice(code) + "\n")
		auto[code] = true
		send(code)
	}
}

func wantState(urlFile string) string {
	b, err := os.ReadFile(urlFile)
	if err != nil {
		return ""
	}
	m := stateRe.FindSubmatch(b)
	if m == nil {
		return ""
	}
	s, err := url.QueryUnescape(string(m[1]))
	if err != nil {
		return string(m[1])
	}
	return s
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return 1
}
