package stubs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/problem"
)

// ExtractorStub is a fake synchronous pdf-extractor for the benchmark /extract
// endpoint. It returns a fixed Markdown content and page count.
type ExtractorStub struct {
	*httptest.Server

	Content   string
	PageCount int
	// Delay, when > 0, is slept before responding (to simulate timeouts).
	Delay time.Duration
	// FailStatus, when != 0, makes every request respond with that status.
	FailStatus int
}

// NewExtractorStub starts an extractor stub on a random port. Callers must
// Close it when done.
func NewExtractorStub() *ExtractorStub {
	s := &ExtractorStub{
		Content:   "# Título\n\ntexto extraído",
		PageCount: 3,
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

func (s *ExtractorStub) serve(w http.ResponseWriter, r *http.Request) {
	if s.Delay > 0 {
		time.Sleep(s.Delay)
	}
	if s.FailStatus != 0 {
		problem.WriteProblem(w, s.FailStatus, "downstream failure", "forced failure", r.URL.Path)
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != dto.PathExtractorExtract {
		problem.WriteProblem(w, http.StatusNotFound, "Not found", "", r.URL.Path)
		return
	}

	var req dto.ExtractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problem.WriteProblem(w, http.StatusBadRequest, "Invalid request", err.Error(), r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, dto.ExtractResponse{Content: s.Content, PageCount: s.PageCount})
}
