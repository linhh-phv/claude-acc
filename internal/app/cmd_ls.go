package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/linhh-phv/claude-acc/internal/fsx"
	"github.com/linhh-phv/claude-acc/internal/i18n"
	"github.com/linhh-phv/claude-acc/internal/maincred"
	"github.com/linhh-phv/claude-acc/internal/pyjson"
	"github.com/linhh-phv/claude-acc/internal/secret"
	"github.com/linhh-phv/claude-acc/internal/ui"
	"github.com/linhh-phv/claude-acc/internal/workspace"
)

// mainEmail is the email `claude auth status --json` reports for main.
func mainEmail() string {
	c := claudeCmd(maincred.WithoutToken(os.Environ()), "auth", "status", "--json")
	c.Stdin, c.Stdout, c.Stderr = nil, nil, nil
	out, _ := c.Output()
	v, err := pyjson.Parse(out)
	if err != nil {
		return ""
	}
	s, _ := findEmail(v)
	return s
}

// findEmail walks the JSON depth-first in key order for the first email-like
// key holding a string.
func findEmail(v any) (string, bool) {
	switch t := v.(type) {
	case *pyjson.Object:
		for _, k := range t.Keys() {
			val, _ := t.Get(k)
			switch strings.ToLower(k) {
			case "email", "emailaddress", "account_email":
				if s, ok := val.(string); ok {
					return s, true
				}
			}
			if s, _ := findEmail(val); s != "" {
				return s, true
			}
		}
	case []any:
		for _, e := range t {
			if s, _ := findEmail(e); s != "" {
				return s, true
			}
		}
	}
	return "", false
}

// usageCached summarises an account's last `usage` reading. Never calls the
// API: looking at the list must not open anyone's 5-hour window.
func usageCached(acc string) string {
	var row []string
	for _, ln := range readLines(usageCache) {
		f := strings.Split(ln, "\t")
		if len(f) >= 8 && f[0] == acc {
			row = f
			break
		}
	}
	if row == nil {
		return ""
	}
	ts, ok1 := pyInt(row[1])
	r5, ok2 := pyInt(row[3])
	r7, ok3 := pyInt(row[6])
	if !ok1 || !ok2 || !ok3 {
		return ""
	}
	now := time.Now().Unix()
	pc := func(u string, reset int64) string {
		if now > reset {
			return i18n.T("ls_reset_done")
		}
		if f, ok := pyFloat(u); ok {
			return strconv.Itoa(pyRound(f*100)) + "%"
		}
		return "?"
	}
	age := now - ts
	var ago string
	switch {
	case age < 90:
		ago = strconv.FormatInt(age, 10) + "s"
	case age < 5400:
		ago = strconv.FormatInt(floorDiv(age, 60), 10) + "m"
	case age < 172800:
		ago = strconv.FormatInt(floorDiv(age, 3600), 10) + "h"
	default:
		ago = strconv.FormatInt(floorDiv(age, 86400), 10) + "d"
	}
	return "5h " + padRight(pc(row[2], r5), 6) + " 7d " + padRight(pc(row[5], r7), 6) + " " + i18n.Tp("ls_ago", ago)
}

// cmdLs shows the accounts, which one is in effect where, and every pin.
func cmdLs(args []string) int {
	who := mainEmail()
	w := ui.Stdout
	B, D, G, R, Z := ui.Bold, ui.Dim, ui.Green, ui.Red, ui.Reset

	global := globalAcc()
	fmt.Fprintf(w, "%s%s%s  %s  %s(%s)%s\n", B, i18n.T("ls_global"), Z, global, D, settingsLabel(), Z)
	printWhere(w, global)
	fmt.Fprintln(w)
	cwd := fsx.Cwd()

	fmt.Fprintf(w, "%smain%s      %s%s%s\n", B, Z, D, usageCached("main"), Z)
	fmt.Fprintf(w, "        %s(%s)%s\n", D, i18n.T("ls_main_desc"), Z)
	if who != "" {
		fmt.Fprintf(w, "        %s%s%s\n", D, who, Z)
	}

	if fsx.NonEmpty(listFile) {
		fmt.Fprintf(w, "\n%s%s%s %s(%s)%s\n", B, i18n.T("ls_sub"), Z, D, i18n.T("ls_sub_desc"), Z)
		for _, a := range accounts() {
			if secret.Has(a) {
				fmt.Fprintf(w, "  %s✓%s %s %s%s%s\n", G, Z, padRight(a, 10), D, usageCached(a), Z)
			} else {
				fmt.Fprintf(w, "  %s✗%s %s %s(%s)%s\n", R, Z, a, D, i18n.Tp("ls_lost_tok", a), Z)
			}
		}
	} else {
		fmt.Fprintf(w, "\n%s%s%s\n", D, i18n.T("ls_none"), Z)
	}

	// Never ran `usage`: the quota column is blank — say how to fill it, or
	// it looks broken.
	if !fsx.NonEmpty(usageCache) {
		fmt.Fprintf(w, "  %s%s%s\n", D, i18n.T("ls_no_usage"), Z)
	}

	if fsx.NonEmpty(reposFile) {
		fmt.Fprintf(w, "\n%s%s%s %s(%s)%s\n", B, i18n.T("ls_pins"), Z, D, i18n.T("ls_pins_hint"), Z)
		listPins(cwd)
	}
	return 0
}

