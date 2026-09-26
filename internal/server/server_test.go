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
	"slices"
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

// testAdminPassword is a dummy password used only by these tests.
const testAdminPassword = "test-only-password"

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newTestServerWithAdmin(t, testAdminPassword)
}

func newTestServerWithAdmin(t *testing.T, adminPassword string) *httptest.Server {
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
	c := client.NewLocal(repo, exams, eng, st)
	ts := httptest.NewServer(New(Config{Client: c, Admin: c, AdminPassword: adminPassword, Web: web, Logger: logger}))
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

// adminCall sends a teacher request with the given password (none if
// empty) and decodes the JSON response into out (if not nil).
func adminCall(t *testing.T, method, url, password, body string, out any) int {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if password != "" {
		req.Header.Set("Authorization", "Bearer "+password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode
}

// createSession opens a session through the teacher API.
func createSession(t *testing.T, ts *httptest.Server, body string) client.SessionView {
	t.Helper()
	var s client.SessionView
	if code := adminCall(t, "POST", ts.URL+"/api/admin/sessions", testAdminPassword, body, &s); code != 201 {
		t.Fatalf("create session: status %d", code)
	}
	return s
}

// join joins a session as student and returns the attempt.
func join(t *testing.T, ts *httptest.Server, sessionID, student string) client.AttemptView {
	t.Helper()
	var a client.AttemptView
	if code := call(t, "POST", ts.URL+"/api/sessions/"+sessionID+"/join", `{"student":"`+student+`"}`, &a); code != 200 {
		t.Fatalf("join %s as %q: status %d", sessionID, student, code)
	}
	return a
}

func TestExamPresets(t *testing.T) {
	ts := newTestServer(t)
	var exams []exam.Exam
	if code := call(t, "GET", ts.URL+"/api/exams", "", &exams); code != 200 || len(exams) != 3 {
		t.Fatalf("exams: status %d, %d exams", code, len(exams))
	}
	var e errorBody
	if code := call(t, "POST", ts.URL+"/api/exams/easy/attempts", "", &e); code != 404 {
		t.Errorf("self-start is gone, but POST /api/exams/easy/attempts gave %d", code)
	}
}

func TestSessionsListIsEmptyArray(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/sessions")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.TrimSpace(string(body)) != "[]" {
		t.Errorf("no open sessions should be [], got %s", body)
	}
}

func TestSessionJoin(t *testing.T) {
	ts := newTestServer(t)

	// The teacher creates a session from a preset; students see it without
	// its exercises.
	s := createSession(t, ts, `{"preset":"medium","duration_minutes":60}`)
	if s.Title != "Teste médio" || len(s.ExerciseIDs) != 4 || !s.Open || s.RemainingSeconds < 3590 {
		t.Fatalf("created session = %+v", s)
	}
	var open []client.SessionView
	if code := call(t, "GET", ts.URL+"/api/sessions", "", &open); code != 200 || len(open) != 1 || open[0].ExerciseIDs != nil {
		t.Fatalf("open sessions: status %d, %+v", code, open)
	}

	// Every student gets the session's exercises; joining again resumes.
	ana := join(t, ts, s.ID, "Ana")
	rui := join(t, ts, s.ID, "Rui")
	if !slices.Equal(ana.ExerciseIDs, s.ExerciseIDs) || !slices.Equal(rui.ExerciseIDs, s.ExerciseIDs) {
		t.Fatalf("exercises differ: session %v, ana %v, rui %v", s.ExerciseIDs, ana.ExerciseIDs, rui.ExerciseIDs)
	}
	if again := join(t, ts, s.ID, " ana "); again.ID != ana.ID {
		t.Errorf("joining again created a new attempt: %s vs %s", again.ID, ana.ID)
	}
	if ana.Title != s.Title || !ana.Open || ana.Score.MaxPoints != 8 {
		t.Errorf("attempt view = %+v", ana)
	}
}

func TestSessionJoinErrors(t *testing.T) {
	ts := newTestServer(t)
	s := createSession(t, ts, `{"preset":"easy","duration_minutes":60}`)
	var e errorBody
	if code := call(t, "POST", ts.URL+"/api/sessions/"+s.ID+"/join", `{"student":" "}`, &e); code != 400 {
		t.Errorf("join without a name: status %d", code)
	}
	if code := call(t, "POST", ts.URL+"/api/sessions/20000101T000000-000000/join", `{"student":"Ana"}`, &e); code != 404 {
		t.Errorf("join unknown session: status %d", code)
	}
}

func TestSessionSubmissions(t *testing.T) {
	ts := newTestServer(t)
	s := createSession(t, ts, `{"preset":"easy","duration_minutes":60}`)
	ana := join(t, ts, s.ID, "Ana")

	// A submission inside the session counts, and takes the attempt's name.
	body := submitBody(t, client.SubmitRequest{ExerciseID: s.ExerciseIDs[0], Code: "package solution\n", AttemptID: ana.ID, Student: "Outro"})
	var r engine.Report
	if code := call(t, "POST", ts.URL+"/api/submissions", body, &r); code != 200 || r.Student != "Ana" {
		t.Fatalf("submit: status %d, student %q", code, r.Student)
	}
	var got client.AttemptView
	call(t, "GET", ts.URL+"/api/attempts/"+ana.ID, "", &got)
	if got.Score.Exercises[0].Submissions != 1 {
		t.Fatalf("score = %+v", got.Score)
	}

	// An exercise outside the session is refused.
	outside := "factorial"
	for _, id := range []string{"factorial", "sum", "reverse", "palindrome", "leap"} {
		if !ana.Contains(id) {
			outside = id
			break
		}
	}
	var e errorBody
	body = submitBody(t, client.SubmitRequest{ExerciseID: outside, Code: "package solution\n", AttemptID: ana.ID})
	if code := call(t, "POST", ts.URL+"/api/submissions", body, &e); code != 400 {
		t.Errorf("exercise outside the session: status %d", code)
	}
}

func TestSessionClose(t *testing.T) {
	ts := newTestServer(t)
	s := createSession(t, ts, `{"preset":"easy","duration_minutes":60}`)
	ana := join(t, ts, s.ID, "Ana")

	var closed client.SessionView
	if code := adminCall(t, "POST", ts.URL+"/api/admin/sessions/"+s.ID+"/close", testAdminPassword, "", &closed); code != 200 || closed.Open {
		t.Fatalf("close: status %d, %+v", code, closed)
	}
	// No new students and no submissions, but the attempt can still be seen.
	var e errorBody
	if code := call(t, "POST", ts.URL+"/api/sessions/"+s.ID+"/join", `{"student":"Novo"}`, &e); code != 400 || !strings.Contains(e.Error, "terminou") {
		t.Errorf("join after close: status %d, error %q", code, e.Error)
	}
	body := submitBody(t, client.SubmitRequest{ExerciseID: s.ExerciseIDs[0], Code: "package solution\n", AttemptID: ana.ID})
	if code := call(t, "POST", ts.URL+"/api/submissions", body, &e); code != 400 || !strings.Contains(e.Error, "terminou") {
		t.Errorf("submit after close: status %d, error %q", code, e.Error)
	}
	if again := join(t, ts, s.ID, "Ana"); again.ID != ana.ID || again.Open {
		t.Errorf("resume after close = %+v", again)
	}
	var open []client.SessionView
	call(t, "GET", ts.URL+"/api/sessions", "", &open)
	if len(open) != 0 {
		t.Errorf("closed session still listed: %+v", open)
	}
}

func TestSessionWithPickedExercises(t *testing.T) {
	ts := newTestServer(t)
	s := createSession(t, ts, `{"title":"Escolhidos","exercise_ids":["roman-numerals","sum","isogram"],"duration_minutes":30}`)
	if !slices.Equal(s.ExerciseIDs, []string{"roman-numerals", "sum", "isogram"}) {
		t.Fatalf("exercises = %v", s.ExerciseIDs)
	}
	if s.Composition[exercise.Easy] != 1 || s.Composition[exercise.Medium] != 1 || s.Composition[exercise.Hard] != 1 {
		t.Errorf("composition = %v", s.Composition)
	}
	if a := join(t, ts, s.ID, "Ana"); !slices.Equal(a.ExerciseIDs, s.ExerciseIDs) || a.Score.MaxPoints != 6 {
		t.Errorf("attempt = %+v", a)
	}

	var e errorBody
	for name, body := range map[string]string{
		"unknown exercise": `{"title":"X","exercise_ids":["sum","nope"],"duration_minutes":10}`,
		"duplicate":        `{"title":"X","exercise_ids":["sum","sum"],"duration_minutes":10}`,
		"no title":         `{"exercise_ids":["sum"],"duration_minutes":10}`,
	} {
		if code := adminCall(t, "POST", ts.URL+"/api/admin/sessions", testAdminPassword, body, &e); code != 400 || e.Error == "" {
			t.Errorf("%s: status %d, error %q", name, code, e.Error)
		}
	}
}

func TestCreateSessionValidation(t *testing.T) {
	ts := newTestServer(t)
	custom := createSession(t, ts, `{"title":"Só difíceis","composition":{"hard":2},"duration_minutes":30}`)
	if custom.Title != "Só difíceis" || len(custom.ExerciseIDs) != 2 {
		t.Fatalf("custom session = %+v", custom)
	}
	var e errorBody
	for name, tc := range map[string]struct {
		body   string
		status int
	}{
		"no duration":      {`{"preset":"easy"}`, 400},
		"too long":         {`{"preset":"easy","duration_minutes":1000}`, 400},
		"unknown preset":   {`{"preset":"nope","duration_minutes":10}`, 404},
		"too many":         {`{"title":"X","composition":{"hard":99},"duration_minutes":10}`, 400},
		"bad level":        {`{"title":"X","composition":{"expert":1},"duration_minutes":10}`, 400},
		"no title, custom": {`{"composition":{"easy":1},"duration_minutes":10}`, 400},
	} {
		if code := adminCall(t, "POST", ts.URL+"/api/admin/sessions", testAdminPassword, tc.body, &e); code != tc.status || e.Error == "" {
			t.Errorf("%s: status %d (want %d), error %q", name, code, tc.status, e.Error)
		}
	}
}

func TestPracticeSubmissionRecordsStudent(t *testing.T) {
	ts := newTestServer(t)
	body := submitBody(t, client.SubmitRequest{ExerciseID: "factorial", Code: "package solution\n", Student: "  Rui "})
	var r engine.Report
	if code := call(t, "POST", ts.URL+"/api/submissions", body, &r); code != 200 || r.Student != "Rui" {
		t.Fatalf("practice submit: status %d, student %q", code, r.Student)
	}
	var e errorBody
	body = submitBody(t, client.SubmitRequest{ExerciseID: "factorial", Student: strings.Repeat("a", 81)})
	if code := call(t, "POST", ts.URL+"/api/submissions", body, &e); code != 400 {
		t.Errorf("long name: status %d", code)
	}
	body = submitBody(t, client.SubmitRequest{ExerciseID: "factorial", Student: "a\u0000b"})
	if code := call(t, "POST", ts.URL+"/api/submissions", body, &e); code != 400 {
		t.Errorf("control character: status %d", code)
	}
}

// assertAdminOnly checks that each {method, path} refuses a missing or wrong
// password with 401.
func assertAdminOnly(t *testing.T, ts *httptest.Server, routes [][2]string) {
	t.Helper()
	var e errorBody
	for _, r := range routes {
		for _, password := range []string{"", "wrong"} {
			if code := adminCall(t, r[0], ts.URL+r[1], password, "{}", &e); code != 401 {
				t.Errorf("%s %s with password %q: status %d, want 401", r[0], r[1], password, code)
			}
		}
	}
}

func TestAdminRoutes(t *testing.T) {
	ts := newTestServer(t)
	s := createSession(t, ts, `{"preset":"easy","duration_minutes":10}`)
	a := join(t, ts, s.ID, "Ana")
	body := submitBody(t, client.SubmitRequest{ExerciseID: a.ExerciseIDs[0], Code: "package solution\n", AttemptID: a.ID})
	call(t, "POST", ts.URL+"/api/submissions", body, nil)

	assertAdminOnly(t, ts, [][2]string{
		{"GET", "/api/admin/sessions"}, {"POST", "/api/admin/sessions"}, {"POST", "/api/admin/sessions/" + s.ID + "/close"},
		{"GET", "/api/admin/submissions"}, {"GET", "/api/admin/attempts"},
	})

	var sessions []client.SessionView
	if code := adminCall(t, "GET", ts.URL+"/api/admin/sessions", testAdminPassword, "", &sessions); code != 200 || len(sessions) != 1 || sessions[0].Students != 1 {
		t.Fatalf("sessions: status %d, %+v", code, sessions)
	}
	var subs []client.SubmissionSummary
	if code := adminCall(t, "GET", ts.URL+"/api/admin/submissions", testAdminPassword, "", &subs); code != 200 || len(subs) != 1 {
		t.Fatalf("submissions: status %d, %+v", code, subs)
	}
	if got := subs[0]; got.Student != "Ana" || got.AttemptID != a.ID || got.Verdict != engine.VerdictRejected {
		t.Errorf("submission summary = %+v", got)
	}
	var attempts []client.AttemptView
	if code := adminCall(t, "GET", ts.URL+"/api/admin/attempts", testAdminPassword, "", &attempts); code != 200 || len(attempts) != 1 {
		t.Fatalf("attempts: status %d, %+v", code, attempts)
	}
	if got := attempts[0]; got.Student != "Ana" || got.Title != "Teste fácil" || got.Score.Exercises[0].Submissions != 1 {
		t.Errorf("attempt = %+v", got)
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
	s := createSession(t, ts, `{"preset":"hard","duration_minutes":10}`)
	sessions, err := c.Sessions(ctx)
	if err != nil || len(sessions) != 1 || sessions[0].ID != s.ID {
		t.Fatalf("Sessions: %+v, %v", sessions, err)
	}
	a, err := c.JoinSession(ctx, s.ID, "Ana")
	if err != nil || len(a.ExerciseIDs) != 4 {
		t.Fatalf("JoinSession: %+v, %v", a, err)
	}
	if _, err := c.JoinSession(ctx, s.ID, ""); !errors.Is(err, client.ErrInvalid) {
		t.Errorf("JoinSession without name error = %v, want ErrInvalid", err)
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

func TestAdminDisabledWithoutPassword(t *testing.T) {
	ts := newTestServerWithAdmin(t, "")
	var e errorBody
	if code := adminCall(t, "GET", ts.URL+"/api/admin/submissions", "", "", &e); code != 404 || !strings.Contains(e.Error, "GAVEL_ADMIN_PASSWORD") {
		t.Fatalf("status %d, error %q", code, e.Error)
	}
}
