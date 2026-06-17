package api

import "net/http"

// statusRoutes registers the rule status endpoint.
func (h *Handler) statusRoutes() []modularRoute {
	return []modularRoute{
		{method: http.MethodGet, path: "/api/status", handler: h.handleStatus},
	}
}

// handleStatus returns the live status for every rule as a 200 status array via
// writeJSON. Read-only with no request input and no side effects; it reads rule
// state via h.app.Manager.
func (h *Handler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.app.Manager.ListStatus())
}
