package signin

import "testing"

func TestTokenFromTranscript(t *testing.T) {
	full := "sk-ant-oat01-" + "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789_-AbCdEfGhIjKlMnOpQrStUvWxYz0123456789_-AbCdEfGhIjKlMnOp-QAA"
	cases := map[string]string{
		// whole token on one line, indented like claude prints it
		"\x1b[32m  Your token:\x1b[0m\r\n  " + full + "\r\n\r\n  Store it safely\r\n": full,
		// a narrow window wrapped it at column 0
		full[:50] + "\r\n" + full[50:91] + "\r\n" + full[91:] + "\r\n\r\n  done": full,
		// the next line is claude's own (indented) text: not part of the token
		"  " + full + "\n  Store this token somewhere safe\n": full,
		// nothing there
		"  Paste code here if prompted >": "",
	}
	for in, want := range cases {
		if got := TokenFromTranscript(in); got != want {
			t.Errorf("TokenFromTranscript(%q) = %q, want %q", in, got, want)
		}
	}
}
