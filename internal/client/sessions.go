package client

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"gavel/internal/exam"
	"gavel/internal/store"
)

// Sessions lists the open exam sessions, without their exercises, which a
// student only sees after joining.
func (c *LocalClient) Sessions(context.Context) ([]SessionView, error) {
	sessions, err := c.store.Sessions()
	if err != nil {
		return nil, fmt.Errorf("ler provas: %w", err)
	}
	now := time.Now()
	out := []SessionView{} // an empty list, not null, in JSON
	for _, s := range sessions {
		if !s.Open(now) {
			continue
		}
		v := sessionView(s, now)
		v.ExerciseIDs = nil
		out = append(out, v)
	}
	return out, nil
}

// JoinSession returns the student's attempt at a session, creating it if the
// session is still open. Joining again with the same name returns the same
// attempt, so a student can resume from another browser.
func (c *LocalClient) JoinSession(_ context.Context, sessionID, student string) (*AttemptView, error) {
	student, err := normalizeStudent(student)
	if err != nil {
		return nil, err
	}
	if student == "" {
		return nil, errorf(ErrInvalid, "indique o nome do aluno para entrar na prova")
	}

	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	sess, err := c.session(sessionID)
	if err != nil {
		return nil, err
	}
	attempts, err := c.store.Attempts()
	if err != nil {
		return nil, fmt.Errorf("ler tentativas: %w", err)
	}
	for _, a := range attempts {
		if a.SessionID == sess.ID && strings.EqualFold(a.Student, student) {
			return c.attemptView(a)
		}
	}

	now := time.Now().UTC()
	if !sess.Open(now) {
		return nil, errorf(ErrInvalid, "a prova %q já terminou", sess.Title)
	}
	id, err := store.NewID(now)
	if err != nil {
		return nil, err
	}
	a := &exam.Attempt{
		ID:          id,
		SessionID:   sess.ID,
		Student:     student,
		StartedAt:   now,
		ExerciseIDs: sess.ExerciseIDs,
	}
	if err := c.store.SaveAttempt(a); err != nil {
		return nil, fmt.Errorf("guardar tentativa: %w", err)
	}
	return c.view(a, sess, nil, now), nil
}

// AllSessions lists every session, open or closed, with how many students
// joined each.
func (c *LocalClient) AllSessions(context.Context) ([]SessionView, error) {
	sessions, err := c.store.Sessions()
	if err != nil {
		return nil, fmt.Errorf("ler provas: %w", err)
	}
	attempts, err := c.store.Attempts()
	if err != nil {
		return nil, fmt.Errorf("ler tentativas: %w", err)
	}
	students := make(map[string]int)
	for _, a := range attempts {
		students[a.SessionID]++
	}
	now := time.Now()
	out := make([]SessionView, len(sessions))
	for i, s := range sessions {
		out[i] = sessionView(s, now)
		out[i].Students = students[s.ID]
	}
	return out, nil
}

// CreateSession opens a session for the given number of minutes, with the
// picked exercises or a draw from the composition.
func (c *LocalClient) CreateSession(_ context.Context, req CreateSessionRequest) (*SessionView, error) {
	title, composition := req.Title, req.Composition
	if req.Preset != "" {
		preset, ok := c.exams.Get(req.Preset)
		if !ok {
			return nil, errorf(ErrNotFound, "modelo de prova %q não encontrado", req.Preset)
		}
		if len(composition) == 0 {
			composition = preset.Composition
		}
		if strings.TrimSpace(title) == "" {
			title = preset.Title
		}
	}
	now := time.Now().UTC()
	duration := time.Duration(req.DurationMinutes) * time.Minute
	var sess *exam.Session
	var err error
	if len(req.ExerciseIDs) > 0 {
		sess, err = exam.NewSessionWithExercises(title, req.ExerciseIDs, duration, c.exercises, now)
	} else {
		seed := rand.Uint64() //nolint:gosec // the draw does not need a cryptographic generator
		sess, err = exam.NewSession(title, composition, duration, c.exercises, seed, now)
	}
	if err != nil {
		return nil, errorf(ErrInvalid, "%v", err)
	}
	if sess.ID, err = store.NewID(now); err != nil {
		return nil, err
	}
	if err := c.store.SaveSession(sess); err != nil {
		return nil, fmt.Errorf("guardar prova: %w", err)
	}
	v := sessionView(sess, now)
	return &v, nil
}

// CloseSession ends a session now. Closing a closed session does nothing.
func (c *LocalClient) CloseSession(_ context.Context, id string) (*SessionView, error) {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	sess, err := c.session(id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	sess.Close(now)
	if err := c.store.SaveSession(sess); err != nil {
		return nil, fmt.Errorf("guardar prova: %w", err)
	}
	v := sessionView(sess, now)
	return &v, nil
}

func (c *LocalClient) session(id string) (*exam.Session, error) {
	sess, err := c.store.Session(id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, errorf(ErrNotFound, "prova %q não encontrada", id)
	}
	return sess, err
}

func sessionView(s *exam.Session, now time.Time) SessionView {
	return SessionView{Session: *s, Open: s.Open(now), RemainingSeconds: remaining(s, now)}
}

// remaining is how many whole seconds the session stays open, or 0.
func remaining(s *exam.Session, now time.Time) int64 {
	return max(int64(s.EndsAt.Sub(now)/time.Second), 0)
}
