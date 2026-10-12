// Package api asks the Anthropic API directly whether a credential works and
// how much of its quota is left.
//
// Deliberately not through `claude -p`: a real repo can have a large
// CLAUDE.md and many MCP servers, and `claude` then dies with "Prompt is too
// long" — the repo's problem, not the token's, yet it used to be reported as
// "pinned but the API does NOT answer".
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Model is the cheapest model; a 1-token reply is all a probe needs.
const Model = "claude-haiku-4-5-20251001"

var client = &http.Client{Timeout: 60 * time.Second}

func baseURL() string {
	if u := os.Getenv("CLAUDE_ACC_API_URL"); u != "" { // tests point this at a fake
		return u
	}
	return "https://api.anthropic.com"
}

// call sends the 1-token request. err is a transport failure; otherwise the
// status, headers and the API's error message (if any) come back.
func call(token string) (code int, h http.Header, reason string, err error) {
	body := []byte(`{"model": "` + Model + `", "max_tokens": 1, "messages": [{"role": "user", "content": "hi"}]}`)
	req, err := http.NewRequest("POST", baseURL()+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return 0, nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("content-type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &e) == nil {
			reason = e.Error.Message
		}
	}
	return resp.StatusCode, resp.Header, reason, nil
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// Probe reports whether token can call the API. A 429 counts as working —
// the account is out of quota, but the credential is fine — and comes back
// as out == "RATE_LIMITED", so the caller can say "wait" instead of the wrong
// advice to log in again.
func Probe(token string) (out string, ok bool) {
	code, _, reason, err := call(token)
	if err != nil {
		return clip(err.Error(), 70), false
	}
	switch {
	case code < 400:
		return "", true
	case code == 429:
		return "RATE_LIMITED", true
	case reason != "":
		return fmt.Sprintf("HTTP %d — %s", code, reason), false
	default:
		return fmt.Sprintf("HTTP %d", code), false
	}
}

// Usage is the quota reading for one account, one line:
//
//	OK\t<5h util>\t<5h reset>\t<5h status>\t<7d util>\t<7d reset>\t<7d status>\t<overage>
//	EXPIRED\t<message>
//	ERR\t<message>
//
// The numbers only exist in the response headers; no CLI command prints them.
// fix is the advice appended to EXPIRED.
func Usage(token, fix string) string {
	code, h, reason, err := call(token)
	if err != nil {
		return "ERR\t" + clip(err.Error(), 70)
	}
	g := func(name, def string) string {
		if v := h.Values("anthropic-ratelimit-unified-" + name); len(v) > 0 {
			return v[0]
		}
		return def
	}
	if code == 401 || code == 403 {
		// The reason first: "revoked" and "expired" call for different
		// thinking even though the fix is the same.
		if reason == "" {
			reason = "token no longer usable"
		}
		return fmt.Sprintf("EXPIRED\tHTTP %d — %s · %s", code, reason, fix)
	}
	if g("5h-status", "") == "" && code >= 400 {
		return fmt.Sprintf("ERR\tHTTP %d", code)
	}
	return "OK\t" + g("5h-utilization", "?") + "\t" + g("5h-reset", "0") + "\t" + g("5h-status", "?") +
		"\t" + g("7d-utilization", "?") + "\t" + g("7d-reset", "0") + "\t" + g("7d-status", "?") +
		"\t" + g("overage-status", "")
}
