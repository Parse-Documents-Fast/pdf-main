package problem

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteProblem(t *testing.T) {
	w := httptest.NewRecorder()

	WriteProblem(w, 404, "PDF no encontrado", "No existe un PDF con ID 'abc123'", "/api/pdfs/abc123")

	if got := w.Code; got != 404 {
		t.Fatalf("status = %d, want 404", got)
	}
	if got := w.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}

	var got ProblemDetails
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	want := ProblemDetails{
		Type:     TypeAboutBlank,
		Title:    "PDF no encontrado",
		Status:   404,
		Detail:   "No existe un PDF con ID 'abc123'",
		Instance: "/api/pdfs/abc123",
	}
	if got != want {
		t.Errorf("body = %+v, want %+v", got, want)
	}
}

func TestWriteProblemWithExistingID(t *testing.T) {
	w := httptest.NewRecorder()

	WriteProblemWithExistingID(w, 409, "Documento duplicado", "Este documento ya fue subido anteriormente", "/api/pdfs", "665f1a2b3c4d5e6f7a8b9c0d")

	if got := w.Code; got != 409 {
		t.Fatalf("status = %d, want 409", got)
	}

	var got ProblemDetails
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if got.ExistingID != "665f1a2b3c4d5e6f7a8b9c0d" {
		t.Errorf("existing_id = %q, want %q", got.ExistingID, "665f1a2b3c4d5e6f7a8b9c0d")
	}
	if got.Title != "Documento duplicado" || got.Status != 409 {
		t.Errorf("body = %+v", got)
	}
}
