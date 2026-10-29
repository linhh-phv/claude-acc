package app

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/linhh-phv/claude-acc/internal/api"
	"github.com/linhh-phv/claude-acc/internal/clip"
	"github.com/linhh-phv/claude-acc/internal/fsx"
	"github.com/linhh-phv/claude-acc/internal/i18n"
	"github.com/linhh-phv/claude-acc/internal/secret"
	"github.com/linhh-phv/claude-acc/internal/signin"
	"github.com/linhh-phv/claude-acc/internal/token"
	"github.com/linhh-phv/claude-acc/internal/tty"
	"github.com/linhh-phv/claude-acc/internal/ui"
	"golang.org/x/term"
)

// cmdAdd adds a secondary account: runs `claude setup-token`, catches the
// token it prints, stores it, and proves it with one real request.
func cmdAdd(args []string) int {
	name, browser, mode, runSetup := "", "", "ask", true
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--browser":
			browser = arg(args, i+1)
			if browser == "" {
				ui.Die(i18n.T("err_br_app"))
			}
			if strings.HasPrefix(browser, "-") {
				ui.Die(i18n.Tp("err_br_bad", browser))
			}
			i++
		case "--url-only":
			browser = signin.ModeClip
		case "--clip":
			mode = "clip"
		case "--stdin":
			mode = "stdin"
		case "--no-setup":
			runSetup = false
		default:
			if strings.HasPrefix(a, "-") {
				ui.Die(i18n.Tp("err_unknown", a))
			}
			if name != "" {
				ui.Die(i18n.Tp("err_extra", a))
			}
			name = a
		}
	}
	if name == "" {
		ui.Die(i18n.T("usage_add"))
	}
	validName(name)
	refuseCaseClash(name)
	// A token piped in cannot also answer setup-token's prompts: piping
	// means the token is already at hand.
	if mode == "stdin" && !term.IsTerminal(int(os.Stdin.Fd())) {
		runSetup = false
	}

	if secret.Has(name) {
		ui.Info(ui.Yellow + i18n.Tp("add_overwrite", name) + ui.Reset)
	}
	warnMissingBrowser(browser)

	tok := ""
	if runSetup {
		bmode := browser
		switch browser {
		case "":
			ui.Info("\n" + ui.Yellow + i18n.T("add_default_br") + ui.Reset + i18n.Tp("add_default_b2", name))
		case signin.ModeClip:
			ui.Info("\n" + ui.Bold + i18n.T("add_urlonly") + ui.Reset + i18n.T("add_urlonly2"))
		default:
			ui.Info("\n" + ui.Bold + i18n.Tp("add_browse_in", browser) + ui.Reset + i18n.T("add_browse_in2"))
		}
		fmt.Fprintf(ui.Stderr, "\n%s%s%s\n\n", ui.Dim, i18n.T("add_note"), ui.Reset)

		ui.Info(ui.Dim + "--- claude setup-token ---" + ui.Reset)
		env := os.Environ()
		if bmode != "" {
			if extra, cleanup, err := signin.Wrapper(bmode, ""); err == nil {
				env = append(env, extra...)
				defer cleanup()
			}
		}
		tok = signin.SetupToken(env)
		ui.Info(ui.Dim + "--------------------------" + ui.Reset)

		if tok != "" {
			ui.Info(ui.Green + "✓ " + i18n.T("add_caught") + ui.Reset +
				i18n.Tp("add_caught2", tok[:min(14, len(tok))], tok[max(0, len(tok)-4):], strconv.Itoa(len(tok))))
		} else {
			ui.Info(ui.Yellow + i18n.T("add_nocatch") + ui.Reset + i18n.T("add_nocatch2"))
		}
	}

	// setup-token is a full-screen program and can leave the terminal raw,
	// after which the prompt below would receive nothing.
	tty.Sane()

	if tok == "" {
		switch mode {
		case "clip":
			if tok = clip.Token(); tok == "" {
				ui.Die(i18n.T("err_clip"))
			}
			ui.Info(ui.Dim + i18n.Tp("add_from_clip", tok[:min(14, len(tok))], tok[max(0, len(tok)-4):], strconv.Itoa(len(tok))) + ui.Reset)
		case "stdin":
			tok = tokenIn(readAll(os.Stdin))
		default:
			tok = readToken(name)
		}
	}
	if tok == "" {
		ui.Die(i18n.T("err_empty_tok"))
	}
	if !strings.HasPrefix(tok, "sk-ant-oat01-") {
		// Most likely the short CODE the browser showed — that is the input
		// of setup-token, not its result.
		ui.Info(ui.Red + i18n.T("add_not_tok") + ui.Reset + i18n.T("add_not_tok2"))
		return 1
	}

	// Straight to the API with this token, not through `claude`: the
	// settings env block overrides the environment, so `claude` would test
	// the PINNED account and call a broken token "ready" (it happened: a
	// truncated token was reported as ready).
	//
	// And BEFORE storing it: re-adding an account spreads the new token to
	// every place pinned to it, so a bad one stored first would replace a
	// working token everywhere.
	out, ok := api.Probe(tok)
	if !ok {
		if out != "" {
			ui.Info(out)
		}
		ui.Info(ui.Red + "✗ " + i18n.T("add_bad_nosave") + ui.Reset)
		ui.Info(i18n.Tp("add_bad2", name))
		return 1
	}

	if secret.Set(name, tok) != nil {
		ui.Die(i18n.T("err_keychain"))
	}
	remember(name)
	resyncAcc(name, tok)
	ui.Info(ui.Dim + i18n.Tp("add_saved", name) + ui.Reset)
	if out != "" {
		ui.Info(out)
	}
	ui.Info(ui.Green + "✓ " + i18n.Tp("add_ready", name) + ui.Reset + i18n.Tp("add_ready2", name))
	return 0
}

