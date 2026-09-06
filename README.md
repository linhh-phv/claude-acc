# claude-acc

Use multiple [Claude Code](https://claude.com/claude-code) accounts on one Mac — switch between them (terminal *and* Cursor/VS Code GUI) without ever logging in or entering an OTP again.

## The problem

macOS Claude Code stores exactly one credential in the system Keychain. Logging into account B overwrites account A. Come back to account A the next day and it's gone — back to the browser, back to the OTP. There's no built-in "switch account" command.

## How this fixes it

One account ("`main`") stays logged in normally, in the Keychain — it's the only one with full features (claude.ai connectors, Remote Control, `/schedule`). Every other account gets a **long-lived OAuth token** (`claude setup-token`, valid ~1 year) stored under its own name. Because `claude setup-token` never touches the Keychain, adding or switching accounts never logs `main` out.

`claude-acc switch <name>` writes the active token into the `env` block of `~/.claude/settings.json` — the config file the CLI and the Cursor/VS Code extension both read — so one command changes the account everywhere. It takes effect immediately: no window reload, no restart, not even a new chat tab.

## Requirements

- macOS (uses the `security` CLI for Keychain access)
- [Claude Code](https://claude.com/claude-code) installed and in `PATH`
- `python3` (ships with macOS)

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/linhh-phv/claude-acc/main/install.sh | bash
```

Or manually:

```bash
git clone https://github.com/linhh-phv/claude-acc.git
cp claude-acc/bin/claude-acc ~/.local/bin/
chmod +x ~/.local/bin/claude-acc
```

Make sure `~/.local/bin` is on your `PATH`.

## Quick start

```bash
# Log into your main account the normal way, once:
claude auth login

# Add a second account. Browse to it in whichever browser/profile/private
# window is already signed into that account:
claude-acc add work --browser "Safari"

# Switch — this is the one command you need day to day:
claude-acc switch work

# Use claude normally — terminal and Cursor both see "work" now:
claude
```

```bash
claude-acc switch main   # back to your main account
claude-acc ls            # see what's configured and which account is active
```

Want this one repo on a different account than everything else? Same command, plus
`--here`:

```bash
cd ~/code/some-repo
claude-acc switch personal --here   # only this repo; the global account is untouched
```

### The whole thing in six lines

| You want | Run |
|---|---|
| Switch account everywhere | `claude-acc switch <name>` |
| …but leave this repo alone | pin the repo first: `claude-acc switch <name> --here` |
| This repo on its own account | `cd <repo> && claude-acc switch <name> --here` |
| This repo back on the main account | `claude-acc switch main --here` |
| Stop pinning this repo | `claude-acc switch off --here` |
| Who is being used, and where? | `claude-acc ls` |
| Read everything in Vietnamese | `claude-acc lang vi` |
| See how much each account has left (and start its 5h window) | `claude-acc usage` |

Everything takes effect on the next request — no reload, no restart, no new chat tab.

### `switch` takes effect immediately

No reload, no restart. Claude Code re-reads `~/.claude/settings.json` on *every* request
rather than once at startup, so the next thing you send — in the terminal or in an
already-open Cursor chat — goes out under the new account.

Measured on extension 2.1.260 and on 2.1.198: swap the token mid-session in a running
`stream-json` session and the very next turn fails with `401 OAuth access token is invalid`.

"Immediately" includes prompts that are already mid-flight. A prompt is many requests
(call a tool, send the result back, call another), and each one re-reads the file.
Measured: with a prompt already running and its first tool call done, switching at
t+9s made the *next* step of that same prompt use the new account at t+12s. So:

- The one API call already in flight finishes on the old account — it is on Anthropic's
  servers, nothing local can recall it.
- Every step after that uses the new account. A healthy account and the prompt carries
  straight on; a dead token and that running prompt stops there with a 401.

That reach is the point of `--here`: a pinned repo is not moved by a global `switch`,
so long-running work there keeps its own account.

## One account per repo

`switch` is global. If you'd rather spread the load — this repo on one account, that
one on another, so no single account eats the whole limit — pin an account to a
directory instead:

```bash
cd ~/work/some-repo
claude-acc switch work --here   # this repo uses "work"
claude-acc switch main --here   # this repo uses main (the Keychain account)
claude-acc switch off --here    # unpin; back to whatever plain `switch` says
```

Same command you already use, plus `--here`. (`claude-acc here work` is the shorter
spelling if you prefer it.) Run a plain `switch` while standing in a pinned directory
and it will tell you that this spot keeps its own account and won't move.

It writes the `env` block into that directory's `.claude/settings.local.json`, which
overrides the global file. Existing contents are merged, not replaced — your
`permissions` and MCP entries survive. Like `switch`, it takes effect on the next
request, in chat tabs that are already open.

When one account gets close to its limit, move everything on it at once:

```bash
claude-acc reassign work other-account   # every directory pinned to "work"
claude-acc sync                          # rewrite tokens everywhere (after re-adding an account)
claude-acc ls                            # who's pinned where, and what's in effect here
```

### Making a whole workspace share one pin

In a multi-root workspace only folder `[0]` matters to the GUI, but a terminal opened in
another folder ignores that pin. If you want the whole workspace to behave as one unit:

```bash
claude-acc switch work --here --all
```

Folder `[0]` gets the real file; every other folder gets a **symlink** to it. Measured:
Claude Code reads through symlinks — put a bad token in the real file and a run from a
folder that only has the link still 401s. So the token lives in exactly one place, and
changing the account once moves the whole workspace:

```bash
claude-acc switch other --here      # no --all needed; the links follow
```

What happens to each folder depends on what it already holds:

| Folder holds | Result |
|---|---|
| nothing | symlink to folder `[0]` |
| only an old pin | symlink (nothing is lost) |
| its own `permissions` / `hooks` / MCP entries | keeps its file, gets the pin written **into** it — and is still updated on every later `switch --here` |

So a folder with real configuration is never overwritten, and never left behind on a
stale account either.

### Two things that will bite you if you don't know them

Both measured on this machine, not inferred from docs.

**Repo settings only apply in the exact directory `claude` runs in.** They do not
cascade into subdirectories. Put a bad token in a repo's `.claude/settings.json`, run
from the repo root and you get a 401; run the same thing from `apps/web` and it sails
straight past into the global account. So `cd apps/web && claude` silently ignores the
pin — which matters most when the global account is your work one. `claude-acc ls`
prints what is actually in effect where you're standing.

**In a multi-root workspace, every chat tab runs in folder `[0]`.** Measured by reading
the cwd of the real processes: three chat sessions of a six-folder workspace all ran in
the first folder of its `.code-workspace`, even though one tab was working on a
different worktree. So a pin applies to the whole workspace, not to one worktree — and
`here` aims at folder `[0]` for you rather than at whatever directory your terminal
happens to be in. It tells you when it does that.

### About the token in your repo

The pin has to contain the real token: the indirection that would avoid it,
`apiKeyHelper`, hangs the CLI outright (tried both an OAuth token and an API key), so
it is not an option. `claude-acc` therefore:

- writes the file `600`, atomically;
- **refuses** to write if `.claude/settings.local.json` is tracked by git, and tells you
  to `git rm --cached` it first;
- adds the path to `.git/info/exclude` — not `.gitignore`, so it never touches a file
  your repo (or your team) has committed;
- cleans the token out of every pinned directory on `claude-acc uninstall`.

`main` is the exception: pinning `main` writes an empty token, which overrides the
global one and falls back to the Keychain. No secret is written at all.

## Seeing how much each account has left

Claude Code has no CLI command for this — `/usage` only exists inside a session — but the
API reports it in response headers, so `claude-acc` asks for it directly:

```bash
claude-acc usage                 # every account, main included
claude-acc usage work personal   # only these two
```

It also doubles as the way to **start the 5-hour windows**. That window only begins
counting from an account's first request, so an account you have not touched is not
running its clock — and asking is itself a request.

```
  account    5-hour window                  7-day window
 * work      #.........   9%  13:40 (3h32 left)   3%  11/09 (6 days left)
 * personal  ..........   2%  11:00 (52m left)    0%  11/09 (6 days left)
 * main      #####.....  50%  13:30 (3h22 left)  19%  05/09 (9h52 left)

Most headroom: personal (5h only 2% used)
  -> use it for the repo you are in:  claude-acc switch personal --here
```

It reads `anthropic-ratelimit-unified-*` off a one-token request, so the numbers are the
real thing rather than a local guess — including when an account is already out, because
a 429 carries those headers too. `main` is covered as well: its access token lives in
Claude Code's own Keychain entry, not in `claude-acc`'s, and is read from there.

**Asking costs a request, so `usage` also starts the 5-hour window** — exactly like
`trigger`. Name the accounts you want if some should stay off the clock.

### What the token can and can't reach

A `setup-token` credential is scoped to model calls. Measured:

| | |
|---|---|
| Usage: 5h / 7d utilization, reset time, allowed-or-out | ✅ response headers |
| Whether overage is permitted | ✅ header |
| The account's email or name | ❌ `403 OAuth token does not meet scope requirement` |
| `/v1/me` | ❌ `404` |
| Sessions / conversations | ❌ not server-side — they're local files under `~/.claude/projects`, shared across accounts |
| `claude auth status --json` | Only `loggedIn`, `authMethod`, `apiProvider`, `analyticsDisabled`, `projectsDirectory` |

That's why `claude-acc ls` can show an email for `main` but not for token accounts.

## Commands

| Command | What it does |
|---|---|
| `add <name> [--browser <app>]` | Add a secondary account. Runs `claude setup-token`, captures the token automatically (no copy/paste), stores it in the Keychain. |
| `switch <name\|main>` | **The command you'll use most.** Makes `<name>` (or `main`) the active account for both the CLI and the GUI extension. |
| `switch <name\|main\|off> --here` | Pin an account to the current repo/workspace, independent of the global one. `--at <dir>` targets another directory, `--no-check` skips the verifying API call. Also spelled `here <name>`. |
| `reassign <old> <new>` | Move every pinned directory from one account to another — for when `<old>` is near its limit. |
| `sync` | Rewrite the token in every pinned directory (after re-adding an account), and drop directories that no longer exist. |
| `usage [name...]` | Show 5h / 7d utilization and reset times per account, and suggest the one with most headroom. Costs one request per account (9 tokens), which also starts that account's 5-hour window. Also spelled `quota`, `trigger`, `warm`. |
| `lang [vi\|en\|auto]` | Show or change the language of every message, `help` included. Defaults to the system `LANG`, falling back to English. |
| `ls` | List configured accounts, which one is active globally, which is in effect where you're standing, and every directory pin. |
| `check <name\|main>` | Verify an account's credential still works, with a real API call — not just a config read. |
| `login [--browser <app>]` | Re-run `claude auth login` for `main` (this overwrites the Keychain — `main` only). |
| `rm <name>` | Remove a secondary account's token. |
| `uninstall` | Remove every token and config `claude-acc` created. |
| `run <name\|main> -- [args]` | Advanced: run one `claude` invocation under a given account without changing what's pinned. Refuses to run if a different account is currently pinned via `switch`, to avoid silently using the wrong one. |
| `env <name\|main>` | Advanced: print an `export` line to change the current shell only, e.g. `eval "$(claude-acc env work)"`. |

## Language

Every message, including `help`, is available in English and Vietnamese:

```bash
claude-acc lang          # what it is now, and where that came from
claude-acc lang vi       # switch to Vietnamese
claude-acc lang en       # switch to English
claude-acc lang auto     # follow the system LANG again
```

Without a stored choice it follows `LANG`/`LC_ALL`, falling back to English. The choice
lives in `~/.config/claude-acc/lang`.

Run `claude-acc help` any time for the full reference in whichever language is active.

## `add` options

| Flag | Use it when |
|---|---|
| `--browser "Safari"` / `"Google Chrome"` / `"Brave Browser"` | The account you're adding is signed in on a specific browser, not your default one. |
| `--url-only` | You'd rather copy the auth URL yourself (e.g. into a private/incognito window) than let it open an app. |
| `--clip` | You already have a token in your clipboard and just want to store it, no prompts. |
| `--stdin` | Pipe a token in: `pbpaste \| claude-acc add work --stdin`. |
| `--no-setup` | Skip running `claude setup-token` entirely — pair with `--clip`/`--stdin` when you already ran it yourself and just want to register the result. |

### What to expect when it runs

`claude setup-token` completes one of two ways, and which one it picks isn't something `claude-acc` controls:

- It redirects straight back to your terminal and finishes on its own, or
- After you authorize in the browser, it shows a short **code** on that page and the terminal waits at `Paste code here if prompted >` for you to bring it back. This is a normal fallback built into Claude Code itself, not a `claude-acc` bug or a stuck terminal — type or paste the code and press Enter.

Either way, once `claude setup-token` prints the final `sk-ant-oat01-…` token, `claude-acc` grabs it automatically (it runs the command through `script(1)`, which gives `claude` a real terminal to render into while still letting `claude-acc` read back what it printed) — no copy/paste needed for that last step. If capture ever fails for some reason, it falls back to asking you to paste that token manually, so `add` can never leave you with no way to finish.

## Why the account's token ends up as plaintext in a JSON file

Cursor and VS Code don't read the Keychain per-account — they read whatever credential `~/.claude/settings.json` points to, same as the CLI. To make `switch` work for the GUI too, the active secondary account's token has to live somewhere both surfaces read, which means that file. `claude-acc` sets it to mode `600` (owner-only) immediately, and does the same for the backup copy it keeps before every switch — so a stray backup from before you upgraded can't stay world-readable if the original file ever had looser permissions.

Your `main` account never has this trade-off: it stays exclusively in the encrypted Keychain, so keep your most-used or most-sensitive account as `main`.

## Limitations

- **macOS only.** It shells out to `security`, `pbcopy`/`pbpaste`, and `open -a`.
- **Only `main` gets claude.ai connectors, Remote Control, and `/schedule`.** Secondary accounts authenticate with a model-only OAuth token.
- **Session/conversation history is shared** across accounts (`CLAUDE_CONFIG_DIR` isn't split per account), so `-r`/`--resume` under one account can see another's sessions.
- **A directory pin only works in that exact directory** (see above), and in a
  multi-root workspace the directory that counts is folder `[0]`, not the worktree you
  are looking at.
- **Tokens expire after about a year.** `claude-acc check <name>` tells you when one has. Run `claude-acc add <name>` again to mint a fresh one — it overwrites the Keychain entry and pushes the new token into every place already pinned to that account, the global pin included.
- **A token is briefly visible via `ps` while `claude-acc add` writes it to the Keychain.** `security add-generic-password -w <password>` requires the password as a command-line argument — its own man page's only non-interactive option — so for the moment that one command runs, the token is technically readable by another local user running `ps -ef`, or by exec-argv-logging security/EDR software on managed machines. This is a limitation of the macOS `security` CLI itself, not something `claude-acc` can close without reimplementing Keychain writes against the native Keychain Services API. Everywhere else (settings.json writes, subprocess env passing) `claude-acc` avoids putting the token in argv.

## License

MIT — see [LICENSE](LICENSE).
