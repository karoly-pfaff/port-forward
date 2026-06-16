package api

import (
	"net/http"
	"strconv"

	"portier/service/sources/activity"
)

// activityRoutes registers the activity log endpoints. Both share the exact path
// /api/activity on different methods — the first modular registration to expose
// two methods on one path; chi matches each (method, path) pair, so a third
// method (e.g. POST) falls through (MethodNotAllowed → legacy) to the generic
// /api 404 envelope.
func (h *Handler) activityRoutes() []modularRoute {
	return []modularRoute{
		{method: http.MethodGet, path: "/api/activity", handler: h.handleListActivity},
		{method: http.MethodDelete, path: "/api/activity", handler: h.handleClearActivity},
	}
}

// handleListActivity returns the bounded activity log, filtered by the optional
// limit/ruleId/type/severity query params (an unparseable limit is ignored,
// falling back to the store default). Behavior — query parsing, defaults, and the
// 200 {"events": ...} body — is identical to the pre-modularization handler; it
// reads the activity store via the h.manager bridge.
func (h *Handler) handleListActivity(w http.ResponseWriter, r *http.Request) {
	params := activity.ListParams{}
	q := r.URL.Query()

	if limitStr := q.Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil {
			params.Limit = n
		}
	}
	if ruleID := q.Get("ruleId"); ruleID != "" {
		params.RuleID = &ruleID
	}
	if eventType := q.Get("type"); eventType != "" {
		params.Type = &eventType
	}
	if severity := q.Get("severity"); severity != "" {
		params.Severity = &severity
	}

	events := h.manager.ListActivity(params)
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

// handleClearActivity clears the activity log and returns 204 with no body (and
// no Content-Type — it does not go through writeJSON). Behavior identical to the
// pre-modularization handler; it mutates the activity store via the h.manager
// bridge.
func (h *Handler) handleClearActivity(w http.ResponseWriter, _ *http.Request) {
	h.manager.ClearActivity()
	w.WriteHeader(http.StatusNoContent)
}
