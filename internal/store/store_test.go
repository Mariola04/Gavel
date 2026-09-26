package store

import (
	"errors"
	"testing"
	"time"

	"gavel/internal/engine"
	"gavel/internal/exam"
)

func TestNewIDFormat(t *testing.T) {
	id, err := NewID(time.Date(2026, 9, 26, 11, 2, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !idPattern.MatchString(id) || id[:15] != "20260926T110200" {
		t.Fatalf("id = %q", id)
	}
}

func TestReportsRoundTrip(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	reports := []*engine.Report{
		{SubmissionID: "20260926T110200-000001", AttemptID: "20260926T110000-aaaaaa", Summary: engine.Summary{Score: 0.5}},
		{SubmissionID: "20260926T110300-000002"},
		{SubmissionID: "20260926T110400-000003", AttemptID: "20260926T110000-aaaaaa"},
	}
	for _, r := range reports {
		if err := s.SaveReport(r); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Report("20260926T110200-000001")
	if err != nil || got.Summary.Score != 0.5 {
		t.Fatalf("Report = %+v, %v", got, err)
	}
	byAttempt, err := s.ReportsByAttempt("20260926T110000-aaaaaa")
	if err != nil || len(byAttempt) != 2 {
		t.Fatalf("ReportsByAttempt = %d reports, %v", len(byAttempt), err)
	}
	all, err := s.Reports()
	if err != nil || len(all) != 3 || all[0].SubmissionID != "20260926T110200-000001" {
		t.Fatalf("Reports = %d reports, %v", len(all), err)
	}
}

func TestAttemptRoundTrip(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := &exam.Attempt{ID: "20260926T110000-abcdef", SessionID: "20260926T100000-000000", Student: "Ana", ExerciseIDs: []string{"sum"}}
	if err := s.SaveAttempt(a); err != nil {
		t.Fatal(err)
	}
	got, err := s.Attempt(a.ID)
	if err != nil || got.Student != "Ana" || !got.Contains("sum") {
		t.Fatalf("Attempt = %+v, %v", got, err)
	}
	all, err := s.Attempts()
	if err != nil || len(all) != 1 || all[0].ID != a.ID {
		t.Fatalf("Attempts = %+v, %v", all, err)
	}
}

func TestSessionRoundTrip(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := &exam.Session{ID: "20260926T100000-000000", Title: "Teste", Seed: 7, ExerciseIDs: []string{"sum"}}
	if err := s.SaveSession(sess); err != nil {
		t.Fatal(err)
	}
	got, err := s.Session(sess.ID)
	if err != nil || got.Seed != 7 || got.Title != "Teste" {
		t.Fatalf("Session = %+v, %v", got, err)
	}
	all, err := s.Sessions()
	if err != nil || len(all) != 1 {
		t.Fatalf("Sessions = %+v, %v", all, err)
	}
	if _, err := s.Session("20260926T100000-ffffff"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing session error = %v", err)
	}
}

func TestNotFoundAndInvalidIDs(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"20260926T110000-abcdef", "../../etc/passwd", ""} {
		if _, err := s.Report(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Report(%q) error = %v, want ErrNotFound", id, err)
		}
	}
	if err := s.SaveReport(&engine.Report{SubmissionID: "../x"}); err == nil {
		t.Error("SaveReport accepted an invalid id")
	}
}
