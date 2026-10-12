package maincred

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"github.com/linhh-phv/claude-acc/internal/pyjson"
)

// Where Claude Code keeps main's credential on Windows — read from its own
// code (2.1.296), since the docs only mention the file:
//
//   - By default: <config dir>\.credentials.json, plain JSON.
//   - When CLAUDE_CODE_FORCE_WINDOWS_CREDMAN=1, or the server-side flag
//     tengu_windows_credman is cached as true in ~/.claude.json: Windows
//     Credential Manager first, the file only as a fallback — and once the
//     credential lands in Credential Manager the file is DELETED. Reading only
//     the file would then find nothing.
//
// In Credential Manager it is written through Bun.secrets: a generic
// credential named "<service>/<name>", the value as UTF-8. service is
// "Claude Code-credentials" (plus "-<sha256(config dir)[:8]>" when
// CLAUDE_CONFIG_DIR is set), name is "claude-code-user". A value over 2400
// bytes is split: "<name>#m" holds {"n":chunks,"l":length}, "<name>#0".."#n-1"
// hold the base64 of the JSON.

const (
	credmanName   = "claude-code-user"
	credmanChunk  = 2400
	credmanChunks = 256
)

// configDir is Claude Code's config directory.
func configDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// storageDir is where the credentials file lives (CLAUDE_SECURESTORAGE_CONFIG_DIR
// overrides, an empty value meaning ~/.claude).
func storageDir() string {
	if d, ok := os.LookupEnv("CLAUDE_SECURESTORAGE_CONFIG_DIR"); ok {
		if d == "" {
			home, _ := os.UserHomeDir()
			return filepath.Join(home, ".claude")
		}
		return d
	}
	return configDir()
}

// credmanService is the Bun.secrets service name of main's credential.
func credmanService() string {
	base := "Claude Code-credentials"
	if d, ok := os.LookupEnv("CLAUDE_SECURESTORAGE_CONFIG_DIR"); ok {
		if d == "" {
			return base
		}
		return base + "-" + shortHash(d)
	}
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return base + "-" + shortHash(d)
	}
	return base
}

func shortHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:8]
}

// credmanEnabled mirrors Claude Code's choice of Credential Manager.
func credmanEnabled() bool {
	if os.Getenv("CLAUDE_CODE_FORCE_WINDOWS_CREDMAN") == "1" {
		return true
	}
	legacy := filepath.Join(configDir(), ".config.json")
	cfgBase := os.Getenv("CLAUDE_CONFIG_DIR")
	if cfgBase == "" {
		cfgBase, _ = os.UserHomeDir()
	}
	path := filepath.Join(cfgBase, ".claude.json")
	if _, err := os.Stat(legacy); err == nil {
		path = legacy
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	d, err := pyjson.ParseObject(b)
	if err != nil {
		return false
	}
	f, _ := d.Get("cachedGrowthBookFeatures")
	o, ok := f.(*pyjson.Object)
	if !ok {
		return false
	}
	v, _ := o.Get("tengu_windows_credman")
	on, _ := v.(bool)
	return on
}

var errNoEntry = errors.New("no credential manager entry")

// credmanRead reassembles the value Bun.secrets stored for (service, name),
// given a reader of generic credentials by target name.
func credmanRead(get func(target string) (string, bool)) (string, error) {
	base := credmanService() + "/" + credmanName
	meta, ok := get(base + "#m")
	if !ok || meta == "" {
		v, ok := get(base)
		if !ok || v == "" {
			return "", errNoEntry
		}
		return v, nil
	}
	var m struct {
		N int `json:"n"`
		L int `json:"l"`
	}
	if json.Unmarshal([]byte(meta), &m) != nil || m.N <= 0 || m.N > credmanChunks || m.L <= 0 || m.L > m.N*credmanChunk {
		return "", errors.New("credential manager: bad chunk header")
	}
	var b64 []byte
	for i := 0; i < m.N; i++ {
		part, ok := get(base + "#" + strconv.Itoa(i))
		if !ok {
			return "", errors.New("credential manager: missing chunk")
		}
		b64 = append(b64, part...)
	}
	if len(b64) != m.L {
		return "", errors.New("credential manager: chunks do not add up")
	}
	out, err := base64.StdEncoding.DecodeString(string(b64))
	if err != nil {
		return "", err
	}
	return string(out), nil
}
