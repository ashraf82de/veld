package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ashraf82de/veld/internal/diag"
	"github.com/ashraf82de/veld/internal/format"
	"github.com/ashraf82de/veld/internal/project"
	"github.com/ashraf82de/veld/internal/syntax"
)

// seedCorpus feeds the repository's own Veld files to the fuzzers.
func seedCorpus(f *testing.F) {
	for _, dir := range []string{"std", "examples", "evals", "testdata"} {
		filepath.WalkDir(filepath.Join("..", dir), func(p string, e os.DirEntry, err error) error {
			if err == nil && !e.IsDir() && filepath.Ext(p) == ".veld" {
				if b, err := os.ReadFile(p); err == nil && len(b) < 8000 {
					f.Add(string(b))
				}
			}
			return nil
		})
	}
	f.Add("fn main() -> Unit\nend fn\n")
	f.Add("match a, b\n  case 1, 2 => 3\nend match")
}

// FuzzParseFormat: parsing and formatting never panic, and formatting a
// program that parses cleanly gives a program that parses cleanly and is
// stable under a second formatting.
func FuzzParseFormat(f *testing.F) {
	seedCorpus(f)
	f.Fuzz(func(t *testing.T, src string) {
		ds := &diag.List{}
		once := format.File(syntax.Parse("fuzz.veld", src, ds))
		if ds.HasErrors() {
			return
		}
		ds2 := &diag.List{}
		twice := format.File(syntax.Parse("fuzz.veld", once, ds2))
		if ds2.HasErrors() {
			t.Fatalf("formatted output does not parse:\n%s\n--- output ---\n%s", diag.Text(ds2.Sorted(), nil), once)
		}
		if once != twice {
			t.Fatalf("formatting is not idempotent:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
		}
	})
}

// FuzzCheck: the whole front end (loader, checker, diagnostics, fixes) never
// panics on arbitrary source.
func FuzzCheck(f *testing.F) {
	seedCorpus(f)
	dir := f.TempDir()
	file := filepath.Join(dir, "fuzz.veld")
	f.Fuzz(func(t *testing.T, src string) {
		os.WriteFile(file, []byte(src), 0o644)
		l := project.Load(dir, []string{file})
		for _, d := range l.Diags.Sorted() {
			if len(d.Fixes) > 0 {
				diag.ApplyFixes(src, d.Fixes[:1])
			}
		}
		_ = diag.Text(l.Diags.Sorted(), l.Sources)
		_ = diag.JSON(l.Diags.Sorted())
	})
}
