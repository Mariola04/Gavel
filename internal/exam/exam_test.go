package exam

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"gavel/internal/exercise"
)

// testRepo builds a repository with n exercises per level, named
// "<level>-<i>".
func testRepo(t *testing.T, n int) *exercise.Repository {
	t.Helper()
	var exs []*exercise.Exercise
	for _, d := range exercise.Difficulties {
		for i := range n {
			exs = append(exs, &exercise.Exercise{
				ID: fmt.Sprintf("%s-%d", d, i), Title: "T", Description: "D", Difficulty: d,
				Function: "F", Returns: "int", TimeoutMS: 1000,
				Tests: []exercise.Test{{Output: json.RawMessage("1")}},
			})
		}
	}
	repo, err := exercise.NewRepository(exs)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestValidate(t *testing.T) {
	repo := testRepo(t, 2)
	tests := []struct {
		name        string
		composition map[exercise.Difficulty]int
		want        string
	}{
		{"valid", map[exercise.Difficulty]int{exercise.Easy: 2, exercise.Hard: 1}, ""},
		{"empty", nil, "vazia"},
		{"invalid level", map[exercise.Difficulty]int{"expert": 1}, "nível inválido"},
		{"zero count", map[exercise.Difficulty]int{exercise.Easy: 0}, "positiva"},
		{"negative count", map[exercise.Difficulty]int{exercise.Easy: -1}, "positiva"},
		{"not enough exercises", map[exercise.Difficulty]int{exercise.Medium: 3}, "só existem 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Exam{ID: "x", Title: "Prova", Composition: tt.composition}
			err := e.Validate(repo)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestLoadCatalogFromData(t *testing.T) {
	repo, err := exercise.Load("../../data/exercises")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range exercise.Difficulties {
		if n := len(repo.List(d)); n < 4 {
			t.Errorf("level %s has %d exercises, want at least 4", d, n)
		}
	}
	c, err := LoadCatalog("../../data/exams", repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"easy", "medium", "hard"} {
		if _, ok := c.Get(id); !ok {
			t.Errorf("exam %q not found", id)
		}
	}
}

func TestDraw(t *testing.T) {
	repo := testRepo(t, 5)
	e := &Exam{Composition: map[exercise.Difficulty]int{exercise.Easy: 1, exercise.Medium: 2, exercise.Hard: 3}}
	for seed := range uint64(50) {
		ids := e.Draw(repo, seed)
		counts := make(map[exercise.Difficulty]int)
		for _, id := range ids {
			ex, _ := repo.Get(id)
			counts[ex.Difficulty]++
		}
		for d, n := range e.Composition {
			if counts[d] != n {
				t.Fatalf("seed %d: %d exercises of %s, want %d (%v)", seed, counts[d], d, n, ids)
			}
		}
		sorted := slices.Clone(ids)
		slices.Sort(sorted)
		if len(slices.Compact(sorted)) != len(ids) {
			t.Fatalf("seed %d: repeated exercises %v", seed, ids)
		}
		if again := e.Draw(repo, seed); !slices.Equal(ids, again) {
			t.Fatalf("seed %d: draw not reproducible: %v vs %v", seed, ids, again)
		}
	}
}

func TestDrawVariesWithSeed(t *testing.T) {
	repo := testRepo(t, 10)
	e := &Exam{Composition: map[exercise.Difficulty]int{exercise.Easy: 3}}
	first := e.Draw(repo, 1)
	for seed := uint64(2); seed < 20; seed++ {
		if !slices.Equal(first, e.Draw(repo, seed)) {
			return
		}
	}
	t.Fatal("every seed produced the same draw")
}

func TestComputeScore(t *testing.T) {
	repo := testRepo(t, 2)
	a := &Attempt{ExerciseIDs: []string{"easy-0", "medium-0", "hard-0"}}
	subs := []Submission{
		{ExerciseID: "easy-0", Score: 0.5},
		{ExerciseID: "easy-0", Score: 1},   // best for easy-0
		{ExerciseID: "easy-0", Score: 0.2}, // a later, worse submission does not count
		{ExerciseID: "medium-0", Score: 0.5},
		{ExerciseID: "easy-1", Score: 1}, // not part of the attempt
	}
	sc := ComputeScore(a, repo, subs)

	want := []struct {
		best, points float64
		max, subs    int
	}{{1, 1, 1, 3}, {0.5, 1, 2, 1}, {0, 0, 3, 0}}
	for i, w := range want {
		got := sc.Exercises[i]
		if got.BestScore != w.best || got.Points != w.points || got.MaxPoints != w.max || got.Submissions != w.subs {
			t.Errorf("exercise %d = %+v, want %+v", i, got, w)
		}
	}
	if sc.Points != 2 || sc.MaxPoints != 6 || math.Abs(sc.Total-2.0/6) > 1e-9 {
		t.Errorf("score = %v/%v (%v), want 2/6", sc.Points, sc.MaxPoints, sc.Total)
	}
}

func TestNewSession(t *testing.T) {
	repo := testRepo(t, 3)
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	comp := map[exercise.Difficulty]int{exercise.Easy: 1, exercise.Hard: 2}

	s, err := NewSession(" Prova 1 ", comp, 90*time.Minute, repo, 42, now)
	if err != nil {
		t.Fatal(err)
	}
	if s.Title != "Prova 1" || len(s.ExerciseIDs) != 3 || !s.EndsAt.Equal(now.Add(90*time.Minute)) {
		t.Fatalf("session = %+v", s)
	}
	again, _ := NewSession("Prova 1", comp, 90*time.Minute, repo, 42, now)
	if !slices.Equal(s.ExerciseIDs, again.ExerciseIDs) {
		t.Errorf("same seed gave different draws: %v vs %v", s.ExerciseIDs, again.ExerciseIDs)
	}

	for name, tc := range map[string]struct {
		title    string
		comp     map[exercise.Difficulty]int
		duration time.Duration
		want     string
	}{
		"too short":       {"P", comp, 30 * time.Second, "duração"},
		"too long":        {"P", comp, 9 * time.Hour, "duração"},
		"no title":        {" ", comp, time.Hour, "title"},
		"empty":           {"P", nil, time.Hour, "vazia"},
		"too many hard":   {"P", map[exercise.Difficulty]int{exercise.Hard: 4}, time.Hour, "só existem 3"},
		"unknown level":   {"P", map[exercise.Difficulty]int{"expert": 1}, time.Hour, "nível inválido"},
		"negative amount": {"P", map[exercise.Difficulty]int{exercise.Easy: -1}, time.Hour, "positiva"},
	} {
		if _, err := NewSession(tc.title, tc.comp, tc.duration, repo, 1, now); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want %q", name, err, tc.want)
		}
	}
}

func TestSessionOpenAndClose(t *testing.T) {
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	s := &Session{EndsAt: now.Add(time.Hour)}
	if !s.Open(now) || s.Open(now.Add(time.Hour)) {
		t.Fatal("open window is wrong")
	}
	s.Close(now.Add(10 * time.Minute))
	if s.Open(now.Add(10*time.Minute)) || !s.EndsAt.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("close did not end the session: %v", s.EndsAt)
	}
	s.Close(now.Add(30 * time.Minute)) // closing again keeps the original end
	if !s.EndsAt.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("second close moved the end: %v", s.EndsAt)
	}
}

func TestNewSessionWithExercises(t *testing.T) {
	repo := testRepo(t, 3)
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	picked := []string{"hard-2", "easy-0", "hard-0"}

	s, err := NewSessionWithExercises("Escolhidos", picked, time.Hour, repo, now)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.ExerciseIDs, picked) || s.Seed != 0 {
		t.Fatalf("exercises = %v (seed %d), want %v in that order", s.ExerciseIDs, s.Seed, picked)
	}
	if s.Composition[exercise.Easy] != 1 || s.Composition[exercise.Hard] != 2 || len(s.Composition) != 2 {
		t.Errorf("composition = %v", s.Composition)
	}
	picked[0] = "changed" // the session keeps its own copy
	if s.ExerciseIDs[0] != "hard-2" {
		t.Error("session shares the caller's slice")
	}

	for name, tc := range map[string]struct {
		title string
		ids   []string
		want  string
	}{
		"no title":  {" ", []string{"easy-0"}, "title"},
		"none":      {"P", nil, "pelo menos um"},
		"unknown":   {"P", []string{"easy-0", "nope"}, "não existe"},
		"duplicate": {"P", []string{"easy-0", "easy-0"}, "mais do que uma vez"},
	} {
		if _, err := NewSessionWithExercises(tc.title, tc.ids, time.Hour, repo, now); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want %q", name, err, tc.want)
		}
	}
	if _, err := NewSessionWithExercises("P", []string{"easy-0"}, 0, repo, now); err == nil {
		t.Error("zero duration accepted")
	}
}
