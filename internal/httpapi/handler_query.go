package httpapi

import (
	"net/http"

	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/go-chi/chi/v5"
)

// handleList handles GET /api/pdfs.
//
// Contract: 200 with a JSON array of PdfSummary (no content field). This is
// the list the CLI/web client renders; the shape is owned by pdf-main and
// consumed by the client, so it is a coordination point.
func (a *api) handleList(w http.ResponseWriter, r *http.Request) {
	summaries, err := orchestrator.List(r.Context(), a.ports)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, summaries)
}

// handleGet handles GET /api/pdfs/{id}.
//
// Contract: 200 with a PdfDocument (summary + Markdown content + error, null
// while pending/failed); 404 when the id does not exist. The content is the
// canonical Markdown (ADR-0005), not HTML nor raw PDF.
func (a *api) handleGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	doc, err := orchestrator.Get(r.Context(), a.ports, id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

// handleDelete handles DELETE /api/pdfs/{id}.
//
// Contract: 204 with an empty body on success; 404 when the id does not exist.
func (a *api) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := orchestrator.Delete(r.Context(), a.ports, id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
