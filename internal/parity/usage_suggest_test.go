//go:build darwin

package parity

import (
	"strings"
	"testing"
	"time"
)

// TestUsageSuggestsSoonestReset: the hint under `usage` names the usable
// account whose 7-day window resets soonest — its tokens are lost at the
// reset otherwise — then the one whose 5-hour window does. 1.x named the
// one with most room left (see suggestByReset).
func TestUsageSuggestsSoonestReset(t *testing.T) {
	goBin, _ := binaries(t)
	specs := map[string]apiSpec{}
	s := newSandbox(t, specs)
	// The handler reads specs on each request, so the resets can hang off
	// s.now. Two 7-day resets 35 minutes apart in the same clock hour tie.
	hour := s.now.Truncate(time.Hour).Add(2 * 24 * time.Hour).Sub(s.now)
	ok := func(u5, u7 string, r5, r7 time.Duration) apiSpec {
		return apiSpec{code: 200, status: [2]string{"allowed", "allowed"}, util: [2]string{u5, u7}, reset: [2]time.Duration{r5, r7}}
	}
	day := 24 * time.Hour
	specs["tok-four"] = ok("0.1", "0.3", 1*time.Hour, 4*day+2*time.Hour)                           // the photo: suggested before
	specs["tok-soon"] = ok("0.2", "0.45", 4*time.Hour+50*time.Minute, 27*time.Hour+30*time.Minute) // the photo: should be
	specs["tok-early"] = ok("0.6", "0.6", 4*time.Hour, hour+5*time.Minute)
	specs["tok-later"] = ok("0.1", "0.1", 1*time.Hour, hour+40*time.Minute)
	specs["tok-out"] = apiSpec{code: 429, status: [2]string{"allowed", "rejected"}, util: [2]string{"0.3", "1.0"},
		reset: [2]time.Duration{time.Hour, time.Hour}, reason: "Rate limited"}
	specs["tok-full"] = apiSpec{code: 429, status: [2]string{"rejected", "allowed"}, util: [2]string{"1.0", "0.5"},
		reset: [2]time.Duration{time.Hour, 2 * time.Hour}, reason: "Rate limited"}
	specs["tok-warn"] = apiSpec{code: 200, status: [2]string{"allowed", "allowed_warning"}, util: [2]string{"0.4", "0.97"},
		reset: [2]time.Duration{2 * time.Hour, 20 * time.Hour}}

	date := func(d time.Duration) string { return s.now.Add(d).Format("02/01") }
	cases := []struct {
		name, lang string
		accounts   []string
		want       string // the hint line; "" = no hint
	}{
		{name: "soonest 7-day reset, not most room (the photo)", accounts: []string{"four", "soon"},
			want: "Use up first: soon (7-day window resets soonest: " + date(27*time.Hour+30*time.Minute) + ", 1 day 3h left; 45% used)"},
		{name: "in Vietnamese", lang: "vi_VN.UTF-8", accounts: []string{"four", "soon"},
			want: "Dùng trước cho hết: soon (7 ngày reset sớm nhất: " + date(27*time.Hour+30*time.Minute) + ", còn 1 ngày 3h; đã dùng 45%)"},
		{name: "7-day resets in one hour: the 5-hour reset decides", accounts: []string{"four", "early", "later"},
			want: "Use up first: later (7-day window resets soonest: " + date(hour+40*time.Minute) + ", "},
		{name: "out of 7-day or 5-hour quota: skipped however soon", accounts: []string{"four", "soon", "out", "full"},
			want: "Use up first: soon ("},
		{name: "near the 7-day ceiling but not out: still first", accounts: []string{"four", "soon", "warn"},
			want: "Use up first: warn ("},
		{name: "one usable account: no hint", accounts: []string{"four", "out", "full"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s.t = t
			s.reset()
			s.accounts(c.accounts...)
			for _, a := range c.accounts {
				s.token(a, "tok-"+a)
			}
			lang := c.lang
			if lang == "" {
				lang = "en_US.UTF-8"
			}
			u := s.run([]string{goBin}, s.w, []string{"usage"}, "", lang)
			if u.code != 0 {
				t.Fatalf("usage exited %d\n%s", u.code, u.stderr)
			}
			hint := ""
			for _, ln := range strings.Split(u.stdout, "\n") {
				if strings.HasPrefix(ln, "Use up first:") || strings.HasPrefix(ln, "Dùng trước cho hết:") {
					hint = ln
				}
			}
			switch {
			case c.want == "" && hint != "":
				t.Errorf("hint %q, want none\n%s", hint, u.stdout)
			case c.want != "" && !strings.HasPrefix(hint, c.want):
				t.Errorf("hint %q, want %q\n%s", hint, c.want, u.stdout)
			}
		})
	}
}
