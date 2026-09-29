package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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
	if flags["url"] == "true" || flags["feedback"] == "true" {
		fmt.Println(issueURL(flags["feedback"] == "true", message, b.String()))
		return 0
	}
	fmt.Print(b.String())
	return 0
}

const issuesNew = "https://github.com/ashraf82de/veld/issues/new"

// issueURL builds a link that opens the right issue form with the report
// already filled in, so an agent (or its operator) can file it with one click.
// The report is truncated to keep the URL within what browsers accept.
func issueURL(feedback bool, message, md string) string {
	const budget = 2500
	if len(md) > budget {
		md = md[:budget] + "\n\n(truncated: run veld report without --url for the full text)"
	}
	q := url.Values{}
	if feedback {
		q.Set("template", "agent_feedback.yml")
		q.Set("task", message)
		q.Set("friction", md)
	} else {
		q.Set("template", "bug_report.yml")
		q.Set("report", md)
	}
	return issuesNew + "?" + q.Encode()
}

// cmdEvalExport prints the eval tasks as JSON lines (one object per task), the
// format used for the published dataset: task id, prompt, hidden tests and a
// reference solution. `veld eval export <tasks-dir>`.
func cmdEvalExport(dir string) int {
	tasks, _ := filepath.Glob(filepath.Join(dir, "*", "tests.veld"))
	sort.Strings(tasks)
	if len(tasks) == 0 {
		fmt.Fprintln(os.Stderr, "veld eval export: no tasks found in", dir)
		return 2
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	for _, t := range tasks {
		d := filepath.Dir(t)
		read := func(name string) string {
			b, _ := os.ReadFile(filepath.Join(d, name))
			return strings.ReplaceAll(string(b), "\r\n", "\n")
		}
		enc.Encode(map[string]string{
			"task_id":   filepath.Base(d),
			"prompt":    read("prompt.md"),
			"tests":     read("tests.veld"),
			"reference": read("reference.veld"),
			"language":  "veld",
			"version":   version,
		})
	}
	return 0
}
