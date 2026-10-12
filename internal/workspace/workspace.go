// Package workspace finds the VS Code / Cursor multi-root workspace a
// directory belongs to.
//
// Measured 2026-09-05: in a multi-root workspace every chat tab runs claude
// with cwd = folder [0] of the .code-workspace file, even a tab working on
// another worktree. A pin therefore has to land in folder [0], not in the
// directory the user happens to stand in.
package workspace

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/linhh-phv/claude-acc/internal/fsx"
	"github.com/linhh-phv/claude-acc/internal/pyjson"
)

// Kind of answer from GuiRoot.
const (
	None  = "NONE"
	Ambig = "AMBIG"
	WS    = "WS"
)

var lineComment = regexp.MustCompile(`(?m)^\s*//.*$`)

type wsFile struct {
	file  string
	paths []string // realpath of every folder, folder [0] first
}

// scan walks up to six parent directories of start (start itself excluded)
// and calls visit for each *.code-workspace there, in sorted order, until
// visit returns false.
func scan(start string, visit func(w wsFile) bool) {
	d := start
	for i := 0; i < 6; i++ {
		parent := filepath.Dir(d)
		if parent == d {
			return
		}
		matches, _ := filepath.Glob(filepath.Join(parent, "*.code-workspace"))
		sort.Strings(matches)
		for _, ws := range matches {
			if strings.HasPrefix(filepath.Base(ws), ".") {
				continue // glob in Python skips dotfiles
			}
			paths, ok := load(ws)
			if !ok {
				continue
			}
			if !visit(wsFile{ws, paths}) {
				return
			}
		}
		d = parent
	}
}

// load reads the folder list of a .code-workspace file. Whole-line //
// comments are tolerated, as VS Code writes them.
func load(ws string) ([]string, bool) {
	raw, err := os.ReadFile(ws)
	if err != nil {
		return nil, false
	}
	data, err := pyjson.ParseObject(raw)
	if err != nil {
		data, err = pyjson.ParseObject(lineComment.ReplaceAll(raw, nil))
		if err != nil {
			return nil, false
		}
	}
	folders, _ := data.Get("folders")
	list, _ := folders.([]any)
	base := filepath.Dir(ws)
	var paths []string
	for _, f := range list {
		o, ok := f.(*pyjson.Object)
		if !ok {
			continue
		}
		p := o.String("path")
		if p == "" {
			continue
		}
		p = filepath.FromSlash(p)
		if !filepath.IsAbs(p) {
			p = filepath.Join(base, p)
		}
		paths = append(paths, fsx.RealPath(p))
	}
	return paths, true
}

func member(start string, paths []string) bool {
	for _, p := range paths {
		if fsx.Within(start, p) {
			return true
		}
	}
	return false
}

// GuiRoot answers where the GUI runs claude for a directory: kind WS with
// folder [0] and a note "<file> (N folder)", kind AMBIG with the names of the
// competing workspaces in note, or kind None.
func GuiRoot(dir string) (kind, root, note string) {
	start := fsx.RealPath(dir)
	type cand struct {
		file, root string
		n          int
	}
	var cands []cand
	scan(start, func(w wsFile) bool {
		if len(w.paths) > 0 && member(start, w.paths) {
			cands = append(cands, cand{w.file, w.paths[0], len(w.paths)})
		}
		return true
	})
	if len(cands) == 0 {
		return None, "", ""
	}
	roots := map[string]bool{}
	for _, c := range cands {
		roots[c.root] = true
	}
	if len(roots) > 1 {
		names := make([]string, len(cands))
		for i, c := range cands {
			names[i] = filepath.Base(c.file)
		}
		return Ambig, "", strings.Join(names, ", ")
	}
	c := cands[0]
	return WS, c.root, filepath.Base(c.file) + " (" + strconv.Itoa(c.n) + " folder)"
}

// Folders lists every existing folder of the first workspace containing dir,
// folder [0] first. Empty when dir belongs to none.
func Folders(dir string) []string {
	start := fsx.RealPath(dir)
	var out []string
	scan(start, func(w wsFile) bool {
		if !member(start, w.paths) {
			return true
		}
		for _, q := range w.paths {
			if fsx.IsDir(q) {
				out = append(out, q)
			}
		}
		return false
	})
	return out
}

// Of returns folder [0] and all folders of the first workspace containing dir.
func Of(dir string) (root string, paths []string) {
	start := fsx.RealPath(dir)
	scan(start, func(w wsFile) bool {
		if len(w.paths) > 0 && member(start, w.paths) {
			root, paths = w.paths[0], w.paths
			return false
		}
		return true
	})
	return root, paths
}
