package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"gavel/internal/client"
	"gavel/internal/engine"
	"gavel/internal/exam"
	"gavel/internal/exercise"
	"gavel/internal/store"
)

const correctFactorial = "package solution\n\nfunc Factorial(n int) int {\n\tif n <= 1 {\n\t\treturn 1\n\t}\n\treturn n * Factorial(n-1)\n}\n"

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	repo, err := exercise.Load("../../data/exercises")
	if err != nil {
		t.Fatal(err)
	}
	exams, err := exam.LoadCatalog("../../data/exams", repo)
	if err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Config{CacheDir: "../../data/.cache", Sandbox: engine.SandboxAuto})
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	web := fstest.MapFS{"index.html": {Data: []byte("<h1>Gavel</h1>")}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ts := httptest.NewServer(New(client.NewLocal(repo, exams, eng, st), web, logger))
	t.Cleanup(ts.Close)
	return ts
}

// call sends a request and decodes the JSON response into out (if not nil).
func call(t *testing.T, method, url, body string, out any) int {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s %s: %v", method, url, err)
		}
	}
	return resp.StatusCode
}

func submitBody(t *testing.T, req client.SubmitRequest) string {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type errorBody struct {
	Error string `json:"error"`
}

func TestExerciseRoutes(t *testing.T) {
	ts := newTestServer(t)

	var all []client.ExerciseSummary
	if code := call(t, "GET", ts.URL+"/api/exercises", "", &all); code != 200 || len(all) < 12 {
		t.Fatalf("list: status %d, %d exercises", code, len(all))
	}
	var easy []client.ExerciseSummary
	if code := call(t, "GET", ts.URL+"/api/exercises?difficulty=easy", "", &easy); code != 200 {
		t.Fatalf("filter: status %d", code)
	}
	for _, e := range easy {
		if e.Difficulty != exercise.Easy {
			t.Errorf("filter returned %s (%s)", e.ID, e.Difficulty)
		}
	}
	var e errorBody
	if code := call(t, "GET", ts.URL+"/api/exercises?difficulty=impossible", "", &e); code != 400 || e.Error == "" {
		t.Errorf("bad filter: status %d, error %q", code, e.Error)
	}

	resp, err := http.Get(ts.URL + "/api/exercises/factorial")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !bytes.Contains(body, []byte(`"signature"`)) {
		t.Fatalf("detail: status %d, body %s", resp.StatusCode, body)
	}
	if bytes.Contains(body, []byte(`"output"`)) {
		t.Error("detail leaks expected outputs")
	}
	if code := call(t, "GET", ts.URL+"/api/exercises/nope", "", &e); code != 404 {
		t.Errorf("unknown exercise: status %d", code)
	}
}

func TestExamAndAttemptRoutes(t *testing.T) {
	ts := newTestServer(t)

	var exams []exam.Exam
	if code := call(t, "GET", ts.URL+"/api/exams", "", &exams); code != 200 || len(exams) != 3 {
		t.Fatalf("exams: status %d, %d exams", code, len(exams))
	}
	var e errorBody
	if code := call(t, "POST", ts.URL+"/api/exams/nope/attempts", "", &e); code != 404 {
		t.Errorf("unknown exam: status %d", code)
	}

	var a client.AttemptView
	if code := call(t, "POST", ts.URL+"/api/exams/easy/attempts", "", &a); code != 201 || len(a.ExerciseIDs) != 3 {
		t.Fatalf("start: status %d, attempt %+v", code, a)
	}
	var got client.AttemptView
	if code := call(t, "GET", ts.URL+"/api/attempts/"+a.ID, "", &got); code != 200 || got.Seed != a.Seed || got.Score.MaxPoints != 4 {
		t.Fatalf("get attempt: status %d, attempt %+v", code, got)
	}
	if code := call(t, "GET", ts.URL+"/api/attempts/20000101T000000-000000", "", &e); code != 404 {
		t.Errorf("unknown attempt: status %d", code)
	}
}

func TestSubmissionRoutes(t *testing.T) {
	ts := newTestServer(t)

	var r engine.Report
	body := submitBody(t, client.SubmitRequest{ExerciseID: "factorial", Code: correctFactorial})
	if code := call(t, "POST", ts.URL+"/api/submissions", body, &r); code != 200 || r.Summary.Verdict != engine.VerdictPassed {
		t.Fatalf("submit: status %d, report %+v", code, r)
	}
	var saved engine.Report
	if code := call(t, "GET", ts.URL+"/api/submissions/"+r.SubmissionID, "", &saved); code != 200 || saved.SubmissionID != r.SubmissionID {
		t.Fatalf("get report: status %d, report %+v", code, saved)
	}

	var e errorBody
	for name, tc := range map[string]struct {
		url, body string
		status    int
	}{
		"unknown report":   {"/api/submissions/20000101T000000-000000", "", 404},
		"unknown exercise": {"/api/submissions", `{"exercise_id":"nope","code":""}`, 404},
		"missing exercise": {"/api/submissions", `{"code":""}`, 400},
		"invalid json":     {"/api/submissions", `{"exercise_id":`, 400},
		"unknown field":    {"/api/submissions", `{"exercise":"factorial"}`, 400},
		"body too large":   {"/api/submissions", `{"code":"` + strings.Repeat("a", maxBodyBytes) + `"}`, 413},
		"unknown route":    {"/api/nothing", "", 404},
	} {
		method := "GET"
		if tc.body != "" {
			method = "POST"
		}
		if code := call(t, method, ts.URL+tc.url, tc.body, &e); code != tc.status || e.Error == "" {
			t.Errorf("%s: status %d (want %d), error %q", name, code, tc.status, e.Error)
		}
	}
}

func TestSubmissionWithAttempt(t *testing.T) {
	ts := newTestServer(t)

	var a client.AttemptView
	if code := call(t, "POST", ts.URL+"/api/exams/easy/attempts", "", &a); code != 201 {
		t.Fatalf("start: status %d", code)
	}

	// An exercise that is not part of the attempt is rejected.
	outside := "factorial"
	for _, id := range []string{"factorial", "sum", "reverse", "palindrome"} {
		if !a.Contains(id) {
			outside = id
			break
		}
	}
	var e errorBody
	body := submitBody(t, client.SubmitRequest{ExerciseID: outside, Code: "package solution\n", AttemptID: a.ID})
	if code := call(t, "POST", ts.URL+"/api/submissions", body, &e); code != 400 {
		t.Fatalf("exercise outside attempt: status %d, error %q", code, e.Error)
	}

	// A submission to an exercise of the attempt is counted in its score.
	target := a.ExerciseIDs[0]
	body = submitBody(t, client.SubmitRequest{ExerciseID: target, Code: "package solution\n", AttemptID: a.ID})
	var r engine.Report
	if code := call(t, "POST", ts.URL+"/api/submissions", body, &r); code != 200 || r.AttemptID != a.ID {
		t.Fatalf("submit in attempt: status %d, report %+v", code, r)
	}
	var got client.AttemptView
	call(t, "GET", ts.URL+"/api/attempts/"+a.ID, "", &got)
	if got.Score.Exercises[0].Submissions != 1 {
		t.Fatalf("attempt score = %+v", got.Score)
	}
}

func TestCORSAndStaticFiles(t *testing.T) {
	ts := newTestServer(t)

	req, _ := http.NewRequest(http.MethodOptions, ts.URL+"/api/submissions", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent || resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("OPTIONS: status %d, headers %v", resp.StatusCode, resp.Header)
	}

	resp, err = http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "Gavel") {
		t.Errorf("GET /: status %d, body %q", resp.StatusCode, body)
	}
}

