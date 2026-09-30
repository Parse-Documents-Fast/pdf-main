package dto

import "github.com/Parse-Documents-Fast/pdf-main/internal/problem"

// ExtractionJob is the message published to queue:extraction (ADR-0004/0005).
// The raw PDF travels base64-encoded.
type ExtractionJob struct {
	PdfID         string `json:"pdf_id"`
	Filename      string `json:"filename"`
	ContentBase64 []byte `json:"content_base64"`
}

// ExtractionResult is the message consumed from queue:extraction-results.
// When done, Content carries the Markdown; when failed, Error carries the
// downstream RFC 9457 problem.
type ExtractionResult struct {
	PdfID   string                  `json:"pdf_id"`
	Status  Status                  `json:"status"`
	Content *string                 `json:"content,omitempty"`
	Error   *problem.ProblemDetails `json:"error,omitempty"`
}

// ExtractRequest is the body sent to pdf-extractor's synchronous /extract
// endpoint (benchmark-only, ADR-0004 deviation: sync instead of queue).
type ExtractRequest struct {
	ContentBase64 []byte `json:"content_base64"`
}

// ExtractResponse is the synchronous extraction result. It doubles as the
// public POST /extract response of pdf-main.
type ExtractResponse struct {
	Content   string `json:"content"`
	PageCount int    `json:"page_count"`
}
