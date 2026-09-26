package exercise

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// fileName is the name of the exercise definition inside its directory.
const fileName = "exercise.json"

// Repository is an immutable, validated set of exercises.
type Repository struct {
	byID   map[string]*Exercise
	sorted []*Exercise
}

// NewRepository validates the given exercises and indexes them by id.
func NewRepository(exercises []*Exercise) (*Repository, error) {
	r := &Repository{byID: make(map[string]*Exercise, len(exercises))}
	for _, e := range exercises {
		if err := e.Validate(); err != nil {
			return nil, fmt.Errorf("exercício %q inválido: %w", e.ID, err)
		}
		if _, dup := r.byID[e.ID]; dup {
			return nil, fmt.Errorf("exercício %q duplicado", e.ID)
		}
		r.byID[e.ID] = e
		r.sorted = append(r.sorted, e)
	}
	sort.Slice(r.sorted, func(i, j int) bool { return r.sorted[i].ID < r.sorted[j].ID })
	return r, nil
}

// Load reads every <dir>/<id>/exercise.json. The directory name is the id.
func Load(dir string) (*Repository, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("ler diretoria de exercícios: %w", err)
	}
	var exercises []*Exercise
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		e, err := loadFile(filepath.Join(dir, entry.Name(), fileName))
		if err != nil {
			return nil, err
		}
		e.ID = entry.Name()
		exercises = append(exercises, e)
	}
	if len(exercises) == 0 {
		return nil, fmt.Errorf("nenhum exercício encontrado em %s", dir)
	}
	return NewRepository(exercises)
}

func loadFile(path string) (*Exercise, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is built from the exercises directory listing
	if err != nil {
		return nil, fmt.Errorf("ler exercício: %w", err)
	}
	var e Exercise
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("exercício %s: JSON inválido: %w", path, err)
	}
	return &e, nil
}

// Get returns the exercise with the given id.
func (r *Repository) Get(id string) (*Exercise, bool) {
	e, ok := r.byID[id]
	return e, ok
}

// List returns the exercises of the given level sorted by id, or all of
// them when d is empty. The returned slice must not be modified.
func (r *Repository) List(d Difficulty) []*Exercise {
	if d == "" {
		return r.sorted
	}
	var out []*Exercise
	for _, e := range r.sorted {
		if e.Difficulty == d {
			out = append(out, e)
		}
	}
	return out
}
