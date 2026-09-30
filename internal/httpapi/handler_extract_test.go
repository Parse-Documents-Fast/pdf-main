package httpapi_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
)

func extractMultipart(t *testing.T, filename string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
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
	req := httptest.NewRequest(http.MethodPost, "/extract", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func decodeExtract(t *testing.T, rr *httptest.ResponseRecorder) dto.ExtractResponse {
	t.Helper()
	var r dto.ExtractResponse
	if err := json.NewDecoder(rr.Body).Decode(&r); err != nil {
		t.Fatalf("decode extract: %v", err)
	}
	return r
}

func TestExtractMultipart(t *testing.T) {
	e := newEnv(t)

	rr := do(t, e.handler, extractMultipart(t, "doc.pdf", []byte("%PDF-1.4\nfake")))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	res := decodeExtract(t, rr)
	if res.Content != e.extractor.Content {
		t.Errorf("Content = %q, want %q", res.Content, e.extractor.Content)
	}
	if res.PageCount != e.extractor.PageCount {
		t.Errorf("PageCount = %d, want %d", res.PageCount, e.extractor.PageCount)
	}
}

func TestExtractRawBody(t *testing.T) {
	e := newEnv(t)

	req := httptest.NewRequest(http.MethodPost, "/extract", bytes.NewReader([]byte("%PDF-1.4\nfake")))
	req.Header.Set("Content-Type", "application/pdf")

	rr := do(t, e.handler, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	res := decodeExtract(t, rr)
	if res.PageCount != e.extractor.PageCount {
		t.Errorf("PageCount = %d, want %d", res.PageCount, e.extractor.PageCount)
	}
}

func TestExtractNotPDF(t *testing.T) {
	e := newEnv(t)

	rr := do(t, e.handler, extractMultipart(t, "nota.md", []byte("# título")))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestExtractInvalid(t *testing.T) {
	e := newEnv(t)
	e.validator.FailStatus = http.StatusBadRequest

	rr := do(t, e.handler, extractMultipart(t, "doc.pdf", []byte("%PDF-1.4")))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestExtractDownstream(t *testing.T) {
	e := newEnv(t)
	e.extractor.FailStatus = http.StatusInternalServerError

	rr := do(t, e.handler, extractMultipart(t, "doc.pdf", []byte("%PDF-1.4")))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}
