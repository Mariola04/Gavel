package exercise

import (
	"encoding/json"
	"strings"
	"testing"
)

func validExercise() *Exercise {
	return &Exercise{
		ID:          "sum",
		Title:       "Soma",
		Description: "Soma os elementos.",
		Difficulty:  Easy,
		Function:    "Sum",
		Params:      []string{"[]int"},
		Returns:     "int",
		TimeoutMS:   1000,
		Tests: []Test{
			{Input: []json.RawMessage{json.RawMessage(`[1,2]`)}, Output: json.RawMessage(`3`)},
		},
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(e *Exercise)
		wantErr string
	}{
		{"valid", func(*Exercise) {}, ""},
		{"missing title", func(e *Exercise) { e.Title = "" }, "title"},
		{"missing description", func(e *Exercise) { e.Description = " " }, "description"},
		{"bad difficulty", func(e *Exercise) { e.Difficulty = "insane" }, "difficulty"},
		{"unexported function", func(e *Exercise) { e.Function = "sum" }, "exportado"},
		{"invalid function name", func(e *Exercise) { e.Function = "Sum()" }, "exportado"},
		{"input length mismatch", func(e *Exercise) { e.Params = []string{"[]int", "int"} }, "input tem 1 valores, esperados 2"},
		{"missing output", func(e *Exercise) { e.Tests[0].Output = nil }, "output"},
		{"no tests", func(e *Exercise) { e.Tests = nil }, "teste"},
		{"zero timeout", func(e *Exercise) { e.TimeoutMS = 0 }, "timeout_ms"},
		{"bad id", func(e *Exercise) { e.ID = "../etc" }, "id inválido"},
		{"code injection in type", func(e *Exercise) { e.Returns = "int { panic(1) }" }, "returns"},
		{"channel type", func(e *Exercise) { e.Returns = "chan int" }, "não suportado"},
		{"map with int keys", func(e *Exercise) { e.Returns = "map[int]string" }, "não suportado"},
		{"map with string keys", func(e *Exercise) { e.Returns = "map[string][]int" }, ""},
		{"param names count", func(e *Exercise) { e.ParamNames = []string{"a", "b"} }, "param_names"},
		{"param name invalid", func(e *Exercise) { e.ParamNames = []string{"1x"} }, "nome inválido"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := validExercise()
			tt.mutate(e)
			err := e.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestSignature(t *testing.T) {
	tests := []struct {
		params, names []string
		want          string
	}{
		{nil, nil, "func Sum() int"},
		{[]string{"[]int", "string"}, nil, "func Sum(p0 []int, p1 string) int"},
		{[]string{"string", "string", "int"}, []string{"a", "b", "n"}, "func Sum(a, b string, n int) int"},
		{[]string{"int", "string", "int"}, []string{"a", "s", "b"}, "func Sum(a int, s string, b int) int"},
	}
	for _, tt := range tests {
		e := validExercise()
		e.Params, e.ParamNames = tt.params, tt.names
		if got := e.Signature(); got != tt.want {
			t.Errorf("Signature() = %q, want %q", got, tt.want)
		}
	}
}

func TestNewRepositoryRejectsDuplicates(t *testing.T) {
	if _, err := NewRepository([]*Exercise{validExercise(), validExercise()}); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestListFiltersAndSorts(t *testing.T) {
	a, b, c := validExercise(), validExercise(), validExercise()
	a.ID, b.ID, c.ID = "zeta", "alpha", "mid"
	c.Difficulty = Hard
	r, err := NewRepository([]*Exercise{a, b, c})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(r.List("")); got != "alpha,mid,zeta" {
		t.Errorf("List(all) = %s", got)
	}
	if got := ids(r.List(Easy)); got != "alpha,zeta" {
		t.Errorf("List(easy) = %s", got)
	}
}

func ids(es []*Exercise) string {
	s := make([]string, len(es))
	for i, e := range es {
		s[i] = e.ID
	}
	return strings.Join(s, ",")
}

func TestPoints(t *testing.T) {
	if Easy.Points() != 1 || Medium.Points() != 2 || Hard.Points() != 3 {
		t.Fatal("unexpected points per level")
	}
}
