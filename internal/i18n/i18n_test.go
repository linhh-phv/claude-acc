package i18n

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var verbs = regexp.MustCompile(`%[s%]`)

func arity(s string) int {
	return strings.Count(verbs.ReplaceAllStringFunc(s, func(v string) string {
		if v == "%%" {
			return ""
		}
		return "\x00"
	}), "\x00")
}

func TestCatalogsMatch(t *testing.T) {
	for k, v := range vi {
		e, ok := en[k]
		if !ok {
			t.Errorf("%s: missing in en", k)
			continue
		}
		if arity(v) != arity(e) {
			t.Errorf("%s: vi has %d %%s, en has %d", k, arity(v), arity(e))
		}
	}
	for k := range en {
		if _, ok := vi[k]; !ok {
			t.Errorf("%s: missing in vi", k)
		}
	}
}

// Every key the code asks for exists.
func TestKeysUsedExist(t *testing.T) {
	call := regexp.MustCompile(`i18n\.Tp?\("([a-z0-9_]+)"`)
	filepath.Walk("..", func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, _ := os.ReadFile(p)
		for _, m := range call.FindAllStringSubmatch(string(b), -1) {
			if _, ok := en[m[1]]; !ok {
				t.Errorf("%s: key %q not in the catalogs", p, m[1])
			}
		}
		return nil
	})
}

// The Windows wording replaces existing keys and keeps their %s count.
func TestWindowsOverrides(t *testing.T) {
	b, err := os.ReadFile("catalog_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	entry := regexp.MustCompile(`(?m)^\t\t"([a-z0-9_]+)":\s+("(?:[^"\\]|\\.)*"),$`)
	n := 0
	for _, m := range entry.FindAllStringSubmatch(string(b), -1) {
		n++
		base, ok := en[m[1]]
		if !ok {
			t.Errorf("windows override %q is not a catalog key", m[1])
			continue
		}
		if arity(base) != arity(m[2]) {
			t.Errorf("windows override %q: %d %%s, catalog has %d", m[1], arity(m[2]), arity(base))
		}
	}
	if n < 20 {
		t.Errorf("parsed only %d overrides", n)
	}
}

func TestPrintf(t *testing.T) {
	cases := []struct {
		f    string
		args []string
		want string
	}{
		{"a %s b", []string{"x"}, "a x b"},
		{"%s%%", []string{"5"}, "5%"},
		{"%s and %s", []string{"x"}, "x and "},
		{"[%s]", []string{"a", "b"}, "[a][b]"},
		{"no verbs", []string{"x"}, "no verbs"},
	}
	for _, c := range cases {
		if got := Printf(c.f, c.args...); got != c.want {
			t.Errorf("Printf(%q, %q) = %q, want %q", c.f, c.args, got, c.want)
		}
	}
}
