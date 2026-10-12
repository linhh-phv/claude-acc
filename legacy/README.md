# legacy/claude-acc.bash

The original bash implementation of claude-acc (v1.x), frozen as it was when the Go
rewrite replaced it. Nothing installs or runs it any more.

It stays for one reason: `internal/parity` runs it side by side with the Go binary in
the same fake world and checks that both print and write the same things. That is how
the rewrite was shown to behave like the version people had been using.

When the Go version changes behaviour on purpose, either add a `fix` to the affected
scenario explaining the difference, or drop the scenario. Once nobody needs that
comparison any more, this directory and `internal/parity` can be deleted together.
