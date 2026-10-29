# claude-acc

Use multiple [Claude Code](https://claude.com/claude-code) accounts on one machine — macOS or Windows — and switch between them (terminal *and* Cursor/VS Code GUI) without ever logging in or entering an OTP again.

## The problem

Claude Code stores exactly one credential — in the Keychain on macOS, in `%USERPROFILE%\.claude\.credentials.json` on Windows. Logging into account B overwrites account A. Come back to account A the next day and it's gone — back to the browser, back to the OTP. There's no built-in "switch account" command.

## How this fixes it

One account ("`main`") stays logged in normally — it's the only one with full features (claude.ai connectors, Remote Control, `/schedule`). Every other account gets a **long-lived OAuth token** (`claude setup-token`, valid ~1 year) stored under its own name in the OS credential store (the Keychain on macOS, Credential Manager on Windows). Because `claude setup-token` never touches main's credential, adding or switching accounts never logs `main` out.

`claude-acc switch <name>` writes the active token into the `env` block of `~/.claude/settings.json` — the config file the CLI and the Cursor/VS Code extension both read — so one command changes the account everywhere. It takes effect immediately: no window reload, no restart, not even a new chat tab.

## Requirements

- macOS, or Windows 10/11 (the Windows build is new — see [Windows](#windows))
- [Claude Code](https://claude.com/claude-code) installed and in `PATH`

`claude-acc` is a single self-contained binary; nothing else is needed (1.x needed
`python3`, 2.x does not).

## Install

macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/linhh-phv/claude-acc/main/install.sh | bash
```

Windows (PowerShell) — or see [Windows: step by step](#windows-step-by-step):

```powershell
irm https://raw.githubusercontent.com/linhh-phv/claude-acc/main/install.ps1 | iex
```

The one-liners download the binary for your CPU from the latest
[release](https://github.com/linhh-phv/claude-acc/releases) into `~/.local/bin`
(`%USERPROFILE%\.local\bin` on Windows — where the Claude Code installer puts `claude.exe`).
Make sure that folder is on your `PATH`; `install.ps1` adds it for you.

From source (needs [Go](https://go.dev/dl/)):

```bash
git clone https://github.com/linhh-phv/claude-acc.git
cd claude-acc && ./install.sh          # builds and installs ~/.local/bin/claude-acc
```

On Windows with a binary you were handed: `.\install.ps1 -From .\claude-acc-windows-amd64.exe`.

### Windows: step by step

*Tiếng Việt: [docs/guides/windows-vi.md](docs/guides/windows-vi.md).*

You need Windows 10 or 11 and Claude Code installed **and signed in** (`claude` runs in
PowerShell). That signed-in account is **main**.

**1. Install (once).** Open PowerShell (Start menu → type `PowerShell` → Enter) and paste:

```powershell
irm https://raw.githubusercontent.com/linhh-phv/claude-acc/main/install.ps1 | iex
```

Then open a **new** PowerShell window and check:

```powershell
claude-acc version        # claude-acc 2.0.2
```

Rather not paste a command? Download `claude-acc-windows-amd64.zip` (`…-arm64.zip` on an ARM
PC) from the [latest release](https://github.com/linhh-phv/claude-acc/releases/latest), unzip
it and double-click **`install.cmd`**. If Windows says *"Windows protected your PC"*, click
**More info → Run anyway**.

**2. Add an account (once per account).**

```powershell
claude-acc add work
```

1. The browser opens. Sign in to **the account you are adding**. If the browser is signed in
   to a different one, run `claude-acc add work --url-only` instead and paste the copied link
   into a private window (Ctrl+Shift+N).
2. Enter the one-time code sent to that account's email, then click **Authorize**.
3. The terminal prints a token — a long string starting with `sk-ant-oat01-`.
   **Select all of it and press Ctrl+C.**
4. When claude-acc asks for the token, **just press Enter** (it reads the clipboard).
5. `✓ "work" is ready.` — done.

**3. Every day.**

| You want | Run |
|---|---|
| Use `work` everywhere (terminal and Cursor/VS Code) | `claude-acc switch work` |
| Back to your main account | `claude-acc switch main` |
| Only the repo you are in on `work` | `cd <repo>` then `claude-acc switch work --here` |
| Drop that repo's own account | `claude-acc switch off --here` |
| Which account is in effect, and where | `claude-acc ls` |
| How much each account has left | `claude-acc usage` |
| Messages in Vietnamese | `claude-acc lang vi` |

A switch takes effect on the next request — no Reload Window, no new chat tab.

**Something wrong?** Send a screenshot (or the copied text of the PowerShell window) with the
output of `claude-acc version` and `claude --version` to whoever gave you claude-acc.

### Upgrading from 1.x (the bash version)

Install 2.x over it and keep going — **no account needs to be added again**. 2.x reads
everything 1.x set up, in the same places and formats:

- tokens in the Keychain (service `claude-code-token`), read through the same `security`
  command, so macOS does not ask for Keychain access again;
- `~/.config/claude-acc/` — the account list, the registry of pinned directories, the
  language choice, the last `usage` reading;
- the global pin in `~/.claude/settings.json` and every directory pin
  (`.claude/settings.local.json`), workspace symlinks included.

It also writes those files byte for byte as 1.x did, so going back to 1.x works too. Both
directions are covered by a test (`TestUpgradeKeepsAccounts`) that sets accounts and pins
up with 1.x, uses them with 2.x, then reads them back with 1.x. The one addition: pins also
hold `ANTHROPIC_AUTH_TOKEN`, which Claude Code 2.1.312 needs — the token for a token
account, empty for `main` and after `off` ([why](#why-a-pin-holds-the-token-twice)). 1.x
ignores that key.
Pins made before have only the old key — run `claude-acc sync` once after upgrading;
until then `ls` says how many are left, and `ls` and `usage` flag the one where you stand.

## Quick start

```bash
# Log into your main account once (this is `claude auth login`, but you can
# see what you paste, and a code you copy from the browser is filled in):
claude-acc login

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
cd some-repo
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
| How much each account has left | `claude-acc ls` (from the last check) |
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
cd some-repo
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

### Why a pin holds the token twice

Claude Code 2.1.312 stopped reading `CLAUDE_CODE_OAUTH_TOKEN` from repo settings: a pin
holding only that key is silently dropped, and the chat runs on the global account.
Measured on 2026-10-29 with a fake API logging which token arrives, then against the real
API, on 2.1.300 and 2.1.312:

| In `.claude/settings.local.json` | 2.1.300 | 2.1.312 |
|---|---|---|
| `CLAUDE_CODE_OAUTH_TOKEN` | used | **ignored** |
| `ANTHROPIC_AUTH_TOKEN`, same token | used | used |

`ANTHROPIC_AUTH_TOKEN` beats the global token, is re-read mid-session (change it between
two turns and the second goes out on the new one — about a second after the file
changes), and the API bills it to the account's plan exactly as before: the same
`anthropic-ratelimit-unified-5h/7d` headers come back. So a token pin carries both keys,
with the same token. One thing differs: with no `model` set, Claude Code picks a different
default model under `ANTHROPIC_AUTH_TOKEN` (measured: Opus instead of Sonnet). Set `model`
if that matters to you.

Two details follow from it:

- **`off` and `main` leave `"ANTHROPIC_AUTH_TOKEN": ""` behind.** Measured: a running
  session keeps a token whose key is *removed* from the file, but drops one that is
  *emptied* — so emptying is what moves an open chat tab off the token account at once.
  Running `off` once more removes the empty key, and the file with it when nothing else is
  left. In a workspace that shares one pin through links, `off` in one folder removes that
  folder's link only.
- **A repo that sets an `ANTHROPIC_AUTH_TOKEN` of its own** (a gateway key, say) is not
  pinned: the pin would have to replace it. `here` says so; `--all` skips that folder and
  pins the others; `off` and `main` leave such a key alone.

**What 2.1.312 cannot do: keep a directory on `main` while the global account is a token
one.** A `main` pin is an empty token that hides the global one, and no repo-level setting
hides it from 2.1.312. So with a token account as the global one, a directory pinned to
`main` runs on that token account; `here`, `ls`, `usage` and `switch` say so. The way that
works on every version: make `main` the global account, and pin token accounts to the repos
that should not use it.

```bash
claude-acc switch main                       # main everywhere…
cd other-repo && claude-acc switch work --here   # …except where you pin a token account
claude-acc switch off --here                 # that repo back on main
```

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

A directory can sit in more than one workspace — typically a repo that is folder `[0]` of
its own `.code-workspace` and an extra folder of a bigger one. Since only folder `[0]`
counts, `here` goes by that: if the directory is (inside) folder `[0]` of exactly one of
them, that workspace wins. The others merely list it — their chat tabs run in their own
folder `[0]`, which a pin here never reaches — and `here` names them so you know. The
other way round, pinning the bigger workspace (with or without `--all`) leaves that
directory's pin alone: it belongs to its own workspace. If the directory is only an extra
folder of workspaces with different folders `[0]`, there is nothing to go by, and `here`
asks you to name the target with `--at`.

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
global one and falls back to the Keychain — on Claude Code 2.1.312 only while `main` is
the global account ([why](#why-a-pin-holds-the-token-twice)). No secret is written at all.

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

HERE      work  (pin of …/some-repo)

Most headroom: personal (5h only 2% used)
  -> use it for the repo you are in:  claude-acc switch personal --here
```

The `HERE` row is the one `ls` prints: the account in effect where you stand, so you can
read its quota off the table without a second command. In a folder of a multi-root
workspace that is not folder `[0]`, a `GUI` row above it names the account the chat tabs
use, as in `ls`.

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
| `add <name> [--browser <app>]` | Add a secondary account. Runs `claude setup-token`, captures the token automatically (no copy/paste; on Windows you copy it once), stores it in the Keychain / Credential Manager. |
| `switch <name\|main>` | **The command you'll use most.** Makes `<name>` (or `main`) the active account for both the CLI and the GUI extension. |
| `switch <name\|main\|off> --here` | Pin an account to the current repo/workspace, independent of the global one. `--at <dir>` targets another directory, `--no-check` skips the verifying API call. Also spelled `here <name>`. |
| `reassign <old> <new>` | Move every pinned directory from one account to another — for when `<old>` is near its limit. |
| `sync` | Rewrite the token in every pinned directory (after re-adding an account), and drop directories that no longer exist. |
| `usage [name...]` | Show 5h / 7d utilization and reset times per account, which account is in effect where you're standing, and suggest the one with most headroom. Costs one request per account (9 tokens), which also starts that account's 5-hour window. Also spelled `quota`, `trigger`, `warm`. |
| `lang [vi\|en\|auto]` | Show or change the language of every message, `help` included. Defaults to the system `LANG`, falling back to English. |
| `ls` | List configured accounts, which one is active globally, which is in effect where you're standing, and every directory pin. Shows each account's last known 5h / 7d usage with the age of that reading — read from cache, so `ls` never spends a request or starts a window. |
| `check <name\|main>` | Verify an account's credential still works, with a real API call — not just a config read. |
| `login [--browser <app>]` | Sign `main` in through `claude auth login` (this overwrites main's credential — `main` only). Unlike running that bare, you can see the code you paste, and a code you copy is filled in for you — see [below](#login-a-code-prompt-you-can-see). |
| `rm <name>` | Remove a secondary account's token. |
| `uninstall` | Remove every token and config `claude-acc` created. |
| `run <name\|main> -- [args]` | Advanced: run one `claude` invocation under a given account without changing what's pinned. Refuses to run if a different account is currently pinned via `switch`, to avoid silently using the wrong one. |
| `env <name\|main>` | Advanced: print a line that changes the current shell only — `eval "$(claude-acc env work)"` on macOS, `claude-acc env work \| Invoke-Expression` in PowerShell. |

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
| `--browser "Safari"` / `"Google Chrome"` / `"Brave Browser"` (Windows: `msedge` / `chrome` / `firefox` / `brave`) | The account you're adding is signed in on a specific browser, not your default one. |
| `--url-only` | You'd rather copy the auth URL yourself (e.g. into a private/incognito window) than let it open an app. |
| `--clip` | You already have a token in your clipboard and just want to store it, no prompts. |
| `--stdin` | Pipe a token in: `pbpaste \| claude-acc add work --stdin` (PowerShell: `Get-Clipboard \| claude-acc add work --stdin`). |
| `--no-setup` | Skip running `claude setup-token` entirely — pair with `--clip`/`--stdin` when you already ran it yourself and just want to register the result. |

### `add` needs an OTP; `switch` never does

Worth being clear about, because it decides whether an account is practical for you:
**`claude setup-token` asks for a one-time code** sent to that account's owner, every time you
add or renew. Having the account already signed in in that browser is not enough — measured on
two different accounts, both asked.

Everything after that is OTP-free: `switch`, `--here`, `usage`, day-to-day use. The one-time
code is the cost of *minting* a credential, not of using it.

The consequence for a shared account: you need its owner present each time you add it, and again
whenever its token expires (about a year) or gets revoked. If that owner is hard to reach, that
account is a poor thing to pin a repo to.

### What to expect when it runs

`claude setup-token` completes one of two ways, and which one it picks isn't something `claude-acc` controls:

- It redirects straight back to your terminal and finishes on its own, or
- After you authorize in the browser, it shows a short **code** on that page and the terminal waits at `Paste code here if prompted >` for you to bring it back. This is a normal fallback built into Claude Code itself, not a `claude-acc` bug or a stuck terminal — type or paste the code and press Enter.

The capture reads the token across a line break, because `script` records what the terminal
*displayed* — including the newline a narrow window inserts mid-token. Reading only the first
line stored a truncated credential that the API rejected as `invalid`.

Either way, once `claude setup-token` prints the final `sk-ant-oat01-…` token, `claude-acc` grabs it automatically (it runs the command through `script(1)`, which gives `claude` a real terminal to render into while still letting `claude-acc` read back what it printed) — no copy/paste needed for that last step. If capture ever fails for some reason, it falls back to asking you to paste that token manually, so `add` can never leave you with no way to finish.

## `login`: a code prompt you can see

Bare `claude auth login` reads the code at `Paste code here if prompted >` with the terminal's
echo switched off (measured on Claude Code 2.1.260). What you type or paste does reach it, and
Enter submits it — but nothing shows on screen, so it looks like the terminal stopped taking
input.

`claude-acc login` runs the same command, with its input passing through `claude-acc` instead:

- What you type or paste shows as usual, and Enter sends it.
- Click **Copy** on the browser's code page and the code is picked up from the clipboard and
  sent for you. Only a code whose `state` matches this login's URL is used, so an old code left
  on the clipboard is never sent.

Stuck at that prompt in bare `claude auth login`? Paste once and press Enter — the code is
there even though you can't see it.

## Windows

The Windows build does everything the macOS one does, with these differences:

| | macOS | Windows |
|---|---|---|
| Secondary tokens | Keychain | Credential Manager (`claude-code-token:<name>`) |
| `main`'s credential | Keychain item `Claude Code-credentials` | `%USERPROFILE%\.claude\.credentials.json` |
| `add`: reading the token back | automatic | copy it once when `setup-token` prints it, then press Enter |
| `--here --all` | one file, symlinked into every folder | every folder gets its own copy, registered, so later `switch --here` / `reassign` / `sync` keep them in step |
| `env` | `eval "$(claude-acc env work)"` | `claude-acc env work \| Invoke-Expression` |
| claude-acc's own files | `~/.config/claude-acc` | `%LOCALAPPDATA%\claude-acc` |
| A pin file holding a token | mode 600 | an access list of its own: you and SYSTEM only |
| Claude Code installed with npm | — | the `claude.exe` behind npm's `claude.cmd` is run directly, so arguments are never re-parsed by `cmd.exe` |

How Claude Code itself behaves on Windows was read from its Windows build (2.1.296), not
assumed: it opens the login URL through `BROWSER` when set (spawned directly, no shell),
reads the login code line by line from stdin, and keeps main's credential either in
`.credentials.json` or — when its `tengu_windows_credman` flag is on — in Credential Manager
(`Claude Code-credentials/claude-code-user`), deleting the file. claude-acc reads both, in the
same order Claude Code does.

For whoever maintains it: `claude-acc-selftest-windows-amd64.exe` (built by
`./scripts/build.sh`, not shipped in the zip) runs every command on a Windows machine —
interactive ones inside a real Windows console — against a temporary profile and a fake
`claude`, without touching real accounts. Useful when someone reports a problem:
`.\claude-acc-selftest-windows-amd64.exe -bin .\claude-acc-windows-amd64.exe`.

## Why the account's token ends up as plaintext in a JSON file

Cursor and VS Code don't read a per-account Keychain or Credential Manager entry — they read whatever credential `~/.claude/settings.json` points to, same as the CLI. To make `switch` work for the GUI too, the active secondary account's token has to live somewhere both surfaces read, which means that file. `claude-acc` sets it to mode `600` (owner-only) immediately, and does the same for the backup copy it keeps before every switch — so a stray backup from before you upgraded can't stay world-readable if the original file ever had looser permissions.

Your `main` account never has this trade-off: it stays wherever Claude Code itself keeps it, so keep your most-used or most-sensitive account as `main`.

## Limitations

- **macOS and Windows.** No Linux build yet: Linux has no single credential store to rely on. The Windows build is untested on real hardware (see [Windows](#windows)).
- **Only `main` gets claude.ai connectors, Remote Control, and `/schedule`.** Secondary accounts authenticate with a model-only OAuth token.
- **Session/conversation history is shared** across accounts (`CLAUDE_CONFIG_DIR` isn't split per account), so `-r`/`--resume` under one account can see another's sessions.
- **On Claude Code 2.1.312, a directory cannot be pinned to `main` while the global account
  is a token one** — it runs on the token account. Keep `main` as the global account and pin
  token accounts instead ([why](#why-a-pin-holds-the-token-twice)). Which release between
  2.1.301 and 2.1.312 changed this is unknown: only those two were measured.
- **A directory pin only works in that exact directory** (see above), and in a
  multi-root workspace the directory that counts is folder `[0]`, not the worktree you
  are looking at.
- **A token can die before its expiry.** The one-year figure is a ceiling, not a promise:
  a credential can also be **revoked** — by signing out everywhere, a password change, or an org
  admin ending sessions. `usage` and `check` print the API's own words, so revoked and expired
  read differently instead of both saying "token no longer usable".
- **Minting a new token does not revoke the old ones.** Measured, because the docs do not say:
  one account held a Keychain session and two separate `setup-token` credentials alive at the
  same time. So adding the same account on a second machine does not cut off the first, and
  `claude auth login` on an account does not invalidate its `setup-token`s either.
- **Tokens expire after about a year.** `claude-acc check <name>` tells you when one has. Run `claude-acc add <name>` again to mint a fresh one — it overwrites the stored entry and pushes the new token into every place already pinned to that account, the global pin included.
- **On macOS, a token is briefly visible via `ps` while `claude-acc add` writes it to the Keychain.** `security add-generic-password -w <password>` requires the password as a command-line argument — its own man page's only non-interactive option — so for the moment that one command runs, the token is technically readable by another local user running `ps -ef`, or by exec-argv-logging security/EDR software on managed machines. This is a limitation of the macOS `security` CLI itself, not something `claude-acc` can close without reimplementing Keychain writes against the native Keychain Services API. Everywhere else (settings.json writes, subprocess env passing) `claude-acc` avoids putting the token in argv.

## Development

```
cmd/claude-acc/      main
internal/app/        the commands
internal/signin/     setup-token capture, the auth login driver, the BROWSER wrapper
internal/secret/     Keychain (macOS) / Credential Manager (Windows)
internal/pyjson/     JSON that round-trips exactly like the Python json.dump 1.x used
internal/i18n/       every message, in English and Vietnamese (+ Windows wording)
internal/parity/     runs legacy/claude-acc.bash and the Go binary side by side
internal/wintest/    the Windows self-test (built into an .exe, run on a Windows machine)
legacy/              the 1.x bash script, kept only as the parity reference
scripts/build.sh     cross-compiles release binaries into dist/
```

```bash
go test ./...          # the parity suite needs macOS and python3 (for the 1.x reference)
./scripts/build.sh     # dist/claude-acc-{darwin,windows}-{arm64,amd64}[.exe], the Windows
                       # self-test .exe, and SHA256SUMS
```

`internal/parity` runs 100 scenarios plus an upgrade test — every command, in both languages, interactive ones
through a real pseudo-terminal — in a fake world (temp `HOME`, fake `security` and `claude`,
a fake API answering per token), and checks that the bash and Go versions print the same
bytes, exit with the same code, write the same files, and call `claude` the same way.
Where 2.x differs on purpose, the scenario says why (its `fix`), or the difference has a
test of its own — e.g. `TestHomeOfOneWorkspace`: a directory that is folder `[0]` of one
workspace and an extra folder of another, which 1.x refused to pin.

**Releasing:** `install.sh` and `install.ps1` download from the latest GitHub release, so
upload the files from `dist/` to a release (tag `vX.Y.Z`) *before* or together with pushing
`main` — otherwise the one-line installers fail with a 404.

## License

MIT — see [LICENSE](LICENSE).
