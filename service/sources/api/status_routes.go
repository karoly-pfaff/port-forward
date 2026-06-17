package api

import "net/http"

// statusRoutes registers the rule status endpoint.
func (h *Handler) statusRoutes() []modularRoute {
	return []modularRoute{
		{method: http.MethodGet, path: "/api/status", handler: h.handleStatus},
	}
}

// handleStatus returns the live status for every rule. Read-only with no request
// input and no side effects; behavior — the 200 status array body via writeJSON —
// is identical to the pre-modularization inline handler. It reads rule state via
// h.app.Manager.
func (h *Handler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.app.Manager.ListStatus())
}
