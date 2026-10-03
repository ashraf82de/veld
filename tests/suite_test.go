// Package tests holds the end-to-end test suite: every example and standard
// library module must check and pass its tests, diagnostics must match the
// golden cases in testdata/errors, formatting must be canonical and stable,
// fixes must repair the errors they target, and every emitted code must be
// documented.
package tests

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/ashraf82de/veld/internal/codes"
	"github.com/ashraf82de/veld/internal/diag"
	"github.com/ashraf82de/veld/internal/format"
	"github.com/ashraf82de/veld/internal/interp"
	"github.com/ashraf82de/veld/internal/project"
	"github.com/ashraf82de/veld/internal/syntax"
	"github.com/ashraf82de/veld/internal/types"
)

func veldFiles(t *testing.T, dirs ...string) []string {
	t.Helper()
	var out []string
	for _, d := range dirs {
		filepath.WalkDir(filepath.Join("..", d), func(p string, e os.DirEntry, err error) error {
			if err == nil && !e.IsDir() && strings.HasSuffix(p, ".veld") {
				out = append(out, p)
			}
			return nil
		})
	}
	sort.Strings(out)
	return out
}

func groupByRoot(files []string) map[string][]string {
	g := map[string][]string{}
	for _, f := range files {
		r := project.FindRoot(f)
		g[r] = append(g[r], f)
	}
	return g
}

func runTests(t *testing.T, l *project.Loaded) {
	t.Helper()
	if l.Diags.HasErrors() {
		t.Fatalf("check failed:\n%s", diag.Text(l.Diags.Sorted(), l.Sources))
	}
	for _, d := range l.Diags.Items {
		if d.Severity == diag.Warning {
			t.Errorf("unexpected warning: %s %s: %s", d.Span, d.Code, d.Message)
		}
	}
	in := interp.New(l.Program, types.AllEffects&^types.EffectBit("net"), l.Sources)
	in.Stdout = func(string) {}
	for _, m := range l.Entries {
		for _, r := range in.RunTests(m, "") {
			if !r.Passed {
				t.Errorf("%s: test %q failed: %v", m.Path, r.Name, r.Error)
			}
		}
	}
}

func TestStdlib(t *testing.T) {
	for _, name := range project.StdModules() {
		t.Run(name, func(t *testing.T) { runTests(t, project.LoadStd(name)) })
	}
}

func TestExamples(t *testing.T) {
	files := veldFiles(t, "examples")
	for root, fs := range groupByRoot(files) {
		t.Run(filepath.Base(root), func(t *testing.T) { runTests(t, project.Load(root, fs)) })
	}
}

// TestSemantics runs the runtime-behaviour tests in testdata/semantics (control
// flow, closures, patterns, recursion) so backend changes cannot alter them.
func TestSemantics(t *testing.T) {
	for _, f := range veldFiles(t, "testdata/semantics") {
		t.Run(filepath.Base(f), func(t *testing.T) { runTests(t, project.Load("", []string{f})) })
	}
}

var expectRe = regexp.MustCompile(`^# expect: (.*)$`)

func TestDiagnostics(t *testing.T) {
	for _, f := range veldFiles(t, "testdata/errors") {
		t.Run(filepath.Base(f), func(t *testing.T) {
			src, _ := os.ReadFile(f)
			m := expectRe.FindStringSubmatch(strings.SplitN(string(src), "\n", 2)[0])
			if m == nil {
				t.Fatalf("missing `# expect: CODE` header")
			}
			want := map[string]bool{}
			for _, c := range strings.Fields(m[1]) {
				want[c] = true
			}
			l := project.Load("", []string{f})
			got := map[string]bool{}
			for _, d := range l.Diags.Items {
				got[d.Code] = true
				if d.Severity == diag.Error && !want[d.Code] {
					t.Errorf("unexpected error %s: %s", d.Code, d.Message)
				}
			}
			for c := range want {
				if !got[c] {
					t.Errorf("expected %s, got:\n%s", c, diag.Text(l.Diags.Sorted(), l.Sources))
				}
			}
		})
	}
}

// TestFixesRepair checks that applying the suggested fixes removes the
// targeted errors, for every golden case whose errors all carry fixes.
func TestFixesRepair(t *testing.T) {
	for _, f := range veldFiles(t, "testdata/errors") {
		src, _ := os.ReadFile(f)
		l := project.Load("", []string{f})
		var fixes []diag.Fix
		fixable := l.Diags.HasErrors()
		for _, d := range l.Diags.Items {
			if d.Severity != diag.Error {
				continue
			}
			if len(d.Fixes) == 0 {
				fixable = false
				break
			}
			fixes = append(fixes, d.Fixes[0])
		}
		if !fixable {
			continue
		}
		t.Run(filepath.Base(f), func(t *testing.T) {
			fixed, _ := diag.ApplyFixes(string(src), fixes)
			tmp := filepath.Join(t.TempDir(), filepath.Base(f))
			os.WriteFile(tmp, []byte(fixed), 0o644)
			l2 := project.Load("", []string{tmp})
			if l2.Diags.HasErrors() {
				t.Errorf("errors remain after applying fixes:\n%s\n--- fixed source ---\n%s", diag.Text(l2.Diags.Sorted(), l2.Sources), fixed)
			}
		})
	}
}

