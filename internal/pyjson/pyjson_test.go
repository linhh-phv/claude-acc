package pyjson

import (
	"os/exec"
	"testing"
)

var samples = []string{
	`{}`,
	`[]`,
	`{"env": {"CLAUDE_CODE_OAUTH_TOKEN": "sk-ant-oat01-abc", "CLAUDE_ACC_NAME": "work"}}`,
	`{"b": 1, "a": [1, 2, {"x": null, "y": true, "z": false}], "c": {}, "d": []}`,
	`{"n": [0, -0, 1.0, 1.5, 1e5, 1E-7, 0.0001, 0.00001, 1e15, 1e16, 123456789012345678901234567890, -2.50, 3.14159265358979, 1e300]}`,
	`{"s": "tiếng Việt — “quotes” \"esc\" \\ / \u0001 \t \n \r \b \f \u007f   <>& 😀"}`,
	`{"dup": 1, "other": 2, "dup": 3}`,
	`{"permissions": {"allow": ["Bash(npm run *)", "Read(./**)"], "deny": []}, "model": "opus"}`,
	"\xef\xbb\xbf{\"bom\": true}",
}

func TestDumpMatchesPython(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	for _, s := range samples {
		v, err := Parse([]byte(s))
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		got := string(Dump(v))
		in := s
		if len(in) >= 3 && in[:3] == "\xef\xbb\xbf" {
			in = in[3:]
		}
		cmd := exec.Command("python3", "-I", "-c",
			"import json,sys; sys.stdout.write(json.dumps(json.loads(sys.stdin.read()), indent=2, ensure_ascii=False))")
		cmd.Stdin = stringReader(in)
		want, err := cmd.Output()
		if err != nil {
			t.Fatalf("python on %q: %v", s, err)
		}
		if got != string(want) {
			t.Errorf("mismatch for %q\n--- go ---\n%s\n--- python ---\n%s", s, got, want)
		}
	}
}

func TestParseRejects(t *testing.T) {
	for _, s := range []string{``, `{`, `{"a":1} x`, `{"a":1,}`, `// c` + "\n{}"} {
		if _, err := Parse([]byte(s)); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
	if _, err := ParseObject([]byte(`[1]`)); err == nil {
		t.Error("expected error for non-object")
	}
}

func TestObjectOrder(t *testing.T) {
	o, _ := ParseObject([]byte(`{"a":1,"env":{},"z":2}`))
	o.Set("env", "x")
	o.Set("new", 1)
	o.Delete("a")
	if got := o.Keys(); len(got) != 3 || got[0] != "env" || got[1] != "z" || got[2] != "new" {
		t.Fatalf("order: %v", got)
	}
}
