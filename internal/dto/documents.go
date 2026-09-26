// Package dto defines the wire contracts (JSON, snake_case) exchanged between
// pdf-main and the other services, plus the public API contract exposed to
// clients. See docs/spec.md §DTOs and ADR-0002.
package dto

import "time"

// Format identifies the original format of an uploaded document.
type Format string

const (
	FormatPDF      Format = "pdf"
	FormatMarkdown Format = "markdown"
)

// Status is the lifecycle state of a stored document.
type Status string

const (
	StatusPending Status = "pending"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

// PdfSummary is the public representation of a document, returned by
// POST /api/pdfs (200) and as an element of GET /api/pdfs (200).
type PdfSummary struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	OriginalFormat Format    `json:"original_format"`
	Checksum       string    `json:"checksum"`
	Status         Status    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// PdfDocument is the full public representation returned by
// GET /api/pdfs/{id}: a PdfSummary plus the Markdown content and, when the
// document failed, the short error detail.
type PdfDocument struct {
	PdfSummary
	Content *string `json:"content"`
	Error   *string `json:"error"`
}

// PersistCreateRequest is the body of POST to pdf-persistence. Content is only
// set for Markdown uploads (persisted synchronously as done).
type PersistCreateRequest struct {
	Title          string  `json:"title"`
	OriginalFormat Format  `json:"original_format"`
	Checksum       string  `json:"checksum"`
	Status         Status  `json:"status"`
	Content        *string `json:"content,omitempty"`
}

// PersistRecord is the record returned by pdf-persistence (create/get/find).
// Content and Error are always present in the wire (null while unset).
type PersistRecord struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	OriginalFormat Format    `json:"original_format"`
	Checksum       string    `json:"checksum"`
	Status         Status    `json:"status"`
	Content        *string   `json:"content"`
	Error          *string   `json:"error"`
	CreatedAt      time.Time `json:"created_at"`
}

// PersistUpdateRequest is the body of PATCH to pdf-persistence, used when a
// queue result arrives (done → content+status, failed → status+error).
type PersistUpdateRequest struct {
	Content *string `json:"content,omitempty"`
	Status  Status  `json:"status"`
	Error   *string `json:"error,omitempty"`
}

// Summary reduces a PersistRecord to its public PdfSummary representation.
func (r PersistRecord) Summary() PdfSummary {
	return PdfSummary{
		ID:             r.ID,
		Title:          r.Title,
		OriginalFormat: r.OriginalFormat,
		Checksum:       r.Checksum,
		Status:         r.Status,
		CreatedAt:      r.CreatedAt,
	}
}

// Document reduces a PersistRecord to its public PdfDocument representation.
func (r PersistRecord) Document() PdfDocument {
	return PdfDocument{
		PdfSummary: r.Summary(),
		Content:    r.Content,
		Error:      r.Error,
	}
}
