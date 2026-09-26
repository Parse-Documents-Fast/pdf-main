package httpapi_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Parse-Documents-Fast/pdf-main/internal/clients"
	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/httpapi"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/Parse-Documents-Fast/pdf-main/internal/problem"
	"github.com/Parse-Documents-Fast/pdf-main/test/stubs"
)

type env struct {
	handler     http.Handler
	validator   *stubs.ValidatorStub
	persistence *stubs.PersistenceStub
	queue       *stubs.MemoryQueue
}

func newEnv(t *testing.T) *env {
	t.Helper()

	vs := stubs.NewValidatorStub()
	ps := stubs.NewPersistenceStub()
	q := stubs.NewMemoryQueue()
	t.Cleanup(func() {
		vs.Close()
		ps.Close()
	})

	ports := orchestrator.Ports{
		Validator:   clients.NewValidator(vs.URL),
		Persistence: clients.NewPersistence(ps.URL),
		Queue:       q,
	}

	return &env{
		handler:     httpapi.Router(ports),
		validator:   vs,
		persistence: ps,
		queue:       q,
	}
}

// multipartRequest builds a multipart/form-data upload request. A non-empty
// title adds the title field.
func multipartRequest(t *testing.T, filename, title string, content []byte) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if title != "" {
		if err := mw.WriteField("title", title); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	}
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/pdfs", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func do(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func decodeSummary(t *testing.T, rr *httptest.ResponseRecorder) dto.PdfSummary {
	t.Helper()
	var s dto.PdfSummary
	if err := json.NewDecoder(rr.Body).Decode(&s); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	return s
}

func decodeProblem(t *testing.T, rr *httptest.ResponseRecorder) problem.ProblemDetails {
	t.Helper()
	var p problem.ProblemDetails
	if err := json.NewDecoder(rr.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	return p
}

func TestUploadPDF(t *testing.T) {
	e := newEnv(t)

	rr := do(t, e.handler, multipartRequest(t, "informe.pdf", "informe", []byte("%PDF-1.4\nfake")))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	s := decodeSummary(t, rr)
	if s.Status != dto.StatusPending {
		t.Errorf("Status = %q, want pending", s.Status)
	}
	if s.OriginalFormat != dto.FormatPDF {
		t.Errorf("OriginalFormat = %q, want pdf", s.OriginalFormat)
	}
	if s.Title != "informe" {
		t.Errorf("Title = %q, want informe", s.Title)
	}
	if jobs := e.queue.PublishedExtractionJobs(); len(jobs) != 1 {
		t.Errorf("published jobs = %d, want 1", len(jobs))
	}
}

func TestUploadMarkdown(t *testing.T) {
	e := newEnv(t)

	rr := do(t, e.handler, multipartRequest(t, "nota.md", "nota", []byte("# título\n\ntexto")))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	s := decodeSummary(t, rr)
	if s.Status != dto.StatusDone {
		t.Errorf("Status = %q, want done", s.Status)
	}
	if s.OriginalFormat != dto.FormatMarkdown {
		t.Errorf("OriginalFormat = %q, want markdown", s.OriginalFormat)
	}
	if jobs := e.queue.PublishedExtractionJobs(); len(jobs) != 0 {
		t.Errorf("published jobs = %d, want 0", len(jobs))
	}
}

func TestUploadTitleDefaultsToFilename(t *testing.T) {
	e := newEnv(t)

	rr := do(t, e.handler, multipartRequest(t, "informe.pdf", "", []byte("%PDF-1.4\nfake")))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	s := decodeSummary(t, rr)
	if s.Title != "informe" {
		t.Errorf("Title = %q, want informe (default from filename)", s.Title)
	}
}

func TestUploadMissingFile(t *testing.T) {
	e := newEnv(t)

	req := httptest.NewRequest(http.MethodPost, "/api/pdfs", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")

	rr := do(t, e.handler, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
}

func TestUploadInvalid(t *testing.T) {
	e := newEnv(t)
	e.validator.FailStatus = http.StatusBadRequest

	rr := do(t, e.handler, multipartRequest(t, "foto.jpg", "", []byte("no es pdf")))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	p := decodeProblem(t, rr)
	if p.Title != "Invalid file" {
		t.Errorf("Title = %q, want Invalid file", p.Title)
	}
}

func TestUploadDuplicate(t *testing.T) {
	e := newEnv(t)
	content := []byte("%PDF-1.4\nsame")

	if rr := do(t, e.handler, multipartRequest(t, "a.pdf", "a", content)); rr.Code != http.StatusOK {
		t.Fatalf("first upload status = %d, want 200", rr.Code)
	}

	rr := do(t, e.handler, multipartRequest(t, "b.pdf", "b", content))
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
	p := decodeProblem(t, rr)
	if p.Title != "Duplicate document" {
		t.Errorf("Title = %q, want Duplicate document", p.Title)
	}
	if p.ExistingID == "" {
		t.Error("existing_id is empty")
	}
}

func TestUploadDownstream(t *testing.T) {
	e := newEnv(t)
	e.validator.FailStatus = http.StatusInternalServerError

	rr := do(t, e.handler, multipartRequest(t, "x.pdf", "", []byte("%PDF-1.4")))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
	p := decodeProblem(t, rr)
	if p.Title != "Service unavailable" {
		t.Errorf("Title = %q, want Service unavailable", p.Title)
	}
}
