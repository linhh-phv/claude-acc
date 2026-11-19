package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/linhh-phv/claude-acc/internal/api"
	"github.com/linhh-phv/claude-acc/internal/fsx"
	"github.com/linhh-phv/claude-acc/internal/i18n"
	"github.com/linhh-phv/claude-acc/internal/maincred"
	"github.com/linhh-phv/claude-acc/internal/secret"
	"github.com/linhh-phv/claude-acc/internal/ui"
)

// usageOne reads one account's quota. A 401 on main is not breakage, only
// its short-lived access token having expired: let Claude Code renew it and
// ask once more instead of reporting an error.
func usageOne(acc string) string {
	var tok string
	if acc == "main" {
		t, err := maincred.AccessToken()
		if err != nil {
			return "ERR\t" + i18n.T("usg_nocred")
		}
		tok = t
	} else {
		t, err := secret.Get(acc)
		if err != nil {
			return "ERR\t" + i18n.Tp("warm_lost", acc)
		}
		tok = t
	}
	fix := i18n.Tp("usg_fix", acc)
	res := api.Usage(tok, fix)
	if acc == "main" && strings.HasPrefix(res, "EXPIRED") {
		maincred.Refresh()
		t, err := maincred.AccessToken()
		if err != nil {
			return res
		}
		res = api.Usage(t, fix)
	}
	return res
}

// cmdUsage shows 5h / 7d utilisation per account. Each reading is a real
// request, so it also opens that account's 5-hour window — which only starts
// on the first request, so this is also how to start them all at once.
func cmdUsage(args []string) int {
	var accs []string
	if len(args) == 0 {
		accs = append([]string{"main"}, accounts()...)
	} else {
		for _, a := range args {
			switch {
			case strings.HasPrefix(a, "-"):
				ui.Die(i18n.Tp("err_only_names", "usage"))
			case a == "main":
			default:
				validName(a)
				if !known(a) {
					ui.Die(i18n.Tp("err_unknown_acc", a))
				}
			}
			accs = append(accs, a)
		}
	}

	ui.Info(ui.Dim + i18n.Tp("usg_running", strconv.Itoa(len(accs))) + ui.Reset)
	ui.Info(ui.Yellow + i18n.T("usg_warn") + ui.Reset)

	res := map[string]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, a := range uniq(accs) {
		wg.Add(1)
		go func(a string) {
			defer wg.Done()
			r := usageOne(a)
			mu.Lock()
			res[a] = r
			mu.Unlock()
		}(a)
	}
	wg.Wait()

	fmt.Fprintln(ui.Stdout)
	saveUsageCache(accs, res)
	printUsage(accs, res)
	return 0
}

func uniq(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// saveUsageCache records the readings for `ls`. If `ls` called the API
// itself, every look at the list would open every account's 5-hour window.
func saveUsageCache(names []string, res map[string]string) {
	rows := map[string][]string{}
	for _, ln := range readLines(usageCache) {
		if f := strings.Split(ln, "\t"); len(f) >= 8 {
			rows[f[0]] = f
		}
	}
	now := strconv.FormatInt(time.Now().Unix(), 10)
	for _, n := range names {
		r := strings.Split(res[n], "\t")
		if len(r) >= 7 && r[0] == "OK" {
			rows[n] = append([]string{n, now}, r[1:7]...)
		}
	}
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(strings.Join(rows[k], "\t") + "\n")
	}
	fsx.WritePrivate(usageCache, ".t", []byte(b.String()))
}

// whenLeft renders a reset time: "13:40 (3h33 left)" / "11/09 (6 days left)".
func whenLeft(ts string, withDate bool) string {
	when, gap := resetAt(ts, withDate)
	if gap == "" {
		return when
	}
	return when + " (" + i18n.Tp("usg_left", gap) + ")"
}

// resetAt splits a reset time into "13:40" / "11/09" and the time left until
// it, "3h33" / "6 days" — empty once it has passed.
func resetAt(ts string, withDate bool) (when, gap string) {
	n, ok := pyInt(ts)
	if !ok || n <= 0 {
		return "", ""
	}
	left := n - time.Now().Unix()
	t := time.Unix(n, 0)
	when = t.Format("15:04")
	if withDate {
		when = t.Format("02/01")
	}
	if left <= 0 {
		return when, ""
	}
	d, rem := floorDiv(left, 86400), left%86400
	h, m := floorDiv(rem/60, 60), (rem/60)%60
	switch {
	case d >= 2:
		gap = i18n.Tp("usg_days", strconv.FormatInt(d, 10))
	case d == 1:
		gap = i18n.Tp("usg_day_h", strconv.FormatInt(h, 10))
	case h > 0:
		gap = fmt.Sprintf("%dh%02d", h, m)
	default:
		gap = fmt.Sprintf("%dm", m)
	}
	return when, gap
}

// useFirst is an account `usage` may suggest: one usable right now.
type useFirst struct {
	name     string
	r7, r5   int64   // reset times, unix seconds; never-known ones sort last
	u7, used float64 // 7-day use; use of the busier window
}

