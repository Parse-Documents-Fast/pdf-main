package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/go-chi/chi/v5"
)

// Router builds the chi router exposing the public API with its middlewares.
//
// The public contract (methods, paths and status codes) is defined in
// docs/spec.md §Endpoints. These routes are the coordination point with the
// CLI/web client and with pdf-infra (Traefik routes by Host + prefix).
func Router(p orchestrator.Ports) http.Handler {
	a := &api{ports: p}

	r := chi.NewRouter()
	r.Use(allowCORS)

	r.Post("/api/pdfs", a.handleUpload)
	r.Get("/api/pdfs", a.handleList)
	r.Get("/api/pdfs/{id}", a.handleGet)
	r.Delete("/api/pdfs/{id}", a.handleDelete)

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

// writeJSON writes v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
