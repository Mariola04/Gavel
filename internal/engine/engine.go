// Package engine implements the fail-fast evaluation pipeline for
// submissions: static checks, external tools, build and execution.
package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"gavel/internal/exercise"
)

// maxConcurrent is how many evaluations may run at the same time.
const maxConcurrent = 2

// minimalPath is the PATH given to the student's program.
const minimalPath = "/usr/bin:/bin"

// Config configures an Engine.
type Config struct {
	// CacheDir holds the shared Go build cache.
	CacheDir string
	// Sandbox selects how the compiled program is isolated.
	Sandbox SandboxMode
}

// Engine evaluates submissions. It is safe for concurrent use.
type Engine struct {
	goPath       string
	gosecPath    string
	firejailPath string
	// sandboxWarning is set when running without isolation in auto mode.
	sandboxWarning string
	path           string
	cacheDir       string
	sem            chan struct{}
}

// New locates the required tools and prepares the build cache.
func New(cfg Config) (*Engine, error) {
	goPath, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("o comando go não foi encontrado no PATH: %w", err)
	}
	cacheDir, err := filepath.Abs(cfg.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("resolver diretoria de cache: %w", err)
	}
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return nil, fmt.Errorf("criar diretoria de cache: %w", err)
	}
	e := &Engine{
		goPath:   goPath,
		path:     filepath.Dir(goPath) + string(os.PathListSeparator) + minimalPath,
		cacheDir: cacheDir,
		sem:      make(chan struct{}, maxConcurrent),
	}
	// gosec is optional: when it is missing the stage is skipped.
	e.gosecPath, _ = exec.LookPath("gosec")

	firejail, lookErr := exec.LookPath("firejail")
	switch cfg.Sandbox {
	case SandboxFirejail:
		if lookErr != nil {
			return nil, fmt.Errorf("sandbox firejail pedida mas o firejail não foi encontrado: %w", lookErr)
		}
		e.firejailPath = firejail
	case SandboxAuto, "":
		if lookErr == nil {
			e.firejailPath = firejail
		} else {
			e.sandboxWarning = "firejail não encontrado: o programa correu sem isolamento de rede e de sistema de ficheiros"
		}
	case SandboxNone:
	default:
		return nil, fmt.Errorf("modo de sandbox inválido %q", cfg.Sandbox)
	}
	return e, nil
}

// Evaluate runs the full pipeline on code for exercise ex. Failures are
// described in the report; the caller sets the submission metadata.
func (e *Engine) Evaluate(ctx context.Context, ex *exercise.Exercise, code string) *Report {
	r := &Report{ExerciseID: ex.ID, Code: code}
	if !checkStatic(r, ex, code) {
		return r
	}

	// Only the expensive stages (tools, build, run) are rate limited.
	select {
	case e.sem <- struct{}{}:
		defer func() { <-e.sem }()
	case <-ctx.Done():
		return r.fail(StageVet, VerdictInternalError, "avaliação cancelada: "+ctx.Err().Error())
	}

	dir, err := os.MkdirTemp("", "gavel-*")
	if err != nil {
		return r.fail(StageBuild, VerdictInternalError, "criar diretoria temporária: "+err.Error())
	}
	defer os.RemoveAll(dir) //nolint:errcheck // best-effort cleanup of a temporary directory

	if err := writeModule(dir, ex, code); err != nil {
		return r.fail(StageBuild, VerdictInternalError, err.Error())
	}
	if !e.checkTools(ctx, r, dir) {
		return r
	}
	if out, err := e.build(ctx, dir); err != nil {
		return r.fail(StageBuild, VerdictCompileError, toolMessages(out, dir)...)
	}
	r.addCheck(StageBuild, StatusOK)
	r.Static.Passed = true

	if e.sandboxWarning != "" {
		r.addCheck(StageSandbox, StatusWarning, e.sandboxWarning)
	}
	tests, err := e.run(ctx, dir, ex)
	if err != nil {
		return r.fail(StageRun, VerdictInternalError, err.Error())
	}
	r.Dynamic = DynamicResult{Executed: true, Tests: tests}
	return r.finish()
}

// checkStatic runs stages 1 to 5 and reports whether evaluation may go on.
func checkStatic(r *Report, ex *exercise.Exercise, code string) bool {
	if err := checkLimits(code); err != nil {
		r.fail(StageLimits, VerdictRejected, err.Error())
		return false
	}
	r.addCheck(StageLimits, StatusOK)

	file, msgs := parse(code)
	if len(msgs) > 0 {
		r.fail(StageParse, VerdictRejected, msgs...)
		return false
	}
	r.addCheck(StageParse, StatusOK)

	if msgs := checkAST(file, ex); len(msgs) > 0 {
		r.fail(StageAST, VerdictRejected, msgs...)
		return false
	}
	r.addCheck(StageAST, StatusOK)

	// Formatting and complexity only produce warnings, so they run in
	// parallel and never stop the pipeline.
	var formatCheck, complexityCheck Check
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		formatCheck = checkFormat(code)
	}()
	go func() {
		defer wg.Done()
		complexityCheck = checkComplexity(file)
	}()
	wg.Wait()
	r.Static.Checks = append(r.Static.Checks, formatCheck, complexityCheck)
	return true
}

// checkTools runs go vet and gosec in parallel and reports whether
// evaluation may go on.
func (e *Engine) checkTools(ctx context.Context, r *Report, dir string) bool {
	var vetRes vetResult
	var gosecCheck Check
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		vetRes = e.vet(ctx, dir)
	}()
	go func() {
		defer wg.Done()
		gosecCheck = e.gosec(ctx, dir)
	}()
	wg.Wait()
	r.Static.Checks = append(r.Static.Checks, vetRes.check, gosecCheck)

	switch {
	case vetRes.typeErrors:
		r.stop(StageVet, VerdictCompileError)
	case vetRes.check.Status == StatusError:
		r.stop(StageVet, VerdictRejected)
	case gosecCheck.Status == StatusError:
		r.stop(StageGosec, VerdictRejected)
	default:
		return true
	}
	return false
}

// writeModule lays out the temporary module:
//
//	go.mod                -> module sandbox
//	solution/solution.go  -> the student's code
//	main.go               -> the generated harness
func writeModule(dir string, ex *exercise.Exercise, code string) error {
	harness, err := generateHarness(ex)
	if err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(dir, "solution"), 0o750); err != nil {
		return fmt.Errorf("criar módulo temporário: %w", err)
	}
	files := map[string][]byte{
		"go.mod":               []byte("module sandbox\n\ngo 1.22\n"),
		"main.go":              harness,
		"solution/solution.go": []byte(code),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			return fmt.Errorf("escrever %s: %w", name, err)
		}
	}
	return nil
}
