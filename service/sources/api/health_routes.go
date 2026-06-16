package api

import "net/http"

// healthRoutes registers the Go-native liveness endpoint.
//
// GET /api/health is a Go-native liveness probe that lives OUTSIDE the
// contract-validated /api surface (the TypeScript server documents /api/health
// as not-implemented). It is preserved here exactly as-is — it must not be
// "aligned" with the TypeScript contract.
func (h *Handler) healthRoutes() []modularRoute {
	return []modularRoute{
		{method: http.MethodGet, path: "/api/health", handler: h.handleHealth},
	}
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"server": "go",
		"name":   "Portier",
	})
}
