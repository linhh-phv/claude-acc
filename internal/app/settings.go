package app

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/linhh-phv/claude-acc/internal/fsx"
	"github.com/linhh-phv/claude-acc/internal/i18n"
	"github.com/linhh-phv/claude-acc/internal/pyjson"
	"github.com/linhh-phv/claude-acc/internal/ui"
)

// The global switch is the `env` block of ~/.claude/settings.json — shared by
// the extension and the CLI (docs). Measured: that block overrides both the
// stored credential and the process environment, and the CLI re-reads the
// file on every request, so a switch takes effect immediately, in every
// window and every repo.

// pinnedAcc is the account pinned globally, or "" for main.
func pinnedAcc() string {
	if !fsx.IsFile(settings) {
		return ""
	}
	d, err := loadObject(settings)
	if err != nil {
		return ""
	}
	env, err := envOf(d)
	if err != nil {
		return ""
	}
	return env.String(envName)
}

// backupSettings copies settings.json to .bak-claude-acc, created 0600 from
// the first byte: the copy holds a token too.
func backupSettings() {
	if !fsx.IsFile(settings) {
		return
	}
	data, err := os.ReadFile(settings)
	if err != nil {
		return
	}
	fsx.WritePrivate(settings+".bak-claude-acc", ".tmp", data)
}

// settingsTarget is the file to write: through a symlink (dotfile managers
// link settings.json), never replacing the link itself. ~/.claude is created
// if Claude Code has not made it yet.
func settingsTarget() string {
	if fsx.IsLink(settings) {
		return fsx.RealPath(settings)
	}
	os.MkdirAll(filepath.Dir(settings), 0o755)
	return settings
}

// writeGlobal pins name globally ("main" removes the pin). The token travels
// in memory, never in argv where `ps` shows it.
func writeGlobal(name, tok string) error {
	d := pyjson.NewObject()
	if fsx.Exists(settings) {
		var err error
		if d, err = loadObject(settings); err != nil {
			return err // a syntax error is not ours to overwrite
		}
	}
	env, err := envOf(d)
	if err != nil {
		return err
	}
	if name == "main" {
		env.Delete(envToken)
		env.Delete(envName)
	} else {
		env.Set(envToken, tok)
		env.Set(envName, name)
	}
	if env.Len() > 0 {
		d.Set("env", env)
	} else {
		d.Delete("env")
	}
	return fsx.WritePrivate(settingsTarget(), ".tmp", append(pyjson.Dump(d), '\n'))
}

// writeGlobalToken puts a renewed token into the global pin, quietly.
func writeGlobalToken(name, tok string) error {
	backupSettings()
	d := pyjson.NewObject()
	if fsx.Exists(settings) {
		var err error
		if d, err = loadObject(settings); err != nil {
			return err
		}
	}
	env, err := envOf(d)
	if err != nil {
		return err
	}
	env.Set(envToken, tok)
	env.Set(envName, name)
	d.Set("env", env)
	return fsx.WritePrivate(settingsTarget(), ".tmp", append(pyjson.Dump(d), '\n'))
}

// resyncAcc spreads a re-added account's new token to every place pinned to
// it: the global pin and each registered directory. Without this the user
// renews, everything keeps running on the old token, and breaks some day
// nobody can predict.
func resyncAcc(name, tok string) {
	n := 0
	if pinnedAcc() == name {
		if writeGlobalToken(name, tok) == nil {
			n++
		}
	}
	for _, r := range regRows() {
		if r.dir == "" || !fsx.IsDir(r.dir) || r.acc != name {
			continue
		}
		writeRepoPin(r.dir, name, tok)
		n++
	}
	if n > 0 {
		ui.Info(ui.Dim + i18n.Tp("resync_done", strconv.Itoa(n)) + ui.Reset)
	}
}
