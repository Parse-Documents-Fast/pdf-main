package stubs_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/test/stubs"
)

// doJSON performs an HTTP request with a JSON body (if body != nil) and
// returns the response without closing it.
func doJSON(t *testing.T, method, url string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

func TestValidatorStubClassifiesPDF(t *testing.T) {
	s := stubs.NewValidatorStub()
	defer s.Close()

	resp := doJSON(t, http.MethodPost, s.URL+dto.PathValidatorValidate, dto.ValidateRequest{
		Filename:      "informe.pdf",
		ContentBase64: []byte("%PDF-1.4\nfake"),
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	got := decode[dto.ValidateResponse](t, resp)
	if got.OriginalFormat != dto.FormatPDF {
		t.Errorf("original_format = %q, want pdf", got.OriginalFormat)
	}
	if got.Checksum == "" {
		t.Error("checksum is empty")
	}
}

func TestValidatorStubClassifiesMarkdown(t *testing.T) {
	s := stubs.NewValidatorStub()
	defer s.Close()

	resp := doJSON(t, http.MethodPost, s.URL+dto.PathValidatorValidate, dto.ValidateRequest{
		Filename:      "nota.md",
		ContentBase64: []byte("# título\n\ntexto"),
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	got := decode[dto.ValidateResponse](t, resp)
	if got.OriginalFormat != dto.FormatMarkdown {
		t.Errorf("original_format = %q, want markdown", got.OriginalFormat)
	}
}

func TestValidatorStubChecksumIsStable(t *testing.T) {
	s := stubs.NewValidatorStub()
	defer s.Close()

	content := []byte("%PDF-1.4\nsame content")
	a := decode[dto.ValidateResponse](t, doJSON(t, http.MethodPost, s.URL+dto.PathValidatorValidate, dto.ValidateRequest{ContentBase64: content}))
	b := decode[dto.ValidateResponse](t, doJSON(t, http.MethodPost, s.URL+dto.PathValidatorValidate, dto.ValidateRequest{ContentBase64: content}))
	c := decode[dto.ValidateResponse](t, doJSON(t, http.MethodPost, s.URL+dto.PathValidatorValidate, dto.ValidateRequest{ContentBase64: []byte("%PDF-1.4\ndifferent")}))

	if a.Checksum != b.Checksum {
		t.Errorf("same content produced different checksums: %q vs %q", a.Checksum, b.Checksum)
	}
	if a.Checksum == c.Checksum {
		t.Errorf("different content produced the same checksum: %q", a.Checksum)
	}
}

func TestValidatorStubForcedFailure(t *testing.T) {
	s := stubs.NewValidatorStub()
	s.FailStatus = http.StatusInternalServerError
	defer s.Close()

	resp := doJSON(t, http.MethodPost, s.URL+dto.PathValidatorValidate, dto.ValidateRequest{ContentBase64: []byte("%PDF-1.4")})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

func TestConverterStub(t *testing.T) {
	s := stubs.NewConverterStub()
	defer s.Close()

	resp := doJSON(t, http.MethodPost, s.URL+dto.PathConverterConvert, dto.ConvertRequest{Content: "# título\n\ntexto"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	got := decode[dto.ConvertResponse](t, resp)
	if got.MimeType != "application/pdf" {
		t.Errorf("mime_type = %q, want application/pdf", got.MimeType)
	}
	if !bytes.Equal(got.ContentBase64, s.PDF) {
		t.Errorf("content_base64 = %q, want %q", got.ContentBase64, s.PDF)
	}
}

func TestPersistanceStubCRUD(t *testing.T) {
	s := stubs.NewPersistanceStub()
	defer s.Close()

	// Create (PDF, pending).
	createResp := doJSON(t, http.MethodPost, s.URL+dto.PathPersistDocuments, dto.PersistCreateRequest{
		Title:          "informe",
		OriginalFormat: dto.FormatPDF,
		Checksum:       "abc123",
		Status:         dto.StatusPending,
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", createResp.StatusCode)
	}
	created := decode[dto.PersistRecord](t, createResp)
	if created.ID == "" {
		t.Fatal("created record has empty id")
	}

	// Get.
	getResp := doJSON(t, http.MethodGet, s.URL+dto.PathPersistDocuments+"/"+created.ID, nil)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, want 200", getResp.StatusCode)
	}
	got := decode[dto.PersistRecord](t, getResp)
	if got.ID != created.ID || got.Title != "informe" || got.Status != dto.StatusPending {
		t.Errorf("get = %+v", got)
	}

	// Find by checksum (found).
	findResp := doJSON(t, http.MethodGet, s.URL+dto.PathPersistFindByChecksum+"?checksum=abc123", nil)
	if findResp.StatusCode != http.StatusOK {
		t.Fatalf("find status = %d, want 200", findResp.StatusCode)
	}
	found := decode[dto.PersistRecord](t, findResp)
	if found.ID != created.ID {
		t.Errorf("find.ID = %q, want %q", found.ID, created.ID)
	}

	// Find by checksum (not found).
	missResp := doJSON(t, http.MethodGet, s.URL+dto.PathPersistFindByChecksum+"?checksum=nope", nil)
	missResp.Body.Close()
	if missResp.StatusCode != http.StatusNotFound {
		t.Errorf("find-miss status = %d, want 404", missResp.StatusCode)
	}

	// Update (done + content).
	updateResp := doJSON(t, http.MethodPatch, s.URL+dto.PathPersistDocuments+"/"+created.ID, dto.PersistUpdateRequest{
		Status:  dto.StatusDone,
		Content: strPtr("# título\n\ntexto"),
	})
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d, want 200", updateResp.StatusCode)
	}
	updated := decode[dto.PersistRecord](t, updateResp)
	if updated.Status != dto.StatusDone || updated.Content == nil || *updated.Content != "# título\n\ntexto" {
		t.Errorf("updated = %+v", updated)
	}

	// List.
	listResp := doJSON(t, http.MethodGet, s.URL+dto.PathPersistDocuments, nil)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want 200", listResp.StatusCode)
	}
	list := decode[[]dto.PersistRecord](t, listResp)
	if len(list) != 1 {
		t.Errorf("list length = %d, want 1", len(list))
	}

	// Delete.
	delResp := doJSON(t, http.MethodDelete, s.URL+dto.PathPersistDocuments+"/"+created.ID, nil)
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", delResp.StatusCode)
	}

	// Get after delete.
	goneResp := doJSON(t, http.MethodGet, s.URL+dto.PathPersistDocuments+"/"+created.ID, nil)
	goneResp.Body.Close()
	if goneResp.StatusCode != http.StatusNotFound {
		t.Errorf("get-after-delete status = %d, want 404", goneResp.StatusCode)
	}
}

func TestMemoryQueue(t *testing.T) {
	q := stubs.NewMemoryQueue()

	job := dto.ExtractionJob{PdfID: "1", Filename: "informe.pdf", ContentBase64: []byte("%PDF-1.4")}
	if err := q.PublishExtraction(context.Background(), job); err != nil {
		t.Fatalf("PublishExtraction: %v", err)
	}

	published := q.PublishedExtractionJobs()
	if len(published) != 1 || published[0].PdfID != "1" || published[0].Filename != "informe.pdf" {
		t.Errorf("PublishedExtractionJobs = %+v", published)
	}

	q.PushResult(dto.ExtractionResult{PdfID: "1", Status: dto.StatusDone})
	got := <-q.Results()
	if got.PdfID != "1" || got.Status != dto.StatusDone {
		t.Errorf("result = %+v", got)
	}
}

func strPtr(s string) *string { return &s }
