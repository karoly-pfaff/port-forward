package api

import "net/http"

// forwardsRoutes registers the forward-rule read/list endpoint. Only the
// read/list route is migrated here; the forwards write/lifecycle/group routes
// (POST/PATCH/DELETE, reorder, start/stop/diagnose, group actions) still flow
// through the legacy ordered serveLegacyAPI dispatch.
func (h *Handler) forwardsRoutes() []modularRoute {
	return []modularRoute{
		{method: http.MethodGet, path: "/api/forwards", handler: h.handleListForwards},
	}
}

// handleListForwards returns every configured forward rule decorated with port
// advisories. Read-only with no request input and no side effects; behavior —
// the 200 ForwardRuleResponse array body via writeJSON — is identical to the
// pre-modularization inline handler. It reads rule state via the h.manager
// bridge. The shared rulesToResponses/toRuleResponse mappers stay in api.go for
// now because the unmigrated write/import handlers still call them; their final
// home is decided when the forwards write group migrates.
func (h *Handler) handleListForwards(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, rulesToResponses(h.manager.ListRules()))
}
