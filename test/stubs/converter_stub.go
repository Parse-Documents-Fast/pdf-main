package stubs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/problem"
)

// ConverterStub is a fake pdf-converter HTTP server. It is download-only
// (ADR-0005): it accepts Markdown and returns a fixed PDF.
type ConverterStub struct {
	*httptest.Server

	// PDF is the raw PDF bytes returned as content_base64.
	PDF []byte
	// MimeType is the Content-Type returned (default application/pdf).
	MimeType string
	// Delay, when > 0, is slept before responding (to simulate timeouts).
	Delay time.Duration
	// FailStatus, when != 0, makes every request respond with that status.
	FailStatus int
}

// NewConverterStub starts a converter stub on a random port. Callers must
// Close it when done.
func NewConverterStub() *ConverterStub {
	s := &ConverterStub{
		PDF:      []byte("%PDF-1.4\n% fake pdf bytes produced by the stub"),
		MimeType: "application/pdf",
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

func (s *ConverterStub) serve(w http.ResponseWriter, r *http.Request) {
	if s.Delay > 0 {
		time.Sleep(s.Delay)
	}
	if s.FailStatus != 0 {
		problem.WriteProblem(w, s.FailStatus, "downstream failure", "forced failure", r.URL.Path)
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != dto.PathConverterConvert {
		problem.WriteProblem(w, http.StatusNotFound, "Not found", "", r.URL.Path)
		return
	}

	var req dto.ConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problem.WriteProblem(w, http.StatusBadRequest, "Invalid request", err.Error(), r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, dto.ConvertResponse{
		ContentBase64: s.PDF,
		MimeType:      s.MimeType,
	})
}
