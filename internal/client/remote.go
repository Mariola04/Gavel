package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gavel/internal/engine"
	"gavel/internal/exam"
	"gavel/internal/exercise"
)

// requestTimeout bounds a whole request, including a synchronous
// evaluation on the server.
const requestTimeout = 2 * time.Minute

// RemoteClient implements Client over the Gavel HTTP API.
type RemoteClient struct {
	baseURL string
	http    *http.Client
}

var _ Client = (*RemoteClient)(nil)

// NewRemote returns a client for the server at baseURL, such as
// "http://localhost:8080".
func NewRemote(baseURL string) *RemoteClient {
	return &RemoteClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http:    &http.Client{Timeout: requestTimeout},
	}
}

// Exercises lists the exercises, optionally filtered by level.
func (c *RemoteClient) Exercises(ctx context.Context, d exercise.Difficulty) ([]ExerciseSummary, error) {
	path := "/api/exercises"
	if d != "" {
		path += "?" + url.Values{"difficulty": {string(d)}}.Encode()
	}
	return call[[]ExerciseSummary](ctx, c, http.MethodGet, path, nil)
}

// Exercise returns an exercise without the expected outputs.
func (c *RemoteClient) Exercise(ctx context.Context, id string) (*ExerciseDetail, error) {
	return call[*ExerciseDetail](ctx, c, http.MethodGet, "/api/exercises/"+url.PathEscape(id), nil)
}

// Exams lists the exam templates.
func (c *RemoteClient) Exams(ctx context.Context) ([]*exam.Exam, error) {
	return call[[]*exam.Exam](ctx, c, http.MethodGet, "/api/exams", nil)
}

// StartAttempt starts a new attempt of an exam.
func (c *RemoteClient) StartAttempt(ctx context.Context, examID string) (*AttemptView, error) {
	return call[*AttemptView](ctx, c, http.MethodPost, "/api/exams/"+url.PathEscape(examID)+"/attempts", nil)
}

// Attempt returns an attempt with its current score.
func (c *RemoteClient) Attempt(ctx context.Context, id string) (*AttemptView, error) {
	return call[*AttemptView](ctx, c, http.MethodGet, "/api/attempts/"+url.PathEscape(id), nil)
}

// Submit sends a submission and waits for its report.
func (c *RemoteClient) Submit(ctx context.Context, req SubmitRequest) (*engine.Report, error) {
	return call[*engine.Report](ctx, c, http.MethodPost, "/api/submissions", req)
}

// Report returns a saved report.
func (c *RemoteClient) Report(ctx context.Context, id string) (*engine.Report, error) {
	return call[*engine.Report](ctx, c, http.MethodGet, "/api/submissions/"+url.PathEscape(id), nil)
}

// call sends a request and decodes the response into a new T.
func call[T any](ctx context.Context, c *RemoteClient, method, path string, body any) (T, error) {
	var out T
	if err := c.do(ctx, method, path, body, &out); err != nil {
		var zero T
		return zero, err
	}
	return out, nil
}

// do sends a JSON request and decodes a JSON response into out. Error
// responses are turned into *Error values with the matching kind.
func (c *RemoteClient) do(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("serializar pedido: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("criar pedido: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("contactar o servidor: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode >= 400 {
		return responseError(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("ler resposta do servidor: %w", err)
	}
	return nil
}

func responseError(resp *http.Response) error {
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Error == "" {
		body.Error = resp.Status
	}
	switch resp.StatusCode {
	case http.StatusNotFound:
		return &Error{Kind: ErrNotFound, Message: body.Error}
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge:
		return &Error{Kind: ErrInvalid, Message: body.Error}
	default:
		return errors.New("erro do servidor: " + body.Error)
	}
}
