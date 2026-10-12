package app

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/linhh-phv/claude-acc/internal/fsx"
	"github.com/linhh-phv/claude-acc/internal/gitx"
	"github.com/linhh-phv/claude-acc/internal/i18n"
	"github.com/linhh-phv/claude-acc/internal/pyjson"
	"github.com/linhh-phv/claude-acc/internal/ui"
	"github.com/linhh-phv/claude-acc/internal/workspace"
)

// Pinning an account to one directory (repo or workspace).
//
// Measured 2026-09-05, not guessed:
//   - .claude/settings.local.json overrides .claude/settings.json and
//     ~/.claude/settings.json; the `env` block merges key by key, so a pin
//     does not wipe the permissions/MCP already in the file.
//   - Repo-level settings only apply when `claude` runs IN that directory —
//     a subdirectory does not inherit them.

const (
	envToken = "CLAUDE_CODE_OAUTH_TOKEN"
	envName  = "CLAUDE_ACC_NAME"
	pinRel   = ".claude/settings.local.json"
)

func repoFile(dir string) string { return filepath.Join(dir, ".claude", "settings.local.json") }

// truthy is Python truthiness for a decoded JSON value.
func truthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t != ""
	case pyjson.Number:
		f, err := strconv.ParseFloat(string(t), 64)
		return err != nil || f != 0
	case []any:
		return len(t) > 0
	case *pyjson.Object:
		return t.Len() > 0
	}
	return true
}

// envOf is `d.get("env") or {}`; a non-object env that is not empty is an
// error, as it was a crash in the Python version.
func envOf(d *pyjson.Object) (*pyjson.Object, error) {
	v, _ := d.Get("env")
	if !truthy(v) {
		return pyjson.NewObject(), nil
	}
	o, ok := v.(*pyjson.Object)
	if !ok {
		return nil, errors.New(`"env" is not an object`)
	}
	return o, nil
}

func loadObject(path string) (*pyjson.Object, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return pyjson.ParseObject(b)
}

// repoPin is the account pinned at dir: a name, "main" (empty token, which
// falls back to the stored credential), or "" when nothing is pinned.
func repoPin(dir string) string {
	f := repoFile(dir)
	if !fsx.IsFile(f) {
		return ""
	}
	d, err := loadObject(f)
	if err != nil {
		return ""
	}
	env, err := envOf(d)
	if err != nil {
		return ""
	}
	if name := env.String(envName); name != "" {
		return name
	}
	if v, ok := env.Get(envToken); ok {
		if s, isStr := v.(string); isStr && s == "" {
			return "main"
		}
	}
	return ""
}

// writeRepoPin pins name at dir — "main" writes an empty token (measured: it
// overrides the inherited token and falls back to the stored credential),
// "off" removes the pin. Merges into an existing file, atomically, 0600.
func writeRepoPin(dir, name, tok string) {
	if err := writeRepoPinErr(dir, name, tok); err != nil {
		ui.Die(i18n.Tp("err_write_repo", repoFile(dir)))
	}
}

func writeRepoPinErr(dir, name, tok string) error {
	os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)
	p := repoFile(dir)
	d := pyjson.NewObject()
	if fsx.Exists(p) {
		var err error
		if d, err = loadObject(p); err != nil {
			return err // a syntax error is not ours to overwrite
		}
	}
	env, err := envOf(d)
	if err != nil {
		return err
	}
	if name == "off" {
		env.Delete(envToken)
		env.Delete(envName)
	} else {
		env.Set(envToken, tok) // "" when name == main
		env.Set(envName, name)
	}
	if env.Len() > 0 {
		d.Set("env", env)
	} else {
		d.Delete("env")
	}
	if d.Len() == 0 {
		if fsx.IsLink(p) || fsx.Exists(p) {
			return os.Remove(p)
		}
		return nil
	}
	// A symlink here means this directory shares its pin with another one:
	// write the file it points to. Replacing the link itself would silently
	// cut a connection the user believes is still there.
	target := p
	if fsx.IsLink(p) {
		target = fsx.RealPath(p)
	}
	return fsx.WritePrivate(target, ".tmp", append(pyjson.Dump(d), '\n'))
}

// gitPinOK is false when the pin file is tracked by git — writing a token
// there is one step from committing it. Callers decide: a single pin dies, a
// batch skips that directory and carries on.
func gitPinOK(dir string) bool {
	if !gitx.IsRepo(dir) {
		return true
	}
	return !gitx.Tracked(dir, pinRel)
}