// readToken asks for the token at the terminal, hidden: it never lands on
// screen or in scrollback. A masked preview afterwards makes up for not
// seeing it. Enter alone takes it from the clipboard.
func readToken(name string) string {
	fmt.Fprintf(ui.Stderr, "%s%s%s%s\n> ", ui.Bold, i18n.Tp("tok_prompt", name), ui.Reset, i18n.T("tok_prompt2"))
	raw, _ := tty.ReadSecret()
	fmt.Fprintln(ui.Stderr)
	raw = tokenIn(raw)
	if raw == "" {
		// ONLY an sk-ant-… string: an old code left on the clipboard used to
		// be taken and then rejected as "not a token".
		raw = trim(clip.Token())
	}
	if raw != "" {
		ui.Info(ui.Dim + i18n.Tp("tok_got", raw[:min(14, len(raw))], raw[max(0, len(raw)-4):], strconv.Itoa(len(raw))) + ui.Reset)
	}
	return raw
}

func warnMissingBrowser(browser string) {
	if browser != "" && browser != signin.ModeClip && signin.BrowserMissing(browser) {
		ui.Info(ui.Yellow + i18n.Tp("add_no_app", browser) + ui.Reset + i18n.T("add_no_app2"))
	}
}

// tokenIn takes the token out of what was typed, pasted or piped — rejoined
// if the terminal broke it across lines. Text with no token in it comes back
// trimmed, so a pasted login code still gets its own explanation.
func tokenIn(s string) string {
	if t := token.Extract(s); t != "" {
		return t
	}
	line, _, _ := strings.Cut(strings.TrimLeft(s, "\r\n"), "\n")
	return trim(line)
}

func readAll(r io.Reader) string {
	b, _ := io.ReadAll(r)
	return string(b)
}

// refuseCaseClash: on Windows, Credential Manager ignores case, so "Work"
// and "work" would share — and overwrite — one token.
func refuseCaseClash(name string) {
	if runtime.GOOS != "windows" {
		return
	}
	for _, a := range accounts() {
		if a != name && strings.EqualFold(a, name) {
			ui.Die(i18n.Tp("err_case_clash", name, a, a))
		}
	}
}

