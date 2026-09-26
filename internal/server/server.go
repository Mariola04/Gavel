// Package server exposes a Client over HTTP and serves the web interface.
package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gavel/internal/client"
	"gavel/internal/engine"
	"gavel/internal/exercise"
)

// maxBodyBytes caps the size of a submission request. It leaves room for
// JSON escaping of code up to engine.MaxCodeSize; larger code that still
// fits is rejected by the engine's limits stage.
const maxBodyBytes = 4 * engine.MaxCodeSize

// Config configures the HTTP handler.
type Config struct {
	Client client.Client
	// Admin serves the teacher's routes under /api/admin/.
	Admin client.Admin
	// AdminPassword protects /api/admin/. When empty, those routes are
	// disabled.
	AdminPassword string
	// Web holds the static files of the web interface.
	Web    fs.FS
	Logger *slog.Logger
}

// New returns the HTTP handler for the API and the web interface.
func New(cfg Config) http.Handler {
	h := &handler{client: cfg.Client, admin: cfg.Admin, logger: cfg.Logger}
	if cfg.AdminPassword != "" {
		sum := sha256.Sum256([]byte(cfg.AdminPassword))
		h.adminHash = sum[:]
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/exercises", h.listExercises)
	mux.HandleFunc("GET /api/exercises/{id}", h.getExercise)
	mux.HandleFunc("GET /api/exams", h.listExams)
	mux.HandleFunc("GET /api/sessions", h.listSessions)
	mux.HandleFunc("POST /api/sessions/{id}/join", h.joinSession)
	mux.HandleFunc("GET /api/attempts/{id}", h.getAttempt)
	mux.HandleFunc("POST /api/submissions", h.submit)
	mux.HandleFunc("GET /api/submissions/{id}", h.getReport)
	mux.HandleFunc("GET /api/admin/sessions", h.requireAdmin(h.adminSessions))
	mux.HandleFunc("POST /api/admin/sessions", h.requireAdmin(h.createSession))
	mux.HandleFunc("POST /api/admin/sessions/{id}/close", h.requireAdmin(h.closeSession))
	mux.HandleFunc("GET /api/admin/submissions", h.requireAdmin(h.adminSubmissions))
	mux.HandleFunc("GET /api/admin/attempts", h.requireAdmin(h.adminAttempts))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "rota não encontrada")
	})
	mux.Handle("/", http.FileServerFS(cfg.Web))
	return logRequests(cfg.Logger, cors(mux))
}

type handler struct {
	client client.Client
	admin  client.Admin
	// adminHash is the SHA-256 of the admin password, or nil when the
	// admin routes are disabled.
	adminHash []byte
	logger    *slog.Logger
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

func (h *handler) listSessions(w http.ResponseWriter, r *http.Request) {
	list, err := h.client.Sessions(r.Context())
	h.respond(w, http.StatusOK, list, err)
}

func (h *handler) joinSession(w http.ResponseWriter, r *http.Request) {
	var req client.JoinRequest
	if !decodeJSON(w, r, &req, false) {
		return
	}
	a, err := h.client.JoinSession(r.Context(), r.PathValue("id"), req.Student)
	h.respond(w, http.StatusOK, a, err)
}

func (h *handler) getAttempt(w http.ResponseWriter, r *http.Request) {
	a, err := h.client.Attempt(r.Context(), r.PathValue("id"))
	h.respond(w, http.StatusOK, a, err)
}

func (h *handler) submit(w http.ResponseWriter, r *http.Request) {
	var req client.SubmitRequest
	if !decodeJSON(w, r, &req, false) {
		return
	}
	report, err := h.client.Submit(r.Context(), req)
	h.respond(w, http.StatusOK, report, err)
}

func (h *handler) getReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.client.Report(r.Context(), r.PathValue("id"))
	h.respond(w, http.StatusOK, report, err)
}

func (h *handler) adminSessions(w http.ResponseWriter, r *http.Request) {
	list, err := h.admin.AllSessions(r.Context())
	h.respond(w, http.StatusOK, list, err)
}

func (h *handler) createSession(w http.ResponseWriter, r *http.Request) {
	var req client.CreateSessionRequest
	if !decodeJSON(w, r, &req, false) {
		return
	}
	s, err := h.admin.CreateSession(r.Context(), req)
	h.respond(w, http.StatusCreated, s, err)
}

func (h *handler) closeSession(w http.ResponseWriter, r *http.Request) {
	s, err := h.admin.CloseSession(r.Context(), r.PathValue("id"))
	h.respond(w, http.StatusOK, s, err)
}

func (h *handler) adminSubmissions(w http.ResponseWriter, r *http.Request) {
	list, err := h.admin.Submissions(r.Context())
	h.respond(w, http.StatusOK, list, err)
}

func (h *handler) adminAttempts(w http.ResponseWriter, r *http.Request) {
	list, err := h.admin.Attempts(r.Context())
	h.respond(w, http.StatusOK, list, err)
}

// requireAdmin only lets through requests with the header
// "Authorization: Bearer <admin password>".
func (h *handler) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.adminHash == nil || h.admin == nil {
			writeError(w, http.StatusNotFound, "área de docente desativada: defina GAVEL_ADMIN_PASSWORD ao arrancar o servidor")
			return
		}
		password, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		// Comparing fixed-size hashes in constant time does not leak the
		// password length or content through timing.
		sum := sha256.Sum256([]byte(password))
		if !ok || subtle.ConstantTimeCompare(sum[:], h.adminHash) != 1 {
			writeError(w, http.StatusUnauthorized, "palavra-passe de docente inválida")
			return
		}
		next(w, r)
	}
}

// decodeJSON reads a size-limited JSON body into v and writes the error
// response when it fails. With optional set, an empty body is accepted.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any, optional bool) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	var tooLarge *http.MaxBytesError
	switch {
	case err == nil, optional && errors.Is(err, io.EOF):
		return true
	case errors.As(err, &tooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "pedido demasiado grande")
	default:
		writeError(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
	}
	return false
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
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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