func TestFormatCanonical(t *testing.T) {
	for _, f := range veldFiles(t, "std", "examples", "evals", "testdata/semantics") {
		t.Run(f, func(t *testing.T) {
			src, _ := os.ReadFile(f)
			ds := &diag.List{}
			out := format.File(syntax.Parse(f, string(src), ds))
			if ds.HasErrors() {
				t.Fatalf("parse errors:\n%s", diag.Text(ds.Sorted(), nil))
			}
			if out != string(src) {
				t.Errorf("file is not in canonical form (run `veld fmt %s`)", f)
			}
		})
	}
}

func TestFormatIdempotent(t *testing.T) {
	for _, f := range veldFiles(t, "std", "examples", "evals", "testdata") {
		src, _ := os.ReadFile(f)
		ds := &diag.List{}
		ast := syntax.Parse(f, string(src), ds)
		if ds.HasErrors() {
			continue
		}
		once := format.File(ast)
		ds2 := &diag.List{}
		twice := format.File(syntax.Parse(f, once, ds2))
		if ds2.HasErrors() {
			t.Errorf("%s: formatted output does not parse:\n%s", f, diag.Text(ds2.Sorted(), nil))
		} else if once != twice {
			t.Errorf("%s: formatting is not idempotent", f)
		}
	}
}

var codeRe = regexp.MustCompile(`"([EWHRT]\d{3})"`)

func TestCodesDocumented(t *testing.T) {
	filepath.WalkDir("../internal", func(p string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() || !strings.HasSuffix(p, ".go") || strings.Contains(p, "codes") {
			return nil
		}
		src, _ := os.ReadFile(p)
		for _, m := range codeRe.FindAllStringSubmatch(string(src), -1) {
			if _, ok := codes.All[m[1]]; !ok && m[1] != "E399" && m[1] != "R999" {
				t.Errorf("%s: code %s is not documented in internal/codes", p, m[1])
			}
		}
		return nil
	})
}

// TestGuideExample keeps the complete example in docs/LANGUAGE.md working.
func TestGuideExample(t *testing.T) {
	doc, _ := os.ReadFile("../docs/LANGUAGE.md")
	_, after, ok := strings.Cut(string(doc), "## Complete example")
	if !ok {
		t.Fatal("guide has no complete example")
	}
	_, code, _ := strings.Cut(after, "```\n")
	code, _, _ = strings.Cut(code, "```")
	f := filepath.Join(t.TempDir(), "guide.veld")
	os.WriteFile(f, []byte(code), 0o644)
	runTests(t, project.Load("", []string{f}))
	ds := &diag.List{}
	if out := format.File(syntax.Parse(f, code, ds)); out != code {
		t.Errorf("guide example is not canonically formatted:\n%s", out)
	}
}

// TestEvalReferences checks that every eval task's reference solution passes
// the task's tests, exactly as a candidate solution would be graded.
func TestEvalReferences(t *testing.T) {
	dirs, _ := filepath.Glob("../evals/tasks/*")
	for _, d := range dirs {
		t.Run(filepath.Base(d), func(t *testing.T) {
			tmp := t.TempDir()
			for _, f := range []string{"veld.json", "tests.veld"} {
				b, err := os.ReadFile(filepath.Join(d, f))
				if err != nil {
					t.Fatal(err)
				}
				os.WriteFile(filepath.Join(tmp, f), b, 0o644)
			}
			ref, _ := os.ReadFile(filepath.Join(d, "reference.veld"))
			os.WriteFile(filepath.Join(tmp, "solution.veld"), ref, 0o644)
			runTests(t, project.Load(tmp, []string{filepath.Join(tmp, "tests.veld")}))
		})
	}
}

// TestCRLF checks that files with Windows line endings parse to the same
// program as their LF form: formatting them yields the canonical LF text.
func TestCRLF(t *testing.T) {
	for _, f := range veldFiles(t, "std", "examples", "evals", "testdata/semantics") {
		src, _ := os.ReadFile(f)
		crlf := strings.ReplaceAll(string(src), "\n", "\r\n")
		ds := &diag.List{}
		out := format.File(syntax.Parse(f, crlf, ds))
		if ds.HasErrors() {
			t.Errorf("%s: CRLF source does not parse:\n%s", f, diag.Text(ds.Sorted(), nil))
		} else if out != string(src) {
			t.Errorf("%s: formatting the CRLF form does not give the canonical LF form", f)
		}
	}
}

// TestCRLFRawString pins that raw multi-line strings do not pick up carriage
// returns from Windows line endings.
func TestCRLFRawString(t *testing.T) {
	src := "fn text() -> Str\r\n  \"\"\"\r\n  a\r\n  b\r\n  \"\"\"\r\nend fn\r\n"
	ds := &diag.List{}
	f := syntax.Parse("raw.veld", src, ds)
	if ds.HasErrors() {
		t.Fatalf("parse errors:\n%s", diag.Text(ds.Sorted(), nil))
	}
	if strings.Contains(format.File(f), "\r") {
		t.Errorf("formatted output contains a carriage return")
	}
}

