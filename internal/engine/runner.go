package engine

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"gavel/internal/exercise"
)

// maxOutput caps how much of the program's stdout and stderr is kept.
const maxOutput = 1 << 20

// runMargin is added to the sum of the per-test timeouts to obtain the
// global timeout (process start-up, JSON encoding, etc.).
const runMargin = 2 * time.Second

// SandboxMode selects how the compiled submission is isolated.
type SandboxMode string

// Supported sandbox modes.
const (
	SandboxAuto     SandboxMode = "auto"
	SandboxFirejail SandboxMode = "firejail"
	SandboxNone     SandboxMode = "none"
)

// ParseSandboxMode validates a --sandbox flag value.
func ParseSandboxMode(s string) (SandboxMode, error) {
	switch m := SandboxMode(s); m {
	case SandboxAuto, SandboxFirejail, SandboxNone:
		return m, nil
	default:
		return "", fmt.Errorf("modo de sandbox inválido %q (use auto, firejail ou none)", s)
	}
}

// harnessResult is one line of harness output.
type harnessResult struct {
	Nonce      string          `json:"nonce"`
	Index      int             `json:"index"`
	Got        json.RawMessage `json:"got"`
	Error      string          `json:"error"`
	DurationMS int64           `json:"duration_ms"`
}

// build compiles the temporary module into dir/prog.
func (e *Engine) build(ctx context.Context, dir string) (string, error) {
	return e.goCommand(ctx, dir, "build", "-o", "prog", ".")
}

// run executes the compiled program with every test and returns the
// per-test results.
func (e *Engine) run(ctx context.Context, dir string, ex *exercise.Exercise) ([]TestResult, error) {
	inputs := make([][]json.RawMessage, len(ex.Tests))
	for i, t := range ex.Tests {
		inputs[i] = t.Input
	}
	stdin, err := json.Marshal(inputs)
	if err != nil {
		return nil, fmt.Errorf("serializar testes: %w", err)
	}
	nonce, err := newNonce()
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(len(ex.Tests))*ex.Timeout() + runMargin
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	prog := filepath.Join(dir, "prog")
	var cmd *exec.Cmd
	if e.firejailPath != "" {
		cmd = exec.CommandContext(ctx, e.firejailPath, //nolint:gosec // fixed arguments
			"--quiet", "--net=none", "--private="+dir, "--noroot", prog)
	} else {
		cmd = exec.CommandContext(ctx, prog) //nolint:gosec // binary we just built
	}
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + minimalPath, nonceEnv + "=" + nonce}
	cmd.Stdin = bytes.NewReader(stdin)
	stdout := &limitedBuffer{limit: maxOutput}
	stderr := &limitedBuffer{limit: maxOutput}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	killProcessGroup(cmd)

	runErr := cmd.Run()
	results := parseResults(stdout.String(), nonce)

	// Explain missing results: an earlier test timed out (the harness stops
	// there), global timeout, output overflow, or a crash.
	missing := ""
	switch {
	case hasTimeout(results):
		missing = "não executado: um teste anterior esgotou o tempo"
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		missing = errTimeout
	case stdout.truncated:
		missing = "a saída excedeu 1 MB"
	case runErr != nil:
		missing = "o programa terminou inesperadamente (" + runErr.Error() + ")"
		if msg := lastLine(stderr.String()); msg != "" {
			missing += ": " + msg
		}
	default:
		missing = "sem resultado"
	}

	tests := make([]TestResult, len(ex.Tests))
	for i, t := range ex.Tests {
		tr := TestResult{Input: t.Input, Expected: t.Output}
		if r, ok := results[i]; ok {
			tr.Got, tr.Error, tr.DurationMS = r.Got, r.Error, r.DurationMS
			tr.Passed = r.Error == "" && equalJSON(r.Got, t.Output)
		} else {
			tr.Error = missing
		}
		if !tr.Passed {
			tr.Hint = t.Hint
		}
		tests[i] = tr
	}
	return tests, nil
}

// parseResults extracts the result lines that carry the expected nonce.
// Every other line is ignored.
func parseResults(out, nonce string) map[int]harnessResult {
	results := make(map[int]harnessResult)
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 0, 64<<10), maxOutput)
	for sc.Scan() {
		data, ok := strings.CutPrefix(sc.Text(), resultPrefix)
		if !ok {
			continue
		}
		var r harnessResult
		if err := json.Unmarshal([]byte(data), &r); err != nil || r.Nonce != nonce {
			continue
		}
		results[r.Index] = r
	}
	return results
}

func hasTimeout(results map[int]harnessResult) bool {
	for _, r := range results {
		if r.Error == errTimeout {
			return true
		}
	}
	return false
}

// equalJSON compares two JSON values structurally. Numbers are compared as
// float64, and null is equal to an empty array or object, because a nil
// slice or map in Go is serialised as null.
func equalJSON(a, b json.RawMessage) bool {
	var va, vb any
	if json.Unmarshal(a, &va) != nil || json.Unmarshal(b, &vb) != nil {
		return false
	}
	return reflect.DeepEqual(normalize(va), normalize(vb))
}

// normalize replaces empty arrays and objects with nil, recursively.
func normalize(v any) any {
	switch v := v.(type) {
	case []any:
		if len(v) == 0 {
			return nil
		}
		for i := range v {
			v[i] = normalize(v[i])
		}
	case map[string]any:
		if len(v) == 0 {
			return nil
		}
		for k := range v {
			v[k] = normalize(v[k])
		}
	}
	return v
}

func newNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("gerar nonce: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// limitedBuffer keeps at most limit bytes and silently discards the rest,
// so that a chatty program cannot exhaust memory or block on a full pipe.
type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.limit - b.buf.Len(); len(p) > room {
		b.buf.Write(p[:max(room, 0)])
		b.truncated = true
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *limitedBuffer) String() string { return b.buf.String() }
