// Package fsx holds the file helpers the bash version leaned on Python and
// coreutils for: realpath, atomic private writes, path comparison.
package fsx

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// RealPath resolves symlinks like Python's os.path.realpath: the longest
// existing prefix is resolved, whatever does not exist yet is kept as is.
func RealPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	cur, rest := abs, ""
	for {
		if r, err := filepath.EvalSymlinks(cur); err == nil {
			if rest == "" {
				return r
			}
			return filepath.Join(r, rest)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// Cwd is the physical working directory (`pwd -P`).
func Cwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return RealPath(wd)
}

// LogicalCwd is $PWD when it still names the working directory, as the shell
// sees it; otherwise the physical one.
func LogicalCwd() string {
	if pwd := os.Getenv("PWD"); pwd != "" && filepath.IsAbs(pwd) {
		a, err1 := os.Stat(pwd)
		b, err2 := os.Stat(".")
		if err1 == nil && err2 == nil && os.SameFile(a, b) {
			return pwd
		}
	}
	return Cwd()
}

// WritePrivate writes data to path through a temporary file opened 0600 and
// an atomic rename, so the file never exists with looser permissions.
func WritePrivate(path, tmpSuffix string, data []byte) error {
	tmp := path + tmpSuffix
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// IsDir reports whether p is an existing directory (following symlinks).
func IsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// IsFile reports whether p is an existing regular file (following symlinks).
func IsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// Exists reports whether p exists (following symlinks).
func Exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// IsLink reports whether p itself is a symlink.
func IsLink(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.Mode()&os.ModeSymlink != 0
}

// NonEmpty reports whether p is a file with content (`[[ -s p ]]`).
func NonEmpty(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Size() > 0
}

// SamePath compares two cleaned paths; case-insensitively on Windows.
func SamePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// Within reports whether p is dir or lies under it.
func Within(p, dir string) bool {
	if SamePath(p, dir) {
		return true
	}
	prefix := dir + string(os.PathSeparator)
	if runtime.GOOS == "windows" {
		return len(p) > len(prefix) && strings.EqualFold(p[:len(prefix)], prefix)
	}
	return strings.HasPrefix(p, prefix)
}
