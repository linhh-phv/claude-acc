// Package token finds a `claude setup-token` token in text that a terminal
// displayed: a recording of the session, the clipboard after the user copied
// it, or what was pasted or piped in.
package token

import (
	"regexp"
	"strings"
)

var (
	csiRe  = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)
	tokRe  = regexp.MustCompile(`sk-ant-oat01-[A-Za-z0-9._-]*`)
	contRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// Prefix every token starts with.
const Prefix = "sk-ant-oat01-"

// Extract returns the first token in s, rejoined if the terminal broke it
// across lines, or "".
//
// Claude Code draws the token with Ink, which hard-wraps a line wider than
// the terminal — a 108-character token in a narrower window comes out as two
// lines, and both a recording and a copy keep that line break. Reading only
// the first line stored a truncated token (measured: 108 characters kept as
// 91, which the API rejected). Ink also indents the token's box (one space in
// 2.1.296), so the continuation line may start with spaces.
//
// So a token that reaches the end of its line continues on the next line
// when that line, without its leading spaces, holds nothing but token
// characters. Claude's own lines around the token (a blank line, "Store this
// token securely…") stop it.
func Extract(s string) string {
	s = csiRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		ln = strings.TrimRight(ln, " \t")
		loc := tokRe.FindStringIndex(ln)
		if loc == nil {
			continue
		}
		tok := ln[loc[0]:loc[1]]
		for j := i + 1; loc[1] == len(ln) && j < len(lines); j++ {
			nxt := strings.TrimSpace(lines[j])
			if nxt == "" || !contRe.MatchString(nxt) {
				break
			}
			tok += nxt
		}
		return tok
	}
	return ""
}
