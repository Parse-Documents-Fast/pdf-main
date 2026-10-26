package httpapi

import (
	"errors"
	"net/http"
)

// multipartOverheadBytes is the margin added to the request-body cap so that a
// file at the exact business limit still fits after multipart delimiters and
// headers. It must stay in sync with the Traefik body limit (pdf-infra) per
// ADR-0008 rule 6.
const multipartOverheadBytes = 1 << 20

// errFileTooLarge marks an upload rejected for exceeding the file-size limit.
var errFileTooLarge = errors.New("file too large")

// maxFileBytes returns the business file-size limit in bytes (ADR-0008).
func (a *api) maxFileBytes() int64 {
	return int64(a.maxFileSizeMB) << 20
}

// wrapBody caps the whole request body at the file limit plus the multipart
// margin (hard cap), so oversized bodies are rejected before being fully read.
// The exact file-size check in checkSize still applies afterwards.
func (a *api) wrapBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, a.maxFileBytes()+multipartOverheadBytes)
}

// checkSize rejects content larger than the file-size limit (exact check).
func (a *api) checkSize(content []byte) error {
	if int64(len(content)) > a.maxFileBytes() {
		return errFileTooLarge
	}
	return nil
}

// isMaxBytesError reports whether err is the hard body cap tripping.
func isMaxBytesError(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}