// TestConcurrentThreads runs interpreted code from many goroutines at once, as
// an HTTP server does: the shared program must be race-free (run with -race),
// std.state updates must be atomic, and persistent lists shared between
// threads must never change under a reader.
func TestConcurrentThreads(t *testing.T) {
	src := `use std.json
use std.list
use std.state

fn bump() -> Int uses state
  state.incr("counter", by: 1)
end fn

fn record_max(n: Int) -> Json uses state
  state.update("max", fn(old) => json.int(pick_max(old, n)))
end fn

fn pick_max(old: Option[Json], n: Int) -> Int
  match old
    case Some(v) => if json.as_int(v) == Some(n)
      n
    else
      match json.as_int(v)
        case Some(m) => if m > n
          m
        else
          n
        end if
        case None => n
      end match
    end if
    case None => n
  end match
end fn

fn sum_shared(xs: List[Int]) -> Int
  var total = 0
  for x in xs
    set total = total + x
  end for
  total
end fn

fn grow(xs: List[Int]) -> List[Int]
  var out = xs
  for i in list.range(0, 200)
    set out = list.push(out, i)
    set out = list.set_at(out, index: 0, item: i)
  end for
  out
end fn
`
	dir := t.TempDir()
	file := filepath.Join(dir, "conc.veld")
	os.WriteFile(file, []byte(src), 0o644)
	l := project.Load(dir, []string{file})
	if l.Diags.HasErrors() {
		t.Fatalf("check failed:\n%s", diag.Text(l.Diags.Sorted(), l.Sources))
	}
	in := interp.New(l.Program, types.AllEffects&^types.EffectBit("net"), l.Sources)
	mod := l.Entries[0]
	shared := interp.NewList(func() []interp.Value {
		out := make([]interp.Value, 1000)
		for i := range out {
			out[i] = int64(i)
		}
		return out
	}())
	const workers, rounds = 16, 200
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			th := in.NewThread(mod)
			errs <- th.Protect(func() {
				for i := 0; i < rounds; i++ {
					th.CallFunc(mod.Funcs["bump"], nil)
					th.CallFunc(mod.Funcs["record_max"], []interp.Value{int64(w*rounds + i)})
					if got := th.CallFunc(mod.Funcs["sum_shared"], []interp.Value{shared}); got != int64(499500) {
						panic(fmt.Sprintf("shared list changed under a reader: sum = %v", got))
					}
					grown := th.CallFunc(mod.Funcs["grow"], []interp.Value{shared}).(interp.List)
					if grown.Len() != 1200 || grown.Get(0) != int64(199) {
						panic("grow returned a wrong list")
					}
				}
			})
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if shared.Get(0) != int64(0) || shared.Len() != 1000 {
		t.Fatal("the shared list was modified")
	}
	th := in.NewThread(mod)
	if got := th.CallFunc(mod.Funcs["bump"], nil); got != int64(workers*rounds+1) {
		t.Errorf("lost updates: counter = %v, want %d", got, workers*rounds+1)
	}
}

// TestDatasetInSync keeps the prepared Hub export in step with evals/tasks.
// Regenerate it with:
// `veld eval export evals/tasks > huggingface/veld-evals.jsonl`.
func TestDatasetInSync(t *testing.T) {
	dirs, err := filepath.Glob("../evals/tasks/*")
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) == 0 {
		t.Fatal("no eval tasks found")
	}
	data, err := os.ReadFile("../huggingface/veld-evals.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != len(dirs) {
		t.Fatalf("dataset has %d tasks, evals/tasks has %d: regenerate huggingface/veld-evals.jsonl", len(lines), len(dirs))
	}
	for i, d := range dirs {
		t.Run(filepath.Base(d), func(t *testing.T) {
			var row map[string]string
			if err := json.Unmarshal([]byte(lines[i]), &row); err != nil {
				t.Fatalf("dataset line %d is not a string-valued JSON object: %v", i+1, err)
			}
			if row["task_id"] != filepath.Base(d) {
				t.Errorf("dataset line %d has task_id %q, want %q", i+1, row["task_id"], filepath.Base(d))
			}
			if row["language"] != "veld" {
				t.Errorf("dataset language = %q, want veld", row["language"])
			}
			for field, name := range map[string]string{
				"prompt":    "prompt.md",
				"tests":     "tests.veld",
				"reference": "reference.veld",
			} {
				src, err := os.ReadFile(filepath.Join(d, name))
				if err != nil {
					t.Fatal(err)
				}
				got, ok := row[field]
				if !ok {
					t.Errorf("dataset line %d lacks %s", i+1, field)
					continue
				}
				// Git may check out CRLF on Windows; preserve all other whitespace.
				want := strings.ReplaceAll(string(src), "\r\n", "\n")
				got = strings.ReplaceAll(got, "\r\n", "\n")
				if got != want {
					t.Errorf("dataset %s differs from %s: regenerate huggingface/veld-evals.jsonl", field, filepath.Join(d, name))
				}
			}
		})
	}
}