// cmdLogin signs main in again through `claude auth login`.
func cmdLogin(args []string) int {
	browser := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--browser":
			browser = arg(args, i+1)
			if browser == "" {
				ui.Die(i18n.T("err_br_app2"))
			}
			if strings.HasPrefix(browser, "-") {
				ui.Die(i18n.Tp("err_br_bad", browser))
			}
			i++
		case "--url-only":
			browser = signin.ModeClip
		default:
			ui.Die(i18n.T("usage_login"))
		}
	}

	warnMissingBrowser(browser)
	ui.Info(ui.Yellow + i18n.T("login_warn") + ui.Reset + i18n.T("login_warn2"))
	ui.Info(i18n.T("login_warn3"))

	// Always a wrapper, even with no browser chosen: it records the URL,
	// whose `state` lets the code be filled in from the clipboard.
	env := os.Environ()
	urlFile := ""
	if f, err := os.CreateTemp("", "claude-acc-url."); err == nil {
		urlFile = f.Name()
		f.Close()
		defer os.Remove(urlFile)
	}
	mode := browser
	if mode == "" {
		mode = signin.ModeDefault
	}
	if extra, cleanup, err := signin.Wrapper(mode, urlFile); err == nil {
		env = append(env, extra...)
		defer cleanup()
	}
	if browser != "" && browser != signin.ModeClip {
		ui.Info(ui.Bold + i18n.Tp("add_browse_in", browser) + ui.Reset)
	}
	ui.Info(ui.Dim + i18n.T("login_note") + ui.Reset + "\n")

	signin.AuthLogin(env, urlFile, func(code string) string {
		return ui.Green + "✓ " + i18n.Tp("login_caught", code[:min(6, len(code))], code[max(0, len(code)-4):]) + ui.Reset
	})

	tty.Sane()
	ui.Info("\n" + ui.Dim + i18n.T("login_result") + ui.Reset)
	return cmdCheck([]string{"main"})
}

// cmdUninstall removes every token and file claude-acc created, after a yes.
func cmdUninstall(args []string) int {
	ui.Info(ui.Bold + i18n.T("un_willremove") + ui.Reset)
	for _, a := range accounts() {
		ui.Info(i18n.Tp("un_tok", a))
	}
	ui.Info(i18n.Tp("un_cfg", cfgDir))
	rows := regRows()
	for _, r := range rows {
		if r.dir != "" {
			ui.Info(i18n.Tp("un_pin", repoFile(r.dir)))
		}
	}
	if pinnedAcc() != "" {
		ui.Info(i18n.Tp("un_glob", settings))
	}
	bak := settings + ".bak-claude-acc"
	if fsx.IsFile(bak) {
		ui.Info(i18n.Tp("un_bak", bak))
	}
	self, err := os.Executable()
	if err != nil {
		self = os.Args[0]
	}
	ui.Info(ui.Dim + i18n.Tp("un_self", self) + ui.Reset)

	yn, _ := tty.ReadLine(i18n.T("un_confirm"))
	if !strings.HasPrefix(yn, "y") && !strings.HasPrefix(yn, "Y") {
		ui.Info(i18n.T("un_cancel"))
		return 1
	}

	for _, a := range accounts() {
		secret.Delete(a)
	}
	// The tokens also sit IN PLAIN TEXT in each pinned directory — clean those
	// before the config dir goes, since the registry pointing at them is in it.
	for _, r := range rows {
		if r.dir == "" || !fsx.IsDir(r.dir) {
			continue
		}
		// Twice: off empties ANTHROPIC_AUTH_TOKEN rather than removing it (for
		// sessions still running); uninstall leaves nothing behind.
		if writeRepoPinErr(r.dir, "off", "") != nil || writeRepoPinErr(r.dir, "off", "") != nil {
			ui.Info(ui.Yellow + i18n.Tp("un_cantclean", repoFile(r.dir)) + ui.Reset)
		}
	}
	if pinnedAcc() != "" {
		toStderr(func() int { return cmdSwitch([]string{"main"}) })
	}
	// switch main above made a FRESH backup (holding the token just unpinned)
	// before it is deleted here — skip this and the last token lives on in
	// the .bak file while the user believes everything is gone.
	os.Remove(bak)
	os.RemoveAll(cfgDir)
	ui.Info(ui.Green + i18n.T("un_done") + ui.Reset + i18n.T("un_done2"))
	return 0
}
