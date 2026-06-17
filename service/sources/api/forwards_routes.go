package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"portier/service/sources/domain"
	"portier/service/sources/validation"
)

// forwardsRoutes registers the forward-rule API surface: the read/list endpoint
// plus the write, ID, lifecycle, and group-action routes (v1.15 Slice 11). The
// {id} and {group} chi patterns are used for route SHAPE only — each handler
// extracts the id/group segment from the request path with the original parsing
// logic (NEVER chi.URLParam), so encoded behavior is byte-identical to the
// pre-migration legacy dispatch:
//
//   - rule ids come from r.URL.Path (decoded) exactly as the old serveForwardByID
//     did; ids are UUIDs, so there is no encoding to preserve there;
//   - group names come from r.URL.EscapedPath() + url.PathUnescape, so an encoded
//     "/" (%2F) or space (%20) in a group name is preserved (see
//     TestGroupActionEncodedSpace / TestGroupActionEncodedSlash). chi matches the
//     {group} segment against r.URL.RawPath when present (encoded slash stays one
//     segment) or the decoded Path otherwise (a space stays within the segment),
//     and the handler re-derives the value itself — so chi.URLParam, which would
//     URL-decode the segment, is deliberately not used.
//
// Static routes (/api/forwards, /api/forwards/reorder, /api/forwards/groups/...)
// take precedence over the {id} param route in chi, so reorder and group actions
// are never misrouted as an id. A wrong method or unknown forwards subpath does
// not match here and is routed (chi NotFound/MethodNotAllowed → writeAPINotFound)
// to the generic /api 404 envelope (never a 405).
func (h *Handler) forwardsRoutes() []modularRoute {
	return []modularRoute{
		{method: http.MethodGet, path: "/api/forwards", handler: h.handleListForwards},
		{method: http.MethodPost, path: "/api/forwards", handler: h.createForward},
		{method: http.MethodPost, path: "/api/forwards/reorder", handler: h.reorderForwards},
		{method: http.MethodPatch, path: "/api/forwards/{id}", handler: h.handleUpdateForward},
		{method: http.MethodDelete, path: "/api/forwards/{id}", handler: h.handleDeleteForward},
		{method: http.MethodPost, path: "/api/forwards/{id}/start", handler: h.handleStartForward},
		{method: http.MethodPost, path: "/api/forwards/{id}/stop", handler: h.handleStopForward},
		{method: http.MethodPost, path: "/api/forwards/{id}/diagnose", handler: h.handleDiagnoseForward},
		{method: http.MethodPost, path: "/api/forwards/groups/{group}/start", handler: h.handleStartGroup},
		{method: http.MethodPost, path: "/api/forwards/groups/{group}/stop", handler: h.handleStopGroup},
	}
}

// handleListForwards returns every configured forward rule decorated with port
// advisories. Read-only, no request input, no side effects. Reads rule state via
// h.app.Manager.
func (h *Handler) handleListForwards(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, rulesToResponses(h.app.Manager.ListRules()))
}

// ruleIDFromPath extracts the {id} segment from a /api/forwards/{id}[/...] path
// using r.URL.Path (decoded) — the exact extraction the legacy serveForwardByID
// used. Rule ids are UUIDs, so there is no percent-encoding to preserve.
func ruleIDFromPath(r *http.Request) string {
	remainder := strings.TrimPrefix(r.URL.Path, "/api/forwards/")
	return strings.SplitN(remainder, "/", 2)[0]
}

func (h *Handler) createForward(w http.ResponseWriter, r *http.Request) {
	var input validation.ForwardRuleInput
	if !decodeRequest(w, r, &input) {
		return
	}

	rule, err := h.app.Manager.CreateRule(input)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toRuleResponse(rule))
}

func (h *Handler) handleUpdateForward(w http.ResponseWriter, r *http.Request) {
	h.updateForward(w, r, ruleIDFromPath(r))
}

func (h *Handler) updateForward(w http.ResponseWriter, r *http.Request, ruleID string) {
	raw, err := readBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {err.Error()}})
		return
	}
	patch, errors := validation.DecodeAndValidateForwardRulePatch(raw)
	if len(errors) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": errors})
		return
	}

	rule, err := h.app.Manager.UpdateRule(ruleID, patch)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toRuleResponse(rule))
}

