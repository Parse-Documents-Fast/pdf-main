package dto

// ConvertRequest is the body sent to pdf-converter (Markdown → PDF). The
// converter is download-only per ADR-0005: it has no ingestion/queue contract.
type ConvertRequest struct {
	Content string `json:"content"`
}

// ConvertResponse is the successful response from pdf-converter. The resulting
// PDF binary travels base64-encoded (ADR-0002).
type ConvertResponse struct {
	ContentBase64 []byte `json:"content_base64"`
	MimeType      string `json:"mime_type"`
}
