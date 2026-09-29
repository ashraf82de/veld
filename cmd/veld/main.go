// Command veld is the Veld toolchain: run, check, test, fmt, describe, and
// the agent-facing helpers spec, explain and ast.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/ashraf82de/veld/docs"
	"github.com/ashraf82de/veld/internal/codes"
	"github.com/ashraf82de/veld/internal/diag"
	"github.com/ashraf82de/veld/internal/format"
	"github.com/ashraf82de/veld/internal/interp"
	"github.com/ashraf82de/veld/internal/project"
	"github.com/ashraf82de/veld/internal/syntax"
	"github.com/ashraf82de/veld/internal/types"
)

// version is set at release time with -ldflags "-X main.version=...".
var version = "0.2.0-dev"

const usage = `veld %s — a programming language designed for AI agents

usage:
  veld run <file.veld> [--deny eff,...] [-- args...]   check, then run main()
  veld check <files...> [--json]                        type/effect check, report diagnostics
  veld test [files or dirs...] [--json] [--filter s]    run test blocks (default: current dir)
  veld fix <files...> [--stdout]                        apply the suggested fix of every error
  veld fmt <files...> [--check] [--stdout]              rewrite files in canonical form
  veld describe [std.module | file.veld] [--json]       list signatures and docs
  veld spec                                             print the language guide (for agent context)
  veld explain <CODE>                                   explain a diagnostic code
  veld ast <file.veld>                                  print the syntax tree as JSON
  veld new <dir>                                        create a starter project
  veld eval <tasks-dir> <solutions-dir> [--json]        grade agent solutions (see evals/README.md)
  veld eval export <tasks-dir>                          print the tasks as JSON lines (dataset format)
  veld report <files...> [-m "what went wrong"] [--url|--feedback]  Markdown bug report; --url or --feedback prints a prefilled GitHub issue link
  veld version

Every command that reports problems supports --json with stable diagnostic
codes, exact spans and machine-applicable fixes.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, usage, version)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var code int
	switch cmd {
	case "run":
		code = cmdRun(args)
	case "check":
		code = cmdCheck(args)
	case "test":
		code = cmdTest(args)
	case "fmt":
		code = cmdFmt(args)
	case "fix":
		code = cmdFix(args)
	case "describe":
		code = cmdDescribe(args)
	case "spec":
		fmt.Print(docs.Language)
	case "explain":
		code = cmdExplain(args)
	case "ast":
		code = cmdAST(args)
	case "new":
		code = cmdNew(args)
	case "eval":
		code = cmdEval(args)
	case "report":
		code = cmdReport(args)
	case "version", "--version", "-v":
		fmt.Println("veld " + version)
	case "help", "--help", "-h":
		fmt.Printf(usage, version)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n"+usage, cmd, version)
		code = 2
	}
	os.Exit(code)
}

// splitArgs separates flags (which may appear anywhere) from positional
// arguments. Everything after "--" is passed through.
func splitArgs(args []string, valued map[string]bool) (flags map[string]string, pos []string, rest []string) {
	flags = map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return flags, pos, args[i+1:]
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			name := strings.TrimLeft(a, "-")
			if k, v, ok := strings.Cut(name, "="); ok {
				flags[k] = v
				continue
			}
			if valued[name] && i+1 < len(args) {
				flags[name] = args[i+1]
				i++
				continue
			}
			flags[name] = "true"
			continue
		}
		pos = append(pos, a)
	}
	return
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

func report(l *project.Loaded, asJSON bool) bool {
	ds := l.Diags.Sorted()
	if asJSON {
		fmt.Println(diag.JSON(ds))
	} else if len(ds) > 0 {
		fmt.Fprint(os.Stderr, diag.Text(ds, l.Sources))
	}
	return !l.Diags.HasErrors()
}

// ---------- check ----------

func cmdCheck(args []string) int {
	flags, files, _ := splitArgs(args, nil)
	files = expandFiles(files)
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "veld check: no .veld files given")
		return 2
	}
	all := &project.Loaded{Diags: &diag.List{}, Sources: map[string]string{}}
	for _, l := range loadGroups(files) {
		all.Diags.Merge(l.Diags)
		for k, v := range l.Sources {
			all.Sources[k] = v
		}
	}
	if !report(all, flags["json"] == "true") {
		return 1
	}
	if flags["json"] != "true" {
		warns := 0
		for _, d := range all.Diags.Sorted() {
			if d.Severity == diag.Warning {
				warns++
			}
		}
		fmt.Fprintf(os.Stderr, "ok: %d file(s) checked, %d warning(s)\n", len(files), warns)
	}
	return 0
}

// ---------- run ----------

func cmdRun(args []string) int {
	flags, pos, rest := splitArgs(args, map[string]bool{"deny": true})
	if len(pos) < 1 {
		fmt.Fprintln(os.Stderr, "veld run: missing file")
		return 2
	}
	file := pos[0]
	progArgs := append(pos[1:], rest...)
	asJSON := flags["json"] == "true"
	l := project.Load("", []string{file})
	if !report(l, asJSON) {
		return 1
	}
	if len(l.Entries) == 0 {
		return 1
	}
	mod := l.Entries[0]
	mainFn := mod.Funcs["main"]
	if mainFn == nil || len(mainFn.Params) != 0 {
		fmt.Fprintf(os.Stderr, "%s: no `fn main() -> Unit` (or `-> Result[Unit, E]`) to run\n", file)
		return 1
	}
	isResult := false
	if tc, ok := types.Prune(mainFn.Ret).(*types.TCon); ok && tc.Info != nil && tc.Info.Name == "Result" {
		isResult = true
	} else if !ok || tc.Name != "Unit" {
		fmt.Fprintf(os.Stderr, "%s: main must return Unit or Result[Unit, E], not %s\n", file, types.Show(mainFn.Ret))
		return 1
	}
	denied := types.EffectsOf(strings.Split(flags["deny"], ","))
	if mainFn.Effects&denied != 0 {
		fmt.Fprintf(os.Stderr, "refusing to run: main uses %s, which --deny forbids\n", (mainFn.Effects & denied).String())
		return 1
	}
	in := interp.New(l.Program, mainFn.Effects&^denied, l.Sources)
	in.Args = progArgs
	in.Stderr = func(s string) { fmt.Fprint(os.Stderr, s) }
	th := in.NewThread(mod)
	var result interp.Value
	code := 0
	func() {
		defer func() {
			if r := recover(); r != nil {
				if ex, ok := r.(*interp.ExitRequest); ok {
					code = ex.Code
					return
				}
				panic(r)
			}
		}()
		err := th.Protect(func() { result = th.CallFunc(mainFn, nil) })
		if err != nil {
			var re *interp.RuntimeError
			errors.As(err, &re)
			if asJSON {
				b, _ := json.MarshalIndent(map[string]any{"runtime_error": re}, "", "  ")
				fmt.Fprintln(os.Stderr, string(b))
			} else {
				fmt.Fprintln(os.Stderr, re.Error())
			}
			code = 1
			return
		}
		if isResult {
			if v, ok := result.(*interp.Variant); ok && v.Ctor.Name == "Err" {
				fmt.Fprintln(os.Stderr, "error: "+interp.Show(v.Fields[0]))
				code = 1
			}
		}
	}()
	return code
}

// ---------- test ----------

func cmdTest(args []string) int {
	flags, pos, _ := splitArgs(args, map[string]bool{"filter": true, "deny": true})
	asJSON := flags["json"] == "true"
	var files []string
	var stdMods []string
	for _, p := range pos {
		if p == "std" || strings.HasPrefix(p, "std.") {
			if p == "std" {
				stdMods = append(stdMods, project.StdModules()...)
			} else {
				stdMods = append(stdMods, strings.TrimPrefix(p, "std."))
			}
			continue
		}
		files = append(files, p)
	}
	if len(pos) == 0 {
		files = []string{"."}
	}
	files = expandFiles(files)
	var results []interp.TestResult
	var allDiags []*diag.Diagnostic
	failedCheck := false
	denied := types.EffectsOf(strings.Split(flags["deny"], ","))
	run := func(l *project.Loaded) {
		allDiags = append(allDiags, l.Diags.Sorted()...)
		if l.Diags.HasErrors() {
			failedCheck = true
			if !asJSON {
				fmt.Fprint(os.Stderr, diag.Text(l.Diags.Sorted(), l.Sources))
			}
			return
		}
		in := interp.New(l.Program, types.AllEffects&^denied, l.Sources)
		for _, m := range l.Entries {
			results = append(results, in.RunTests(m, flags["filter"])...)
		}
	}
	for _, l := range loadGroups(files) {
		run(l)
	}
	for _, m := range stdMods {
		run(project.LoadStd(m))
	}
	passed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		}
	}
	if asJSON {
		if results == nil {
			results = []interp.TestResult{}
		}
		if allDiags == nil {
			allDiags = []*diag.Diagnostic{}
		}
		printJSON(map[string]any{"ok": !failedCheck && passed == len(results), "passed": passed, "failed": len(results) - passed, "tests": results, "diagnostics": allDiags})
	} else {
		for _, r := range results {
			if r.Passed {
				fmt.Printf("PASS  %s: %s\n", r.Module, r.Name)
			} else {
				fmt.Printf("FAIL  %s: %s\n", r.Module, r.Name)
				for _, line := range strings.Split(r.Error.Error(), "\n") {
					fmt.Printf("      %s\n", line)
				}
			}
		}
		fmt.Printf("\n%d passed, %d failed\n", passed, len(results)-passed)
	}
	if failedCheck || passed != len(results) {
		return 1
	}
	return 0
}

// ---------- fmt ----------

func cmdFmt(args []string) int {
	flags, files, _ := splitArgs(args, nil)
	files = expandFiles(files)
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "veld fmt: no .veld files given")
		return 2
	}
	code := 0
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		ds := &diag.List{}
		ast := syntax.Parse(f, string(src), ds)
		if ds.HasErrors() {
			fmt.Fprint(os.Stderr, diag.Text(ds.Sorted(), map[string]string{f: string(src)}))
			code = 1
			continue
		}
		out := format.File(ast)
		switch {
		case flags["check"] == "true":
			if out != string(src) {
				fmt.Printf("%s: not formatted\n", f)
				code = 1
			}
		case flags["stdout"] == "true":
			fmt.Print(out)
		default:
			if out != string(src) {
				if err := os.WriteFile(f, []byte(out), 0o644); err != nil {
					fmt.Fprintln(os.Stderr, err)
					return 2
				}
				fmt.Printf("formatted %s\n", f)
			}
		}
	}
	return code
}

// ---------- fix ----------

// cmdFix repeatedly applies the first suggested fix of each error until no
// more apply (at most 5 rounds), then reports what is left.
func cmdFix(args []string) int {
	flags, files, _ := splitArgs(args, nil)
	files = expandFiles(files)
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "veld fix: no .veld files given")
		return 2
	}
	targets := map[string]bool{}
	for _, f := range files {
		targets[f] = true
	}
	contents := map[string]string{}
	total := 0
	var last []*diag.Diagnostic
	var sources map[string]string
	for round := 0; round < 5; round++ {
		for f := range targets {
			if _, ok := contents[f]; !ok {
				b, err := os.ReadFile(f)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					return 2
				}
				contents[f] = string(b)
			}
			if err := os.WriteFile(f, []byte(contents[f]), 0o644); err != nil && flags["stdout"] != "true" {
				fmt.Fprintln(os.Stderr, err)
				return 2
			}
		}
		last = nil
		sources = map[string]string{}
		for _, l := range loadGroups(files) {
			last = append(last, l.Diags.Sorted()...)
			for k, v := range l.Sources {
				sources[k] = v
			}
		}
		byFile := map[string][]diag.Fix{}
		for _, d := range last {
			if d.Severity != diag.Error || len(d.Fixes) == 0 {
				continue
			}
			f := d.Fixes[0]
			file := d.Span.File
			if len(f.Edits) > 0 {
				file = f.Edits[0].Span.File
			}
			if !targets[file] {
				continue
			}
			byFile[file] = append(byFile[file], f)
			fmt.Fprintf(os.Stderr, "%s:%d: [%s] %s\n", file, d.Span.Start.Line, d.Code, f.Message)
		}
		applied := 0
		for file, fixes := range byFile {
			out, n := diag.ApplyFixes(contents[file], fixes)
			contents[file] = out
			applied += n
		}
		total += applied
		if applied == 0 {
			break
		}
	}
	for f := range targets {
		if flags["stdout"] == "true" {
			fmt.Print(contents[f])
		} else {
			os.WriteFile(f, []byte(contents[f]), 0o644)
		}
	}
	remaining := 0
	for _, d := range last {
		if d.Severity == diag.Error {
			remaining++
		}
	}
	fmt.Fprintf(os.Stderr, "applied %d fix(es); %d error(s) remain\n", total, remaining)
	if remaining > 0 {
		fmt.Fprint(os.Stderr, diag.Text(last, sources))
		return 1
	}
	return 0
}

// ---------- describe ----------

type fnDoc struct {
	Name      string   `json:"name"`
	Signature string   `json:"signature"`
	Doc       string   `json:"doc,omitempty"`
	Effects   []string `json:"effects,omitempty"`
}

type typeDoc struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Doc    string `json:"doc,omitempty"`
}

func cmdDescribe(args []string) int {
	flags, pos, _ := splitArgs(args, nil)
	asJSON := flags["json"] == "true"
	if len(pos) == 0 {
		type modDoc struct {
			Module  string `json:"module"`
			Summary string `json:"summary"`
		}
		var mods []modDoc
		for _, n := range project.StdModules() {
			src, _ := project.StdSource(n)
			sum := ""
			if first, _, _ := strings.Cut(src, "\n"); strings.HasPrefix(first, "#") {
				sum = strings.TrimSpace(strings.TrimLeft(first, "#"))
			}
			name := "std." + n
			if n == "prelude" {
				name = "prelude (always imported)"
			}
			mods = append(mods, modDoc{name, sum})
		}
		if asJSON {
			printJSON(map[string]any{"modules": mods})
			return 0
		}
		for _, m := range mods {
			fmt.Printf("%-28s %s\n", m.Module, m.Summary)
		}
		fmt.Println("\nrun `veld describe std.<name>` for a module's functions and types")
		return 0
	}
	target := pos[0]
	var l *project.Loaded
	if strings.HasSuffix(target, ".veld") {
		l = project.Load("", []string{target})
	} else {
		name := strings.TrimPrefix(target, "std.")
		if _, ok := project.StdSource(name); !ok {
			fmt.Fprintf(os.Stderr, "unknown module %q; run `veld describe` to list modules\n", target)
			return 1
		}
		l = project.LoadStd(name)
	}
	if len(l.Entries) == 0 {
		report(l, asJSON)
		return 1
	}
	m := l.Entries[0]
	var fns []fnDoc
	var tys []typeDoc
	for _, d := range m.File.Decls {
		switch d := d.(type) {
		case *syntax.FnDecl:
			if !d.Pub && !m.IsEntry {
				continue
			}
			sig := format.Signature(&syntax.FnDecl{Name: d.Name, TParams: d.TParams, Params: d.Params, Ret: d.Ret, Effects: d.Effects})
			fns = append(fns, fnDoc{Name: d.Name, Signature: sig, Doc: strings.Join(d.Doc, " "), Effects: d.Effects})
		case *syntax.TypeDecl, *syntax.RecordDecl:
			src := format.File(&syntax.File{Decls: []syntax.Decl{stripDoc(d)}})
			doc := ""
			switch dd := d.(type) {
			case *syntax.TypeDecl:
				doc = strings.Join(dd.Doc, " ")
			case *syntax.RecordDecl:
				doc = strings.Join(dd.Doc, " ")
			}
			tys = append(tys, typeDoc{Name: d.DeclName(), Source: strings.TrimSpace(src), Doc: doc})
		}
	}
	if asJSON {
		if fns == nil {
			fns = []fnDoc{}
		}
		if tys == nil {
			tys = []typeDoc{}
		}
		printJSON(map[string]any{"module": m.Path, "functions": fns, "types": tys})
		return 0
	}
	fmt.Printf("module %s", m.Path)
	if m.IsStd && m.Path != "std.prelude" {
		fmt.Printf("   (import with: use %s; call as %s.name(...))", m.Path, m.Name)
	}
	fmt.Println()
	for _, t := range tys {
		fmt.Println()
		if t.Doc != "" {
			fmt.Println("## " + t.Doc)
		}
		fmt.Println(t.Source)
	}
	for _, f := range fns {
		fmt.Println()
		if f.Doc != "" {
			fmt.Println("## " + f.Doc)
		}
		fmt.Println(f.Signature)
	}
	return 0
}

func stripDoc(d syntax.Decl) syntax.Decl {
	switch d := d.(type) {
	case *syntax.TypeDecl:
		c := *d
		c.Doc, c.Leading = nil, nil
		return &c
	case *syntax.RecordDecl:
		c := *d
		c.Doc, c.Leading = nil, nil
		return &c
	}
	return d
}

// ---------- explain ----------

func cmdExplain(args []string) int {
	if len(args) == 0 {
		var ks []string
		for k := range codes.All {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			fmt.Printf("%s  %s\n", k, codes.All[k].Title)
		}
		return 0
	}
	c := strings.ToUpper(args[0])
	info, ok := codes.All[c]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown code %s; run `veld explain` for the list\n", c)
		return 1
	}
	fmt.Printf("%s: %s\n\n%s\n", c, info.Title, info.Explain)
	return 0
}

// ---------- ast ----------

func cmdAST(args []string) int {
	_, pos, _ := splitArgs(args, nil)
	if len(pos) != 1 {
		fmt.Fprintln(os.Stderr, "veld ast: expected one file")
		return 2
	}
	src, err := os.ReadFile(pos[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	ds := &diag.List{}
	f := syntax.Parse(pos[0], string(src), ds)
	printJSON(map[string]any{"ast": astJSON(reflect.ValueOf(f)), "diagnostics": nonNil(ds.Sorted())})
	if ds.HasErrors() {
		return 1
	}
	return 0
}

func nonNil(ds []*diag.Diagnostic) []*diag.Diagnostic {
	if ds == nil {
		return []*diag.Diagnostic{}
	}
	return ds
}

// astJSON converts AST nodes to JSON-friendly values, tagging every node
// struct with its kind.
func astJSON(v reflect.Value) any {
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return nil
		}
		return astJSON(v.Elem())
	case reflect.Struct:
		t := v.Type()
		if t.PkgPath() != "github.com/ashraf82de/veld/internal/syntax" {
			return v.Interface()
		}
		out := map[string]any{"node": t.Name()}
		var add func(v reflect.Value)
		add = func(v reflect.Value) {
			t := v.Type()
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				if !f.IsExported() {
					continue
				}
				if f.Anonymous {
					add(v.Field(i))
					continue
				}
				tag := strings.Split(f.Tag.Get("json"), ",")[0]
				if tag == "-" {
					continue
				}
				if tag == "" {
					tag = strings.ToLower(f.Name)
				}
				fv := v.Field(i)
				if fv.IsZero() && strings.Contains(f.Tag.Get("json"), "omitempty") {
					continue
				}
				if f.Type == reflect.TypeOf(syntax.Kind(0)) {
					out[tag] = syntax.Kind(fv.Int()).String()
					continue
				}
				out[tag] = astJSON(fv)
			}
		}
		add(v)
		return out
	case reflect.Slice:
		out := make([]any, v.Len())
		for i := range out {
			out[i] = astJSON(v.Index(i))
		}
		return out
	}
	return v.Interface()
}

// ---------- eval ----------

type evalResult struct {
	Task        string   `json:"task"`
	Found       bool     `json:"found"`
	Compiles    bool     `json:"compiles"`
	Passed      int      `json:"passed"`
	Failed      int      `json:"failed"`
	Solved      bool     `json:"solved"`
	ErrorCodes  []string `json:"error_codes,omitempty"`
	FirstErrors []string `json:"first_errors,omitempty"`
}

// cmdEval grades <solutions-dir>/<task>.veld against each task in
// <tasks-dir>/<task>/ (tests.veld + veld.json).
func cmdEval(args []string) int {
	flags, pos, _ := splitArgs(args, nil)
	if len(pos) == 2 && pos[0] == "export" {
		return cmdEvalExport(pos[1])
	}
	if len(pos) != 2 {
		fmt.Fprintln(os.Stderr, "usage: veld eval <tasks-dir> <solutions-dir> [--json]")
		return 2
	}
	tasks, _ := filepath.Glob(filepath.Join(pos[0], "*", "tests.veld"))
	sort.Strings(tasks)
	var results []evalResult
	codeCount := map[string]int{}
	solved := 0
	for _, testsPath := range tasks {
		dir := filepath.Dir(testsPath)
		name := filepath.Base(dir)
		r := evalResult{Task: name}
		sol, err := os.ReadFile(filepath.Join(pos[1], name+".veld"))
		if err != nil {
			results = append(results, r)
			continue
		}
		r.Found = true
		tmp, _ := os.MkdirTemp("", "veld-eval-")
		for _, f := range []string{"veld.json", "tests.veld"} {
			b, _ := os.ReadFile(filepath.Join(dir, f))
			os.WriteFile(filepath.Join(tmp, f), b, 0o644)
		}
		os.WriteFile(filepath.Join(tmp, "solution.veld"), sol, 0o644)
		l := project.Load(tmp, []string{filepath.Join(tmp, "tests.veld")})
		seen := map[string]bool{}
		for _, d := range l.Diags.Sorted() {
			if d.Severity != diag.Error {
				continue
			}
			if !seen[d.Code] {
				seen[d.Code] = true
				r.ErrorCodes = append(r.ErrorCodes, d.Code)
				codeCount[d.Code]++
			}
			if len(r.FirstErrors) < 3 {
				r.FirstErrors = append(r.FirstErrors, fmt.Sprintf("%s %d:%d %s", d.Code, d.Span.Start.Line, d.Span.Start.Col, d.Message))
			}
		}
		if !l.Diags.HasErrors() {
			r.Compiles = true
			in := interp.New(l.Program, types.AllEffects&^types.EffectBit("net"), l.Sources)
			in.Stdout = func(string) {}
			for _, m := range l.Entries {
				for _, tr := range in.RunTests(m, "") {
					if tr.Passed {
						r.Passed++
					} else {
						r.Failed++
					}
				}
			}
			r.Solved = r.Failed == 0 && r.Passed > 0
		}
		if r.Solved {
			solved++
		}
		os.RemoveAll(tmp)
		results = append(results, r)
	}
	if flags["json"] == "true" {
		printJSON(map[string]any{"tasks": len(results), "solved": solved, "results": results, "error_code_counts": codeCount})
		return 0
	}
	for _, r := range results {
		status := "SOLVED"
		switch {
		case !r.Found:
			status = "MISSING"
		case !r.Compiles:
			status = "NO-COMPILE " + strings.Join(r.ErrorCodes, ",")
		case !r.Solved:
			status = fmt.Sprintf("FAILED %d/%d tests", r.Failed, r.Passed+r.Failed)
		}
		fmt.Printf("%-24s %s\n", r.Task, status)
	}
	fmt.Printf("\nsolved %d/%d\n", solved, len(results))
	return 0
}

// ---------- new ----------

func cmdNew(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "veld new: expected a directory name")
		return 2
	}
	dir := args[0]
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	main := `use std.io

