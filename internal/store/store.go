// Package store persists submission reports and exam attempts as JSON
// files.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gavel/internal/engine"
	"gavel/internal/exam"
)

// ErrNotFound is returned when a report or attempt does not exist.
var ErrNotFound = errors.New("não encontrado")

// idPattern matches ids produced by NewID. Checking it before building a
// path rules out path traversal.
var idPattern = regexp.MustCompile(`^\d{8}T\d{6}-[0-9a-f]{6}$`)

// NewID returns a timestamp followed by 6 random hex characters, such as
// "20260926T110200-a1b2c3".
func NewID(now time.Time) (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("gerar id: %w", err)
	}
	return now.UTC().Format("20060102T150405") + "-" + hex.EncodeToString(b), nil
}

// Store reads and writes JSON files under a data directory.
type Store struct {
	reportsDir  string
	attemptsDir string
}

// New creates the reports and attempts directories under dataDir.
func New(dataDir string) (*Store, error) {
	s := &Store{
		reportsDir:  filepath.Join(dataDir, "reports"),
		attemptsDir: filepath.Join(dataDir, "attempts"),
	}
	for _, dir := range []string{s.reportsDir, s.attemptsDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("criar diretoria %s: %w", dir, err)
		}
	}
	return s, nil
}

// SaveReport writes a submission report.
func (s *Store) SaveReport(r *engine.Report) error {
	return save(s.reportsDir, r.SubmissionID, r)
}

// Report reads a submission report.
func (s *Store) Report(id string) (*engine.Report, error) {
	var r engine.Report
	if err := load(s.reportsDir, id, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ReportsByAttempt returns every report associated with the attempt.
func (s *Store) ReportsByAttempt(attemptID string) ([]*engine.Report, error) {
	entries, err := os.ReadDir(s.reportsDir)
	if err != nil {
		return nil, fmt.Errorf("listar relatórios: %w", err)
	}
	var reports []*engine.Report
	for _, entry := range entries {
		id, ok := strings.CutSuffix(entry.Name(), ".json")
		if !ok || !idPattern.MatchString(id) {
			continue
		}
		r, err := s.Report(id)
		if err != nil {
			return nil, err
		}
		if r.AttemptID == attemptID {
			reports = append(reports, r)
		}
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].SubmissionID < reports[j].SubmissionID })
	return reports, nil
}

// SaveAttempt writes an exam attempt.
func (s *Store) SaveAttempt(a *exam.Attempt) error {
	return save(s.attemptsDir, a.ID, a)
}

// Attempt reads an exam attempt.
func (s *Store) Attempt(id string) (*exam.Attempt, error) {
	var a exam.Attempt
	if err := load(s.attemptsDir, id, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// save writes v to dir/id.json atomically, so readers never see a
// partially written file.
func save(dir, id string, v any) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("id inválido %q", id)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar %s: %w", id, err)
	}
	tmp, err := os.CreateTemp(dir, id+".*.tmp")
	if err != nil {
		return fmt.Errorf("criar ficheiro: %w", err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("escrever %s: %w", id, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("escrever %s: %w", id, err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, id+".json")); err != nil {
		return fmt.Errorf("guardar %s: %w", id, err)
	}
	return nil
}

func load(dir, id string, v any) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	data, err := os.ReadFile(filepath.Join(dir, id+".json")) //nolint:gosec // id validated above
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	if err != nil {
		return fmt.Errorf("ler %s: %w", id, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("ler %s: JSON inválido: %w", id, err)
	}
	return nil
}
