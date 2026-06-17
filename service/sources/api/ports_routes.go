package api

import (
	"net/http"
	"strconv"

	"portier/service/sources/advisory"
)

// portsRoutes registers the port advisory endpoint.
func (h *Handler) portsRoutes() []modularRoute {
	return []modularRoute{
		{method: http.MethodGet, path: "/api/ports/advisory", handler: h.handlePortAdvisory},
	}
}

// handlePortAdvisory returns port advisories for the requested port + purpose.
// It is read-only and stateless (no manager dependency). It parses and validates
// the query, returning a 400 with an error message for a missing/out-of-range
// port or an invalid purpose, otherwise the advisory list.
func (h *Handler) handlePortAdvisory(w http.ResponseWriter, r *http.Request) {
	port, err := strconv.Atoi(r.URL.Query().Get("port"))
	if err != nil || port < 1 || port > 65535 {
		writeJSON(w, http.StatusBadRequest, map[string][]string{
			"errors": {"port must be an integer from 1 to 65535."},
		})
		return
	}

	purpose := r.URL.Query().Get("purpose")
	if purpose != advisory.PurposeManagement && purpose != advisory.PurposeForward {
		writeJSON(w, http.StatusBadRequest, map[string][]string{
			"errors": {"purpose must be management or forward."},
		})
		return
	}

	writeJSON(w, http.StatusOK, advisory.GetPortAdvisories(advisory.Input{
		Port:       port,
		ListenHost: r.URL.Query().Get("listenHost"),
		Purpose:    purpose,
	}))
}
