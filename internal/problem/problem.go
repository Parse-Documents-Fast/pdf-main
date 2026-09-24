// Package problem provides the RFC 9457 (Problem Details) type and HTTP
// helpers used to serialize errors to clients (ADR-0001).
package problem

import (
	"encoding/json"
	"net/http"
)

// TypeAboutBlank is the default "type" used when there is no real
// documentation URI for an error (ADR-0001).
const TypeAboutBlank = "about:blank"

// ProblemDetails is the RFC 9457 representation of an HTTP error.
//
// ExistingID is a domain extension (ADR-0001) populated only for duplicate
// uploads; it is omitted from the wire otherwise.
type ProblemDetails struct {
	Type       string `json:"type"`
	Title      string `json:"title"`
	Status     int    `json:"status"`
	Detail     string `json:"detail"`
	Instance   string `json:"instance"`
	ExistingID string `json:"existing_id,omitempty"`
}

// New builds a ProblemDetails from the five standard fields, defaulting Type
// to about:blank.
func New(status int, title, detail, instance string) ProblemDetails {
	return ProblemDetails{
		Type:     TypeAboutBlank,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
	}
}

// WriteProblem writes a problem response with Content-Type
// application/problem+json and the given status code.
func WriteProblem(w http.ResponseWriter, status int, title, detail, instance string) {
	write(w, New(status, title, detail, instance))
}

// WriteProblemWithExistingID writes a problem response including the
// existing_id extension, used for duplicate uploads (ADR-0001).
func WriteProblemWithExistingID(w http.ResponseWriter, status int, title, detail, instance, existingID string) {
	p := New(status, title, detail, instance)
	p.ExistingID = existingID
	write(w, p)
}

func write(w http.ResponseWriter, p ProblemDetails) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
