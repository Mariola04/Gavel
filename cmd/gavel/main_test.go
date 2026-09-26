package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupData points GAVEL_DATA at a temporary directory that shares the
// repository's exercises, exams and build cache, so reports written by the
// tests do not end up in the repository.
func setupData(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"exercises", "exams", ".cache"} {
		src, err := filepath.Abs(filepath.Join("../../data", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(src, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(src, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GAVEL_DATA", dir)
	t.Setenv("SERVER", "")
}

func runCLI(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRunUsage(t *testing.T) {
	setupData(t)
	tests := []struct {
		args []string
		code int
	}{
		{nil, 2},
		{[]string{"help"}, 0},
		{[]string{"unknown"}, 2},
		{[]string{"show"}, 2},
		{[]string{"exercises", "-difficulty", "impossible"}, 2},
		{[]string{"serve", "-sandbox", "docker"}, 2},
		{[]string{"show", "nope"}, 1},
	}
	for _, tt := range tests {
		if code, _, stderr := runCLI(tt.args...); code != tt.code {
			t.Errorf("gavel %v: exit %d, want %d (stderr: %s)", tt.args, code, tt.code, stderr)
		}
	}
}

func TestRunExercises(t *testing.T) {
	setupData(t)
	code, out, _ := runCLI("exercises", "-difficulty", "easy")
	if code != 0 || !strings.Contains(out, "factorial") || strings.Contains(out, "difícil") {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
}

func TestRunSubmitExitCode(t *testing.T) {
	setupData(t)
	tests := []struct {
		file string
		code int
	}{
		{"correct.go", 0},
		{"wrong_answer.go", 1},
		{"import_os.go", 1},
	}
	for _, tt := range tests {
		code, out, stderr := runCLI("submit", "factorial", filepath.Join("../../testdata/submissions", tt.file))
		if code != tt.code {
			t.Errorf("submit %s: exit %d, want %d\n%s%s", tt.file, code, tt.code, out, stderr)
		}
	}
}
