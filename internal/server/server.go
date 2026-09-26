// Package server exposes a Client over HTTP and serves the web interface.
package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"gavel/internal/client"
	"gavel/internal/engine"
	"gavel/internal/exercise"
)

// maxBodyBytes caps the size of a submission request. It leaves room for
// JSON escaping of code up to engine.MaxCodeSize; larger code that still
// fits is rejected by the engine's limits stage.
const maxBodyBytes = 4 * engine.MaxCodeSize

// New returns the HTTP handler for the API and the static files in web.
func New(c client.Client, web fs.FS, logger *slog.Logger) http.Handler {
	h := &handler{client: c, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/exercises", h.listExercises)
	mux.HandleFunc("GET /api/exercises/{id}", h.getExercise)
	mux.HandleFunc("GET /api/exams", h.listExams)
	mux.HandleFunc("POST /api/exams/{id}/attempts", h.startAttempt)
	mux.HandleFunc("GET /api/attempts/{id}", h.getAttempt)
	mux.HandleFunc("POST /api/submissions", h.submit)
	mux.HandleFunc("GET /api/submissions/{id}", h.getReport)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "rota não encontrada")
	})
	mux.Handle("/", http.FileServerFS(web))
	return logRequests(logger, cors(mux))
}

type handler struct {
	client client.Client
	logger *slog.Logger
}

func (h *handler) listExercises(w http.ResponseWriter, r *http.Request) {
	d, err := exercise.ParseDifficulty(r.URL.Query().Get("difficulty"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	list, err := h.client.Exercises(r.Context(), d)
	h.respond(w, http.StatusOK, list, err)
}

func (h *handler) getExercise(w http.ResponseWriter, r *http.Request) {
	ex, err := h.client.Exercise(r.Context(), r.PathValue("id"))
	h.respond(w, http.StatusOK, ex, err)
}

func (h *handler) listExams(w http.ResponseWriter, r *http.Request) {
	exams, err := h.client.Exams(r.Context())
	h.respond(w, http.StatusOK, exams, err)
}

func (h *handler) startAttempt(w http.ResponseWriter, r *http.Request) {
	a, err := h.client.StartAttempt(r.Context(), r.PathValue("id"))
	h.respond(w, http.StatusCreated, a, err)
}

func (h *handler) getAttempt(w http.ResponseWriter, r *http.Request) {
	a, err := h.client.Attempt(r.Context(), r.PathValue("id"))
	h.respond(w, http.StatusOK, a, err)
}

func (h *handler) submit(w http.ResponseWriter, r *http.Request) {
	var req client.SubmitRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "pedido demasiado grande")
			return
		}
		writeError(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	report, err := h.client.Submit(r.Context(), req)
	h.respond(w, http.StatusOK, report, err)
}

func (h *handler) getReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.client.Report(r.Context(), r.PathValue("id"))
	h.respond(w, http.StatusOK, report, err)
}

// respond writes v with the given status, or the error with the status
// that matches its kind. Unexpected errors are logged and hidden.
func (h *handler) respond(w http.ResponseWriter, status int, v any, err error) {
	switch {
	case err == nil:
		writeJSON(w, status, v)
	case errors.Is(err, client.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, client.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		h.logger.Error("erro interno", "err", err)
		writeError(w, http.StatusInternalServerError, "erro interno do servidor")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // the client may have gone away; nothing to do
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// cors allows the API to be used from any origin and answers preflight
// requests.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// statusRecorder captures the response status for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.Info("pedido",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start),
		)
	})
}
