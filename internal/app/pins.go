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
//
// Measured 2026-10-29 (fake API logging the bearer, then the real API):
//   - Claude Code 2.1.312 ignores CLAUDE_CODE_OAUTH_TOKEN in repo-level
//     settings; 2.1.300 did not. The same token as ANTHROPIC_AUTH_TOKEN there
//     is used by both, beats the global CLAUDE_CODE_OAUTH_TOKEN, is re-read
//     mid-session like the global one, and the API bills it to the
//     account's plan exactly as before (same unified 5h/7d headers).
//   - Nothing at repo level makes 2.1.312 drop the global token, so a main
//     pin (empty token) only works while the global account is main.
//
// A token pin therefore carries both keys: CLAUDE_CODE_OAUTH_TOKEN for 1.x
// and older Claude Code, ANTHROPIC_AUTH_TOKEN for 2.1.312 on.

const (
	envToken = "CLAUDE_CODE_OAUTH_TOKEN"
	envAuth  = "ANTHROPIC_AUTH_TOKEN"
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
	if err := writeRepoPinErr(dir, name, tok); err == errForeignAuth {
		ui.Die(i18n.Tp("err_foreign_auth", repoFile(dir)))
	} else if err != nil {
		ui.Die(i18n.Tp("err_write_repo", repoFile(dir)))
	}
}

// errForeignAuth: the directory sets an ANTHROPIC_AUTH_TOKEN of its own,
// which a pin would have to replace — and Claude Code would send it in
// place of the pinned token if it stayed.
var errForeignAuth = errors.New("the directory has its own ANTHROPIC_AUTH_TOKEN")

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
	// ANTHROPIC_AUTH_TOKEN is the directory's own (a gateway key, say) unless
	// it repeats the pinned token or is the empty value left below.
	auth := env.String(envAuth)
	ours := auth == "" || auth == env.String(envToken)
	switch name {
	case "off":
		env.Delete(envToken)
		env.Delete(envName)
		// A running claude keeps a token whose key is removed (measured on
		// 2.1.300 and 2.1.312); an empty one makes it let go at once. Once
		// empty, the key can go.
		if ours && auth != "" {
			env.Set(envAuth, "")
		} else if ours {
			env.Delete(envAuth)
		}
	case "main":
		env.Set(envToken, "") // falls back to the stored credential
		env.Set(envName, name)
		if ours {
			env.Set(envAuth, "")
		}
	default:
		if !ours {
			return errForeignAuth
		}
		env.Set(envToken, tok)
		env.Set(envName, name)
		env.Set(envAuth, tok)
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

// target is where a pin for a directory has to land.
type target struct {
	dir string
	// note names the workspace when dir is its folder [0].
	note string
	// others are workspaces that also list the directory; their chat tabs
	// run in their own folder [0], out of this pin's reach.
	others []string
}

// resolveTarget is the directory a pin for base has to land in: folder [0]
// of its workspace (with a note naming the workspace), else the git root,
// else base itself.
func resolveTarget(base string) (target, error) {
	a := workspace.GuiRoot(base)
	if a.Kind == workspace.Ambig {
		return target{}, errAmbig{a.Note}
	}
	if a.Kind == workspace.WS && a.Root != "" {
		return target{a.Root, a.Note, a.Others}, nil
	}
	if top, ok := gitx.Toplevel(fsx.RealPath(base)); ok {
		return target{dir: top}, nil
	}
	return target{dir: fsx.RealPath(base)}, nil
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
		if k != envToken && k != envName && k != envAuth {
			return false
		}
	}
	auth := env.String(envAuth)
	return auth == "" || auth == env.String(envToken)
}

// pinOutdated: a token pin written before ANTHROPIC_AUTH_TOKEN, which
// Claude Code 2.1.312 ignores until `sync` rewrites it.
func pinOutdated(dir string) bool {
	p := repoPin(dir)
	if p == "" || p == "main" {
		return false
	}
	d, err := loadObject(repoFile(dir))
	if err != nil {
		return false
	}
	env, err := envOf(d)
	if err != nil {
		return false
	}
	return env.String(envAuth) == "" || env.String(envAuth) != env.String(envToken)
}

// homeElsewhere reports whether folder f of root's workspace is folder [0]
// of a workspace of its own, and names that workspace. Its pin belongs to
// that workspace: moving it along with root's would switch a workspace the
// user never touched.
func homeElsewhere(f, root string) (string, bool) {
	a := workspace.GuiRoot(f)
	if a.Kind == workspace.WS && a.Root != root {
		return a.File, true
	}
	return "", false
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
		if ws, ok := homeElsewhere(f, root); ok {
			ui.Info(ui.Dim + i18n.Tp("home_skip", f, ws) + ui.Reset)
			continue
		}
		if !gitPinOK(f) {
			ui.Info(ui.Yellow + i18n.Tp("git_skip", f) + ui.Reset)
			continue
		}
		if !writeMemberPin(f, name, tok) {
			continue
		}
		regSet(f, name)
		n++
	}
	if n > 0 {
		ui.Info(ui.Dim + i18n.Tp("share_merged_n", strconv.Itoa(n)) + ui.Reset)
	}
}

// writeMemberPin pins one more folder of a workspace. A folder setting its
// own ANTHROPIC_AUTH_TOKEN is left out, and said so, rather than stopping
// the rest.
func writeMemberPin(f, name, tok string) bool {
	if err := writeRepoPinErr(f, name, tok); err == errForeignAuth {
		ui.Info(ui.Yellow + i18n.Tp("err_foreign_auth", repoFile(f)) + ui.Reset)
		return false
	} else if err != nil {
		ui.Die(i18n.Tp("err_write_repo", repoFile(f)))
	}
	return true
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
		if ws, ok := homeElsewhere(f, root); ok {
			ui.Info(ui.Dim + i18n.Tp("home_skip", f, ws) + ui.Reset)
			skipped++
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
				if !writeMemberPin(f, accName, accTok) {
					skipped++
					continue
				}
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
