package dto

// ValidateRequest is the body sent to pdf-validator (HTTP sync). The binary
// PDF travels base64-encoded per ADR-0002.
type ValidateRequest struct {
	Filename      string `json:"filename"`
	ContentBase64 []byte `json:"content_base64"`
}

// ValidateResponse is the successful response from pdf-validator.
type ValidateResponse struct {
	OriginalFormat Format `json:"original_format"`
	Checksum       string `json:"checksum"`
}
