package exam

import "gavel/internal/exercise"

// Submission is the part of a submission that matters for scoring.
type Submission struct {
	ExerciseID string
	Score      float64
}

// ExerciseScore is the scoring detail for one exercise of an attempt.
type ExerciseScore struct {
	ExerciseID  string              `json:"exercise_id"`
	Title       string              `json:"title"`
	Difficulty  exercise.Difficulty `json:"difficulty"`
	Submissions int                 `json:"submissions"`
	BestScore   float64             `json:"best_score"`
	Points      float64             `json:"points"`
	MaxPoints   int                 `json:"max_points"`
}

// Score is the current result of an attempt.
type Score struct {
	Exercises []ExerciseScore `json:"exercises"`
	Points    float64         `json:"points"`
	MaxPoints int             `json:"max_points"`
	// Total is Points / MaxPoints, between 0 and 1.
	Total float64 `json:"total"`
}

// ComputeScore scores an attempt: for each exercise only the best
// submission counts, weighted by the points of its level.
func ComputeScore(a *Attempt, repo *exercise.Repository, subs []Submission) Score {
	best := make(map[string]float64)
	count := make(map[string]int)
	for _, s := range subs {
		count[s.ExerciseID]++
		best[s.ExerciseID] = max(best[s.ExerciseID], s.Score)
	}
	var sc Score
	for _, id := range a.ExerciseIDs {
		es := ExerciseScore{ExerciseID: id, Submissions: count[id], BestScore: best[id]}
		if ex, ok := repo.Get(id); ok {
			es.Title = ex.Title
			es.Difficulty = ex.Difficulty
			es.MaxPoints = ex.Difficulty.Points()
		}
		es.Points = es.BestScore * float64(es.MaxPoints)
		sc.Exercises = append(sc.Exercises, es)
		sc.Points += es.Points
		sc.MaxPoints += es.MaxPoints
	}
	if sc.MaxPoints > 0 {
		sc.Total = sc.Points / float64(sc.MaxPoints)
	}
	return sc
}
