// Package tests holds the end-to-end test suite: every example and standard
// library module must check and pass its tests, diagnostics must match the
// golden cases in testdata/errors, formatting must be canonical and stable,
// fixes must repair the errors they target, and every emitted code must be
// documented.
package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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
	for _, f := range veldFiles(t, "std", "examples", "evals") {
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