// spendBefore orders the accounts to use: the one whose 7-day window resets
// soonest first, so its tokens are spent instead of lost at the reset; then
// the one whose 5-hour window resets soonest. Resets are compared as finely
// as the table shows them — 7-day by the hour ("1 day 3h left"), 5-hour by
// the minute ("13:40"): to the second two accounts would never tie, leaving
// the next step dead. When both resets match, the one with most room left,
// then by name: what 1.x picked.
func spendBefore(c []useFirst) {
	key := func(ts int64) int64 {
		if ts <= 0 {
			return 1<<62 + ts
		}
		return ts
	}
	sort.SliceStable(c, func(i, j int) bool {
		a, b := c[i], c[j]
		if h7a, h7b := floorDiv(key(a.r7), 3600), floorDiv(key(b.r7), 3600); h7a != h7b {
			return h7a < h7b
		}
		if m5a, m5b := floorDiv(key(a.r5), 60), floorDiv(key(b.r5), 60); m5a != m5b {
			return m5a < m5b
		}
		if a.used != b.used {
			return a.used < b.used
		}
		return a.name < b.name
	})
}

func printUsage(names []string, res map[string]string) {
	w := ui.Stdout
	G, R, Y, D, Z := ui.Green, ui.Red, ui.Yellow, ui.Dim, ui.Reset

	cell := func(u, colour string) string {
		v, ok := pyFloat(u)
		if !ok {
			return "??????????   ?%"
		}
		n := pyRound(v * 10)
		n = max(0, min(10, n))
		body := strings.Repeat("#", n) + strings.Repeat(".", 10-n) + " " + pct3(pyRound(v*100)) + "%"
		if colour != "" {
			return colour + body + Z
		}
		return body
	}
	pctOr0 := func(u string) float64 {
		v, _ := pyFloat(u)
		return v
	}

	fmt.Fprintf(w, "%s  %s %s %s%s\n", Y, padRight(i18n.T("usg_hdr_acc"), 10),
		padRight(i18n.T("usg_hdr_5h"), 30), i18n.T("usg_hdr_7d"), Z)

	var best []useFirst
	for _, name := range names {
		f := strings.Split(res[name], "\t")
		if f[0] != "OK" || len(f) < 7 {
			note := ""
			if len(f) > 1 {
				note = f[1]
			}
			fmt.Fprintf(w, "  %s%s%s %s%s%s\n", R, padRight(name, 10), Z, D, note, Z)
			continue
		}
		u5, r5, s5, u7, r7, s7 := f[1], f[2], f[3], f[4], f[5], f[6]

		// Three levels, not two: "allowed", "allowed_warning" (past the
		// warning line but STILL usable) and "rejected" (really out). Folding
		// the first two into "out" showed a usable account as OUT.
		out5, out7 := s5 == "rejected", s7 == "rejected"
		warn5, warn7 := strings.Contains(s5, "warning"), strings.Contains(s7, "warning")
		ok := !(out5 || out7)

		var five, seven string
		if out5 {
			// Keep the %: "LIMIT REACHED" next to the 90% of a previous run
			// read as the tool contradicting itself.
			five = fmt.Sprintf("%s%s %d%%%s  %s", R, i18n.T("usg_out"), pyRound(pctOr0(u5)*100), Z,
				i18n.Tp("usg_out_back", whenLeft(r5, false)))
		} else {
			c := ""
			if warn5 {
				c = Y
			}
			five = cell(u5, c) + "  " + whenLeft(r5, false)
		}
		if out7 {
			seven = fmt.Sprintf("%s%s %d%%%s  %s", R, i18n.T("usg_out_short"), pyRound(pctOr0(u7)*100), Z,
				i18n.Tp("usg_out_back", whenLeft(r7, true)))
		} else {
			p7 := pct3(pyRound(pctOr0(u7)*100)) + "%"
			if warn7 {
				p7 = Y + p7 + Z
			}
			seven = p7 + "  " + whenLeft(r7, true)
		}

		mark := R + "!" + Z
		switch {
		case ok && !(warn5 || warn7):
			mark = G + "*" + Z
		case ok:
			mark = Y + "*" + Z
		}
		fmt.Fprintf(w, " %s %s %s %s\n", mark, padRight(name, 10), padVisible(five, 30), seven)
		if ok {
			t7, _ := pyInt(r7)
			t5, _ := pyInt(r5)
			best = append(best, useFirst{name, t7, t5, pctOr0(u7), max(pctOr0(u5), pctOr0(u7))})
		}
	}

	// The account in effect where you stand, under the table that shows its
	// quota: otherwise finding it takes a second command (`ls`).
	fmt.Fprintln(w)
	printWhere(w, globalAcc())

	if len(best) > 1 {
		spendBefore(best)
		when, gap := resetAt(strconv.FormatInt(best[0].r7, 10), true)
		switch {
		case when == "": // the API left the reset out
			when = "?"
		case gap != "":
			when += ", " + i18n.Tp("usg_left", gap)
		}
		fmt.Fprintf(w, "\n%s%s %s%s%s%s\n", D, i18n.T("usg_most"), Z, best[0].name, D,
			i18n.Tp("usg_most2", when, strconv.Itoa(pyRound(best[0].u7*100))))
		fmt.Fprintf(w, "%s%s\n", i18n.Tp("usg_most3", best[0].name), Z)
	}
}
