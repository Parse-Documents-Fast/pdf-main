package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/go-chi/chi/v5"
)

// Router builds the chi router exposing the public API with its middlewares.
//
// The public contract (methods, paths and status codes) is defined in
// docs/spec.md §Endpoints. These routes are the coordination point with the
// CLI/web client and with pdf-infra (Traefik routes by Host + prefix).
func Router(logger *slog.Logger, p orchestrator.Ports, maxFileSizeMB int) http.Handler {
	a := &api{ports: p, logger: logger, maxFileSizeMB: maxFileSizeMB}

	r := chi.NewRouter()
	r.Use(logRequests(logger))
	r.Use(allowCORS)

	r.Get("/health", a.handleHealth)

	r.Post("/api/pdfs", a.handleUpload)
	r.Get("/api/pdfs", a.handleList)
	r.Get("/api/pdfs/{id}", a.handleGet)
	r.Get("/api/pdfs/{id}/download", a.handleDownload)
	r.Delete("/api/pdfs/{id}", a.handleDelete)

	// Benchmark-only synchronous extraction (ADR-0004 deviation).
	r.Post("/extract", a.handleExtract)

	return r
}

// logRequests logs one "http request completed" event per request (ADR-0007):
// INFO for 2xx/3xx, WARN for 4xx, ERROR for 5xx.
func logRequests(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			level := slog.LevelInfo
			switch {
			case rec.status >= 500:
				level = slog.LevelError
			case rec.status >= 400:
				level = slog.LevelWarn
			}

			logger.Log(r.Context(), level, "http request completed",
				"http_method", r.Method,
				"http_path", r.URL.Path,
				"http_status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

// statusRecorder captures the response status code written by the handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// allowCORS is a permissive CORS middleware: the future web client is served
// from a different origin (docs/spec.md §Endpoints).
func allowCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeJSON writes v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
