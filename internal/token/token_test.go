package token

import "testing"

func TestExtract(t *testing.T) {
	full := Prefix + "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789_-AbCdEfGhIjKlMnOpQrStUvWxYz0123456789_-AbCdEfGhIjKlMnOp-QAA"
	cases := map[string]string{
		// whole token on one line, indented like claude prints it
		"\x1b[32m  Your token:\x1b[0m\r\n  " + full + "\r\n\r\n  Store it safely\r\n": full,
		// a narrow window wrapped it at column 0 (older Claude Code)
		full[:50] + "\r\n" + full[50:91] + "\r\n" + full[91:] + "\r\n\r\n  done": full,
		// wrapped inside Ink's indented box (2.1.296), as copied in Windows Terminal
		" " + full[:70] + "\r\n " + full[70:] + "\r\n\r\n Store this token securely.": full,
		// colour codes around the parts, trailing spaces on the first line
		" \x1b[33m" + full[:60] + "   \x1b[39m\n \x1b[33m" + full[60:] + "\x1b[39m\n": full,
		// the next line is claude's own (indented) text: not part of the token
		"  " + full + "\n  Store this token somewhere safe\n": full,
		// copied with words around it
		"token: " + full + " (copied)": full,
		// nothing there
		"  Paste code here if prompted >": "",
		"abc#def":                         "",
	}
	for in, want := range cases {
		if got := Extract(in); got != want {
			t.Errorf("Extract(%q) = %q, want %q", in, got, want)
		}
	}
}
