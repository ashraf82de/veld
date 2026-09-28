// Package diag defines machine-readable diagnostics shared by every stage of
// the Veld toolchain. Every diagnostic has a stable code, an exact span, and,
// where possible, a concrete fix an agent can apply mechanically.
package diag

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Pos is a 1-based line/column position plus a 0-based byte offset.
type Pos struct {
	Line   int `json:"line"`
	Col    int `json:"col"`
	Offset int `json:"offset"`
}

// Span is a half-open source range [Start, End).
type Span struct {
	File  string `json:"file"`
	Start Pos    `json:"start"`
	End   Pos    `json:"end"`
}

func (s Span) String() string {
	return fmt.Sprintf("%s:%d:%d", s.File, s.Start.Line, s.Start.Col)
}

// IsZero reports whether the span is unset.
func (s Span) IsZero() bool { return s.Start.Line == 0 }

// Join returns the smallest span covering a and b (same file assumed).
func Join(a, b Span) Span {
	if a.IsZero() {
		return b
	}
	if b.IsZero() {
		return a
	}
	out := a
	if b.Start.Offset < out.Start.Offset {
		out.Start = b.Start
	}
	if b.End.Offset > out.End.Offset {
		out.End = b.End
	}
	return out
}

type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
	Info    Severity = "info"
)

// Edit replaces the text in Span with Text. An empty span (Start == End)
// is an insertion.
type Edit struct {
	Span Span   `json:"span"`
	Text string `json:"text"`
}

// Fix is a named, mechanically applicable set of edits.
type Fix struct {
	Message string `json:"message"`
	Edits   []Edit `json:"edits"`
}

// Diagnostic is one compiler message.
type Diagnostic struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Span     Span     `json:"span"`
	Notes    []string `json:"notes,omitempty"`
	Fixes    []Fix    `json:"fixes,omitempty"`
}

// List accumulates diagnostics.
type List struct {
	Items []*Diagnostic
}

func (l *List) Add(d *Diagnostic) *Diagnostic {
	l.Items = append(l.Items, d)
	return d
}

func (l *List) Errorf(code string, span Span, format string, args ...any) *Diagnostic {
	return l.Add(&Diagnostic{Code: code, Severity: Error, Span: span, Message: fmt.Sprintf(format, args...)})
}

func (l *List) Warnf(code string, span Span, format string, args ...any) *Diagnostic {
	return l.Add(&Diagnostic{Code: code, Severity: Warning, Span: span, Message: fmt.Sprintf(format, args...)})
}

func (l *List) Infof(code string, span Span, format string, args ...any) *Diagnostic {
	return l.Add(&Diagnostic{Code: code, Severity: Info, Span: span, Message: fmt.Sprintf(format, args...)})
}

func (l *List) HasErrors() bool {
	for _, d := range l.Items {
		if d.Severity == Error {
			return true
		}
	}
	return false
}

func (l *List) Merge(o *List) {
	l.Items = append(l.Items, o.Items...)
}

// Sorted returns diagnostics ordered by file, then position, deduplicated.
func (l *List) Sorted() []*Diagnostic {
	out := make([]*Diagnostic, 0, len(l.Items))
	seen := map[string]bool{}
	for _, d := range l.Items {
		key := fmt.Sprintf("%s|%s|%d|%d|%s", d.Code, d.Span.File, d.Span.Start.Line, d.Span.Start.Col, d.Message)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, d)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Span, out[j].Span
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Start.Offset < b.Start.Offset
	})
	return out
}

// Note appends an explanatory note.
func (d *Diagnostic) Note(format string, args ...any) *Diagnostic {
	d.Notes = append(d.Notes, fmt.Sprintf(format, args...))
	return d
}

// WithFix attaches a single-edit fix.
func (d *Diagnostic) WithFix(msg string, span Span, text string) *Diagnostic {
	d.Fixes = append(d.Fixes, Fix{Message: msg, Edits: []Edit{{Span: span, Text: text}}})
	return d
}

