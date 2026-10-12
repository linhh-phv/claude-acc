package app

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/linhh-phv/claude-acc/internal/fsx"
	"github.com/linhh-phv/claude-acc/internal/i18n"
	"github.com/linhh-phv/claude-acc/internal/secret"
	"github.com/linhh-phv/claude-acc/internal/ui"
)

// Files claude-acc keeps. Same names and formats as the bash version, so an
// existing setup carries over untouched.
var (
	cfgDir     string // ${XDG_CONFIG_HOME:-~/.config}/claude-acc (%APPDATA% on Windows)
	listFile   string // one account name per line
	reposFile  string // registry of pinned directories: <path>\t<account>
	langFile   string // "vi" or "en" when chosen by hand
	usageCache string // last `usage` reading per account, read by `ls`
	settings   string // ~/.claude/settings.json, or $CLAUDE_ACC_SETTINGS in tests
)

func setupPaths() {
	home, _ := os.UserHomeDir()
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(home, ".config")
		if runtime.GOOS == "windows" {
			if ad := os.Getenv("APPDATA"); ad != "" {
				base = ad
			}
		}
	}
	cfgDir = filepath.Join(base, "claude-acc")
	listFile = filepath.Join(cfgDir, "accounts")
	reposFile = filepath.Join(cfgDir, "repos")
	langFile = filepath.Join(cfgDir, "lang")
	usageCache = filepath.Join(cfgDir, "usage-cache")
	settings = os.Getenv("CLAUDE_ACC_SETTINGS")
	if settings == "" {
		settings = filepath.Join(home, ".claude", "settings.json")
	}
}

// ensureFiles creates the config dir and the two list files if missing.
func ensureFiles() {
	os.MkdirAll(cfgDir, 0o755)
	for _, f := range []string{listFile, reposFile} {
		if fh, err := os.OpenFile(f, os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fh.Close()
		}
	}
}

// readLines returns the lines of a file, without line endings.
func readLines(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return nil
	}
	s := strings.TrimSuffix(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	return strings.Split(s, "\n")
}

// ---------- accounts --------------------------------------------------------

var nameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// validName dies unless name is a usable secondary account name.
func validName(name string) {
	if !nameRe.MatchString(name) {
		ui.Die(i18n.Tp("err_bad_name", name))
	}
	if name == "main" {
		ui.Die(i18n.T("err_main_res"))
	}
}

// accounts lists the secondary accounts, in the order they were added.
func accounts() []string {
	var out []string
	for _, a := range readLines(listFile) {
		if a != "" {
			out = append(out, a)
		}
	}
	return out
}

func known(name string) bool {
	for _, a := range readLines(listFile) {
		if a == name {
			return true
		}
	}
	return false
}

func remember(name string) {
	if known(name) {
		return
	}
	f, err := os.OpenFile(listFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(name + "\n")
}

func forget(name string) {
	var keep []string
	for _, a := range readLines(listFile) {
		if a != name {
			keep = append(keep, a)
		}
	}
	data := ""
	if len(keep) > 0 {
		data = strings.Join(keep, "\n") + "\n"
	}
	fsx.WritePrivate(listFile, ".tmp", []byte(data))
}

// tokenOf returns the stored token of name, or dies telling how to add it.
func tokenOf(name string) string {
	tok, err := secret.Get(name)
	if err != nil {
		ui.Die(i18n.Tp("err_no_token", name, name))
	}
	return tok
}

// trim drops every CR/LF and the surrounding whitespace of a pasted value.
func trim(s string) string {
	s = strings.NewReplacer("\r", "", "\n", "").Replace(s)
	return strings.TrimSpace(s)
}

// ---------- registry of pinned directories ---------------------------------
//
// Only there so ls/reassign/sync know where to look — the truth is the file
// in each directory.

type pinRow struct{ dir, acc string }

func regRows() []pinRow {
	var rows []pinRow
	for _, ln := range readLines(reposFile) {
		d, a, ok := strings.Cut(ln, "\t")
		if !ok {
			if strings.TrimSpace(ln) == "" {
				continue
			}
			d, a = ln, ""
		}
		rows = append(rows, pinRow{d, a})
	}
	return rows
}

func regWrite(rows []string) {
	data := ""
	if len(rows) > 0 {
		data = strings.Join(rows, "\n") + "\n"
	}
	fsx.WritePrivate(reposFile, ".t", []byte(data))
}

func regSet(path, acc string) {
	var rows []string
	for _, ln := range readLines(reposFile) {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if p, _, _ := strings.Cut(ln, "\t"); p != path {
			rows = append(rows, ln)
		}
	}
	rows = append(rows, path+"\t"+acc)
	sort.Strings(rows)
	regWrite(rows)
}

func regDel(path string) {
	if !fsx.Exists(reposFile) {
		return
	}
	var rows []string
	for _, ln := range readLines(reposFile) {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if p, _, _ := strings.Cut(ln, "\t"); p != path {
			rows = append(rows, ln)
		}
	}
	regWrite(rows)
}
