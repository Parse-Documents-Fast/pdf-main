package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/Parse-Documents-Fast/pdf-main/internal/problem"
)

type api struct {
	ports         orchestrator.Ports
	logger        *slog.Logger
	maxFileSizeMB int
}

// handleUpload handles POST /api/pdfs: it reads the multipart form, delegates
// to the orchestrator core and maps the result to a JSON response.
func (a *api) handleUpload(w http.ResponseWriter, r *http.Request) {
	a.wrapBody(w, r)

	file, header, err := r.FormFile("file")
	if err != nil {
		if isMaxBytesError(err) {
			a.writeTooLarge(w, r)
			return
		}
		problem.WriteProblem(w, http.StatusBadRequest, titleInvalid, "Missing 'file' field in the form", r.URL.Path)
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		if isMaxBytesError(err) {
			a.writeTooLarge(w, r)
			return
		}
		problem.WriteProblem(w, http.StatusBadRequest, titleInvalid, "Could not read the file", r.URL.Path)
		return
	}
	if err := a.checkSize(content); err != nil {
		a.writeTooLarge(w, r)
		return
	}

	title := r.FormValue("title")
	if title == "" {
		title = titleFromFilename(header.Filename)
	}

	summary, err := orchestrator.Submit(r.Context(), a.ports, content, header.Filename, title)
	if err != nil {
		a.writeError(w, r, err)
		return
	}

	a.logger.Info("pdf received", "size_bytes", len(content))
	switch summary.Status {
	case dto.StatusPending:
		a.logger.Debug("document queued", "pdf_id", summary.ID)
	case dto.StatusDone:
		a.logger.Debug("markdown persisted", "pdf_id", summary.ID)
	}

	writeJSON(w, http.StatusOK, summary)
}

// titleFromFilename derives a document title by stripping the extension:
// "informe.pdf" → "informe". A leading dot is preserved (".hidden" stays).
func titleFromFilename(filename string) string {
	if i := strings.LastIndex(filename, "."); i > 0 {
		return filename[:i]
	}
	return filename
}
