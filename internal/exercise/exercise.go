// Package exercise loads and validates exercises stored on disk.
package exercise

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"time"
)

// Difficulty is the level of an exercise.
type Difficulty string

// Supported difficulty levels.
const (
	Easy   Difficulty = "easy"
	Medium Difficulty = "medium"
	Hard   Difficulty = "hard"
)

// Difficulties lists every level, from easiest to hardest.
var Difficulties = []Difficulty{Easy, Medium, Hard}

// points is the single source of truth for how much each level is worth.
var points = map[Difficulty]int{Easy: 1, Medium: 2, Hard: 3}

var labels = map[Difficulty]string{Easy: "fácil", Medium: "médio", Hard: "difícil"}

// Valid reports whether d is a known difficulty level.
func (d Difficulty) Valid() bool {
	_, ok := points[d]
	return ok
}

// Points returns how many points an exercise of this level is worth.
func (d Difficulty) Points() int { return points[d] }

// Label returns the Portuguese name of the level.
func (d Difficulty) Label() string { return labels[d] }

// ParseDifficulty converts s into a Difficulty. The empty string is accepted
// and means "any level".
func ParseDifficulty(s string) (Difficulty, error) {
	d := Difficulty(s)
	if s != "" && !d.Valid() {
		return "", fmt.Errorf("nível inválido %q (use easy, medium ou hard)", s)
	}
	return d, nil
}

// Test is a single test case. Input and Output are kept as raw JSON so that
// large numbers are passed to the harness without losing precision.
type Test struct {
	Input  []json.RawMessage `json:"input"`
	Output json.RawMessage   `json:"output"`
	Hint   string            `json:"hint,omitempty"`
}

// Exercise describes a function the student must implement.
type Exercise struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Difficulty  Difficulty `json:"difficulty"`
	Function    string     `json:"function"`
	Params      []string   `json:"params"`
	Returns     string     `json:"returns"`
	TimeoutMS   int        `json:"timeout_ms"`
	Tests       []Test     `json:"tests"`
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Validate checks that the exercise is complete and consistent.
func (e *Exercise) Validate() error {
	var errs []error
	if !idPattern.MatchString(e.ID) {
		errs = append(errs, fmt.Errorf("id inválido %q (use minúsculas, dígitos e hífenes)", e.ID))
	}
	if strings.TrimSpace(e.Title) == "" {
		errs = append(errs, errors.New("campo obrigatório em falta: title"))
	}
	if strings.TrimSpace(e.Description) == "" {
		errs = append(errs, errors.New("campo obrigatório em falta: description"))
	}
	if !e.Difficulty.Valid() {
		errs = append(errs, fmt.Errorf("difficulty inválida %q", e.Difficulty))
	}
	if !token.IsIdentifier(e.Function) || !token.IsExported(e.Function) {
		errs = append(errs, fmt.Errorf("function %q tem de ser um identificador exportado", e.Function))
	}
	for i, p := range e.Params {
		if err := validateType(p); err != nil {
			errs = append(errs, fmt.Errorf("params[%d]: %w", i, err))
		}
	}
	if err := validateType(e.Returns); err != nil {
		errs = append(errs, fmt.Errorf("returns: %w", err))
	}
	if e.TimeoutMS <= 0 {
		errs = append(errs, errors.New("timeout_ms tem de ser positivo"))
	}
	if len(e.Tests) == 0 {
		errs = append(errs, errors.New("é necessário pelo menos um teste"))
	}
	for i, t := range e.Tests {
		if len(t.Input) != len(e.Params) {
			errs = append(errs, fmt.Errorf("tests[%d]: input tem %d valores, esperados %d", i, len(t.Input), len(e.Params)))
		}
		if len(t.Output) == 0 {
			errs = append(errs, fmt.Errorf("tests[%d]: output em falta", i))
		}
	}
	return errors.Join(errs...)
}

// validateType accepts only JSON-serialisable Go type expressions: named
// types, slices and maps with string keys. Because these strings end up in
// generated code, this is also what keeps the harness template safe.
func validateType(s string) error {
	expr, err := parser.ParseExpr(s)
	if err != nil {
		return fmt.Errorf("tipo inválido %q", s)
	}
	if !isJSONType(expr) {
		return fmt.Errorf("tipo não suportado %q", s)
	}
	return nil
}

func isJSONType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return true
	case *ast.ArrayType:
		return t.Len == nil && isJSONType(t.Elt)
	case *ast.MapType:
		key, ok := t.Key.(*ast.Ident)
		return ok && key.Name == "string" && isJSONType(t.Value)
	default:
		return false
	}
}

// Signature returns the Go declaration the student must write,
// e.g. "func Sum(a0 []int) int".
func (e *Exercise) Signature() string {
	params := make([]string, len(e.Params))
	for i, p := range e.Params {
		params[i] = fmt.Sprintf("a%d %s", i, p)
	}
	return fmt.Sprintf("func %s(%s) %s", e.Function, strings.Join(params, ", "), e.Returns)
}

// Timeout returns the per-test time limit.
func (e *Exercise) Timeout() time.Duration {
	return time.Duration(e.TimeoutMS) * time.Millisecond
}
