// Package exam defines exam templates, draws exercises for attempts and
// scores them.
package exam

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gavel/internal/exercise"
)

// Exam is a preset: how many exercises of each level to draw. Teachers
// start exam sessions from a preset or from a custom composition.
type Exam struct {
	ID          string                      `json:"id"`
	Title       string                      `json:"title"`
	Description string                      `json:"description"`
	Composition map[exercise.Difficulty]int `json:"composition"`
}

// Validate checks the template against the exercises available.
func (e *Exam) Validate(repo *exercise.Repository) error {
	var errs []error
	if strings.TrimSpace(e.Title) == "" {
		errs = append(errs, errors.New("campo obrigatório em falta: title"))
	}
	if len(e.Composition) == 0 {
		errs = append(errs, errors.New("composition está vazia"))
	}
	for d, n := range e.Composition {
		switch {
		case !d.Valid():
			errs = append(errs, fmt.Errorf("nível inválido %q", d))
		case n <= 0:
			errs = append(errs, fmt.Errorf("a contagem para %q tem de ser positiva, é %d", d, n))
		case len(repo.List(d)) < n:
			errs = append(errs, fmt.Errorf("pede %d exercícios %q mas só existem %d", n, d, len(repo.List(d))))
		}
	}
	return errors.Join(errs...)
}

// MaxPoints is the number of points an attempt of this exam is worth.
func (e *Exam) MaxPoints() int {
	total := 0
	for d, n := range e.Composition {
		total += n * d.Points()
	}
	return total
}

// Draw picks the exercises for a session. The result depends only on the
// composition, the repository and the seed, so a draw can be reproduced.
// Levels are disjoint, so no exercise is picked twice.
func (e *Exam) Draw(repo *exercise.Repository, seed uint64) []string {
	rng := rand.New(rand.NewPCG(seed, seed)) //nolint:gosec // reproducible draws need a seeded generator
	var ids []string
	for _, d := range exercise.Difficulties {
		pool := repo.List(d)
		for _, i := range rng.Perm(len(pool))[:e.Composition[d]] {
			ids = append(ids, pool[i].ID)
		}
	}
	return ids
}

// Catalog is the validated set of exam templates.
type Catalog struct {
	byID   map[string]*Exam
	sorted []*Exam
}

// LoadCatalog reads every <dir>/<id>.json and validates it against repo.
func LoadCatalog(dir string, repo *exercise.Repository) (*Catalog, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("listar provas: %w", err)
	}
	var exams []*Exam
	for _, path := range paths {
		data, err := os.ReadFile(path) //nolint:gosec // path comes from the exams directory listing
		if err != nil {
			return nil, fmt.Errorf("ler prova: %w", err)
		}
		var e Exam
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("prova %s: JSON inválido: %w", path, err)
		}
		e.ID = strings.TrimSuffix(filepath.Base(path), ".json")
		exams = append(exams, &e)
	}
	return newCatalog(exams, repo)
}

// newCatalog validates the given templates and indexes them by id.
func newCatalog(exams []*Exam, repo *exercise.Repository) (*Catalog, error) {
	c := &Catalog{byID: make(map[string]*Exam, len(exams))}
	for _, e := range exams {
		if err := e.Validate(repo); err != nil {
			return nil, fmt.Errorf("prova %q inválida: %w", e.ID, err)
		}
		c.byID[e.ID] = e
		c.sorted = append(c.sorted, e)
	}
	// Easier exams (fewer points in total) come first.
	sort.Slice(c.sorted, func(i, j int) bool {
		pi, pj := c.sorted[i].MaxPoints(), c.sorted[j].MaxPoints()
		if pi != pj {
			return pi < pj
		}
		return c.sorted[i].ID < c.sorted[j].ID
	})
	return c, nil
}

// Get returns the template with the given id.
func (c *Catalog) Get(id string) (*Exam, bool) {
	e, ok := c.byID[id]
	return e, ok
}

// List returns every template, easiest first.
func (c *Catalog) List() []*Exam { return c.sorted }

// Attempt is one student's participation in an exam session. Every
// attempt of a session has the session's exercises.
type Attempt struct {
	ID          string    `json:"attempt_id"`
	SessionID   string    `json:"session_id"`
	Student     string    `json:"student,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	ExerciseIDs []string  `json:"exercise_ids"`
}

// Contains reports whether the exercise was drawn for this attempt.
func (a *Attempt) Contains(exerciseID string) bool {
	for _, id := range a.ExerciseIDs {
		if id == exerciseID {
			return true
		}
	}
	return false
}
