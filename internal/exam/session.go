package exam

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"gavel/internal/exercise"
)

// Duration limits for a session.
const (
	MinDuration = time.Minute
	MaxDuration = 8 * time.Hour
)

// Session is an exam run by a teacher. Its exercises are fixed when it is
// created (drawn from a composition, or picked by the teacher), so every
// student gets the same set, and it accepts joins and submissions only until
// EndsAt. Seed is 0 when the exercises were picked.
type Session struct {
	ID          string                      `json:"session_id"`
	Title       string                      `json:"title"`
	Composition map[exercise.Difficulty]int `json:"composition"`
	Seed        uint64                      `json:"seed"`
	ExerciseIDs []string                    `json:"exercise_ids"`
	CreatedAt   time.Time                   `json:"created_at"`
	EndsAt      time.Time                   `json:"ends_at"`
}

// NewSession validates the composition and duration and draws the
// exercises. The caller sets the id.
func NewSession(title string, composition map[exercise.Difficulty]int, duration time.Duration,
	repo *exercise.Repository, seed uint64, now time.Time) (*Session, error) {
	title = strings.TrimSpace(title)
	if err := checkDuration(duration); err != nil {
		return nil, err
	}
	e := &Exam{Title: title, Composition: composition}
	if err := e.Validate(repo); err != nil {
		return nil, err
	}
	return &Session{
		Title:       title,
		Composition: composition,
		Seed:        seed,
		ExerciseIDs: e.Draw(repo, seed),
		CreatedAt:   now,
		EndsAt:      now.Add(duration),
	}, nil
}

// NewSessionWithExercises creates a session with exercises picked by the
// teacher, kept in the given order. The composition is derived from them.
func NewSessionWithExercises(title string, ids []string, duration time.Duration,
	repo *exercise.Repository, now time.Time) (*Session, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("campo obrigatório em falta: title")
	}
	if err := checkDuration(duration); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, errors.New("escolha pelo menos um exercício")
	}
	composition := make(map[exercise.Difficulty]int)
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		ex, ok := repo.Get(id)
		if !ok {
			return nil, fmt.Errorf("o exercício %q não existe", id)
		}
		if seen[id] {
			return nil, fmt.Errorf("o exercício %q foi escolhido mais do que uma vez", id)
		}
		seen[id] = true
		composition[ex.Difficulty]++
	}
	return &Session{
		Title:       title,
		Composition: composition,
		ExerciseIDs: slices.Clone(ids),
		CreatedAt:   now,
		EndsAt:      now.Add(duration),
	}, nil
}

func checkDuration(d time.Duration) error {
	if d < MinDuration || d > MaxDuration {
		return fmt.Errorf("a duração tem de estar entre %v e %v", MinDuration, MaxDuration)
	}
	return nil
}

// Open reports whether the session still accepts joins and submissions.
func (s *Session) Open(now time.Time) bool {
	return now.Before(s.EndsAt)
}

// Close ends the session now, if it is still open.
func (s *Session) Close(now time.Time) {
	if s.Open(now) {
		s.EndsAt = now
	}
}
