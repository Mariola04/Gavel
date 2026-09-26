package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"gavel/internal/client"
	"gavel/internal/engine"
	"gavel/internal/exercise"
	"gavel/internal/server"
	"gavel/web"
)

// newFlags returns a flag set that reports errors instead of exiting.
func newFlags(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

// parseArgs parses flags and checks the number of positional arguments.
func parseArgs(fs *flag.FlagSet, args []string, names ...string) ([]string, error) {
	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() != len(names) {
		return nil, fmt.Errorf("%w: %s espera os argumentos %v", errUsage, fs.Name(), names)
	}
	return fs.Args(), nil
}

func cmdExercises(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := newFlags("exercises", stderr)
	level := fs.String("difficulty", "", "filtrar por nível (easy, medium, hard)")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	d, err := exercise.ParseDifficulty(*level)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	list, err := c.Exercises(ctx, d)
	if err != nil {
		return err
	}
	printExercises(stdout, list)
	return nil
}

func cmdShow(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	pos, err := parseArgs(newFlags("show", stderr), args, "<id>")
	if err != nil {
		return err
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	ex, err := c.Exercise(ctx, pos[0])
	if err != nil {
		return err
	}
	printExercise(stdout, ex)
	return nil
}

func cmdExams(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if _, err := parseArgs(newFlags("exams", stderr), args); err != nil {
		return err
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	exams, err := c.Exams(ctx)
	if err != nil {
		return err
	}
	printExams(stdout, exams)
	return nil
}

func cmdStart(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	pos, err := parseArgs(newFlags("start", stderr), args, "<exam_id>")
	if err != nil {
		return err
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	a, err := c.StartAttempt(ctx, pos[0])
	if err != nil {
		return err
	}
	printAttempt(stdout, a)
	fmt.Fprintf(stdout, "\nPara submeter: gavel submit -attempt %s <exercise_id> <ficheiro.go>\n", a.ID)
	return nil
}

func cmdSubmit(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := newFlags("submit", stderr)
	attemptID := fs.String("attempt", "", "id da tentativa de prova (opcional)")
	pos, err := parseArgs(fs, args, "<exercise_id>", "<ficheiro.go>")
	if err != nil {
		return err
	}
	code, err := readCode(pos[1])
	if err != nil {
		return err
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	r, err := c.Submit(ctx, client.SubmitRequest{ExerciseID: pos[0], Code: code, AttemptID: *attemptID})
	if err != nil {
		return err
	}
	printReport(stdout, r)
	if r.Summary.Verdict != engine.VerdictPassed {
		return errNotPassed
	}
	return nil
}

// readCode reads at most one byte more than the engine accepts, so that
// oversized files are rejected by the pipeline without being loaded whole.
func readCode(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // the user chooses which file to submit
	if err != nil {
		return "", fmt.Errorf("abrir submissão: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only file
	data, err := io.ReadAll(io.LimitReader(f, engine.MaxCodeSize+1))
	if err != nil {
		return "", fmt.Errorf("ler submissão: %w", err)
	}
	return string(data), nil
}

func cmdAttempt(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	pos, err := parseArgs(newFlags("attempt", stderr), args, "<attempt_id>")
	if err != nil {
		return err
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	a, err := c.Attempt(ctx, pos[0])
	if err != nil {
		return err
	}
	printAttempt(stdout, a)
	return nil
}

func cmdReport(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	pos, err := parseArgs(newFlags("report", stderr), args, "<submission_id>")
	if err != nil {
		return err
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	r, err := c.Report(ctx, pos[0])
	if err != nil {
		return err
	}
	printReport(stdout, r)
	return nil
}

func cmdServe(ctx context.Context, args []string, _, stderr io.Writer) error {
	fs := newFlags("serve", stderr)
	addr := fs.String("addr", "localhost:8080", "endereço onde o servidor escuta")
	sandbox := fs.String("sandbox", string(engine.SandboxAuto), "isolamento da execução: auto, firejail ou none")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	mode, err := engine.ParseSandboxMode(*sandbox)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	c, err := client.OpenLocal(dataDir(), mode)
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(stderr, nil))
	srv := &http.Server{
		Addr:              *addr,
		Handler:           server.New(c, web.FS, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		// Submissions are evaluated synchronously, so responses may be slow.
		WriteTimeout: 3 * time.Minute,
		IdleTimeout:  2 * time.Minute,
		// Cancelling ctx (Ctrl+C) also cancels evaluations in progress.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	logger.Info("servidor a correr", "url", "http://"+*addr, "sandbox", mode)

	select {
	case err := <-errc:
		return fmt.Errorf("servidor: %w", err)
	case <-ctx.Done():
	}
	logger.Info("a terminar o servidor")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("terminar servidor: %w", err)
	}
	return nil
}
