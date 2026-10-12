//go:build darwin

// Package parity runs the bash claude-acc and the Go one side by side in the
// same fake world, and compares everything they print and write.
package parity

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------- the fake world ---------------------------------------------------

// fakeSecurity stands in for macOS `security`: one file per item.
const fakeSecurity = `#!/bin/sh
cmd=$1; shift
svc=""; acc=""; pw=""; want=0
while [ $# -gt 0 ]; do
  case "$1" in
    -s) svc=$2; shift 2 ;;
    -a) acc=$2; shift 2 ;;
    -w) if [ "$cmd" = add-generic-password ]; then pw=$2; shift 2; else want=1; shift; fi ;;
    -D|-j|-l|-c|-C|-T) shift 2 ;;
    *) shift ;;
  esac
done
f="$FAKE_KC/$(printf '%s' "$svc" | tr ' /' '__')@$acc"
case "$cmd" in
  find-generic-password) [ -f "$f" ] || exit 44; [ $want = 1 ] && { cat "$f"; echo; }; exit 0 ;;
  add-generic-password) printf '%s' "$pw" > "$f"; exit 0 ;;
  delete-generic-password) [ -f "$f" ] || exit 44; rm -f "$f"; exit 0 ;;
esac
exit 1
`

// fakeClaude answers the few commands claude-acc runs, and logs each call
// with the token it saw.
const fakeClaude = `#!/bin/sh
printf '%s | tok=%s | cwd=%s\n' "$*" "${CLAUDE_CODE_OAUTH_TOKEN-UNSET}" "$(pwd -P)" >> "$FAKE_LOG"
case "$1 $2" in
  "auth status")
    if [ "$3" = "--json" ]; then
      echo '{"loggedIn": true, "authMethod": "claude.ai", "account": {"plan": "max", "emailAddress": "main@example.com"}}'
    else
      echo "Login method: Claude Max Account"
      echo "Email: main@example.com"
    fi
    exit 0 ;;
esac
if [ "$1" = "-p" ]; then
  if [ -n "$FAKE_REFRESH_TO" ]; then
    printf '{"claudeAiOauth":{"accessToken":"%s","refreshToken":"r"}}' "$FAKE_REFRESH_TO" > "$FAKE_KC/Claude_Code-credentials@"
  fi
  echo ok; exit 0
fi
exit 0
`

type sandbox struct {
	t                        *testing.T
	root, home, bin, kc, tmp string
	w                        string // where scenario directories go
	api                      *httptest.Server
	now                      time.Time
	pathEnv                  string
	extraEnv                 map[string]string
}

type result struct {
	stdout, stderr string
	code           int
	files          map[string]string
	claudeLog      string
}

var (
	buildOnce sync.Once
	goBin     string
	bashBin   string
	buildErr  error
)

// binaries builds the Go claude-acc and prepares a copy of the bash one that
// talks to the fake API.
func binaries(t *testing.T) (string, string) {
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "claude-acc-parity-bin.")
		if err != nil {
			buildErr = err
			return
		}
		goBin = filepath.Join(dir, "claude-acc-go")
		out, err := exec.Command("go", "build", "-o", goBin, "../../cmd/claude-acc").CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("go build: %v\n%s", err, out)
			return
		}
		src, err := os.ReadFile("../../bin/claude-acc")
		if err != nil {
			buildErr = err
			return
		}
		patched := strings.ReplaceAll(string(src), `"https://api.anthropic.com/v1/messages"`,
			`os.environ.get("CLAUDE_ACC_API_URL", "https://api.anthropic.com") + "/v1/messages"`)
		bashBin = filepath.Join(dir, "claude-acc-bash")
		buildErr = os.WriteFile(bashBin, []byte(patched), 0o755)
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return goBin, bashBin
}

// apiSpec says how the fake API answers one token.
type apiSpec struct {
	code   int
	status [2]string // 5h, 7d status header
	util   [2]string // 5h, 7d utilization
	reason string    // error.message in the body
}

func newSandbox(t *testing.T, specs map[string]apiSpec) *sandbox {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &sandbox{t: t, root: root, now: time.Now(), extraEnv: map[string]string{}}
	s.home = filepath.Join(root, "home")
	s.bin = filepath.Join(root, "bin")
	s.kc = filepath.Join(root, "kc")
	s.tmp = filepath.Join(root, "tmp")
	s.w = filepath.Join(root, "w")

	reset5 := s.now.Add(3*time.Hour + 30*time.Minute + 50*time.Second).Unix()
	reset7 := s.now.Add(4*24*time.Hour + 5*time.Hour).Unix()
	s.api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		sp, ok := specs[tok]
		if !ok {
			sp = apiSpec{code: 401, reason: "Invalid bearer token"}
		}
		if sp.status[0] != "" {
			h := w.Header()
			h.Set("anthropic-ratelimit-unified-5h-utilization", sp.util[0])
			h.Set("anthropic-ratelimit-unified-5h-reset", strconv.FormatInt(reset5, 10))
			h.Set("anthropic-ratelimit-unified-5h-status", sp.status[0])
			h.Set("anthropic-ratelimit-unified-7d-utilization", sp.util[1])
			h.Set("anthropic-ratelimit-unified-7d-reset", strconv.FormatInt(reset7, 10))
			h.Set("anthropic-ratelimit-unified-7d-status", sp.status[1])
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(sp.code)
		if sp.reason != "" {
			fmt.Fprintf(w, `{"type":"error","error":{"type":"x","message":%q}}`, sp.reason)
		} else {
			w.Write([]byte(`{"id":"msg"}`))
		}
	}))
	t.Cleanup(s.api.Close)

	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 needed by the bash version")
	}
	s.pathEnv = s.bin + ":" + filepath.Dir(py) + ":/usr/bin:/bin:/usr/sbin:/sbin"
	return s
}

