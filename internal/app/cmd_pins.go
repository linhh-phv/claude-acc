package app

import (
	"strconv"

	"github.com/linhh-phv/claude-acc/internal/fsx"
	"github.com/linhh-phv/claude-acc/internal/i18n"
	"github.com/linhh-phv/claude-acc/internal/secret"
	"github.com/linhh-phv/claude-acc/internal/ui"
)

// cmdReassign moves every directory pinned to <old> over to <new> — for when
// one account nears its limit and the load should go elsewhere.
func cmdReassign(args []string) int {
	oldAcc, newAcc := arg(args, 0), arg(args, 1)
	if oldAcc == "" || newAcc == "" {
		ui.Die(i18n.T("usage_reassign"))
	}
	tok := ""
	if newAcc != "main" {
		validName(newAcc)
		tok = tokenOf(newAcc)
	}

	n, missing := 0, 0
	for _, r := range regRows() {
		if r.dir == "" || r.acc != oldAcc {
			continue
		}
		if !fsx.IsDir(r.dir) {
			ui.Info(ui.Yellow + i18n.Tp("ra_skip", r.dir) + ui.Reset)
			missing++
			continue
		}
		if !gitPinOK(r.dir) {
			ui.Info(ui.Yellow + i18n.Tp("git_skip", r.dir) + ui.Reset)
			continue
		}
		writeRepoPin(r.dir, newAcc, tok)
		regSet(r.dir, newAcc)
		ui.Info("  " + r.dir)
		n++
	}
	if n == 0 {
		ui.Die(i18n.Tp("ra_none", oldAcc))
	}
	ui.Info(ui.Green + i18n.Tp("ra_done", strconv.Itoa(n), oldAcc, newAcc) + ui.Reset)
	if missing > 0 {
		ui.Info(ui.Dim + i18n.Tp("ra_missing", strconv.Itoa(missing)) + ui.Reset)
	}
	ui.Info(ui.Dim + i18n.T("ra_now") + ui.Reset)
	return 0
}

// cmdSync rewrites the current token into every pinned directory — after an
// account was re-added (new token), or when a repo file looks edited.
func cmdSync(args []string) int {
	n, fixed, gone := 0, 0, 0
	globalTok := ""
	for _, r := range regRows() {
		if r.dir == "" {
			continue
		}
		if !fsx.IsDir(r.dir) {
			ui.Info(ui.Yellow + i18n.Tp("sync_drop", r.dir) + ui.Reset)
			regDel(r.dir)
			gone++
			continue
		}
		tok := ""
		if r.acc != "main" {
			t, err := secret.Get(r.acc)
			if err != nil {
				ui.Info(ui.Red + i18n.Tp("sync_lost", r.dir, r.acc, r.acc) + ui.Reset)
				continue
			}
			tok = t
		}
		before := repoPin(r.dir)
		if !gitPinOK(r.dir) {
			ui.Info(ui.Yellow + i18n.Tp("git_skip", r.dir) + ui.Reset)
			continue
		}
		writeRepoPin(r.dir, r.acc, tok)
		// the global pin follows too, but is written once, below
		if pinnedAcc() == r.acc {
			globalTok = tok
		}
		if before != r.acc {
			ui.Info(i18n.Tp("sync_fix", r.dir, before, r.acc))
			fixed++
		}
		n++
	}
	if globalTok != "" {
		if writeGlobalToken(pinnedAcc(), globalTok) == nil {
			ui.Info(ui.Dim + i18n.T("sync_global") + ui.Reset)
		}
	}
	ui.Info(ui.Green + i18n.Tp("sync_done", strconv.Itoa(n)) + ui.Reset)
	if fixed > 0 {
		ui.Info(ui.Dim + i18n.Tp("sync_fixn", strconv.Itoa(fixed)) + ui.Reset)
	}
	if gone > 0 {
		ui.Info(ui.Dim + i18n.Tp("sync_gonen", strconv.Itoa(gone)) + ui.Reset)
	}
	return 0
}

// cmdRm removes a secondary account.
func cmdRm(args []string) int {
	name := arg(args, 0)
	if name == "" {
		ui.Die(i18n.T("usage_rm"))
	}
	validName(name)
	if secret.Delete(name) != nil {
		ui.Info(ui.Yellow + i18n.Tp("rm_notfound", name) + ui.Reset)
	}
	forget(name)
	if pinnedAcc() == name {
		ui.Info(ui.Yellow + i18n.Tp("rm_pinned", name) + ui.Reset)
		toStderr(func() int { return cmdSwitch([]string{"main"}) })
	}
	ui.Info(ui.Green + i18n.Tp("rm_done", name) + ui.Reset)
	return 0
}