// JSON renders diagnostics as a JSON document.
func JSON(ds []*Diagnostic) string {
	if ds == nil {
		ds = []*Diagnostic{}
	}
	b, _ := json.MarshalIndent(map[string]any{"diagnostics": ds, "ok": !hasErr(ds)}, "", "  ")
	return string(b)
}

func hasErr(ds []*Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == Error {
			return true
		}
	}
	return false
}

// Text renders diagnostics for humans (and for agents that prefer text):
// file:line:col: severity[CODE]: message, followed by the source line.
func Text(ds []*Diagnostic, sources map[string]string) string {
	var b strings.Builder
	for _, d := range ds {
		fmt.Fprintf(&b, "%s:%d:%d: %s[%s]: %s\n", d.Span.File, d.Span.Start.Line, d.Span.Start.Col, d.Severity, d.Code, d.Message)
		if src, ok := sources[d.Span.File]; ok && d.Span.Start.Line > 0 {
			lines := strings.Split(src, "\n")
			if d.Span.Start.Line-1 < len(lines) {
				line := lines[d.Span.Start.Line-1]
				fmt.Fprintf(&b, "  | %s\n", line)
				width := 1
				if d.Span.End.Line == d.Span.Start.Line && d.Span.End.Col > d.Span.Start.Col {
					width = d.Span.End.Col - d.Span.Start.Col
				}
				fmt.Fprintf(&b, "  | %s%s\n", strings.Repeat(" ", max(0, d.Span.Start.Col-1)), strings.Repeat("^", width))
			}
		}
		for _, n := range d.Notes {
			fmt.Fprintf(&b, "  = note: %s\n", n)
		}
		for _, f := range d.Fixes {
			fmt.Fprintf(&b, "  = fix: %s\n", f.Message)
			for _, e := range f.Edits {
				if e.Span.Start.Offset == e.Span.End.Offset {
					fmt.Fprintf(&b, "      insert at %d:%d: %q\n", e.Span.Start.Line, e.Span.Start.Col, e.Text)
				} else {
					fmt.Fprintf(&b, "      replace %d:%d-%d:%d with: %q\n", e.Span.Start.Line, e.Span.Start.Col, e.Span.End.Line, e.Span.End.Col, e.Text)
				}
			}
		}
	}
	return b.String()
}

// Suggest returns the candidate closest to name by edit distance, or "".
func Suggest(name string, candidates []string) string {
	best, bestD := "", 1<<30
	limit := max(1, len(name)/3+1)
	for _, c := range candidates {
		if c == name {
			continue
		}
		d := levenshtein(strings.ToLower(name), strings.ToLower(c))
		if d < bestD && d <= limit {
			best, bestD = c, d
		}
	}
	return best
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// ApplyFixes applies the edits of the given fixes to src. Edits are applied
// from the end of the file backwards; an edit overlapping one already
// applied is skipped. It returns the new source and the number of fixes
// applied.
func ApplyFixes(src string, fixes []Fix) (string, int) {
	type edit struct {
		start, end int
		text       string
		fix        int
	}
	var edits []edit
	for i, f := range fixes {
		for _, e := range f.Edits {
			s, en := e.Span.Start.Offset, e.Span.End.Offset
			if en < s {
				en = s
			}
			if s < 0 || en > len(src) {
				continue
			}
			edits = append(edits, edit{s, en, e.Text, i})
		}
	}
	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].start != edits[j].start {
			return edits[i].start > edits[j].start
		}
		return edits[i].fix > edits[j].fix
	})
	applied := map[int]bool{}
	skipped := map[int]bool{}
	limit := len(src) + 1
	for _, e := range edits {
		if skipped[e.fix] {
			continue
		}
		if e.end > limit || (e.end == limit && e.start != e.end) {
			skipped[e.fix] = true
			continue
		}
		src = src[:e.start] + e.text + src[e.end:]
		limit = e.start
		applied[e.fix] = true
	}
	return src, len(applied)
}
