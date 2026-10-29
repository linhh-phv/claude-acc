package app

import (
	"strings"

	"github.com/linhh-phv/claude-acc/internal/api"
	"github.com/linhh-phv/claude-acc/internal/fsx"
	"github.com/linhh-phv/claude-acc/internal/i18n"
	"github.com/linhh-phv/claude-acc/internal/maincred"
	"github.com/linhh-phv/claude-acc/internal/ui"
)

// cmdHere pins an account to ONE directory (repo or workspace), independent
// of the global pin.
func cmdHere(args []string) int {
	name, at := "", ""
	check, shareAll := true, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--at":
			at = arg(args, i+1)
			if at == "" {
				ui.Die(i18n.T("err_at_needs"))
			}
			i++
		case strings.HasPrefix(a, "--at="):
			at = strings.TrimPrefix(a, "--at=")
		case a == "--no-check":
			check = false
		case a == "--all":
			shareAll = true
		case strings.HasPrefix(a, "-"):
			ui.Die(i18n.Tp("err_unknown", a))
		default:
			if name != "" {
				ui.Die(i18n.Tp("err_one_acc", a))
			}
			name = a
		}
	}
	if name == "" {
		ui.Die(i18n.T("usage_here"))
	}

	var tgt target
	if at != "" {
		if !fsx.IsDir(at) {
			ui.Die(i18n.Tp("err_no_dir", at))
		}
		tgt.dir = fsx.RealPath(at)
	} else {
		var err error
		if tgt, err = resolveTarget(fsx.LogicalCwd()); err != nil {
			ui.Die(err.Error())
		}
	}
	dir := tgt.dir

	tok := ""
	if name != "off" && name != "main" {
		validName(name)
		tok = tokenOf(name)
	}

	if name == "off" {
		// What a first off leaves (an empty ANTHROPIC_AUTH_TOKEN) a second
		// one clears.
		if repoPin(dir) == "" && !emptyAuthLeft(dir) {
			ui.Die(i18n.Tp("err_not_pinned", dir))
		}
		writeRepoPin(dir, "off", "")
		regDel(dir)
		ui.Info(ui.Green + i18n.Tp("here_unpin", dir) + ui.Reset)
		ui.Info(ui.Dim + i18n.T("here_unpin2") + ui.Reset)
		return 0
	}

	was := repoPin(dir)
	guardGit(dir)
	writeRepoPin(dir, name, tok)
	regSet(dir, name)

	switch {
	case was == name:
		ui.Info(ui.Green + i18n.Tp("here_same", name, dir) + ui.Reset)
	case name == "main":
		ui.Info(ui.Green + i18n.T("here_main") + ui.Reset + i18n.Tp("here_main2", dir))
	default:
		ui.Info(ui.Green + i18n.Tp("here_acc", name) + ui.Reset + i18n.Tp("here_for", dir))
		ui.Info(ui.Yellow + i18n.Tp("here_plain", repoFile(dir)) + ui.Reset)
	}
	if tgt.note != "" {
		ui.Info(ui.Dim + i18n.Tp("here_ws", tgt.note) + ui.Reset)
		ui.Info(ui.Dim + i18n.T("here_ws2") + ui.Reset)
	}
	if len(tgt.others) > 0 {
		ui.Info(ui.Dim + i18n.Tp("here_ws_other", strings.Join(tgt.others, ", ")) + ui.Reset)
		ui.Info(ui.Dim + i18n.T("here_ws_other2") + ui.Reset)
	}

	if now := fsx.Cwd(); now != dir {
		ui.Info(ui.Dim + i18n.Tp("here_diff", now) + ui.Reset)
		ui.Info(ui.Dim + i18n.T("here_diff2") + ui.Reset)
	}

	if shareAll {
		shareWorkspace(dir, name, tok)
	} else {
		// Without --all, still move the workspace folders that were pinned
		// before (real files). Symlinks follow on their own.
		syncWsMembers(dir, name, tok)
	}

	ui.Info(ui.Bold + i18n.T("here_now") + ui.Reset + i18n.T("here_now2"))
	if g := globalAcc(); name == "main" && g != "main" {
		ui.Info(ui.Yellow + i18n.Tp("pin_main_ignored", g, g) + ui.Reset)
	}
	if !check {
		return 0
	}
	ui.Info(ui.Dim + i18n.T("here_check") + ui.Reset)
	vtok := tok
	if name == "main" {
		vtok, _ = maincred.AccessToken()
		if vtok == "" {
			ui.Info(ui.Dim + "  " + i18n.T("usg_nocred") + ui.Reset)
			return 0
		}
	}
	out, ok := api.Probe(vtok)
	if !ok && name == "main" {
		maincred.Refresh()
		if vtok, _ = maincred.AccessToken(); vtok != "" {
			out, ok = api.Probe(vtok)
		}
	}
	if ok {
		ui.Info(ui.Green + "✓ " + i18n.Tp("here_ok", name) + ui.Reset)
		return 0
	}
	ui.Info(ui.Red + "✗ " + i18n.T("here_fail") + ui.Reset)
	ui.Info("  " + out)
	return 1
}
