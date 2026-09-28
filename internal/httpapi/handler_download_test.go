package httpapi_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
)

func TestDownloadMarkdown(t *testing.T) {
	e := newEnv(t)
	content := []byte("# título\n\ntexto")

	s := upload(t, e, "nota.md", "nota", content)

	rr := do(t, e.handler, httptest.NewRequest(http.MethodGet, "/api/pdfs/"+s.ID+"/download", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/markdown" {
		t.Errorf("Content-Type = %q, want text/markdown", ct)
	}
	if cd := rr.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="nota.md"`) {
		t.Errorf("Content-Disposition = %q, want filename=nota.md", cd)
	}
	if !bytes.Equal(rr.Body.Bytes(), content) {
		t.Errorf("body = %q, want %q", rr.Body.String(), content)
	}
}

func TestDownloadPDF(t *testing.T) {
	e := newEnv(t)

	s := upload(t, e, "nota.md", "nota", []byte("# título"))

	rr := do(t, e.handler, httptest.NewRequest(http.MethodGet, "/api/pdfs/"+s.ID+"/download?format=pdf", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q, want application/pdf", ct)
	}
	if cd := rr.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="nota.pdf"`) {
		t.Errorf("Content-Disposition = %q, want filename=nota.pdf", cd)
	}
	if !bytes.Equal(rr.Body.Bytes(), e.converter.PDF) {
		t.Errorf("body = %q, want the converter output", rr.Body.String())
	}
}

func TestDownloadInvalidFormat(t *testing.T) {
	e := newEnv(t)

	s := upload(t, e, "nota.md", "nota", []byte("# título"))

	rr := do(t, e.handler, httptest.NewRequest(http.MethodGet, "/api/pdfs/"+s.ID+"/download?format=exe", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestDownloadNotFound(t *testing.T) {
	e := newEnv(t)

	rr := do(t, e.handler, httptest.NewRequest(http.MethodGet, "/api/pdfs/no-existe/download", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestDownloadPending(t *testing.T) {
	e := newEnv(t)

	s := upload(t, e, "x.pdf", "x", []byte("%PDF-1.4\npending"))

	rr := do(t, e.handler, httptest.NewRequest(http.MethodGet, "/api/pdfs/"+s.ID+"/download", nil))
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
}

func TestDownloadFailed(t *testing.T) {
	e := newEnv(t)

	rec, err := e.persistCache.Create(context.Background(), dto.PersistCreateRequest{
		Title:          "fail",
		OriginalFormat: dto.FormatPDF,
		Checksum:       "c-fail",
		Status:         dto.StatusFailed,
	})
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	rr := do(t, e.handler, httptest.NewRequest(http.MethodGet, "/api/pdfs/"+rec.ID+"/download", nil))
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
}
