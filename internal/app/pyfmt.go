package app

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Helpers reproducing the Python formatting the bash version printed with.

// pyRound is Python's round(): half to even.
func pyRound(f float64) int { return int(math.RoundToEven(f)) }

// pyFloat is float(s), or ok=false.
func pyFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// pyInt is int(s), or ok=false.
func pyInt(s string) (int64, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n, err == nil
}

// floorDiv is Python's a // b.
func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// padRight is "%-Ns": pad to n characters (code points, as Python counts).
func padRight(s string, n int) string {
	if c := utf8.RuneCountInString(s); c < n {
		return s + strings.Repeat(" ", n-c)
	}
	return s
}

var ansiRe = regexp.MustCompile("\033\\[[0-9;]*m")

// padVisible pads by the visible width: colour codes take no column.
func padVisible(s string, n int) string {
	if c := utf8.RuneCountInString(ansiRe.ReplaceAllString(s, "")); c < n {
		return s + strings.Repeat(" ", n-c)
	}
	return s
}

// pct3 is "%3d".
func pct3(n int) string {
	s := strconv.Itoa(n)
	if len(s) < 3 {
		s = strings.Repeat(" ", 3-len(s)) + s
	}
	return s
}
