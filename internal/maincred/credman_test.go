package maincred

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const cred = `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-ACCESS","refreshToken":"sk-ant-ort01-R","expiresAt":1,"scopes":["user:inference"]}}`

// store mimics what Bun.secrets.set leaves in Credential Manager for the
// value, chunking it the way Claude Code 2.1.296 does past 2400 bytes.
func store(service, value string) map[string]string {
	base := service + "/" + credmanName
	if len(value) <= credmanChunk {
		return map[string]string{base: value}
	}
	b64 := base64.StdEncoding.EncodeToString([]byte(value))
	n := (len(b64) + credmanChunk - 1) / credmanChunk
	m := map[string]string{base + "#m": `{"n":` + strconv.Itoa(n) + `,"l":` + strconv.Itoa(len(b64)) + `}`}
	for i := 0; i < n; i++ {
		end := min((i+1)*credmanChunk, len(b64))
		m[base+"#"+strconv.Itoa(i)] = b64[i*credmanChunk : end]
	}
	return m
}

func getter(m map[string]string) func(string) (string, bool) {
	return func(t string) (string, bool) { v, ok := m[t]; return v, ok }
}

func TestCredmanReadSingle(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	got, err := credmanRead(getter(store("Claude Code-credentials", cred)))
	if err != nil || got != cred {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestCredmanReadChunked(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	big := strings.Replace(cred, `"scopes"`, `"pad":"`+strings.Repeat("x", 6000)+`","scopes"`, 1)
	m := store("Claude Code-credentials", big)
	if _, ok := m["Claude Code-credentials/claude-code-user#m"]; !ok {
		t.Fatal("test value was not chunked")
	}
	got, err := credmanRead(getter(m))
	if err != nil || got != big {
		t.Fatalf("chunked read: %v (len %d vs %d)", err, len(got), len(big))
	}
	delete(m, "Claude Code-credentials/claude-code-user#1")
	if _, err := credmanRead(getter(m)); err == nil {
		t.Fatal("expected an error for a missing chunk")
	}
}

func TestCredmanReadMissing(t *testing.T) {
	if _, err := credmanRead(getter(map[string]string{})); err == nil {
		t.Fatal("expected errNoEntry")
	}
}

func TestCredmanServiceName(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	os.Unsetenv("CLAUDE_SECURESTORAGE_CONFIG_DIR")
	if s := credmanService(); s != "Claude Code-credentials" {
		t.Fatalf("default service %q", s)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", `C:\cfg`)
	if s := credmanService(); !strings.HasPrefix(s, "Claude Code-credentials-") || len(s) != len("Claude Code-credentials-")+8 {
		t.Fatalf("service with CLAUDE_CONFIG_DIR %q", s)
	}
}

func TestCredmanEnabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CLAUDE_CODE_FORCE_WINDOWS_CREDMAN", "")
	if credmanEnabled() {
		t.Fatal("enabled with no config at all")
	}
	os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"cachedGrowthBookFeatures":{"tengu_windows_credman":false}}`), 0o600)
	if credmanEnabled() {
		t.Fatal("enabled with the flag false")
	}
	os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"cachedGrowthBookFeatures":{"other":1,"tengu_windows_credman":true}}`), 0o600)
	if !credmanEnabled() {
		t.Fatal("not enabled with the flag true")
	}
	// the legacy file wins when it exists
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	os.WriteFile(filepath.Join(home, ".claude", ".config.json"), []byte(`{}`), 0o600)
	if credmanEnabled() {
		t.Fatal("legacy .config.json should take precedence")
	}
	t.Setenv("CLAUDE_CODE_FORCE_WINDOWS_CREDMAN", "1")
	if !credmanEnabled() {
		t.Fatal("force env ignored")
	}
}
