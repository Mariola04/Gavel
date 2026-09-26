package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gavel/internal/exercise"
)

const (
	exercisesDir   = "../../data/exercises"
	submissionsDir = "../../testdata/submissions"
	// testCacheDir is the same build cache the application uses, so tests
	// do not rebuild the standard library every time.
	testCacheDir = "../../data/.cache"
)

var (
	repoOnce sync.Once
	repo     *exercise.Repository
	errRepo  error
)

func loadRepo(t *testing.T) *exercise.Repository {
	t.Helper()
	repoOnce.Do(func() { repo, errRepo = exercise.Load(exercisesDir) })
	if errRepo != nil {
		t.Fatalf("load exercises: %v", errRepo)
	}
	return repo
}

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := New(Config{CacheDir: testCacheDir, Sandbox: SandboxAuto})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return e
}

func TestEvaluateSubmissions(t *testing.T) {
	e := newTestEngine(t)
	factorial, ok := loadRepo(t).Get("factorial")
	if !ok {
		t.Fatal("exercise factorial not found")
	}
	tests := []struct {
		file      string
		verdict   Verdict
		stoppedAt string
		check     string // a check expected to have a non-ok status
		message   string // a substring expected somewhere in the report
	}{
		{file: "correct.go", verdict: VerdictPassed},
		{file: "wrong_answer.go", verdict: VerdictFailed, message: "Ver caso base"},
		{file: "syntax_error.go", verdict: VerdictRejected, stoppedAt: StageParse, check: StageParse},
		{file: "type_error.go", verdict: VerdictCompileError, stoppedAt: StageVet, check: StageVet, message: "cannot use"},
		{file: "import_os.go", verdict: VerdictRejected, stoppedAt: StageAST, check: StageAST, message: "import não permitido: os"},
		{file: "import_net_http.go", verdict: VerdictRejected, stoppedAt: StageAST, check: StageAST, message: "import não permitido: net/http"},
		{file: "infinite_loop.go", verdict: VerdictTimeout, message: errTimeout},
		{file: "panic.go", verdict: VerdictFailed, message: "panic: não sei calcular 5!"},
		{file: "unformatted.go", verdict: VerdictPassed, check: StageFormat, message: "código não formatado"},
		{file: "println.go", verdict: VerdictPassed},
		{file: "forged_output.go", verdict: VerdictFailed},
		{file: "vet_error.go", verdict: VerdictRejected, stoppedAt: StageVet, check: StageVet, message: "wrong type"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()
			code, err := os.ReadFile(filepath.Join(submissionsDir, tt.file))
			if err != nil {
				t.Fatal(err)
			}
			r := e.Evaluate(context.Background(), factorial, string(code))
			if r.Summary.Verdict != tt.verdict || r.Summary.StoppedAt != tt.stoppedAt {
				t.Fatalf("verdict = %s, stopped at %q; want %s, %q\nreport: %+v",
					r.Summary.Verdict, r.Summary.StoppedAt, tt.verdict, tt.stoppedAt, r)
			}
			if tt.check != "" && checkStatus(r, tt.check) == StatusOK {
				t.Errorf("check %s has status ok", tt.check)
			}
			if tt.message != "" && !reportContains(r, tt.message) {
				t.Errorf("report does not mention %q: %+v", tt.message, r)
			}
		})
	}
}

func TestEvaluateScoresPartialResults(t *testing.T) {
	e := newTestEngine(t)
	factorial, _ := loadRepo(t).Get("factorial")
	code, err := os.ReadFile(filepath.Join(submissionsDir, "panic.go"))
	if err != nil {
		t.Fatal(err)
	}
	r := e.Evaluate(context.Background(), factorial, string(code))
	if !r.Dynamic.Executed || r.Summary.Total != len(factorial.Tests) || r.Summary.Passed != r.Summary.Total-1 {
		t.Fatalf("summary = %+v", r.Summary)
	}
	for _, tr := range r.Dynamic.Tests {
		if tr.Passed && tr.Hint != "" {
			t.Errorf("passed test shows hint %q", tr.Hint)
		}
	}
}

func TestEvaluateRejectsOversizedCode(t *testing.T) {
	e := newTestEngine(t)
	factorial, _ := loadRepo(t).Get("factorial")
	r := e.Evaluate(context.Background(), factorial, strings.Repeat(" ", MaxCodeSize+1))
	if r.Summary.Verdict != VerdictRejected || r.Summary.StoppedAt != StageLimits {
		t.Fatalf("summary = %+v", r.Summary)
	}
}

func checkStatus(r *Report, name string) Status {
	for _, c := range r.Static.Checks {
		if c.Name == name {
			return c.Status
		}
	}
	return StatusOK
}

func reportContains(r *Report, s string) bool {
	for _, c := range r.Static.Checks {
		for _, m := range c.Messages {
			if strings.Contains(m, s) {
				return true
			}
		}
	}
	for _, tr := range r.Dynamic.Tests {
		if strings.Contains(tr.Error, s) || strings.Contains(tr.Hint, s) {
			return true
		}
	}
	return false
}
