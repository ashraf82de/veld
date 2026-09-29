package tests

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ashraf82de/veld/internal/diag"
	"github.com/ashraf82de/veld/internal/interp"
	"github.com/ashraf82de/veld/internal/project"
	"github.com/ashraf82de/veld/internal/types"
)

// genProgram writes a random program that mutates lists and a string with
// every update form, and observes them in every way (snapshots, aliasing,
// interpolation, passing to functions, iterating while updating).
func genProgram(rng *rand.Rand, stmts int) string {
	var b strings.Builder
	b.WriteString(`use std.list

fn ident(l: List[Int]) -> List[Int]
  l
end fn

fn first_or_zero(l: List[Int]) -> Int
  list.get_or(l, index: 0, fallback: 0)
end fn

fn run() -> Str
  var xs: List[Int] = [1, 2, 3]
  var ys: List[Int] = []
  var s = "a"
  var t = ""
  var log: List[Str] = []
`)
	snaps := 0
	names := []string{"xs", "ys"}
	pick := func() string { return names[rng.Intn(2)] }
	for i := 0; i < stmts; i++ {
		v, w := pick(), pick()
		k := rng.Intn(50)
		switch rng.Intn(22) {
		case 0, 1:
			fmt.Fprintf(&b, "  set %s = list.push(%s, %d)\n", v, v, k)
		case 2, 3:
			fmt.Fprintf(&b, "  if list.len(%s) > 0\n    set %s = list.set_at(%s, index: %d %% list.len(%s), item: %d)\n  end if\n", v, v, v, k, v, rng.Intn(50))
		case 4:
			fmt.Fprintf(&b, "  let snap%d = %s\n", snaps, v)
			snaps++
		case 5:
			fmt.Fprintf(&b, "  set %s = %s\n", v, w)
		case 6:
			fmt.Fprintf(&b, "  set log = list.push(log, \"${%s}\")\n", v)
		case 7:
			fmt.Fprintf(&b, "  set s = s + \"%d\"\n", k)
		case 8:
			fmt.Fprintf(&b, "  set s = s + \"${%s}\" + \"-\"\n", v)
		case 9:
			fmt.Fprintf(&b, "  for x in %s\n    set %s = list.push(%s, x + 1)\n  end for\n", v, w, w)
		case 10:
			fmt.Fprintf(&b, "  set %s = ident(%s)\n", v, w)
		case 11:
			fmt.Fprintf(&b, "  set %s = list.take(%s, %d)\n", v, v, k%7)
		case 12:
			fmt.Fprintf(&b, "  set %s = list.drop(%s, %d)\n", v, v, k%4)
		case 13:
			fmt.Fprintf(&b, "  set %s = %s + %s\n", v, v, w)
		case 14:
			fmt.Fprintf(&b, "  set log = list.push(log, s)\n")
		case 15:
			fmt.Fprintf(&b, "  set t = t + s + \"|\"\n  set s = s + t\n")
		case 16:
			fmt.Fprintf(&b, "  set log = list.push(log, \"${list.len(%s)}:${first_or_zero(%s)}\")\n", v, v)
		case 17:
			fmt.Fprintf(&b, "  match %s\n    case [first, ..rest] =>\n      set %s = list.push(rest, first)\n    case _ => set %s = list.push(%s, 7)\n  end match\n", v, v, v, v)
		case 18:
			fmt.Fprintf(&b, "  set %s = list.reverse(%s)\n", v, v)
		case 19:
			fmt.Fprintf(&b, "  set ys = list.push(ys, list.len(xs))\n  set xs = list.push(xs, list.len(ys))\n")
		case 20:
			fmt.Fprintf(&b, "  set log = list.push(log, \"${t}\")\n  set t = \"\"\n")
		case 21:
			fmt.Fprintf(&b, "  set %s = list.push(%s, first_or_zero(%s))\n", v, v, w)
		}
	}
	b.WriteString("  var out = \"${xs} ${ys} ${s} ${t} ${log}\"\n")
	for i := 0; i < snaps; i++ {
		fmt.Fprintf(&b, "  set out = out + \" ${snap%d}\"\n", i)
	}
	b.WriteString("  out\nend fn\n")
	return b.String()
}

func runProgram(t *testing.T, dir, src string, disable bool) string {
	t.Helper()
	file := filepath.Join(dir, "gen.veld")
	os.WriteFile(file, []byte(src), 0o644)
	l := project.Load(dir, []string{file})
	if l.Diags.HasErrors() {
		t.Fatalf("generated program does not check:\n%s\n%s", diag.Text(l.Diags.Sorted(), l.Sources), src)
	}
	interp.DisableInPlace = disable
	defer func() { interp.DisableInPlace = false }()
	in := interp.New(l.Program, types.AllEffects, l.Sources)
	mod := l.Entries[0]
	th := in.NewThread(mod)
	var out interp.Value
	if err := th.Protect(func() { out = th.CallFunc(mod.Funcs["run"], nil) }); err != nil {
		t.Fatalf("runtime error (in-place disabled=%v): %v\n%s", disable, err, src)
	}
	return out.(string)
}

// TestInPlaceUpdatesAreUnobservable compares random programs run with and
// without in-place updates: the results must be identical.
func TestInPlaceUpdatesAreUnobservable(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	dir := t.TempDir()
	rounds := 300
	if testing.Short() {
		rounds = 50
	}
	for i := 0; i < rounds; i++ {
		src := genProgram(rng, 10+rng.Intn(30))
		want := runProgram(t, dir, src, true)
		got := runProgram(t, dir, src, false)
		if got != want {
			t.Fatalf("in-place updates changed the result of program %d\nwith:    %s\nwithout: %s\n%s", i, got, want, src)
		}
	}
}