// reset wipes the world and lays down the fakes.
func (s *sandbox) reset() {
	entries, _ := os.ReadDir(s.root)
	for _, e := range entries {
		os.RemoveAll(filepath.Join(s.root, e.Name()))
	}
	for _, d := range []string{s.home, filepath.Join(s.home, ".claude"), s.bin, s.kc, s.tmp, s.w} {
		must(s.t, os.MkdirAll(d, 0o755))
	}
	must(s.t, os.WriteFile(filepath.Join(s.bin, "security"), []byte(fakeSecurity), 0o755))
	must(s.t, os.WriteFile(filepath.Join(s.bin, "claude"), []byte(fakeClaude), 0o755))
	s.mainCred("main-ok")
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// ---------- seeding --------------------------------------------------------

func (s *sandbox) mainCred(tok string) {
	s.writeAbs(filepath.Join(s.kc, "Claude_Code-credentials@"), `{"claudeAiOauth":{"accessToken":"`+tok+`","refreshToken":"r"}}`)
}

func (s *sandbox) token(acc, tok string) {
	s.writeAbs(filepath.Join(s.kc, "claude-code-token@"+acc), tok)
}

func (s *sandbox) accounts(names ...string) {
	s.writeAbs(filepath.Join(s.home, ".config/claude-acc/accounts"), strings.Join(names, "\n")+"\n")
}

func (s *sandbox) writeAbs(p, content string) {
	must(s.t, os.MkdirAll(filepath.Dir(p), 0o755))
	must(s.t, os.WriteFile(p, []byte(content), 0o644))
}

// file writes under the scenario dir w.
func (s *sandbox) file(rel, content string) string {
	p := filepath.Join(s.w, rel)
	s.writeAbs(p, content)
	return p
}

func (s *sandbox) dir(rel string) string {
	p := filepath.Join(s.w, rel)
	must(s.t, os.MkdirAll(p, 0o755))
	return p
}

func (s *sandbox) git(dir string, args ...string) {
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	c.Env = append(os.Environ(), "HOME="+s.home, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := c.CombinedOutput(); err != nil {
		s.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func (s *sandbox) repo(rel string) string {
	d := s.dir(rel)
	s.git(d, "init", "-q")
	return d
}

func (s *sandbox) usageCache(lines ...string) {
	s.writeAbs(filepath.Join(s.home, ".config/claude-acc/usage-cache"), strings.Join(lines, "\n")+"\n")
}

func (s *sandbox) registry(lines ...string) {
	s.writeAbs(filepath.Join(s.home, ".config/claude-acc/repos"), strings.Join(lines, "\n")+"\n")
}

// ---------- running ----------------------------------------------------------

func (s *sandbox) run(bin []string, cwd string, args []string, stdin string, lang string) result {
	cmd := exec.Command(bin[0], append(bin[1:], args...)...)
	cmd.Dir = cwd
	env := []string{
		"PATH=" + s.pathEnv, "HOME=" + s.home, "LANG=" + lang, "TMPDIR=" + s.tmp + "/",
		"PWD=" + cwd, "TERM=dumb", "USER=t",
		"FAKE_KC=" + s.kc, "FAKE_LOG=" + filepath.Join(s.tmp, "claude.log"),
		"CLAUDE_ACC_API_URL=" + s.api.URL,
	}
	for k, v := range s.extraEnv {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		s.t.Fatalf("run %v: %v", bin, err)
	}
	logb, _ := os.ReadFile(filepath.Join(s.tmp, "claude.log"))
	return result{stdout: out.String(), stderr: errb.String(), code: code, files: s.snapshot(), claudeLog: string(logb)}
}

// snapshot records every file of the world that claude-acc may touch:
// content, mode, and where symlinks point.
func (s *sandbox) snapshot() map[string]string {
	snap := map[string]string{}
	filepath.WalkDir(s.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(s.root, p)
		if rel == "bin" || rel == "tmp" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				if b, err := os.ReadFile(filepath.Join(p, "info", "exclude")); err == nil {
					snap[rel+"/info/exclude"] = string(b)
				}
				return filepath.SkipDir
			}
			return nil
		}
		info, _ := os.Lstat(p)
		if info.Mode()&fs.ModeSymlink != 0 {
			target, _ := os.Readlink(p)
			snap[rel] = "symlink -> " + target
			return nil
		}
		b, _ := os.ReadFile(p)
		snap[rel] = fmt.Sprintf("[%v] %s", info.Mode().Perm(), b)
		return nil
	})
	return snap
}

// ---------- comparing --------------------------------------------------------

var tsField = regexp.MustCompile(`(?m)^([^\t\n]+)\t\d{9,}\t`)

func normalize(snap map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range snap {
		if strings.HasSuffix(k, "usage-cache") {
			v = tsField.ReplaceAllString(v, "$1\tTS\t")
		}
		out[k] = v
	}
	return out
}

func diffMaps(a, b map[string]string) string {
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	var ks []string
	for k := range keys {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var d strings.Builder
	for _, k := range ks {
		if a[k] != b[k] {
			fmt.Fprintf(&d, "  file %s\n    bash: %q\n    go:   %q\n", k, a[k], b[k])
		}
	}
	return d.String()
}

// TestMain removes the binaries built for the run.
func TestMain(m *testing.M) {
	code := m.Run()
	if goBin != "" {
		os.RemoveAll(filepath.Dir(goBin))
	}
	os.Exit(code)
}
