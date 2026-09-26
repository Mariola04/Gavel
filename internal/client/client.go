// Package client defines the operations offered by Gavel and two
// implementations: LocalClient, which uses the engine and the disk
// directly, and RemoteClient, which talks to a Gavel server over HTTP.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gavel/internal/engine"
	"gavel/internal/exam"
	"gavel/internal/exercise"
)

// Client is everything the CLI and the HTTP server need.
type Client interface {
	Exercises(ctx context.Context, d exercise.Difficulty) ([]ExerciseSummary, error)
	Exercise(ctx context.Context, id string) (*ExerciseDetail, error)
	Exams(ctx context.Context) ([]*exam.Exam, error)
	StartAttempt(ctx context.Context, examID string) (*AttemptView, error)
	Attempt(ctx context.Context, id string) (*AttemptView, error)
	Submit(ctx context.Context, req SubmitRequest) (*engine.Report, error)
	Report(ctx context.Context, id string) (*engine.Report, error)
}

// ExerciseSummary is an entry of the exercise list.
type ExerciseSummary struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Difficulty  exercise.Difficulty `json:"difficulty"`
}

// PublicTest is a test case without its expected output.
type PublicTest struct {
	Input []json.RawMessage `json:"input"`
}

// ExerciseDetail is an exercise as shown to students: the expected outputs
// are not included.
type ExerciseDetail struct {
	ExerciseSummary
	Function  string       `json:"function"`
	Params    []string     `json:"params"`
	Returns   string       `json:"returns"`
	Signature string       `json:"signature"`
	TimeoutMS int          `json:"timeout_ms"`
	Tests     []PublicTest `json:"tests"`
}

// AttemptView is an attempt together with its current score.
type AttemptView struct {
	exam.Attempt
	ExamTitle string     `json:"exam_title"`
	Score     exam.Score `json:"score"`
}

// SubmitRequest is a submission. AttemptID is optional.
type SubmitRequest struct {
	ExerciseID string `json:"exercise_id"`
	Code       string `json:"code"`
	AttemptID  string `json:"attempt_id,omitempty"`
}

// Error kinds, to be matched with errors.Is.
var (
	ErrNotFound = errors.New("não encontrado")
	ErrInvalid  = errors.New("pedido inválido")
)

// Error is an error with a user-facing message and a kind (ErrNotFound or
// ErrInvalid) that decides the HTTP status.
type Error struct {
	Kind    error
	Message string
}

func (e *Error) Error() string { return e.Message }

// Unwrap lets errors.Is match the kind.
func (e *Error) Unwrap() error { return e.Kind }

func errorf(kind error, format string, args ...any) error {
	return &Error{Kind: kind, Message: fmt.Sprintf(format, args...)}
}
