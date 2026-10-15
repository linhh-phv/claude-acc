package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/linhh-phv/claude-acc/internal/fsx"
)

// world lays out dirs under a fresh base and writes each workspace file
// (name -> folder paths, relative to base) next to them.
func world(t *testing.T, dirs []string, files map[string][]string) string {
	t.Helper()
	base := fsx.RealPath(t.TempDir())
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(base, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, folders := range files {
		var b strings.Builder
		b.WriteString(`{"folders": [`)
		for i, f := range folders {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(`{"path": "` + f + `"}`)
		}
		b.WriteString("]}")
		if err := os.WriteFile(filepath.Join(base, name+".code-workspace"), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return base
}

func abs(base string, rels ...string) []string {
	out := make([]string, len(rels))
	for i, r := range rels {
		out[i] = filepath.Join(base, r)
	}
	return out
}

// A repo with a workspace of its own that a bigger workspace also lists:
// the repo's own workspace counts, though the bigger one sorts first.
func TestHomeBeatsWorkspaceThatOnlyListsIt(t *testing.T) {
	base := world(t, []string{"app", "lib", "svc/src", "svc-wt"}, map[string][]string{
		"app": {"app", "lib", "svc"},
		"svc": {"svc", "svc-wt"},
	})
	svc := filepath.Join(base, "svc")
	for _, dir := range []string{svc, filepath.Join(svc, "src")} {
		got := GuiRoot(dir)
		want := Answer{Kind: WS, Root: svc, File: "svc.code-workspace", Note: "svc.code-workspace (2 folder)", Others: []string{"app.code-workspace"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GuiRoot(%s) = %+v, want %+v", dir, got, want)
		}
	}
	if got, want := Folders(svc), abs(base, "svc", "svc-wt"); !reflect.DeepEqual(got, want) {
		t.Errorf("Folders = %v, want %v", got, want)
	}
	if root, _ := Of(svc); root != svc {
		t.Errorf("Of root = %q, want %q", root, svc)
	}

	// svc-wt is an extra folder of svc's workspace only.
	if got := GuiRoot(filepath.Join(base, "svc-wt")); got.Root != svc || got.Others != nil {
		t.Errorf("GuiRoot(svc-wt) = %+v, want root svc and no others", got)
	}
	// The bigger workspace still answers for its own folders.
	app := filepath.Join(base, "app")
	if got := GuiRoot(filepath.Join(base, "lib")); got.Kind != WS || got.Root != app || got.Others != nil {
		t.Errorf("GuiRoot(lib) = %+v, want root app", got)
	}
	if got, want := Folders(app), abs(base, "app", "lib", "svc"); !reflect.DeepEqual(got, want) {
		t.Errorf("Folders(app) = %v, want %v", got, want)
	}
}

// An extra folder of two workspaces with different homes: nothing to go by.
func TestExtraFolderOfTwoIsAmbiguous(t *testing.T) {
	base := world(t, []string{"a", "b", "shared"}, map[string][]string{
		"one": {"a", "shared"},
		"two": {"b", "shared"},
	})
	shared := filepath.Join(base, "shared")
	got := GuiRoot(shared)
	if got.Kind != Ambig || got.Note != "one.code-workspace, two.code-workspace" {
		t.Errorf("GuiRoot = %+v, want Ambig naming both", got)
	}
	if f := Folders(shared); f != nil {
		t.Errorf("Folders = %v, want none", f)
	}
	if root, paths := Of(shared); root != "" || paths != nil {
		t.Errorf("Of = %q %v, want nothing", root, paths)
	}
}

// Homes nested in one another: both hold the directory, neither is "the" one.
func TestNestedHomesAreAmbiguous(t *testing.T) {
	base := world(t, []string{"outer/inner/src", "x"}, map[string][]string{
		"outer": {"outer"},
		"inner": {"outer/inner", "x"},
	})
	if got := GuiRoot(filepath.Join(base, "outer/inner/src")); got.Kind != Ambig {
		t.Errorf("GuiRoot = %+v, want Ambig", got)
	}
}

// Two files with the same folder [0] agree, whatever else they list.
func TestSameHomeIsNotAmbiguous(t *testing.T) {
	base := world(t, []string{"a", "b", "c"}, map[string][]string{
		"full": {"a", "b", "c"},
		"lite": {"a", "b"},
	})
	got := GuiRoot(filepath.Join(base, "b"))
	want := Answer{Kind: WS, Root: filepath.Join(base, "a"), File: "full.code-workspace", Note: "full.code-workspace (3 folder)"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GuiRoot = %+v, want %+v", got, want)
	}
}

func TestOutsideAnyWorkspace(t *testing.T) {
	base := world(t, []string{"a", "loose"}, map[string][]string{"one": {"a"}})
	loose := filepath.Join(base, "loose")
	if got := GuiRoot(loose); got.Kind != None {
		t.Errorf("GuiRoot = %+v, want None", got)
	}
	if f := Folders(loose); f != nil {
		t.Errorf("Folders = %v, want none", f)
	}
}