## Returns a greeting for name.
fn greet(name: Str) -> Str
  "Hello, ${name}!"
end fn

fn main() -> Unit uses io
  io.print(greet("world"))
end fn

test "greet"
  expect greet("Veld") == "Hello, Veld!"
end test
`
	p := filepath.Join(dir, "main.veld")
	if _, err := os.Stat(p); err == nil {
		fmt.Fprintf(os.Stderr, "%s already exists\n", p)
		return 1
	}
	if err := os.WriteFile(p, []byte(main), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("created %s\nrun it:   veld run %s\ntest it:  veld test %s\n", p, p, dir)
	return 0
}

// ---------- files ----------

// expandFiles replaces directories with the .veld files beneath them.
func expandFiles(paths []string) []string {
	var out []string
	for _, p := range paths {
		// Clean, so that paths compare equal to the (cleaned) paths in spans;
		// otherwise `veld fix C:/dir/a.veld` matched no diagnostics on Windows.
		p = filepath.Clean(p)
		st, err := os.Stat(p)
		if err != nil || !st.IsDir() {
			out = append(out, p)
			continue
		}
		filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".veld") {
				out = append(out, path)
			}
			return nil
		})
	}
	return out
}

// loadGroups loads files grouped by project root (see project.FindRoot).
func loadGroups(files []string) []*project.Loaded {
	groups := map[string][]string{}
	var dirs []string
	for _, f := range files {
		d := project.FindRoot(f)
		if _, ok := groups[d]; !ok {
			dirs = append(dirs, d)
		}
		groups[d] = append(groups[d], f)
	}
	var out []*project.Loaded
	for _, d := range dirs {
		out = append(out, project.Load(d, groups[d]))
	}
	return out
}
