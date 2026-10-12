// Package i18n holds every user-facing string of claude-acc in Vietnamese and
// English, and picks one of the two.
package i18n

import (
	_ "embed"
	"os"
	"strings"
)

// Lang is the active language: "vi" or "en".
var Lang = "en"

// platform holds per-OS replacements for catalog entries, keyed by language
// then key. A string that names the Keychain is wrong on Windows, where tokens
// live in Credential Manager and the main account in a credentials file.
var platform = map[string]map[string]string{}

//go:embed help_vi.txt
var helpVI string

//go:embed help_en.txt
var helpEN string

// Detect picks the language: the saved choice in langFile, then the locale
// variables, then (Windows only) the UI language, then English.
func Detect(langFile string) string {
	if b, err := os.ReadFile(langFile); err == nil && len(b) > 0 {
		l := strings.NewReplacer(" ", "", "\n", "").Replace(string(b))
		if l == "vi" || l == "en" {
			return l
		}
	}
	loc := firstSet("LC_ALL", "LC_MESSAGES", "LANG")
	if loc == "" {
		loc = systemLocale()
	}
	if strings.HasPrefix(loc, "vi") || strings.Contains(loc, "_VN") {
		return "vi"
	}
	return "en"
}

func firstSet(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}

// T returns the string for key, or the key itself when there is none.
func T(key string) string {
	if s, ok := platform[Lang][key]; ok {
		return s
	}
	cat := en
	if Lang == "vi" {
		cat = vi
	}
	if s, ok := cat[key]; ok {
		return s
	}
	return key
}

// Tp formats the string for key the way bash `printf` did: each %s takes the
// next argument (an empty one when they run out), %% is a percent sign, and
// leftover arguments run the format again.
func Tp(key string, args ...string) string {
	return Printf(T(key), args...)
}

// Printf is the bash-printf subset the catalogs use: %s and %%.
func Printf(format string, args ...string) string {
	var b strings.Builder
	for {
		used := 0
		for i := 0; i < len(format); i++ {
			c := format[i]
			if c != '%' || i+1 >= len(format) {
				b.WriteByte(c)
				continue
			}
			switch format[i+1] {
			case '%':
				b.WriteByte('%')
				i++
			case 's':
				if len(args) > 0 {
					b.WriteString(args[0])
					args = args[1:]
				}
				used++
				i++
			default:
				b.WriteByte(c)
			}
		}
		if used == 0 || len(args) == 0 {
			return b.String()
		}
	}
}

// Help returns the help text with colour codes and the version filled in.
func Help(bold, reset, version string) string {
	h := helpEN
	if Lang == "vi" {
		h = helpVI
	}
	if p, ok := platform[Lang]["__help__"]; ok {
		h = p
	}
	return strings.NewReplacer("{{B}}", bold, "{{R}}", reset, "{{V}}", version).Replace(h)
}
