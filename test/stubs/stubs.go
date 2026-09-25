// Package stubs provides fake, in-process implementations of the downstream
// services (pdf-validator, pdf-persistence, pdf-converter) and of the Redis
// Streams queue, so the business logic of pdf-main can be built and tested
// before the real services exist.
//
// Each stub is an httptest server (or an in-memory queue) that speaks the same
// JSON wire contract as the real service (docs/spec.md §DTOs). Stubs allow
// tests to inject failures and delays to exercise the resilience paths of
// pdf-main. This package is test-only: cmd/pdf-main never imports it, so it
// never reaches the binary.
package stubs

import (
	"encoding/json"
	"net/http"
)

// writeJSON writes v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