func TestRemoteClient(t *testing.T) {
	ts := newTestServer(t)
	c := client.NewRemote(ts.URL)
	ctx := context.Background()

	list, err := c.Exercises(ctx, exercise.Hard)
	if err != nil || len(list) == 0 {
		t.Fatalf("Exercises: %v, %v", list, err)
	}
	if _, err := c.Exercise(ctx, "nope"); !errors.Is(err, client.ErrNotFound) {
		t.Errorf("Exercise(nope) error = %v, want ErrNotFound", err)
	}
	a, err := c.StartAttempt(ctx, "hard")
	if err != nil || len(a.ExerciseIDs) != 4 {
		t.Fatalf("StartAttempt: %+v, %v", a, err)
	}
	_, err = c.Submit(ctx, client.SubmitRequest{ExerciseID: "factorial", Code: correctFactorial, AttemptID: a.ID})
	if !errors.Is(err, client.ErrInvalid) {
		t.Errorf("Submit outside attempt error = %v, want ErrInvalid", err)
	}
	r, err := c.Submit(ctx, client.SubmitRequest{ExerciseID: "factorial", Code: correctFactorial})
	if err != nil || r.Summary.Verdict != engine.VerdictPassed {
		t.Fatalf("Submit: %+v, %v", r, err)
	}
	if _, err := c.Report(ctx, r.SubmissionID); err != nil {
		t.Errorf("Report: %v", err)
	}
}
