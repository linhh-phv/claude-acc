//go:build darwin

package parity

import (
	"reflect"
	"strings"
	"testing"
)

// whereOf returns the GUI / HERE rows of an output, in order.
func whereOf(out string) []string {
	var rows []string
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "GUI ") || strings.HasPrefix(ln, "HERE ") {
			rows = append(rows, ln)
		}
	}
	return rows
}

// TestUsageSaysWhatIsInEffectHere: `usage` tells which account is in effect
// where you stand, with the very rows `ls` prints there, between its table
// and the "Most headroom" hint. 1.x usage had none (see hereInUsage).
func TestUsageSaysWhatIsInEffectHere(t *testing.T) {
	goBin, _ := binaries(t)
	s := newSandbox(t, apiSpecs)
	s.extraEnv = map[string]string{}
	lang := "en_US.UTF-8"

	cases := []struct {
		name  string
		setup func() string // returns the directory to stand in
		args  []string
		want  []string // the rows; "{a}" / "{dir}" stand for paths
	}{
		{name: "own pin", args: []string{"usage"}, setup: func() string {
			r := s.repo("r1")
			pin(s, r, "play", "tok-play")
			return r
		}, want: []string{"HERE      play  (pin of {dir})"}},
		{name: "extra folder of a workspace", args: []string{"usage"}, setup: func() string {
			a, b, _ := workspace(s, "proj")
			pin(s, a, "work", "tok-work")
			return b
		}, want: []string{
			"GUI       work  ({a} — proj.code-workspace (3 folder))",
			"HERE      main  (only if you run claude from a terminal here — the GUI uses the row above)",
		}},
		{name: "no pin, global account", args: []string{"usage"}, setup: func() string {
			s.writeAbs(settingsFile(s), `{"env": {"CLAUDE_CODE_OAUTH_TOKEN": "tok-play", "CLAUDE_ACC_NAME": "play"}}`)
			return s.dir("loose")
		}, want: []string{"HERE      play  (no pin here — follows the global one)"}},
		{name: "one account asked", args: []string{"usage", "work"}, setup: func() string {
			r := s.repo("r1")
			pin(s, r, "play", "tok-play")
			return r
		}, want: []string{"HERE      play  (pin of {dir})"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s.t = t
			s.reset()
			std(s)
			dir := c.setup()
			want := make([]string, len(c.want))
			for i, w := range c.want {
				w = strings.ReplaceAll(w, "{dir}", dir)
				want[i] = strings.ReplaceAll(w, "{a}", s.w+"/hub/a")
			}

			u := s.run([]string{goBin}, dir, c.args, "", lang)
			if u.code != 0 {
				t.Fatalf("usage exited %d\n%s", u.code, u.stderr)
			}
			if got := whereOf(u.stdout); !reflect.DeepEqual(got, want) {
				t.Errorf("usage rows %q, want %q\n%s", got, want, u.stdout)
			}
			l := s.run([]string{goBin}, dir, []string{"ls"}, "", lang)
			if got := whereOf(l.stdout); !reflect.DeepEqual(got, want) {
				t.Errorf("ls rows %q, want %q — the two commands disagree", got, want)
			}

			// Under the table, above the hint (when there is one).
			table := strings.Index(u.stdout, "\n * ")
			here := strings.Index(u.stdout, want[0])
			if table < 0 || here < table {
				t.Errorf("rows are not under the table:\n%s", u.stdout)
			}
			if hint := strings.Index(u.stdout, "Most headroom:"); hint >= 0 && hint < here {
				t.Errorf("rows come after the hint:\n%s", u.stdout)
			}
		})
	}
}
