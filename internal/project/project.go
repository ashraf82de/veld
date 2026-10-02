// Package project loads a Veld program: it parses the entry files, resolves
// `use` declarations to local files or the embedded standard library, orders
// modules by dependency and runs the checker.
package project

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ashraf82de/veld/internal/diag"
	"github.com/ashraf82de/veld/internal/syntax"
	"github.com/ashraf82de/veld/internal/types"
	"github.com/ashraf82de/veld/std"
)

// Loaded is a parsed and checked program.
type Loaded struct {
	Program *types.Program
	Diags   *diag.List
	Sources map[string]string // display file name -> source
	Entries []*types.Module
	Root    string
}

type loader struct {
	root      string
	diags     *diag.List
	sources   map[string]string
	order     []*types.Input
	state     map[string]int // module path -> 1 visiting, 2 done
	inputs    map[string]*types.Input
	overrides map[string]string // absolute local file path -> replacement source
}

// FindRoot returns the project root for a file: the nearest ancestor
// directory containing a veld.json file, or else the file's own directory.
// Local imports (`use a.b`) resolve to <root>/a/b.veld.
func FindRoot(file string) string {
	abs, err := filepath.Abs(file)
	if err != nil {
		return filepath.Dir(file)
	}
	dir := filepath.Dir(abs)
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, "veld.json")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return dir
		}
		d = parent
	}
}

// StdModules lists the names of the embedded standard library modules.
func StdModules() []string {
	entries, _ := std.FS.ReadDir(".")
	var out []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".veld") {
			out = append(out, strings.TrimSuffix(e.Name(), ".veld"))
		}
	}
	sort.Strings(out)
	return out
}

// StdSource returns the source of std module name.
func StdSource(name string) (string, bool) {
	b, err := std.FS.ReadFile(name + ".veld")
	return string(b), err == nil
}

// Load parses and checks the program rooted at the entry files. All entry
// files must share a root directory (the directory of the first entry, or
// root if non-empty).
func Load(root string, entries []string) *Loaded {
	return LoadWithSources(root, entries, nil)
}

// LoadWithSources checks a program using in-memory replacements for local
// files. Keys may be relative or absolute file paths. Files not in sources
// are read from disk; the embedded standard library is unchanged.
func LoadWithSources(root string, entries []string, sources map[string]string) *Loaded {
	l := &loader{diags: &diag.List{}, sources: map[string]string{}, state: map[string]int{}, inputs: map[string]*types.Input{}}
	l.overrides = make(map[string]string, len(sources))
	for file, src := range sources {
		abs, err := filepath.Abs(file)
		if err == nil {
			l.overrides[abs] = src
		}
	}
	if root == "" && len(entries) > 0 {
		root = FindRoot(entries[0])
	}
	l.root, _ = filepath.Abs(root)
	l.visit("std.prelude", diag.Span{})
	var entryPaths []string
	for _, e := range entries {
		abs, _ := filepath.Abs(e)
		rel, err := filepath.Rel(l.root, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			rel = filepath.Base(abs)
		}
		mp := strings.ReplaceAll(strings.TrimSuffix(rel, ".veld"), string(filepath.Separator), ".")
		entryPaths = append(entryPaths, mp)
		l.visit(mp, diag.Span{})
		if in := l.inputs[mp]; in != nil {
			in.IsEntry = true
		}
	}
	prog, cd := types.Check(l.order)
	l.diags.Merge(cd)
	out := &Loaded{Program: prog, Diags: l.diags, Sources: l.sources, Root: l.root}
	for _, p := range entryPaths {
		if m := prog.ByPath[p]; m != nil {
			out.Entries = append(out.Entries, m)
		}
	}
	return out
}

func (l *loader) visit(path string, from diag.Span) {
	switch l.state[path] {
	case 1:
		l.diags.Errorf("E602", from, "import cycle: module `%s` imports itself (directly or indirectly)", path).
			Note("move shared declarations into a third module that both can import")
		return
	case 2:
		return
	}
	l.state[path] = 1
	var src, display string
	isStd := strings.HasPrefix(path, "std.")
	if isStd {
		s, ok := StdSource(strings.TrimPrefix(path, "std."))
		if !ok {
			d := l.diags.Errorf("E603", from, "unknown standard library module `%s`", path)
			if s := diag.Suggest(strings.TrimPrefix(path, "std."), StdModules()); s != "" {
				d.Note("did you mean `std.%s`?", s)
			}
			d.Note("available: %s", strings.Join(stdNames(), ", "))
			l.state[path] = 2
			return
		}
		src, display = s, "std/"+strings.TrimPrefix(path, "std.")+".veld"
	} else {
		fp := filepath.Join(l.root, filepath.FromSlash(strings.ReplaceAll(path, ".", "/"))+".veld")
		if replacement, ok := l.overrides[fp]; ok {
			src = replacement
		} else {
			b, err := os.ReadFile(fp)
			if err != nil {
				l.diags.Errorf("E603", from, "cannot find module `%s` (looked for %s)", path, fp)
				l.state[path] = 2
				return
			}
			src = string(b)
		}
		display = fp
		if rel, err := filepath.Rel(mustCwd(), fp); err == nil && !strings.HasPrefix(rel, "..") {
			display = rel
		}
	}
	l.sources[display] = src
	f := syntax.Parse(display, src, l.diags)
	in := &types.Input{Path: path, File: f, IsStd: isStd, IsPrelude: path == "std.prelude", Uses: map[string]string{}}
	for _, u := range f.Uses {
		dep := strings.Join(u.Path, ".")
		if dep == "std.prelude" {
			l.diags.Errorf("E603", u.Span, "the prelude is imported automatically")
			continue
		}
		if !isStd && u.Path[0] == "std" && len(u.Path) != 2 {
			l.diags.Errorf("E603", u.Span, "unknown standard library module `%s`", dep)
			continue
		}
		l.visit(dep, u.Span)
		in.Uses[u.Name()] = dep
	}
	l.inputs[path] = in
	l.order = append(l.order, in)
	l.state[path] = 2
}

func stdNames() []string {
	var out []string
	for _, n := range StdModules() {
		if n != "prelude" {
			out = append(out, "std."+n)
		}
	}
	return out
}

func mustCwd() string {
	wd, _ := os.Getwd()
	return wd
}

// LoadStd loads and checks a single standard library module.
func LoadStd(name string) *Loaded {
	l := &loader{diags: &diag.List{}, sources: map[string]string{}, state: map[string]int{}, inputs: map[string]*types.Input{}}
	l.visit("std.prelude", diag.Span{})
	l.visit("std."+name, diag.Span{})
	if in := l.inputs["std."+name]; in != nil {
		in.IsEntry = true
	}
	prog, cd := types.Check(l.order)
	l.diags.Merge(cd)
	out := &Loaded{Program: prog, Diags: l.diags, Sources: l.sources}
	if m := prog.ByPath["std."+name]; m != nil {
		out.Entries = append(out.Entries, m)
	}
	return out
}