func (h *Handler) handleDeleteForward(w http.ResponseWriter, r *http.Request) {
	h.deleteForward(w, ruleIDFromPath(r))
}

func (h *Handler) deleteForward(w http.ResponseWriter, ruleID string) {
	if err := h.app.Manager.DeleteRule(ruleID); err != nil {
		writeManagerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleStartForward(w http.ResponseWriter, r *http.Request) {
	h.startForward(w, ruleIDFromPath(r))
}

func (h *Handler) startForward(w http.ResponseWriter, ruleID string) {
	status, err := h.app.Manager.StartRule(ruleID)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) handleStopForward(w http.ResponseWriter, r *http.Request) {
	h.stopForward(w, ruleIDFromPath(r))
}

func (h *Handler) stopForward(w http.ResponseWriter, ruleID string) {
	status, err := h.app.Manager.StopRule(ruleID)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) handleDiagnoseForward(w http.ResponseWriter, r *http.Request) {
	h.diagnoseForward(w, ruleIDFromPath(r))
}

func (h *Handler) diagnoseForward(w http.ResponseWriter, ruleID string) {
	var targetRule *domain.ForwardRule
	for _, r := range h.app.Manager.ListRules() {
		if r.ID == ruleID {
			rule := r
			targetRule = &rule
			break
		}
	}
	if targetRule == nil {
		writeJSON(w, http.StatusNotFound, map[string][]string{
			"errors": {notFoundMessage},
		})
		return
	}

	var isRunning bool
	for _, s := range h.app.Manager.ListStatus() {
		if s.RuleID == ruleID {
			isRunning = s.Running
			break
		}
	}

	result := diagnoseRule(*targetRule, isRunning)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) reorderForwards(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	if body.IDs == nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {"ids must be an array of strings."}})
		return
	}

	if err := h.app.Manager.ReorderRules(body.IDs); err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rulesToResponses(h.app.Manager.ListRules()))
}

func (h *Handler) handleStartGroup(w http.ResponseWriter, r *http.Request) {
	h.groupAction(w, r, "start")
}

func (h *Handler) handleStopGroup(w http.ResponseWriter, r *http.Request) {
	h.groupAction(w, r, "stop")
}

// groupAction handles POST /api/forwards/groups/{group}/{start,stop}. The action
// is fixed by the chi route; the {group} segment is re-parsed from the escaped
// path (NOT chi.URLParam) so an encoded "/" inside a group name stays a single
// segment and is decoded exactly as the legacy serveGroupAction did. Behaviour
// over existing rule metadata — never mutates rule definitions, order, or
// metadata.
func (h *Handler) groupAction(w http.ResponseWriter, r *http.Request, action string) {
	remainder := strings.TrimPrefix(r.URL.EscapedPath(), "/api/forwards/groups/")
	groupSegment := strings.SplitN(remainder, "/", 2)[0]

	rawGroup, err := url.PathUnescape(groupSegment)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {"group is required."}})
		return
	}
	group, errs := validation.ValidateGroupName(rawGroup)
	if len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": errs})
		return
	}

	var results []domain.GroupActionResult
	if action == "start" {
		results = h.app.Manager.StartGroup(group)
	} else {
		results = h.app.Manager.StopGroup(group)
	}
	if len(results) == 0 {
		writeJSON(w, http.StatusNotFound, map[string][]string{
			"errors": {fmt.Sprintf("No rules found in group %q.", group)},
		})
		return
	}

	writeJSON(w, http.StatusOK, buildGroupActionResponse(group, action, results))
}

// buildGroupActionResponse computes the summary counts from the ordered per-rule
// results. Mirrors the TypeScript summarizeGroupAction (parity-tested).
func buildGroupActionResponse(group, action string, results []domain.GroupActionResult) domain.GroupActionResponse {
	succeeded, skipped, failed := 0, 0, 0
	for _, result := range results {
		switch result.Status {
		case "started", "stopped":
			succeeded++
		case "skipped":
			skipped++
		case "failed":
			failed++
		}
	}
	return domain.GroupActionResponse{
		Group:     group,
		Action:    action,
		Total:     len(results),
		Succeeded: succeeded,
		Skipped:   skipped,
		Failed:    failed,
		Results:   results,
	}
}
