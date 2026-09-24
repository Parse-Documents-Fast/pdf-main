package dto

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/Parse-Documents-Fast/pdf-main/internal/problem"
)

const (
	id          = "665f1a2b3c4d5e6f7a8b9c0d"
	checksum    = "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"
	markdown    = "# Título\n\ntexto…"
	createdJSON = "2026-09-18T10:00:00Z"
)

var (
	pdfBytes  = []byte("%PDF-1.4\nfake")
	createdAt = time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
)

// toMap marshals v to JSON and decodes it back into a map, so tests can assert
// on the actual wire shape without depending on field order or whitespace.
func toMap(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

// assertJSON checks that v marshals to exactly the keys/values in want (all
// values must be scalar or nil, i.e. no nested objects).
func assertJSON(t *testing.T, v any, want map[string]any) {
	t.Helper()
	got := toMap(t, v)
	if len(got) != len(want) {
		t.Errorf("%T: got %d fields %v, want %d fields %v", v, len(got), got, len(want), want)
		return
	}
	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			t.Errorf("%T: missing key %q (got %v)", v, k, got)
			continue
		}
		if gv != wv {
			t.Errorf("%T: key %q = %#v, want %#v", v, k, gv, wv)
		}
	}
}

func strPtr(s string) *string { return &s }

func TestPdfSummaryJSON(t *testing.T) {
	s := PdfSummary{
		ID:             id,
		Title:          "informe",
		OriginalFormat: FormatPDF,
		Checksum:       checksum,
		Status:         StatusPending,
		CreatedAt:      createdAt,
	}
	assertJSON(t, s, map[string]any{
		"id":              id,
		"title":           "informe",
		"original_format": "pdf",
		"checksum":        checksum,
		"status":          "pending",
		"created_at":      createdJSON,
	})
}

func TestPdfDocumentJSONPending(t *testing.T) {
	d := PdfDocument{
		PdfSummary: PdfSummary{
			ID:             id,
			Title:          "informe",
			OriginalFormat: FormatPDF,
			Checksum:       checksum,
			Status:         StatusPending,
			CreatedAt:      createdAt,
		},
		Content: nil,
		Error:   nil,
	}
	assertJSON(t, d, map[string]any{
		"id":              id,
		"title":           "informe",
		"original_format": "pdf",
		"checksum":        checksum,
		"status":          "pending",
		"created_at":      createdJSON,
		"content":         nil,
		"error":           nil,
	})
}

func TestPdfDocumentJSONDone(t *testing.T) {
	d := PdfDocument{
		PdfSummary: PdfSummary{
			ID:             id,
			Title:          "informe",
			OriginalFormat: FormatMarkdown,
			Checksum:       checksum,
			Status:         StatusDone,
			CreatedAt:      createdAt,
		},
		Content: strPtr(markdown),
		Error:   nil,
	}
	assertJSON(t, d, map[string]any{
		"id":              id,
		"title":           "informe",
		"original_format": "markdown",
		"checksum":        checksum,
		"status":          "done",
		"created_at":      createdJSON,
		"content":         markdown,
		"error":           nil,
	})
}

func TestValidateRequestJSON(t *testing.T) {
	r := ValidateRequest{Filename: "informe.pdf", ContentBase64: pdfBytes}
	assertJSON(t, r, map[string]any{
		"filename":       "informe.pdf",
		"content_base64": base64.StdEncoding.EncodeToString(pdfBytes),
	})
}

func TestValidateResponseJSON(t *testing.T) {
	r := ValidateResponse{OriginalFormat: FormatPDF, Checksum: checksum}
	assertJSON(t, r, map[string]any{
		"original_format": "pdf",
		"checksum":        checksum,
	})
}

func TestExtractionJobJSON(t *testing.T) {
	j := ExtractionJob{PdfID: id, Filename: "informe.pdf", ContentBase64: pdfBytes}
	assertJSON(t, j, map[string]any{
		"pdf_id":         id,
		"filename":       "informe.pdf",
		"content_base64": base64.StdEncoding.EncodeToString(pdfBytes),
	})
}

func TestExtractionResultDoneJSON(t *testing.T) {
	r := ExtractionResult{PdfID: id, Status: StatusDone, Content: strPtr(markdown)}
	assertJSON(t, r, map[string]any{
		"pdf_id":  id,
		"status":  "done",
		"content": markdown,
	})
}