// guardGit keeps the pin file out of git without touching the repo's
// .gitignore (a company repo is not ours to edit): it goes into
// .git/info/exclude, which exists on this machine only.
func guardGit(dir string) {
	if !gitx.IsRepo(dir) {
		return
	}
	if !gitPinOK(dir) {
		ui.Die(i18n.Tp("err_git_track", pinRel, dir, pinRel))
	}
	if gitx.Ignored(dir, pinRel) {
		return
	}
	common := gitx.CommonDir(dir)
	if common == "" || !fsx.IsDir(common) {
		return
	}
	info := filepath.Join(common, "info")
	os.MkdirAll(info, 0o755)
	excl := filepath.Join(info, "exclude")
	cur, _ := os.ReadFile(excl)
	for _, ln := range strings.Split(strings.ReplaceAll(string(cur), "\r\n", "\n"), "\n") {
		if ln == pinRel {
			return
		}
	}
	add := pinRel + "\n"
	if len(cur) > 0 && cur[len(cur)-1] != '\n' {
		add = "\n" + add
	}
	f, err := os.OpenFile(excl, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	f.WriteString(add)
	f.Close()
	ui.Info(ui.Dim + i18n.Tp("git_excluded", pinRel) + ui.Reset)
}

// errAmbig carries the "several workspaces" message to callers that die on it.
type errAmbig struct{ names string }

func (e errAmbig) Error() string { return i18n.Tp("err_ambig_ws", e.names) }

// resolveTarget is the directory a pin for base has to land in: folder [0]
// of its workspace (with a note naming the workspace), else the git root,
// else base itself.
func resolveTarget(base string) (dir, note string, err error) {
	kind, root, n := workspace.GuiRoot(base)
	if kind == workspace.Ambig {
		return "", "", errAmbig{n}
	}
	if kind == workspace.WS && root != "" {
		return root, n, nil
	}
	if top, ok := gitx.Toplevel(base); ok {
		// git's spelling of the path (forward slashes, an 8.3 short name or
		// another letter case on Windows) is normalised like every other path
		// claude-acc compares.
		return fsx.RealPath(top), "", nil
	}
	return fsx.RealPath(base), "", nil
}

// onlyPin reports whether a settings file holds nothing but a claude-acc pin
// (no permissions, hooks, MCP…), so replacing it loses nothing.
func onlyPin(path string) bool {
	d, err := loadObject(path)
	if err != nil {
		return false
	}
	for _, k := range d.Keys() {
		if k != "env" {
			return false
		}
	}
	env, err := envOf(d)
	if err != nil {
		return false
	}
	for _, k := range env.Keys() {
		if k != envToken && k != envName {
			return false
		}
	}
	return true
}

// syncWsMembers moves the other folders of the workspace that already hold
// a pin of their own (a real file) to the new account. It creates no new
// pins — it only keeps existing ones from drifting.
func syncWsMembers(root, name, tok string) {
	n := 0
	for _, f := range workspace.Folders(root) {
		if f == "" || f == root {
			continue
		}
		tf := repoFile(f)
		if !fsx.IsFile(tf) || fsx.IsLink(tf) {
			continue
		}
		if repoPin(f) == "" {
			continue
		}
		if !gitPinOK(f) {
			ui.Info(ui.Yellow + i18n.Tp("git_skip", f) + ui.Reset)
			continue
		}
		writeRepoPin(f, name, tok)
		regSet(f, name)
		n++
	}
	if n > 0 {
		ui.Info(ui.Dim + i18n.Tp("share_merged_n", strconv.Itoa(n)) + ui.Reset)
	}
}

// shareWorkspace makes every other folder of the workspace use folder [0]'s
// pin. Measured: Claude Code reads through symlinks, so on macOS one file is
// linked everywhere — the token lives in one place and one switch moves the
// whole workspace. A folder whose file holds the user's own settings is never
// replaced: the pin is merged into it and registered, so later switches
// still reach it. Where symlinks are not an option (Windows), every folder
// gets its own registered copy instead.
func shareWorkspace(root, accName, accTok string) {
	src := repoFile(root)
	if !fsx.IsFile(src) {
		return
	}
	n, skipped, merged, copied := 0, 0, 0, 0
	for _, f := range workspace.Folders(root) {
		if f == "" || f == root {
			continue
		}
		target := repoFile(f)
		if fsx.IsLink(target) {
			if fsx.RealPath(target) == fsx.RealPath(src) {
				continue
			}
		} else if fsx.Exists(target) {
			if onlyPin(target) && useSymlinks {
				os.Remove(target)
			} else {
				if !gitPinOK(f) {
					ui.Info(ui.Yellow + i18n.Tp("git_skip", f) + ui.Reset)
					skipped++
					continue
				}
				writeRepoPin(f, accName, accTok)
				regSet(f, accName)
				if onlyPin(target) {
					copied++
				} else {
					ui.Info(ui.Dim + i18n.Tp("share_merged", f) + ui.Reset)
					merged++
				}
				continue
			}
		}
		if !gitPinOK(f) {
			ui.Info(ui.Yellow + i18n.Tp("git_skip", f) + ui.Reset)
			skipped++
			continue
		}
		guardGit(f)
		os.MkdirAll(filepath.Join(f, ".claude"), 0o755)
		if useSymlinks {
			os.Remove(target)
			if os.Symlink(src, target) == nil {
				n++
			}
			continue
		}
		writeRepoPin(f, accName, accTok)
		regSet(f, accName)
		copied++
	}
	if n > 0 {
		ui.Info(ui.Green + i18n.Tp("share_done", strconv.Itoa(n)) + ui.Reset)
	}
	if copied > 0 {
		ui.Info(ui.Green + i18n.Tp("share_copied", strconv.Itoa(copied)) + ui.Reset)
	}
	if merged > 0 {
		ui.Info(ui.Dim + i18n.Tp("share_merged_n", strconv.Itoa(merged)) + ui.Reset)
	}
	if skipped > 0 {
		ui.Info(ui.Dim + i18n.Tp("share_skip_n", strconv.Itoa(skipped)) + ui.Reset)
	}
}
