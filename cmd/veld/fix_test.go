package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func captureFix(t *testing.T, args ...string) (int, string) {
	t.Helper()
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = out
	defer func() { os.Stdout = previous; out.Close() }()
	code := cmdFix(args)
	b, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	return code, string(b)
}

func TestFixStdoutPreservesSources(t *testing.T) {
	for _, withImport := range []bool{false, true} {
		name := "single file"
		if withImport {
			name = "local import"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			source := "pub fn answer() -> Int\n  let value = 1\n  vlaue\nend fn\n"
			file := filepath.Join(dir, "answer.veld")
			if err := os.WriteFile(file, []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			args := []string{file, "--stdout"}
			if withImport {
				main := filepath.Join(dir, "main.veld")
				if err := os.WriteFile(main, []byte("use answer\nfn main() -> Int\n  answer.answer()\nend fn\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				args = append(args, main)
			}
			code, output := captureFix(t, args...)
			if code != 0 || !strings.Contains(output, "\n  value\n") || strings.Contains(output, "vlaue") {
				t.Fatalf("fix preview: exit %d, output %q", code, output)
			}
			b, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != source {
				t.Fatalf("--stdout changed source on disk: %q", b)
			}
		})
	}
}

func TestFixStdoutDoesNotTouchValidFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "answer.veld")
	source := "pub fn answer() -> Int\n  1\nend fn\n"
	if err := os.WriteFile(file, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(file, old, old); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	code, output := captureFix(t, file, "--stdout")
	if code != 0 || output != source {
		t.Fatalf("fix preview: exit %d, output %q", code, output)
	}
	after, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("--stdout rewrote a valid file")
	}
}

func TestFixWritesRepairedSource(t *testing.T) {
	file := filepath.Join(t.TempDir(), "answer.veld")
	source := "pub fn answer() -> Int\n  let value = 1\n  vlaue\nend fn\n"
	if err := os.WriteFile(file, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _ := captureFix(t, file)
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || string(b) != strings.ReplaceAll(source, "vlaue", "value") {
		t.Fatalf("fix: exit %d, source %q", code, b)
	}
}
