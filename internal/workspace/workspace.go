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

// Kind of Answer from GuiRoot.
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

// pick decides which workspace a directory belongs to. cands is every
// workspace holding start, in scan order; ok is false when none does or when
// the choice is not guessable.
//
// One directory can sit in several workspaces: a repo is folder [0] of its
// own workspace and an extra folder of a bigger one. Only folder [0] matters
// to the GUI, so:
//   - all of them share folder [0]: that is the answer either way;
//   - exactly one folder [0] holds start: start is that workspace's home —
//     the others merely list it, and their chat tabs never run there;
//   - otherwise (an extra folder in workspaces with different homes, or
//     homes nested in one another): not guessable.
func pick(start string) (w wsFile, cands []wsFile, ok bool) {
	scan(start, func(c wsFile) bool {
		if len(c.paths) > 0 && member(start, c.paths) {
			cands = append(cands, c)
		}
		return true
	})
	if len(cands) == 0 {
		return wsFile{}, nil, false
	}
	if len(roots(cands)) == 1 {
		return cands[0], cands, true
	}
	var homes []wsFile
	for _, c := range cands {
		if fsx.Within(start, c.paths[0]) {
			homes = append(homes, c)
		}
	}
	if len(homes) > 0 && len(roots(homes)) == 1 {
		return homes[0], cands, true
	}
	return wsFile{}, cands, false
}

func roots(ws []wsFile) map[string]bool {
	m := map[string]bool{}
	for _, w := range ws {
		m[w.paths[0]] = true
	}
	return m
}

// Answer is where the GUI runs claude for a directory.
type Answer struct {
	Kind string // WS, Ambig or None
	Root string // WS: folder [0]
	File string // WS: the .code-workspace file's name
	// Note is "<file> (N folder)" for WS, the competing workspaces for Ambig.
	Note string
	// Others names, for WS, the workspaces that list the directory without
	// running there: a pin landing in Root does not reach their chat tabs.
	Others []string
}

// GuiRoot answers where the GUI runs claude for a directory.
func GuiRoot(dir string) Answer {
	w, cands, ok := pick(fsx.RealPath(dir))
	if len(cands) == 0 {
		return Answer{Kind: None}
	}
	if !ok {
		names := make([]string, len(cands))
		for i, c := range cands {
			names[i] = filepath.Base(c.file)
		}
		return Answer{Kind: Ambig, Note: strings.Join(names, ", ")}
	}
	name := filepath.Base(w.file)
	a := Answer{Kind: WS, Root: w.paths[0], File: name, Note: name + " (" + strconv.Itoa(len(w.paths)) + " folder)"}
	for _, c := range cands {
		if c.paths[0] != a.Root {
			a.Others = append(a.Others, filepath.Base(c.file))
		}
	}
	return a
}

// Folders lists every existing folder of the workspace dir belongs to,
// folder [0] first. Empty when dir belongs to none, or to several and it is
// not guessable which.
func Folders(dir string) []string {
	var out []string
	for _, q := range paths(dir) {
		if fsx.IsDir(q) {
			out = append(out, q)
		}
	}
	return out
}

// Of returns folder [0] and all folders of the workspace dir belongs to.
func Of(dir string) (root string, all []string) {
	all = paths(dir)
	if len(all) > 0 {
		root = all[0]
	}
	return root, all
}

func paths(dir string) []string {
	if w, _, ok := pick(fsx.RealPath(dir)); ok {
		return w.paths
	}
	return nil
}
