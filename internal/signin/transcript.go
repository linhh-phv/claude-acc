// Package signin drives Claude Code's two browser sign-in flows:
// `claude setup-token` (mints a secondary account's token) and
// `claude auth login` (signs in main).
package signin

import (
	"regexp"
	"strings"
)

var (
	csiRe   = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)
	tokRe   = regexp.MustCompile(`sk-ant-oat01-[A-Za-z0-9._-]*`)
	contRe  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	spaceRe = regexp.MustCompile(`^\s`)
)

// TokenFromTranscript pulls the token out of a recording of what the terminal
// displayed during `claude setup-token`.
//
// Not a plain search: the recording holds what the terminal SHOWED, including
// the line break a narrow window inserts in the middle of the token. Stopping
// at the end of the line stored a truncated token (108 chars kept as 91) that
// the API rejected as invalid. A token touching the very end of its line
// continues on the next line, as long as that line starts at column 0 (every
// line claude itself prints is indented) and holds only token characters.
func TokenFromTranscript(raw string) string {
	raw = csiRe.ReplaceAllString(raw, "")
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	lines := strings.Split(raw, "\n")
	for i, ln := range lines {
		loc := tokRe.FindStringIndex(ln)
		if loc == nil {
			continue
		}
		tok := ln[loc[0]:loc[1]]
		end := loc[1] == len(ln)
		for j := i + 1; end && j < len(lines); j++ {
			nxt := lines[j]
			if nxt == "" || spaceRe.MatchString(nxt) || !contRe.MatchString(nxt) {
				break
			}
			tok += nxt
		}
		return tok
	}
	return ""
}
