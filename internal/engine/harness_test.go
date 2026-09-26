package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"gavel/internal/exercise"
)

func TestGenerateHarnessCompiles(t *testing.T) {
	e := newTestEngine(t)
	signatures := []struct {
		params  []string
		returns string
		zero    string
	}{
		{nil, "int", "0"},
		{[]string{"int"}, "int", "0"},
		{[]string{"float64", "float64"}, "float64", "0"},
		{[]string{"string"}, "bool", "false"},
		{[]string{"[]int", "int"}, "[]int", "nil"},
		{[]string{"map[string]int"}, "map[string][]string", "nil"},
		{[]string{"[][]string"}, "uint64", "0"},
	}
	for _, sig := range signatures {
		name := fmt.Sprintf("(%s) %s", strings.Join(sig.params, ","), sig.returns)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ex := &exercise.Exercise{ID: "x", Function: "F", Params: sig.params, Returns: sig.returns, TimeoutMS: 1000}
			args := make([]string, len(sig.params))
			for i, p := range sig.params {
				args[i] = fmt.Sprintf("_ %s", p)
			}
			code := fmt.Sprintf("package solution\n\nfunc F(%s) %s { return %s }\n", strings.Join(args, ", "), sig.returns, sig.zero)
			dir := t.TempDir()
			if err := writeModule(dir, ex, code); err != nil {
				t.Fatal(err)
			}
			if out, err := e.build(context.Background(), dir); err != nil {
				t.Fatalf("build failed: %v\n%s", err, out)
			}
		})
	}
}

func TestHarnessReportsBadArguments(t *testing.T) {
	e := newTestEngine(t)
	ex := &exercise.Exercise{
		ID: "x", Function: "F", Params: []string{"int"}, Returns: "int", TimeoutMS: 1000,
		Tests: []exercise.Test{
			{Input: []json.RawMessage{json.RawMessage(`"not an int"`)}, Output: json.RawMessage(`1`)},
			{Input: []json.RawMessage{json.RawMessage(`2`)}, Output: json.RawMessage(`2`)},
		},
	}
	dir := t.TempDir()
	if err := writeModule(dir, ex, "package solution\n\nfunc F(n int) int { return n }\n"); err != nil {
		t.Fatal(err)
	}
	if out, err := e.build(context.Background(), dir); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	tests, err := e.run(context.Background(), dir, ex)
	if err != nil {
		t.Fatal(err)
	}
	if tests[0].Passed || !strings.Contains(tests[0].Error, "argumento 0 inválido") {
		t.Errorf("test 0 = %+v, want argument error", tests[0])
	}
	if !tests[1].Passed {
		t.Errorf("test 1 = %+v, want passed", tests[1])
	}
}
