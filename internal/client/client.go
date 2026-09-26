// Package client defines the operations offered by Gavel and two
// implementations: LocalClient, which uses the engine and the disk
// directly, and RemoteClient, which talks to a Gavel server over HTTP.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gavel/internal/engine"
	"gavel/internal/exam"
	"gavel/internal/exercise"
)

// Client is everything the CLI and the HTTP server need.
type Client interface {
	Exercises(ctx context.Context, d exercise.Difficulty) ([]ExerciseSummary, error)
	Exercise(ctx context.Context, id string) (*ExerciseDetail, error)
	Exams(ctx context.Context) ([]*exam.Exam, error)
	Sessions(ctx context.Context) ([]SessionView, error)
	JoinSession(ctx context.Context, sessionID, student string) (*AttemptView, error)
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

// AttemptView is an attempt together with its session's timing and its
// current score.
type AttemptView struct {
	exam.Attempt
	Title            string     `json:"title"`
	EndsAt           time.Time  `json:"ends_at"`
	Open             bool       `json:"open"`
	RemainingSeconds int64      `json:"remaining_seconds"`
	Score            exam.Score `json:"score"`
}

// SessionView is an exam session with its status. Students counts the
// attempts and is only filled in for the teacher.
type SessionView struct {
	exam.Session
	Open             bool  `json:"open"`
	RemainingSeconds int64 `json:"remaining_seconds"`
	Students         int   `json:"students"`
}

// CreateSessionRequest asks to start an exam session, in one of three ways:
// ExerciseIDs picks the exact exercises (and wins over the rest);
// otherwise exercises are drawn from Composition, or from the composition of
// the Preset (an exam template from data/exams).
type CreateSessionRequest struct {
	Title           string                      `json:"title"`
	Preset          string                      `json:"preset,omitempty"`
	Composition     map[exercise.Difficulty]int `json:"composition,omitempty"`
	ExerciseIDs     []string                    `json:"exercise_ids,omitempty"`
	DurationMinutes int                         `json:"duration_minutes"`
}

// JoinRequest is the body of a request to join a session.
type JoinRequest struct {
	Student string `json:"student"`
}

// SubmitRequest is a submission. AttemptID and Student are optional; a
// submission to an attempt takes the attempt's student.
type SubmitRequest struct {
	ExerciseID string `json:"exercise_id"`
	Code       string `json:"code"`
	AttemptID  string `json:"attempt_id,omitempty"`
	Student    string `json:"student,omitempty"`
}

// Admin is the teacher's side: exam sessions and every submission and
// attempt. It is only exposed by the HTTP server, behind the admin password.
type Admin interface {
	AllSessions(ctx context.Context) ([]SessionView, error)
	CreateSession(ctx context.Context, req CreateSessionRequest) (*SessionView, error)
	CloseSession(ctx context.Context, id string) (*SessionView, error)
	Submissions(ctx context.Context) ([]SubmissionSummary, error)
	Attempts(ctx context.Context) ([]*AttemptView, error)
}

// SubmissionSummary is a report without the code and test details.
type SubmissionSummary struct {
	SubmissionID string         `json:"submission_id"`
	Student      string         `json:"student,omitempty"`
	ExerciseID   string         `json:"exercise_id"`
	AttemptID    string         `json:"attempt_id,omitempty"`
	SubmittedAt  time.Time      `json:"submitted_at"`
	Verdict      engine.Verdict `json:"verdict"`
	Passed       int            `json:"passed"`
	Total        int            `json:"total"`
	Score        float64        `json:"score"`
	StoppedAt    string         `json:"stopped_at,omitempty"`
}

// maxStudentName caps the length of a student name, in runes.
const maxStudentName = 80

// normalizeStudent trims a student name and rejects names that are too long
// or contain control characters. Names are self-declared, not authenticated.
func normalizeStudent(name string) (string, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > maxStudentName {
		return "", errorf(ErrInvalid, "o nome do aluno tem mais de %d caracteres", maxStudentName)
	}
	if strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", errorf(ErrInvalid, "o nome do aluno contém caracteres inválidos")
	}
	return name, nil
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
