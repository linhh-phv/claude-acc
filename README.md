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

### `switch` takes effect immediately

No reload, no restart. Claude Code re-reads `~/.claude/settings.json` on *every* request
rather than once at startup, so the next thing you send — in the terminal or in an
already-open Cursor chat — goes out under the new account.

Measured on extension 2.1.260 and on 2.1.198: swap the token mid-session in a running
`stream-json` session and the very next turn fails with `401 OAuth access token is invalid`.

The flip side is worth knowing: because it applies per request, `switch` also moves
**sessions that are already running** in your other windows and repos onto the new
account, mid-prompt. They don't get killed — the next turn is simply billed to the new
account. If you want one repo pinned to its own account regardless, put the `env` block
in that repo's `.claude/settings.local.json` instead; repo-level settings override the
user-level file (and keep that path in `.gitignore` — the token is plaintext).

## Commands

| Command | What it does |
|---|---|
| `add <name> [--browser <app>]` | Add a secondary account. Runs `claude setup-token`, captures the token automatically (no copy/paste), stores it in the Keychain. |
| `switch <name\|main>` | **The command you'll use most.** Makes `<name>` (or `main`) the active account for both the CLI and the GUI extension. |
| `ls` | List configured accounts and which one is currently active. |
| `check <name\|main>` | Verify an account's credential still works, with a real API call — not just a config read. |
| `login [--browser <app>]` | Re-run `claude auth login` for `main` (this overwrites the Keychain — `main` only). |
| `rm <name>` | Remove a secondary account's token. |
| `uninstall` | Remove every token and config `claude-acc` created. |
| `run <name\|main> -- [args]` | Advanced: run one `claude` invocation under a given account without changing what's pinned. Refuses to run if a different account is currently pinned via `switch`, to avoid silently using the wrong one. |
| `env <name\|main>` | Advanced: print an `export` line to change the current shell only, e.g. `eval "$(claude-acc env work)"`. |

Run `claude-acc help` any time for the full reference (in Vietnamese, matching how the tool was originally built — the behavior is identical either way).

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
- **Tokens expire after about a year.** `claude-acc check <name>` will tell you when one has; just `claude-acc add <name>` again.
- **A token is briefly visible via `ps` while `claude-acc add` writes it to the Keychain.** `security add-generic-password -w <password>` requires the password as a command-line argument — its own man page's only non-interactive option — so for the moment that one command runs, the token is technically readable by another local user running `ps -ef`, or by exec-argv-logging security/EDR software on managed machines. This is a limitation of the macOS `security` CLI itself, not something `claude-acc` can close without reimplementing Keychain writes against the native Keychain Services API. Everywhere else (settings.json writes, subprocess env passing) `claude-acc` avoids putting the token in argv.

## License

MIT — see [LICENSE](LICENSE).
