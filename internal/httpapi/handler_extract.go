package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/problem"
)

// handleExtract handles POST /extract (benchmark-only).
//
// It synchronously validates a PDF and forwards it to pdf-extractor, holding
// the connection open until the extracted Markdown and page count are ready.
// This deviates from ADR-0004 (extraction normally goes through the queue),
// but is required by the load-test benchmark. The endpoint is additive and
// does not touch the async /api/pdfs flow.
func (a *api) handleExtract(w http.ResponseWriter, r *http.Request) {
	a.wrapBody(w, r)

	content, filename, err := a.readPDF(r)
	if err != nil {
		if errors.Is(err, errFileTooLarge) || isMaxBytesError(err) {
			a.writeTooLarge(w, r)
			return
		}
		problem.WriteProblem(w, http.StatusBadRequest, titleInvalid, "Missing or unreadable PDF file", r.URL.Path)
		return
	}
	a.logger.Info("pdf received", "size_bytes", len(content))

	v, err := a.ports.Validator.Validate(r.Context(), content, filename)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	if v.OriginalFormat != dto.FormatPDF {
		problem.WriteProblem(w, http.StatusBadRequest, titleInvalid, "The file is not a PDF", r.URL.Path)
		return
	}

	result, err := a.ports.Extractor.Extract(r.Context(), content)
	if err != nil {
		a.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// readPDF reads the PDF from a multipart "file" field or, failing that, from
// the raw request body (application/pdf), enforcing the file-size limit.
func (a *api) readPDF(r *http.Request) ([]byte, string, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		file, header, err := r.FormFile("file")
		if err != nil {
			return nil, "", err
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, "", err
		}
		if err := a.checkSize(data); err != nil {
			return nil, "", err
		}
		return data, header.Filename, nil
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, "", err
	}
	if err := a.checkSize(data); err != nil {
		return nil, "", err
	}
	return data, "document.pdf", nil
}
