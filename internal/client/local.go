package client

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"time"

	"gavel/internal/engine"
	"gavel/internal/exam"
	"gavel/internal/exercise"
	"gavel/internal/store"
)

// LocalClient implements Client with the engine and the disk.
type LocalClient struct {
	exercises *exercise.Repository
	exams     *exam.Catalog
	engine    *engine.Engine
	store     *store.Store
}

var (
	_ Client = (*LocalClient)(nil)
	_ Admin  = (*LocalClient)(nil)
)

// OpenLocal loads and validates everything under dataDir. It fails if any
// exercise or exam template is invalid.
func OpenLocal(dataDir string, sandbox engine.SandboxMode) (*LocalClient, error) {
	exercises, err := exercise.Load(filepath.Join(dataDir, "exercises"))
	if err != nil {
		return nil, err
	}
	exams, err := exam.LoadCatalog(filepath.Join(dataDir, "exams"), exercises)
	if err != nil {
		return nil, err
	}
	eng, err := engine.New(engine.Config{CacheDir: filepath.Join(dataDir, ".cache"), Sandbox: sandbox})
	if err != nil {
		return nil, err
	}
	st, err := store.New(dataDir)
	if err != nil {
		return nil, err
	}
	return NewLocal(exercises, exams, eng, st), nil
}

// NewLocal builds a LocalClient from its parts.
func NewLocal(exercises *exercise.Repository, exams *exam.Catalog, eng *engine.Engine, st *store.Store) *LocalClient {
	return &LocalClient{exercises: exercises, exams: exams, engine: eng, store: st}
}

// Exercises lists the exercises, optionally filtered by level.
func (c *LocalClient) Exercises(_ context.Context, d exercise.Difficulty) ([]ExerciseSummary, error) {
	if d != "" && !d.Valid() {
		return nil, errorf(ErrInvalid, "nível inválido %q", d)
	}
	list := c.exercises.List(d)
	out := make([]ExerciseSummary, len(list))
	for i, e := range list {
		out[i] = summary(e)
	}
	return out, nil
}

// Exercise returns an exercise without the expected outputs.
func (c *LocalClient) Exercise(_ context.Context, id string) (*ExerciseDetail, error) {
	e, err := c.exercise(id)
	if err != nil {
		return nil, err
	}
	tests := make([]PublicTest, len(e.Tests))
	for i, t := range e.Tests {
		tests[i] = PublicTest{Input: t.Input}
	}
	return &ExerciseDetail{
		ExerciseSummary: summary(e),
		Function:        e.Function,
		Params:          e.Params,
		Returns:         e.Returns,
		Signature:       e.Signature(),
		TimeoutMS:       e.TimeoutMS,
		Tests:           tests,
	}, nil
}

// Exams lists the exam templates.
func (c *LocalClient) Exams(context.Context) ([]*exam.Exam, error) {
	return c.exams.List(), nil
}

// StartAttempt draws the exercises for a new attempt and saves it.
func (c *LocalClient) StartAttempt(_ context.Context, examID, student string) (*AttemptView, error) {
	e, ok := c.exams.Get(examID)
	if !ok {
		return nil, errorf(ErrNotFound, "prova %q não encontrada", examID)
	}
	student, err := normalizeStudent(student)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	id, err := store.NewID(now)
	if err != nil {
		return nil, err
	}
	seed := rand.Uint64() //nolint:gosec // the draw does not need a cryptographic generator
	a := &exam.Attempt{
		ID:          id,
		ExamID:      e.ID,
		Student:     student,
		StartedAt:   now,
		Seed:        seed,
		ExerciseIDs: e.Draw(c.exercises, seed),
	}
	if err := c.store.SaveAttempt(a); err != nil {
		return nil, fmt.Errorf("guardar tentativa: %w", err)
	}
	return c.view(a, nil), nil
}

// Attempt returns an attempt with its current score.
func (c *LocalClient) Attempt(_ context.Context, id string) (*AttemptView, error) {
	a, err := c.attempt(id)
	if err != nil {
		return nil, err
	}
	reports, err := c.store.ReportsByAttempt(a.ID)
	if err != nil {
		return nil, fmt.Errorf("ler submissões da tentativa: %w", err)
	}
	return c.view(a, reports), nil
}

