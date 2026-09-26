// Package httpapi contains the HTTP adapters (handlers, router and
// middlewares) that translate requests into orchestrator calls and map domain
// errors to RFC 9457 responses (ADR-0001).
package httpapi

import (
	"net/http"

	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/go-chi/chi/v5"
)

// Router builds the chi router exposing the public API with its middlewares.
func Router(p orchestrator.Ports) http.Handler {
	a := &api{ports: p}

	r := chi.NewRouter()
	r.Use(allowCORS)
	r.Post("/api/pdfs", a.handleUpload)
	return r
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
