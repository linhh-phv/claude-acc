package app

import (
	"fmt"
	"os"

	"github.com/linhh-phv/claude-acc/internal/api"
	"github.com/linhh-phv/claude-acc/internal/fsx"
	"github.com/linhh-phv/claude-acc/internal/i18n"
	"github.com/linhh-phv/claude-acc/internal/maincred"
	"github.com/linhh-phv/claude-acc/internal/ui"
)

// cmdSwitch makes an account the global one, for the CLI and the GUI alike.
func cmdSwitch(args []string) int {
	name := arg(args, 0)
	if name == "" {
		ui.Die(i18n.T("usage_switch"))
	}
	tok := ""
	if name != "main" {
		validName(name)
		tok = tokenOf(name)
	}

	backupSettings()
	if err := writeGlobal(name, tok); err != nil {
		ui.Die(i18n.Tp("err_write_glob", settings, settings+".bak-claude-acc"))
	}

	if name == "main" {
		ui.Info(ui.Green + i18n.T("sw_to_main") + ui.Reset + i18n.T("sw_to_main2"))
	} else {
		ui.Info(ui.Green + i18n.Tp("sw_to_acc", name) + ui.Reset + i18n.T("sw_to_acc2"))
		ui.Info(ui.Yellow + i18n.Tp("sw_plain", settings) + ui.Reset)
	}
	ui.Info(ui.Bold + i18n.T("sw_now") + ui.Reset)
	ui.Info(ui.Dim + i18n.T("sw_now2") + ui.Reset)
	ui.Info(ui.Dim + i18n.T("sw_now3") + ui.Reset)

	// ...except directories pinned on their own: a repo pin beats the global
	// one. Standing in one of those and saying nothing, the user would think
	// this repo switched too.
	cwd := fsx.Cwd()
	gt, _ := resolveTarget(fsx.LogicalCwd())
	gdir := gt.dir
	gpin := repoPin(cwd)
	if gpin == "" && gdir != "" && gdir != cwd {
		gpin = repoPin(gdir)
	}
	if gpin != "" && gpin != name {
		ui.Info("")
		ui.Info(ui.Yellow + i18n.T("sw_pinned") + ui.Reset + i18n.Tp("sw_pinned2", gpin))
		ui.Info(ui.Dim + i18n.T("sw_pinned3") + ui.Reset)
		ui.Info(ui.Dim + i18n.Tp("sw_fix1", name) + ui.Reset)
		ui.Info(ui.Dim + i18n.T("sw_fix2") + ui.Reset)
	}

	ui.Info(ui.Dim + i18n.T("sw_check") + ui.Reset)
	return cmdCheck([]string{name})
}

// cmdCheck confirms with one real API call that an account's credential
// works. Straight to the API, not through `claude`: the settings.json env
// block overrides the process environment, so `claude` would test whichever
// account is pinned instead of the one asked about.
func cmdCheck(args []string) int {
	name := arg(args, 0)
	if name == "" {
		ui.Die(i18n.T("usage_check"))
	}
	var tok string
	if name == "main" {
		claudeCmd(maincred.WithoutToken(os.Environ()), "auth", "status", "--text").Run()
		tok, _ = maincred.AccessToken()
		if tok == "" {
			fmt.Fprintf(ui.Stdout, "%s✗ %s%s\n", ui.Red, i18n.T("usg_nocred"), ui.Reset)
			return 1
		}
	} else {
		validName(name)
		tok = tokenOf(name)
		claudeCmd(append(os.Environ(), envToken+"="+tok), "auth", "status", "--text").Run()
	}

	fmt.Fprintf(ui.Stderr, "%s%s%s\n", ui.Dim, i18n.T("trying_real"), ui.Reset)

	// ONE call. Calling again "to be sure" burns quota of the very account
	// being checked.
	out, ok := api.Probe(tok)
	if !ok && name == "main" {
		// main's access token lives a few hours; let Claude Code renew it.
		maincred.Refresh()
		if t, _ := maincred.AccessToken(); t != "" {
			out, ok = api.Probe(t)
		}
	}

	label := i18n.Tp("chk_ok", name)
	if name == "main" {
		label = i18n.T("chk_main_ok")
	}
	if ok {
		if out == "RATE_LIMITED" {
			fmt.Fprintf(ui.Stdout, "%s✓ %s%s %s%s%s\n", ui.Green, label, ui.Reset, ui.Yellow, i18n.T("chk_rate_limited"), ui.Reset)
		} else {
			fmt.Fprintf(ui.Stdout, "%s✓ %s%s\n", ui.Green, label, ui.Reset)
		}
		return 0
	}
	if name == "main" {
		fmt.Fprintf(ui.Stdout, "%s✗ %s%s%s\n", ui.Red, i18n.T("chk_main_bad"), ui.Reset, i18n.T("chk_main_bad2"))
	} else {
		fmt.Fprintf(ui.Stdout, "%s✗ %s%s%s\n", ui.Red, i18n.Tp("chk_bad", name), ui.Reset, i18n.Tp("chk_bad2", name))
	}
	if out != "" {
		fmt.Fprintf(ui.Stdout, "  %s%s%s\n", ui.Dim, out, ui.Reset)
	}
	return 1
}
