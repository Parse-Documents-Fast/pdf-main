package httpapi

import (
	"io"
	"net/http"
	"strings"

	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/Parse-Documents-Fast/pdf-main/internal/problem"
)

type api struct {
	ports orchestrator.Ports
}

// handleUpload handles POST /api/pdfs: it reads the multipart form, delegates
// to the orchestrator core and maps the result to a JSON response.
func (a *api) handleUpload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		problem.WriteProblem(w, http.StatusBadRequest, titleInvalid, "Missing 'file' field in the form", r.URL.Path)
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		problem.WriteProblem(w, http.StatusBadRequest, titleInvalid, "Could not read the file", r.URL.Path)
		return
	}

	title := r.FormValue("title")
	if title == "" {
		title = titleFromFilename(header.Filename)
	}

	summary, err := orchestrator.Submit(r.Context(), a.ports, content, header.Filename, title)
	if err != nil {
		writeError(w, r, err)
		return
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
