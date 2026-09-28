package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/ashraf82de/veld/internal/diag"
	"github.com/ashraf82de/veld/internal/interp"
	"github.com/ashraf82de/veld/internal/project"
	"github.com/ashraf82de/veld/internal/types"
)

// cmdReport prints a Markdown bug report for a program: environment, source,
// diagnostics and test results. It is meant to be piped straight into an
// issue (`veld report app/main.veld -m "wrong result" | gh issue create ...`),
// so that reports from agents always contain what a maintainer needs.
func cmdReport(args []string) int {
	flags, files, _ := splitArgs(args, map[string]bool{"m": true, "message": true})
	files = expandFiles(files)
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "veld report: no .veld files given")
		return 2
	}
	message := flags["message"]
	if message == "" {
		message = flags["m"]
	}

	var b strings.Builder
	fmt.Fprintf(&b, "### Environment\n\n`veld %s` on %s/%s (built with %s)\n\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
	if message != "" {
		fmt.Fprintf(&b, "### What happened\n\n%s\n\n", message)
	}

	const maxSource = 24 * 1024
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		text := strings.ReplaceAll(string(src), "\r\n", "\n")
		note := ""
		if len(text) > maxSource {
			text, note = text[:maxSource], "\n(truncated)"
		}
		fmt.Fprintf(&b, "### `%s`\n\n```veld\n%s%s\n```\n\n", f, strings.TrimRight(text, "\n"), note)
	}

	var diags []*diag.Diagnostic
	sources := map[string]string{}
	var loaded []*project.Loaded
	for _, l := range loadGroups(files) {
		loaded = append(loaded, l)
		diags = append(diags, l.Diags.Sorted()...)
		for k, v := range l.Sources {
			sources[k] = v
		}
	}
	failed := false
	for _, d := range diags {
		if d.Severity == diag.Error {
			failed = true
		}
	}
	if len(diags) > 0 {
		fmt.Fprintf(&b, "### `veld check` output\n\n```\n%s```\n\n", diag.Text(diags, sources))
	} else {
		b.WriteString("### `veld check` output\n\nno diagnostics\n\n")
	}

	if !failed {
		var out strings.Builder
		passed, total := 0, 0
		for _, l := range loaded {
			in := interp.New(l.Program, types.AllEffects&^types.EffectBit("net"), l.Sources)
			in.Stdout = func(string) {}
			for _, m := range l.Entries {
				for _, r := range in.RunTests(m, "") {
					total++
					if r.Passed {
						passed++
						continue
					}
					fmt.Fprintf(&out, "FAIL %s: %s\n%s\n", m.Path, r.Name, r.Error)
				}
			}
		}
		fmt.Fprintf(&b, "### `veld test` output\n\n%d of %d tests passed\n", passed, total)
		if out.Len() > 0 {
			fmt.Fprintf(&b, "\n```\n%s```\n", out.String())
		}
	}
	fmt.Print(b.String())
	return 0
}
