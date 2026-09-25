package stubs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/problem"
)

// ValidatorStub is a fake pdf-validator HTTP server. By default it classifies
// content starting with "%PDF-" as PDF and anything else as Markdown, and
// returns the real SHA-256 checksum of the content (so duplicate detection can
// be exercised end-to-end).
type ValidatorStub struct {
	*httptest.Server

	// ForceFormat, when set, overrides the auto-classification by content.
	ForceFormat dto.Format
	// Delay, when > 0, is slept before responding (to simulate timeouts).
	Delay time.Duration
	// FailStatus, when != 0, makes every request respond with that status.
	FailStatus int
}

// NewValidatorStub starts a validator stub on a random port. Callers must
// Close it when done.
func NewValidatorStub() *ValidatorStub {
	s := &ValidatorStub{}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

func (s *ValidatorStub) serve(w http.ResponseWriter, r *http.Request) {
	if s.Delay > 0 {
		time.Sleep(s.Delay)
	}
	if s.FailStatus != 0 {
		problem.WriteProblem(w, s.FailStatus, "downstream failure", "forced failure", r.URL.Path)
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != dto.PathValidatorValidate {
		problem.WriteProblem(w, http.StatusNotFound, "Not found", "", r.URL.Path)
		return
	}

	var req dto.ValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problem.WriteProblem(w, http.StatusBadRequest, "Invalid request", err.Error(), r.URL.Path)
		return
	}

	sum := sha256.Sum256(req.ContentBase64)
	format := s.ForceFormat
	if format == "" {
		if len(req.ContentBase64) >= 5 && string(req.ContentBase64[:5]) == "%PDF-" {
			format = dto.FormatPDF
		} else {
			format = dto.FormatMarkdown
		}
	}

	writeJSON(w, http.StatusOK, dto.ValidateResponse{
		OriginalFormat: format,
		Checksum:       hex.EncodeToString(sum[:]),
	})
}
