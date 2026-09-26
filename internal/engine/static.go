package engine

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"gavel/internal/exercise"
)

// MaxCodeSize is the largest submission accepted, in bytes.
const MaxCodeSize = 64 << 10

// maxComplexity is the cyclomatic complexity above which a warning is issued.
const maxComplexity = 10

// maxParseErrors caps how many syntax errors are reported.
const maxParseErrors = 10

// allowedImports is the import allowlist. It is the main line of defence:
// without os, net, syscall, unsafe, etc. the code cannot reach the system.
var allowedImports = map[string]bool{
	"fmt": true, "math": true, "strings": true, "strconv": true, "sort": true,
	"slices": true, "maps": true, "unicode": true, "unicode/utf8": true, "errors": true,
}

func checkLimits(code string) error {
	if len(code) > MaxCodeSize {
		return fmt.Errorf("o código tem %d bytes, o máximo é %d (64 KB)", len(code), MaxCodeSize)
	}
	return nil
}

// parse parses the submission and returns human-readable syntax errors.
func parse(code string) (*ast.File, []string) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "solution.go", code, parser.ParseComments|parser.AllErrors)
	if err == nil {
		return file, nil
	}
	var list scanner.ErrorList
	if !errors.As(err, &list) {
		return nil, []string{err.Error()}
	}
	var msgs []string
	for i, e := range list {
		if i == maxParseErrors {
			msgs = append(msgs, fmt.Sprintf("... e mais %d erros", len(list)-i))
			break
		}
		msgs = append(msgs, e.Error())
	}
	return nil, msgs
}

// checkAST enforces the structural rules on the submission and returns
// every violation found.
func checkAST(file *ast.File, ex *exercise.Exercise) []string {
	var msgs []string
	if file.Name.Name != "solution" {
		msgs = append(msgs, fmt.Sprintf("o package tem de ser solution, não %s", file.Name.Name))
	}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || !allowedImports[path] {
			msgs = append(msgs, fmt.Sprintf("import não permitido: %s", strings.Trim(imp.Path.Value, `"`)))
		}
	}
	for _, group := range file.Comments {
		for _, c := range group.List {
			if strings.HasPrefix(c.Text, "//go:") {
				msgs = append(msgs, fmt.Sprintf("diretiva não permitida: %s", c.Text))
			}
		}
	}
	found := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		switch fn.Name.Name {
		case "main", "init":
			msgs = append(msgs, fmt.Sprintf("não é permitido definir func %s", fn.Name.Name))
		case ex.Function:
			found = true
			if msg := checkSignature(fn, ex); msg != "" {
				msgs = append(msgs, msg)
			}
		}
	}
	if !found {
		msgs = append(msgs, fmt.Sprintf("função %s não encontrada; assinatura esperada: %s", ex.Function, ex.Signature()))
	}
	return msgs
}

func checkSignature(fn *ast.FuncDecl, ex *exercise.Exercise) string {
	var params []string
	for _, field := range fn.Type.Params.List {
		typ := types.ExprString(field.Type)
		// "a, b int" declares two parameters of the same type.
		for range max(len(field.Names), 1) {
			params = append(params, typ)
		}
	}
	var results []string
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			for range max(len(field.Names), 1) {
				results = append(results, types.ExprString(field.Type))
			}
		}
	}
	ok := fn.Type.TypeParams == nil &&
		equalTypes(params, ex.Params) &&
		equalTypes(results, []string{ex.Returns})
	if ok {
		return ""
	}
	return fmt.Sprintf("assinatura errada de %s; esperada: %s", ex.Function, ex.Signature())
}

// equalTypes compares type expressions after normalising their spelling.
func equalTypes(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		expr, err := parser.ParseExpr(want[i])
		if err != nil || types.ExprString(expr) != got[i] {
			return false
		}
	}
	return true
}

func checkFormat(code string) Check {
	formatted, err := format.Source([]byte(code))
	switch {
	case err != nil:
		return Check{Name: StageFormat, Status: StatusWarning, Messages: []string{"não foi possível formatar o código: " + err.Error()}}
	case !bytes.Equal(formatted, []byte(code)):
		return Check{Name: StageFormat, Status: StatusWarning, Messages: []string{"código não formatado (use gofmt)"}}
	default:
		return Check{Name: StageFormat, Status: StatusOK}
	}
}

func checkComplexity(file *ast.File) Check {
	var msgs []string
	for _, fc := range complexities(file) {
		if fc.value > maxComplexity {
			msgs = append(msgs, fmt.Sprintf("a função %s tem complexidade ciclomática %d (máximo recomendado: %d)", fc.name, fc.value, maxComplexity))
		}
	}
	if len(msgs) > 0 {
		return Check{Name: StageComplexity, Status: StatusWarning, Messages: msgs}
	}
	return Check{Name: StageComplexity, Status: StatusOK}
}

type funcComplexity struct {
	name  string
	value int
}

// complexities returns the cyclomatic complexity of every function and
// method in file, sorted by name. Function literals count towards the
// function that contains them.
func complexities(file *ast.File) []funcComplexity {
	var out []funcComplexity
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		out = append(out, funcComplexity{name: funcName(fn), value: complexity(fn.Body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return types.ExprString(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

// complexity is 1 plus the number of decision points: if, for, range,
// non-default case clauses, && and ||.
func complexity(body ast.Node) int {
	c := 1
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			c++
		case *ast.CaseClause:
			if n.List != nil {
				c++
			}
		case *ast.CommClause:
			if n.Comm != nil {
				c++
			}
		case *ast.BinaryExpr:
			if n.Op == token.LAND || n.Op == token.LOR {
				c++
			}
		}
		return true
	})
	return c
}