// Submit evaluates a submission and saves its report.
func (c *LocalClient) Submit(ctx context.Context, req SubmitRequest) (*engine.Report, error) {
	if req.ExerciseID == "" {
		return nil, errorf(ErrInvalid, "exercise_id é obrigatório")
	}
	ex, err := c.exercise(req.ExerciseID)
	if err != nil {
		return nil, err
	}
	student, err := normalizeStudent(req.Student)
	if err != nil {
		return nil, err
	}
	if req.AttemptID != "" {
		a, err := c.attempt(req.AttemptID)
		if err != nil {
			return nil, err
		}
		if !a.Contains(ex.ID) {
			return nil, errorf(ErrInvalid, "o exercício %q não pertence à tentativa %s", ex.ID, a.ID)
		}
		if a.Student != "" {
			student = a.Student
		}
	}

	r := c.engine.Evaluate(ctx, ex, req.Code)
	r.AttemptID = req.AttemptID
	r.Student = student
	r.SubmittedAt = time.Now().UTC().Truncate(time.Second)
	if r.SubmissionID, err = store.NewID(r.SubmittedAt); err != nil {
		return nil, err
	}
	if err := c.store.SaveReport(r); err != nil {
		return nil, fmt.Errorf("guardar relatório: %w", err)
	}
	return r, nil
}

// Report returns a saved report.
func (c *LocalClient) Report(_ context.Context, id string) (*engine.Report, error) {
	r, err := c.store.Report(id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, errorf(ErrNotFound, "relatório %q não encontrado", id)
	}
	return r, err
}

func (c *LocalClient) exercise(id string) (*exercise.Exercise, error) {
	e, ok := c.exercises.Get(id)
	if !ok {
		return nil, errorf(ErrNotFound, "exercício %q não encontrado", id)
	}
	return e, nil
}

func (c *LocalClient) attempt(id string) (*exam.Attempt, error) {
	a, err := c.store.Attempt(id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, errorf(ErrNotFound, "tentativa %q não encontrada", id)
	}
	return a, err
}

// Submissions lists every submission, oldest first.
func (c *LocalClient) Submissions(context.Context) ([]SubmissionSummary, error) {
	reports, err := c.store.Reports()
	if err != nil {
		return nil, fmt.Errorf("ler relatórios: %w", err)
	}
	out := make([]SubmissionSummary, len(reports))
	for i, r := range reports {
		out[i] = SubmissionSummary{
			SubmissionID: r.SubmissionID,
			Student:      r.Student,
			ExerciseID:   r.ExerciseID,
			AttemptID:    r.AttemptID,
			SubmittedAt:  r.SubmittedAt,
			Verdict:      r.Summary.Verdict,
			Passed:       r.Summary.Passed,
			Total:        r.Summary.Total,
			Score:        r.Summary.Score,
			StoppedAt:    r.Summary.StoppedAt,
		}
	}
	return out, nil
}

// Attempts lists every attempt with its current score, oldest first.
func (c *LocalClient) Attempts(context.Context) ([]*AttemptView, error) {
	attempts, err := c.store.Attempts()
	if err != nil {
		return nil, fmt.Errorf("ler tentativas: %w", err)
	}
	reports, err := c.store.Reports()
	if err != nil {
		return nil, fmt.Errorf("ler relatórios: %w", err)
	}
	byAttempt := make(map[string][]*engine.Report)
	for _, r := range reports {
		if r.AttemptID != "" {
			byAttempt[r.AttemptID] = append(byAttempt[r.AttemptID], r)
		}
	}
	out := make([]*AttemptView, len(attempts))
	for i, a := range attempts {
		out[i] = c.view(a, byAttempt[a.ID])
	}
	return out, nil
}

// view scores an attempt from its reports.
func (c *LocalClient) view(a *exam.Attempt, reports []*engine.Report) *AttemptView {
	subs := make([]exam.Submission, len(reports))
	for i, r := range reports {
		subs[i] = exam.Submission{ExerciseID: r.ExerciseID, Score: r.Summary.Score}
	}
	v := &AttemptView{Attempt: *a, Score: exam.ComputeScore(a, c.exercises, subs)}
	if e, ok := c.exams.Get(a.ExamID); ok {
		v.ExamTitle = e.Title
	}
	return v
}

func summary(e *exercise.Exercise) ExerciseSummary {
	return ExerciseSummary{ID: e.ID, Title: e.Title, Description: e.Description, Difficulty: e.Difficulty}
}
