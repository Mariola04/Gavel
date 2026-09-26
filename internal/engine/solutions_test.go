package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestReferenceSolutions checks that every exercise's reference solution
// passes its own tests through the full pipeline.
func TestReferenceSolutions(t *testing.T) {
	e := newTestEngine(t)
	for _, ex := range loadRepo(t).List("") {
		t.Run(ex.ID, func(t *testing.T) {
			t.Parallel()
			code, err := os.ReadFile(filepath.Join(exercisesDir, ex.ID, "solution.go"))
			if err != nil {
				t.Fatal(err)
			}
			r := e.Evaluate(context.Background(), ex, string(code))
			if r.Summary.Verdict != VerdictPassed {
				t.Fatalf("verdict = %s (stopped at %q)\nchecks: %+v\ntests: %+v",
					r.Summary.Verdict, r.Summary.StoppedAt, r.Static.Checks, r.Dynamic.Tests)
			}
			for _, c := range r.Static.Checks {
				if c.Status == StatusError || (c.Status == StatusWarning && c.Name != StageSandbox) {
					t.Errorf("check %s: %s %v", c.Name, c.Status, c.Messages)
				}
			}
		})
	}
}
