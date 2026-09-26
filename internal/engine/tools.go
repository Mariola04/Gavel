package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// toolTimeout bounds each external tool (vet, gosec, build).
const toolTimeout = 60 * time.Second

// vetResult is the outcome of go vet. typeErrors is set when vet failed
// because the code does not type-check, which is a compilation error rather
// than a vet finding.
type vetResult struct {
	check      Check
	typeErrors bool
}

func (e *Engine) vet(ctx context.Context, dir string) vetResult {
	out, err := e.goCommand(ctx, dir, "vet", "./solution")
	if err == nil {
		return vetResult{check: Check{Name: StageVet, Status: StatusOK}}
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return vetResult{check: Check{Name: StageVet, Status: StatusError, Messages: []string{"falha ao correr go vet: " + err.Error()}}}
	}
	msgs := toolMessages(out, dir)
	typeErrors := false
	for i, m := range msgs {
		if rest, ok := strings.CutPrefix(m, "vet: "); ok {
			msgs[i] = rest
			typeErrors = true
		}
	}
	return vetResult{check: Check{Name: StageVet, Status: StatusError, Messages: msgs}, typeErrors: typeErrors}
}

// gosecReport is the subset of gosec's JSON output that we use.
type gosecReport struct {
	Issues []struct {
		Severity string `json:"severity"`
		RuleID   string `json:"rule_id"`
		Details  string `json:"details"`
		Line     string `json:"line"`
	} `json:"Issues"`
}

// gosec runs the gosec security scanner. HIGH severity issues produce an
// error status (blocking); anything else is a warning.
func (e *Engine) gosec(ctx context.Context, dir string) Check {
	if e.gosecPath == "" {
		return Check{Name: StageGosec, Status: StatusSkipped, Messages: []string{"gosec não está instalado"}}
	}
	ctx, cancel := context.WithTimeout(ctx, toolTimeout)
	defer cancel()
	// -nosec makes gosec ignore "#nosec" annotations written by the student.
	cmd := exec.CommandContext(ctx, e.gosecPath, "-fmt=json", "-nosec", "./solution") //nolint:gosec // fixed arguments
	cmd.Dir = dir
	cmd.Env = e.goEnv(dir)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	runErr := cmd.Run()

	var report gosecReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		msg := "não foi possível interpretar o resultado do gosec"
		if runErr != nil {
			msg += ": " + runErr.Error()
		}
		return Check{Name: StageGosec, Status: StatusWarning, Messages: []string{msg}}
	}
	status := StatusOK
	var msgs []string
	for _, issue := range report.Issues {
		if issue.Severity == "HIGH" {
			status = StatusError
		} else if status == StatusOK {
			status = StatusWarning
		}
		msgs = append(msgs, fmt.Sprintf("linha %s: [%s %s] %s", issue.Line, issue.Severity, issue.RuleID, issue.Details))
	}
	return Check{Name: StageGosec, Status: status, Messages: msgs}
}

// goCommand runs the go tool inside the temporary module and returns its
// combined output.
func (e *Engine) goCommand(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, toolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.goPath, args...) //nolint:gosec // arguments are fixed by the engine
	cmd.Dir = dir
	cmd.Env = e.goEnv(dir)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// goEnv is the environment for the go tool: offline, no cgo, no toolchain
// downloads, and a shared build cache so repeated builds are fast.
func (e *Engine) goEnv(dir string) []string {
	return []string{
		"PATH=" + e.path,
		"HOME=" + dir,
		"GOCACHE=" + filepath.Join(e.cacheDir, "go-build"),
		"GOPATH=" + filepath.Join(e.cacheDir, "gopath"),
		"GOTOOLCHAIN=local",
		"GOPROXY=off",
		"CGO_ENABLED=0",
	}
}

// toolMessages turns tool output into one message per line, dropping
// package headers and the temporary directory prefix.
func toolMessages(out, dir string) []string {
	var msgs []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.ReplaceAll(line, dir+string(filepath.Separator), ""))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		msgs = append(msgs, line)
	}
	return msgs
}
