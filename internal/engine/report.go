package engine

import (
	"encoding/json"
	"time"
)

// Status is the outcome of a single check.
type Status string

// Possible check statuses.
const (
	StatusOK      Status = "ok"
	StatusWarning Status = "warning"
	StatusError   Status = "error"
	StatusSkipped Status = "skipped"
)

// Verdict is the overall outcome of a submission.
type Verdict string

// Possible verdicts.
const (
	VerdictPassed        Verdict = "passed"
	VerdictFailed        Verdict = "failed"
	VerdictRejected      Verdict = "rejected"
	VerdictCompileError  Verdict = "compile_error"
	VerdictTimeout       Verdict = "timeout"
	VerdictInternalError Verdict = "internal_error"
)

// Pipeline stage names, as they appear in checks and in Summary.StoppedAt.
const (
	StageLimits     = "limits"
	StageParse      = "parse"
	StageAST        = "ast"
	StageFormat     = "gofmt"
	StageComplexity = "complexity"
	StageVet        = "vet"
	StageGosec      = "gosec"
	StageBuild      = "build"
	StageRun        = "run"
	StageSandbox    = "sandbox"
)

// Check is the result of one pipeline stage.
type Check struct {
	Name     string   `json:"name"`
	Status   Status   `json:"status"`
	Messages []string `json:"messages,omitempty"`
}

// StaticResult groups the checks that run before execution.
type StaticResult struct {
	Passed bool    `json:"passed"`
	Checks []Check `json:"checks"`
}

// TestResult is the outcome of one test case.
type TestResult struct {
	Input      []json.RawMessage `json:"input"`
	Expected   json.RawMessage   `json:"expected"`
	Got        json.RawMessage   `json:"got,omitempty"`
	Error      string            `json:"error,omitempty"`
	Passed     bool              `json:"passed"`
	DurationMS int64             `json:"duration_ms"`
	Hint       string            `json:"hint,omitempty"`
}

// DynamicResult holds the execution results.
type DynamicResult struct {
	Executed bool         `json:"executed"`
	Tests    []TestResult `json:"tests"`
}

// Summary is the overall result.
type Summary struct {
	Verdict   Verdict `json:"verdict"`
	Passed    int     `json:"passed"`
	Total     int     `json:"total"`
	Score     float64 `json:"score"`
	StoppedAt string  `json:"stopped_at"`
}

// Report is the full evaluation of a submission.
type Report struct {
	SubmissionID string        `json:"submission_id"`
	ExerciseID   string        `json:"exercise_id"`
	AttemptID    string        `json:"attempt_id"`
	SubmittedAt  time.Time     `json:"submitted_at"`
	Code         string        `json:"code"`
	Static       StaticResult  `json:"static"`
	Dynamic      DynamicResult `json:"dynamic"`
	Summary      Summary       `json:"summary"`
}

func (r *Report) addCheck(name string, status Status, messages ...string) {
	r.Static.Checks = append(r.Static.Checks, Check{Name: name, Status: status, Messages: messages})
}

// stop ends the pipeline at the given stage with a verdict.
func (r *Report) stop(stage string, v Verdict) *Report {
	r.Summary.StoppedAt = stage
	r.Summary.Verdict = v
	return r
}

// fail records a failed check and stops the pipeline at that stage.
func (r *Report) fail(stage string, v Verdict, messages ...string) *Report {
	r.addCheck(stage, StatusError, messages...)
	return r.stop(stage, v)
}

// finish computes the score and verdict from the test results.
func (r *Report) finish() *Report {
	r.Summary.Total = len(r.Dynamic.Tests)
	timedOut := false
	for _, t := range r.Dynamic.Tests {
		if t.Passed {
			r.Summary.Passed++
		}
		if t.Error == errTimeout {
			timedOut = true
		}
	}
	if r.Summary.Total > 0 {
		r.Summary.Score = float64(r.Summary.Passed) / float64(r.Summary.Total)
	}
	switch {
	case timedOut:
		r.Summary.Verdict = VerdictTimeout
	case r.Summary.Passed == r.Summary.Total:
		r.Summary.Verdict = VerdictPassed
	default:
		r.Summary.Verdict = VerdictFailed
	}
	return r
}

var verdictLabels = map[Verdict]string{
	VerdictPassed:        "aprovado",
	VerdictFailed:        "reprovado",
	VerdictRejected:      "rejeitado",
	VerdictCompileError:  "erro de compilação",
	VerdictTimeout:       "tempo esgotado",
	VerdictInternalError: "erro interno",
}

// Label returns the Portuguese name of the verdict.
func (v Verdict) Label() string { return verdictLabels[v] }

var statusLabels = map[Status]string{
	StatusOK:      "ok",
	StatusWarning: "aviso",
	StatusError:   "erro",
	StatusSkipped: "ignorado",
}

// Label returns the Portuguese name of the status.
func (s Status) Label() string { return statusLabels[s] }
