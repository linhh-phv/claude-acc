// Package wintest is claude-acc's Windows self-test: a single .exe that
// someone runs once on a real Windows machine, next to claude-acc.exe, to
// check every command there without touching their real accounts.
//
// Build it (from macOS or anywhere):
//
//	GOOS=windows GOARCH=amd64 go test -c -o claude-acc-selftest.exe ./internal/wintest
//
// Run it on Windows (PowerShell):
//
//	.\claude-acc-selftest.exe -bin .\claude-acc-windows-amd64.exe
//
// It runs claude-acc against a temporary profile (USERPROFILE, APPDATA), a
// fake `claude` (this same .exe), a fake API on 127.0.0.1, and tokens filed
// in Credential Manager under a test-only name that is deleted afterwards.
// The clipboard is saved and put back. Interactive commands run inside a
// Windows pseudo-console (ConPTY), typing only once the prompt appears.
// If a real claude.exe is installed, two extra checks run it — without
// signing in — to confirm how Claude Code itself behaves on this machine.
package wintest
