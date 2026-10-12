package httpapi

import "net/http"

// handleHealth handles GET /health (liveness). It reports the service is up
// without checking downstream dependencies, so the Docker healthcheck can poll
// it periodically.
func (a *api) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
