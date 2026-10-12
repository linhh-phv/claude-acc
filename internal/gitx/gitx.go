// Package gitx asks git the few questions claude-acc needs. Without git
// installed every question answers "not a repo", which is what the bash
// version did too.
package gitx

import (
	"os/exec"
	"path/filepath"
	"strings"
)

func run(dir string, args ...string) (string, bool) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimRight(string(out), "\r\n"), err == nil
}

// Toplevel is the root of the work tree containing dir.
func Toplevel(dir string) (string, bool) {
	top, ok := run(dir, "rev-parse", "--show-toplevel")
	if !ok || top == "" {
		return "", false
	}
	return filepath.FromSlash(top), true
}

// IsRepo reports whether dir is inside a git repository.
func IsRepo(dir string) bool {
	_, ok := run(dir, "rev-parse", "--git-dir")
	return ok
}

// Tracked reports whether rel (relative to dir) is tracked by git.
func Tracked(dir, rel string) bool {
	_, ok := run(dir, "ls-files", "--error-unmatch", rel)
	return ok
}

// Ignored reports whether rel is ignored by git.
func Ignored(dir, rel string) bool {
	_, ok := run(dir, "check-ignore", "-q", rel)
	return ok
}

// CommonDir is the absolute path of the repository's common git dir — shared
// by every worktree, which is where info/exclude lives.
func CommonDir(dir string) string {
	if c, ok := run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir"); ok && c != "" {
		return filepath.FromSlash(c)
	}
	c, _ := run(dir, "rev-parse", "--git-common-dir")
	if c == "" {
		return ""
	}
	c = filepath.FromSlash(c)
	if !filepath.IsAbs(c) {
		c = filepath.Join(dir, c)
	}
	return c
}
