package engine

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"gavel/internal/exercise"
)

func testExercise() *exercise.Exercise {
	return &exercise.Exercise{
		ID:       "hamming",
		Function: "Distance",
		Params:   []string{"string", "string"},
		Returns:  "int",
	}
}

func TestCheckAST(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string // empty means no violation
	}{
		{"valid", "package solution\n\nfunc Distance(a, b string) int { return 0 }\n", ""},
		{"separate params", "package solution\n\nfunc Distance(a string, b string) (n int) { return 0 }\n", ""},
		{"allowed imports", "package solution\n\nimport (\n\t\"strings\"\n\t\"unicode/utf8\"\n)\n\nfunc Distance(a, b string) int { return utf8.RuneCountInString(strings.TrimSpace(a+b)) }\n", ""},
		{"wrong package", "package main\n\nfunc Distance(a, b string) int { return 0 }\n", "o package tem de ser solution"},
		{"import os", "package solution\n\nimport \"os\"\n\nfunc Distance(a, b string) int { return os.Getpid() }\n", "import não permitido: os"},
		{"import net/http", "package solution\n\nimport _ \"net/http\"\n\nfunc Distance(a, b string) int { return 0 }\n", "import não permitido: net/http"},
		{"import C", "package solution\n\nimport \"C\"\n\nfunc Distance(a, b string) int { return 0 }\n", "import não permitido: C"},
		{"import unsafe", "package solution\n\nimport \"unsafe\"\n\nfunc Distance(a, b string) int { return int(unsafe.Sizeof(a)) }\n", "import não permitido: unsafe"},
		{"func init", "package solution\n\nfunc init() {}\n\nfunc Distance(a, b string) int { return 0 }\n", "func init"},
		{"func main", "package solution\n\nfunc main() {}\n\nfunc Distance(a, b string) int { return 0 }\n", "func main"},
		{"method named init is fine", "package solution\n\ntype T struct{}\n\nfunc (T) init() {}\n\nfunc Distance(a, b string) int { return 0 }\n", ""},
		{"directive", "package solution\n\n//go:noinline\nfunc Distance(a, b string) int { return 0 }\n", "diretiva não permitida"},
		{"missing function", "package solution\n\nfunc Other(a, b string) int { return 0 }\n", "função Distance não encontrada"},
		{"wrong param type", "package solution\n\nfunc Distance(a, b []byte) int { return 0 }\n", "assinatura errada"},
		{"wrong param count", "package solution\n\nfunc Distance(a string) int { return 0 }\n", "assinatura errada"},
		{"wrong return", "package solution\n\nfunc Distance(a, b string) int64 { return 0 }\n", "assinatura errada"},
		{"two returns", "package solution\n\nfunc Distance(a, b string) (int, error) { return 0, nil }\n", "assinatura errada"},
		{"no return", "package solution\n\nfunc Distance(a, b string) {}\n", "assinatura errada"},
		{"generic", "package solution\n\nfunc Distance[T any](a, b string) int { return 0 }\n", "assinatura errada"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, errs := parse(tt.code)
			if len(errs) > 0 {
				t.Fatalf("parse: %v", errs)
			}
			msgs := strings.Join(checkAST(file, testExercise()), "\n")
			if tt.want == "" {
				if msgs != "" {
					t.Fatalf("unexpected violations: %s", msgs)
				}
				return
			}
			if !strings.Contains(msgs, tt.want) {
				t.Fatalf("violations = %q, want %q", msgs, tt.want)
			}
		})
	}
}

func TestCheckASTNormalisesTypeSpelling(t *testing.T) {
	ex := &exercise.Exercise{Function: "F", Params: []string{"map[string][]int"}, Returns: "[]string"}
	file, _ := parse("package solution\n\nfunc F(m map[string] []int) []string { return nil }\n")
	if msgs := checkAST(file, ex); len(msgs) > 0 {
		t.Fatalf("unexpected violations: %v", msgs)
	}
}

func TestParseReportsSyntaxErrors(t *testing.T) {
	_, errs := parse("package solution\n\nfunc F( {\n")
	if len(errs) == 0 {
		t.Fatal("expected syntax errors")
	}
}

func TestCheckLimits(t *testing.T) {
	if err := checkLimits(strings.Repeat("a", MaxCodeSize)); err != nil {
		t.Errorf("code at the limit rejected: %v", err)
	}
	if err := checkLimits(strings.Repeat("a", MaxCodeSize+1)); err == nil {
		t.Error("code over the limit accepted")
	}
}

func TestCheckFormat(t *testing.T) {
	if c := checkFormat("package solution\n\nfunc F() int { return 1 }\n"); c.Status != StatusOK {
		t.Errorf("formatted code: status = %s", c.Status)
	}
	if c := checkFormat("package solution\nfunc F() int {\nreturn 1\n}\n"); c.Status != StatusWarning {
		t.Errorf("unformatted code: status = %s", c.Status)
	}
}

func TestComplexity(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{"straight line", "x := 1\n_ = x", 1},
		{"if", "if a > 0 {\n}", 2},
		{"if else-if", "if a > 0 {\n} else if a < 0 {\n}", 3},
		{"for and range", "for i := 0; i < a; i++ {\n}\nfor range []int{} {\n}", 3},
		{"logical operators", "_ = a > 0 && a < 10 || a == 20", 3},
		{"switch ignores default", "switch a {\ncase 1:\ncase 2, 3:\ndefault:\n}", 3},
		{"select ignores default", "var c chan int\nselect {\ncase <-c:\ndefault:\n}", 2},
		{"func literal counts", "f := func() {\n\tif a > 0 {\n\t}\n}\nf()", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "package p\n\nfunc F(a int) {\n" + tt.body + "\n}\n"
			file, err := parser.ParseFile(token.NewFileSet(), "", src, 0)
			if err != nil {
				t.Fatal(err)
			}
			got := complexities(file)
			if len(got) != 1 || got[0].value != tt.want {
				t.Fatalf("complexities = %+v, want %d", got, tt.want)
			}
		})
	}
}

func TestCheckComplexityWarnsAboveLimit(t *testing.T) {
	src := "package p\n\nfunc F(a int) {\n" + strings.Repeat("if a > 0 {\n}\n", maxComplexity) + "}\n"
	file, err := parser.ParseFile(token.NewFileSet(), "", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	c := checkComplexity(file)
	if c.Status != StatusWarning || !strings.Contains(c.Messages[0], "F") {
		t.Fatalf("check = %+v, want warning for F", c)
	}
}
