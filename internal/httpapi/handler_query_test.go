package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
)

// upload posts a file through the handler and returns the created summary.
func upload(t *testing.T, e *env, filename, title string, content []byte) dto.PdfSummary {
	t.Helper()
	rr := do(t, e.handler, multipartRequest(t, filename, title, content))
	if rr.Code != http.StatusOK {
		t.Fatalf("upload status = %d (body: %s)", rr.Code, rr.Body.String())
	}
	return decodeSummary(t, rr)
}

func decodeList(t *testing.T, rr *httptest.ResponseRecorder) []dto.PdfSummary {
	t.Helper()
	var list []dto.PdfSummary
	if err := json.NewDecoder(rr.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	return list
}

func decodeDocument(t *testing.T, rr *httptest.ResponseRecorder) dto.PdfDocument {
	t.Helper()
	var doc dto.PdfDocument
	if err := json.NewDecoder(rr.Body).Decode(&doc); err != nil {
		t.Fatalf("decode document: %v", err)
	}
	return doc
}

func TestList(t *testing.T) {
	e := newEnv(t)

	upload(t, e, "a.pdf", "a", []byte("%PDF-1.4\nfirst"))
	upload(t, e, "b.md", "b", []byte("# b"))

	rr := do(t, e.handler, httptest.NewRequest(http.MethodGet, "/api/pdfs", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	list := decodeList(t, rr)
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].Title != "a" || list[1].Title != "b" {
		t.Errorf("list = %+v, want ordered by creation", list)
	}
}

func TestGet(t *testing.T) {
	e := newEnv(t)
	md := "# título\n\ntexto"

	s := upload(t, e, "nota.md", "nota", []byte(md))

	rr := do(t, e.handler, httptest.NewRequest(http.MethodGet, "/api/pdfs/"+s.ID, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	doc := decodeDocument(t, rr)
	if doc.ID != s.ID {
		t.Errorf("ID = %q, want %q", doc.ID, s.ID)
	}
	if doc.Content == nil || *doc.Content != md {
		t.Errorf("Content = %v, want %q", doc.Content, md)
	}
}

func TestGetNotFound(t *testing.T) {
	e := newEnv(t)

	rr := do(t, e.handler, httptest.NewRequest(http.MethodGet, "/api/pdfs/no-existe", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	p := decodeProblem(t, rr)
	if p.Title != "Document not found" {
		t.Errorf("Title = %q, want Document not found", p.Title)
	}
}

func TestDelete(t *testing.T) {
	e := newEnv(t)

	s := upload(t, e, "x.pdf", "x", []byte("%PDF-1.4\ndelete me"))

	rr := do(t, e.handler, httptest.NewRequest(http.MethodDelete, "/api/pdfs/"+s.ID, nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}

	after := do(t, e.handler, httptest.NewRequest(http.MethodGet, "/api/pdfs/"+s.ID, nil))
	if after.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", after.Code)
	}
}

func TestDeleteNotFound(t *testing.T) {
	e := newEnv(t)

	rr := do(t, e.handler, httptest.NewRequest(http.MethodDelete, "/api/pdfs/no-existe", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}