func TestExtractionResultFailedJSON(t *testing.T) {
	r := ExtractionResult{
		PdfID:  id,
		Status: StatusFailed,
		Error: &problem.ProblemDetails{
			Type:   problem.TypeAboutBlank,
			Title:  "Extracción fallida",
			Status: 422,
			Detail: "No se pudo extraer texto del PDF",
		},
	}
	m := toMap(t, r)
	if m["pdf_id"] != id {
		t.Errorf("pdf_id = %#v, want %q", m["pdf_id"], id)
	}
	if m["status"] != "failed" {
		t.Errorf("status = %#v, want %q", m["status"], "failed")
	}
	if _, ok := m["content"]; ok {
		t.Errorf("content should be omitted, got %v", m)
	}
	errObj, ok := m["error"].(map[string]any)
	if !ok {
		t.Fatalf("error = %#v, want object", m["error"])
	}
	if errObj["detail"] != "No se pudo extraer texto del PDF" {
		t.Errorf("error.detail = %#v", errObj["detail"])
	}
}

func TestConvertRequestJSON(t *testing.T) {
	r := ConvertRequest{Content: markdown}
	assertJSON(t, r, map[string]any{"content": markdown})
}

func TestConvertResponseJSON(t *testing.T) {
	r := ConvertResponse{ContentBase64: pdfBytes, MimeType: "application/pdf"}
	assertJSON(t, r, map[string]any{
		"content_base64": base64.StdEncoding.EncodeToString(pdfBytes),
		"mime_type":      "application/pdf",
	})
}

func TestPersistCreateRequestJSONPending(t *testing.T) {
	r := PersistCreateRequest{
		Title:          "informe",
		OriginalFormat: FormatPDF,
		Checksum:       checksum,
		Status:         StatusPending,
	}
	assertJSON(t, r, map[string]any{
		"title":           "informe",
		"original_format": "pdf",
		"checksum":        checksum,
		"status":          "pending",
	})
}

func TestPersistCreateRequestJSONDone(t *testing.T) {
	r := PersistCreateRequest{
		Title:          "informe",
		OriginalFormat: FormatMarkdown,
		Checksum:       checksum,
		Status:         StatusDone,
		Content:        strPtr(markdown),
	}
	assertJSON(t, r, map[string]any{
		"title":           "informe",
		"original_format": "markdown",
		"checksum":        checksum,
		"status":          "done",
		"content":         markdown,
	})
}

func TestPersistRecordJSONPending(t *testing.T) {
	r := PersistRecord{
		ID:             id,
		Title:          "informe",
		OriginalFormat: FormatPDF,
		Checksum:       checksum,
		Status:         StatusPending,
		Content:        nil,
		Error:          nil,
		CreatedAt:      createdAt,
	}
	assertJSON(t, r, map[string]any{
		"id":              id,
		"title":           "informe",
		"original_format": "pdf",
		"checksum":        checksum,
		"status":          "pending",
		"content":         nil,
		"error":           nil,
		"created_at":      createdJSON,
	})
}

func TestPersistUpdateRequestJSONDone(t *testing.T) {
	r := PersistUpdateRequest{Content: strPtr(markdown), Status: StatusDone}
	assertJSON(t, r, map[string]any{"content": markdown, "status": "done"})
}

func TestPersistUpdateRequestJSONFailed(t *testing.T) {
	r := PersistUpdateRequest{Status: StatusFailed, Error: strPtr("No se pudo extraer texto del PDF")}
	assertJSON(t, r, map[string]any{"status": "failed", "error": "No se pudo extraer texto del PDF"})
}

func TestPersistRecordSummary(t *testing.T) {
	r := PersistRecord{
		ID:             id,
		Title:          "informe",
		OriginalFormat: FormatPDF,
		Checksum:       checksum,
		Status:         StatusPending,
		Content:        nil,
		Error:          nil,
		CreatedAt:      createdAt,
	}
	want := PdfSummary{
		ID:             id,
		Title:          "informe",
		OriginalFormat: FormatPDF,
		Checksum:       checksum,
		Status:         StatusPending,
		CreatedAt:      createdAt,
	}
	if got := r.Summary(); got != want {
		t.Errorf("Summary() = %+v, want %+v", got, want)
	}
}

func TestPersistRecordDocument(t *testing.T) {
	r := PersistRecord{
		ID:             id,
		Title:          "informe",
		OriginalFormat: FormatMarkdown,
		Checksum:       checksum,
		Status:         StatusDone,
		Content:        strPtr(markdown),
		Error:          nil,
		CreatedAt:      createdAt,
	}
	got := r.Document()
	if got.Content == nil || *got.Content != markdown {
		t.Errorf("Document().Content = %v, want %q", got.Content, markdown)
	}
	if got.ID != id || got.Status != StatusDone {
		t.Errorf("Document() = %+v", got)
	}
}