// printWhere prints which account is in effect where you stand: the GUI row
// when the workspace runs claude in another folder, then the HERE row.
// global is the global account ("main" when none is pinned).
func printWhere(w io.Writer, global string) {
	B, D, Z := ui.Bold, ui.Dim, ui.Reset

	// The GUI runs in folder [0] of the workspace, not where you stand. This
	// line goes BEFORE "HERE": someone who just pinned the workspace and then
	// reads "HERE <other account>" would think the pin did not take.
	cwd := fsx.Cwd()
	gt, _ := resolveTarget(fsx.LogicalCwd())
	gdir, gnote := gt.dir, gt.note
	hasGUI := false
	if gdir != "" && gdir != cwd {
		hasGUI = true
		gpin := repoPin(gdir)
		if gpin == "" {
			gpin = global
		}
		extra := ""
		if gnote != "" {
			extra = " — " + gnote
		}
		fmt.Fprintf(w, "%s%s%s  %s  %s(%s%s)%s\n", B, i18n.T("ls_gui"), Z, gpin, D, gdir, extra, Z)
	}

	// Which account is really in effect running claude HERE? Repo settings
	// only apply in the very directory that holds them.
	hp := repoPin(cwd)
	hf := repoFile(cwd)
	var noteHere string
	switch {
	case hp != "" && fsx.IsLink(hf):
		// Shares its pin file with another directory — say so, or it reads
		// like a pin of its own.
		noteHere = i18n.Tp("ls_shared_with", filepath.Dir(filepath.Dir(fsx.RealPath(hf))))
	case hp != "":
		noteHere = i18n.Tp("ls_pin_of", cwd)
	case hasGUI:
		noteHere = i18n.T("ls_here_term")
	default:
		noteHere = i18n.T("ls_nopin")
	}
	herePin := hp
	if herePin == "" {
		herePin = global
	}
	fmt.Fprintf(w, "%s%s%s  %s  %s(%s)%s\n", B, i18n.T("ls_here"), Z, herePin, D, noteHere, Z)
}

// listPins prints the registry. Folders of one workspace are not listed side
// by side: the user pinned the workspace ONCE, and three lines would read as
// three separate pins. They are summed up under folder [0].
func listPins(cwd string) {
	w := ui.Stdout
	D, Z := ui.Dim, ui.Reset
	home, _ := os.UserHomeDir()

	type row struct{ d, a string }
	var rows []row
	for _, ln := range readLines(reposFile) {
		if d, a, ok := strings.Cut(ln, "\t"); ok {
			rows = append(rows, row{d, a})
		}
	}
	known := map[string]bool{}
	for _, r := range rows {
		known[fsx.RealPath(r.d)] = true
	}
	// Which workspace are you in? Standing in a worktree inside it still
	// counts as "here" for the * mark.
	cwdRoot, _ := workspace.Of(cwd)
	rcwd := fsx.RealPath(cwd)
	members := map[string]bool{}
	for _, r := range rows {
		root, _ := workspace.Of(r.d)
		if root != "" && fsx.RealPath(r.d) != root && known[root] {
			members[fsx.RealPath(r.d)] = true
		}
	}
	for _, r := range rows {
		rd := fsx.RealPath(r.d)
		if members[rd] {
			continue
		}
		mark := " "
		if rd == rcwd || (cwdRoot != "" && rd == cwdRoot) {
			mark = "*"
		}
		label := homeLabel(r.d, home)
		if !fsx.IsDir(r.d) {
			fmt.Fprintf(w, "  %s %s %s %s(%s)%s\n", mark, padRight(r.a, 8), label, D, i18n.T("ls_dir_gone"), Z)
			continue
		}
		fmt.Fprintf(w, "  %s %s %s\n", mark, padRight(r.a, 8), label)
		root, paths := workspace.Of(r.d)
		if root == rd && len(paths) > 1 {
			sym, own := 0, 0
			for _, q := range paths[1:] {
				if _, ok := homeElsewhere(q, rd); ok {
					continue // listed on its own line
				}
				f := repoFile(q)
				if fsx.IsLink(f) {
					sym++
				} else if fsx.IsFile(f) && known[fsx.RealPath(q)] {
					own++
				}
			}
			if sym > 0 || own > 0 {
				fmt.Fprintf(w, "             %s%s%s\n", D,
					i18n.Tp("ls_ws_summary", strconv.Itoa(len(paths)), strconv.Itoa(sym), strconv.Itoa(own)), Z)
			}
		}
	}
}
