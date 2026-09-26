package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"gavel/internal/client"
	"gavel/internal/engine"
	"gavel/internal/exam"
	"gavel/internal/exercise"
)

// maxCell is the widest a table cell is allowed to be, in runes.
const maxCell = 50

func newTable(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
}

func printExercises(w io.Writer, list []client.ExerciseSummary) {
	tw := newTable(w)
	fmt.Fprintln(tw, "Id\tNível\tTítulo\tDescrição")
	for _, e := range list {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.ID, e.Difficulty.Label(), e.Title, truncate(e.Description))
	}
	_ = tw.Flush()
}

func printExercise(w io.Writer, e *client.ExerciseDetail) {
	fmt.Fprintf(w, "%s (%s)\n", e.Title, e.ID)
	fmt.Fprintf(w, "Nível:      %s (%s)\n", e.Difficulty.Label(), pointsLabel(e.Difficulty.Points()))
	fmt.Fprintf(w, "Assinatura: %s\n", e.Signature)
	fmt.Fprintf(w, "Tempo:      %d ms por teste\n\n", e.TimeoutMS)
	fmt.Fprintf(w, "%s\n\n", e.Description)
	fmt.Fprintf(w, "Entradas dos %d testes:\n", len(e.Tests))
	for i, t := range e.Tests {
		fmt.Fprintf(w, "  %2d. %s(%s)\n", i+1, e.Function, joinJSON(t.Input))
	}
	fmt.Fprintf(w, "\nEsqueleto:\n\npackage solution\n\n%s {\n}\n", e.Signature)
}

func printExams(w io.Writer, exams []*exam.Exam) {
	tw := newTable(w)
	fmt.Fprintln(tw, "Id\tTítulo\tComposição\tDescrição")
	for _, e := range exams {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.ID, e.Title, composition(e.Composition), e.Description)
	}
	_ = tw.Flush()
}

func printSessions(w io.Writer, sessions []client.SessionView) {
	if len(sessions) == 0 {
		fmt.Fprintln(w, "Não há provas abertas. Aguarde que o docente abra uma.")
		return
	}
	tw := newTable(w)
	fmt.Fprintln(tw, "Id\tTítulo\tComposição\tEstado")
	for _, s := range sessions {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.ID, s.Title, composition(s.Composition), sessionState(s.Open, s.EndsAt, s.RemainingSeconds))
	}
	_ = tw.Flush()
	fmt.Fprintln(w, "\nPara entrar: gavel join -student <nome> <session_id>")
}

// sessionState describes whether a session is open and until when.
func sessionState(open bool, endsAt time.Time, remaining int64) string {
	end := endsAt.Local().Format("15:04")
	if !open {
		return "terminada às " + end
	}
	return fmt.Sprintf("aberta até às %s (faltam %d min)", end, (remaining+59)/60)
}

func composition(c map[exercise.Difficulty]int) string {
	var parts []string
	for _, d := range exercise.Difficulties {
		if n := c[d]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d × %s", n, d.Label()))
		}
	}
	return strings.Join(parts, ", ")
}

func printAttempt(w io.Writer, a *client.AttemptView) {
	fmt.Fprintf(w, "Tentativa: %s\n", a.ID)
	if a.Student != "" {
		fmt.Fprintf(w, "Aluno:     %s\n", a.Student)
	}
	fmt.Fprintf(w, "Prova:     %s (%s)\n", a.Title, a.SessionID)
	fmt.Fprintf(w, "Início:    %s\n", a.StartedAt.Local().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "Estado:    %s\n\n", sessionState(a.Open, a.EndsAt, a.RemainingSeconds))
	tw := newTable(w)
	fmt.Fprintln(tw, "Exercício\tNível\tTítulo\tSubmissões\tMelhor score\tPontos")
	for _, e := range a.Score.Exercises {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%.0f%%\t%.2f / %d\n",
			e.ExerciseID, e.Difficulty.Label(), e.Title, e.Submissions, e.BestScore*100, e.Points, e.MaxPoints)
	}
	_ = tw.Flush()
	fmt.Fprintf(w, "\nTotal: %.2f / %d pontos (%.0f%%)\n", a.Score.Points, a.Score.MaxPoints, a.Score.Total*100)
}

func printReport(w io.Writer, r *engine.Report) {
	fmt.Fprintf(w, "Submissão: %s\n", r.SubmissionID)
	fmt.Fprintf(w, "Exercício: %s\n", r.ExerciseID)
	if r.Student != "" {
		fmt.Fprintf(w, "Aluno:     %s\n", r.Student)
	}
	if r.AttemptID != "" {
		fmt.Fprintf(w, "Tentativa: %s\n", r.AttemptID)
	}
	s := r.Summary
	fmt.Fprintf(w, "Veredito:  %s", strings.ToUpper(s.Verdict.Label()))
	if r.Dynamic.Executed {
		fmt.Fprintf(w, " (%d/%d testes, %.0f%%)", s.Passed, s.Total, s.Score*100)
	}
	fmt.Fprintln(w)
	if s.StoppedAt != "" {
		fmt.Fprintf(w, "Parou em:  %s\n", s.StoppedAt)
	}

	fmt.Fprintln(w, "\nAnálise estática:")
	tw := newTable(w)
	for _, c := range r.Static.Checks {
		msgs := c.Messages
		if len(msgs) == 0 {
			msgs = []string{""}
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", c.Status.Label(), c.Name, msgs[0])
		for _, m := range msgs[1:] {
			fmt.Fprintf(tw, "  \t\t%s\n", m)
		}
	}
	_ = tw.Flush()

	if !r.Dynamic.Executed {
		fmt.Fprintln(w, "\nAnálise dinâmica: não executada (o código não passou a análise estática).")
		return
	}
	fmt.Fprintln(w, "\nAnálise dinâmica (testes):")
	tw = newTable(w)
	fmt.Fprintln(tw, "  #\tResultado\tEntrada\tEsperado\tObtido\tTempo\tObservações")
	for i, t := range r.Dynamic.Tests {
		result := "ok"
		if !t.Passed {
			result = "falhou"
		}
		notes := t.Error
		if t.Hint != "" {
			notes = strings.TrimPrefix(notes+"; dica: "+t.Hint, "; ")
		}
		fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%s\t%d ms\t%s\n", i+1, result,
			truncate(joinJSON(t.Input)), truncate(string(t.Expected)), truncate(string(t.Got)), t.DurationMS, notes)
	}
	_ = tw.Flush()
}

func pointsLabel(n int) string {
	if n == 1 {
		return "1 ponto"
	}
	return fmt.Sprintf("%d pontos", n)
}

func joinJSON(values []json.RawMessage) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = string(v)
	}
	return strings.Join(parts, ", ")
}

func truncate(s string) string {
	r := []rune(s)
	if len(r) <= maxCell {
		return s
	}
	return string(r[:maxCell-1]) + "…"
}
