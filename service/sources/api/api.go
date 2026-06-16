package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"portier/service/sources/advisory"
	"portier/service/sources/app"
	"portier/service/sources/configplan"
	"portier/service/sources/domain"
	"portier/service/sources/manager"
	"portier/service/sources/static"
	"portier/service/sources/validation"
)

const notFoundMessage = "API route was not found."

// Handler is the service HTTP entry point. It holds the explicit dependency
// container (app.App) and the chi API router (v1.15). Routes migrated into
// feature modules are served by the chi router; everything else falls through
// to the ordered serveLegacyAPI dispatch.
type Handler struct {
	app             *app.App
	staticAvailable bool

	// apiRouter is the chi router owning the migrated API routes (v1.15 Slice
	// 10). It only ever serves requests under /api; its NotFound/MethodNotAllowed
	// handlers delegate to serveLegacyAPI so unmigrated routes keep working.
	apiRouter http.Handler

	// manager is a temporary bridge for the not-yet-migrated ordered
	// serveLegacyAPI dispatch (v1.15 Slice 2). It mirrors app.Manager; later
	// slices migrate each feature handler to read app.Manager directly and remove
	// this field.
	manager *manager.Manager
}

// NewHandler builds the HTTP handler from the explicit app dependency container.
// This is the preferred construction path (v1.15); main.go and tests build an
// app.App via app.New and pass it here.
func NewHandler(application *app.App) *Handler {
	h := &Handler{
		app:             application,
		staticAvailable: static.HasClient(application.Options.StaticDir),
		manager:         application.Manager,
	}
	h.apiRouter = h.buildAPIRouter()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The top-level API/static boundary stays owned by Portier (not chi): /api
	// and /api/* go to the chi-backed API router, everything else to static
	// serving (or a plain 404 when there is no client build).
	if isAPIPath(r.URL.Path) {
		h.apiRouter.ServeHTTP(w, r)
		return
	}

	if h.staticAvailable {
		static.ServeClient(w, r, h.app.Options.StaticDir)
		return
	}

	http.NotFound(w, r)
}

func isAPIPath(route string) bool {
	return route == "/api" || strings.HasPrefix(route, "/api/")
}

// serveLegacyAPI is the ordered dispatch for API routes not yet migrated onto
// the chi router. It is reached via the chi router's NotFound/MethodNotAllowed
// fallback, so the migrated routes have already been tried; an unmatched request
// ends in the generic /api 404 envelope (preserving 404-not-405).
func (h *Handler) serveLegacyAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/api/forwards" {
		h.createForward(w, r)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/api/forwards/reorder" {
		h.reorderForwards(w, r)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/api/forwards/groups/") {
		h.serveGroupAction(w, r)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/api/forwards/") {
		h.serveForwardByID(w, r)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/api/config/export" {
		writeJSON(w, http.StatusOK, h.manager.ExportConfig())
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/api/config/import" {
		h.importConfig(w, r)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/api/config/plan" {
		h.configPlan(w, r)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/api/config/apply" {
		h.configApply(w, r)
		return
	}

	writeJSON(w, http.StatusNotFound, map[string][]string{
		"errors": {notFoundMessage},
	})
}

func (h *Handler) createForward(w http.ResponseWriter, r *http.Request) {
	var input validation.ForwardRuleInput
	if !decodeRequest(w, r, &input) {
		return
	}

	rule, err := h.manager.CreateRule(input)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toRuleResponse(rule))
}

func (h *Handler) serveForwardByID(w http.ResponseWriter, r *http.Request) {
	remainder := strings.TrimPrefix(r.URL.Path, "/api/forwards/")
	if remainder == "" {
		writeJSON(w, http.StatusNotFound, map[string][]string{"errors": {notFoundMessage}})
		return
	}
	parts := strings.Split(remainder, "/")
	ruleID := parts[0]

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodPatch:
			h.updateForward(w, r, ruleID)
		case http.MethodDelete:
			h.deleteForward(w, ruleID)
		default:
			writeJSON(w, http.StatusNotFound, map[string][]string{"errors": {notFoundMessage}})
		}
		return
	}

	if len(parts) == 2 && r.Method == http.MethodPost {
		switch parts[1] {
		case "start":
			h.startForward(w, ruleID)
			return
		case "stop":
			h.stopForward(w, ruleID)
			return
		case "diagnose":
			h.diagnoseForward(w, ruleID)
			return
		}
	}

	writeJSON(w, http.StatusNotFound, map[string][]string{"errors": {notFoundMessage}})
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

	rule, err := h.manager.UpdateRule(ruleID, patch)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toRuleResponse(rule))
}

func (h *Handler) deleteForward(w http.ResponseWriter, ruleID string) {
	if err := h.manager.DeleteRule(ruleID); err != nil {
		writeManagerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// serveGroupAction handles POST /api/forwards/groups/:group/{start,stop}.
// The :group segment is parsed from the escaped path so an encoded "/" inside a
// group name stays a single segment. Behaviour over existing rule metadata —
// never mutates rule definitions, order, or metadata.
//
// v1.15 Slice 10 note (hard acceptance criterion for the future parameterized
// migration): this handler is still reached through the chi router's NotFound
// fallback (group routes are not chi-registered), so r.URL.EscapedPath() is
// untouched and encoded group names are preserved (see TestGroupActionEncodedSpace).
// When the group/id routes migrate onto chi {param} patterns, chi.URLParam
// URL-DECODES the segment — that would collapse an encoded "/" — so the
// migration must NOT rely on chi.URLParam for the group/id segment without
// re-establishing the encoded-slash behavior (keep reading EscapedPath, or use
// a chi route that preserves the raw segment), and must keep
// TestGroupActionEncodedSpace green.
func (h *Handler) serveGroupAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusNotFound, map[string][]string{"errors": {notFoundMessage}})
		return
	}

	remainder := strings.TrimPrefix(r.URL.EscapedPath(), "/api/forwards/groups/")
	parts := strings.Split(remainder, "/")
	if len(parts) != 2 {
		writeJSON(w, http.StatusNotFound, map[string][]string{"errors": {notFoundMessage}})
		return
	}
	action := parts[1]
	if action != "start" && action != "stop" {
		writeJSON(w, http.StatusNotFound, map[string][]string{"errors": {notFoundMessage}})
		return
	}

	rawGroup, err := url.PathUnescape(parts[0])
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
		results = h.manager.StartGroup(group)
	} else {
		results = h.manager.StopGroup(group)
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

func (h *Handler) startForward(w http.ResponseWriter, ruleID string) {
	status, err := h.manager.StartRule(ruleID)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) stopForward(w http.ResponseWriter, ruleID string) {
	status, err := h.manager.StopRule(ruleID)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) diagnoseForward(w http.ResponseWriter, ruleID string) {
	var targetRule *domain.ForwardRule
	for _, r := range h.manager.ListRules() {
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
	for _, s := range h.manager.ListStatus() {
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

	if err := h.manager.ReorderRules(body.IDs); err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rulesToResponses(h.manager.ListRules()))
}

func (h *Handler) configPlan(w http.ResponseWriter, r *http.Request) {
	raw, err := readBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {err.Error()}})
		return
	}

	var body struct {
		Desired *json.RawMessage `json:"desired"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {err.Error()}})
		return
	}
	if body.Desired == nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {"desired is required."}})
		return
	}

	plan := configplan.BuildConfigPlan(configplan.Input{
		CurrentRules: h.manager.ListRules(),
		DesiredRaw:   *body.Desired,
	})
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handler) importConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode   string          `json:"mode"`
		Config json.RawMessage `json:"config"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	if body.Mode != "replace" && body.Mode != "merge" {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {"mode must be replace or merge."}})
		return
	}

	var importedConfig struct {
		Version    string               `json:"version"`
		ExportedAt string               `json:"exportedAt"`
		Rules      []domain.ForwardRule `json:"rules"`
	}
	if len(body.Config) == 0 || !json.Valid(body.Config) {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {"config must be a valid Portier config object with version 1 and a rules array."}})
		return
	}
	if err := json.Unmarshal(body.Config, &importedConfig); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {"config must be a valid Portier config object with version 1 and a rules array."}})
		return
	}
	if importedConfig.Version != "1" || importedConfig.Rules == nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {"config must be a valid Portier config object with version 1 and a rules array."}})
		return
	}

	result, err := h.manager.ImportConfig(domain.ExportedConfig{
		Version:    importedConfig.Version,
		ExportedAt: importedConfig.ExportedAt,
		Rules:      importedConfig.Rules,
	}, body.Mode)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	if len(result.Errors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": result.Errors, "result": result})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result, "rules": rulesToResponses(h.manager.ListRules())})
}

func rulesToResponses(rules []domain.ForwardRule) []domain.ForwardRuleResponse {
	responses := make([]domain.ForwardRuleResponse, 0, len(rules))
	for _, rule := range rules {
		responses = append(responses, toRuleResponse(rule))
	}
	return responses
}

func toRuleResponse(rule domain.ForwardRule) domain.ForwardRuleResponse {
	return domain.ForwardRuleResponse{
		ID:         rule.ID,
		Name:       rule.Name,
		Protocol:   rule.Protocol,
		ListenHost: rule.ListenHost,
		ListenPort: rule.ListenPort,
		TargetHost: rule.TargetHost,
		TargetPort: rule.TargetPort,
		Enabled:    rule.Enabled,
		UdpMode:    rule.UdpMode,
		Group:      rule.Group,
		Advisories: advisory.GetPortAdvisories(advisory.Input{
			Port:       rule.ListenPort,
			ListenHost: rule.ListenHost,
			Purpose:    advisory.PurposeForward,
		}),
	}
}

type applyResponse struct {
	Ok        bool                `json:"ok"`
	DryRun    bool                `json:"dryRun"`
	AppliedAt string              `json:"appliedAt"`
	Plan      configplan.Response `json:"plan"`
	Applied   map[string]int      `json:"applied"`
}

func (h *Handler) configApply(w http.ResponseWriter, r *http.Request) {
	raw, err := readBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {err.Error()}})
		return
	}

	var body struct {
		Desired *json.RawMessage `json:"desired"`
		Yes     bool             `json:"yes"`
		DryRun  bool             `json:"dryRun"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {err.Error()}})
		return
	}
	if body.Desired == nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {"desired is required."}})
		return
	}

	appliedAt := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	plan := configplan.BuildConfigPlan(configplan.Input{
		CurrentRules: h.manager.ListRules(),
		DesiredRaw:   *body.Desired,
	})

	zeroCounts := map[string]int{"add": 0, "update": 0, "remove": 0, "unchanged": plan.Summary.Unchanged}

	if plan.Summary.HasErrors {
		writeJSON(w, http.StatusOK, applyResponse{
			Ok: false, DryRun: body.DryRun, AppliedAt: appliedAt, Plan: plan, Applied: zeroCounts,
		})
		return
	}

	// Apply transformation (desired-state rule list + counts) lives in the plan
	// engine; the handler keeps only request/response and gating concerns.
	applyResult := configplan.BuildApplyImportFromPlan(plan, domain.NewRuleID)

	if body.DryRun {
		writeJSON(w, http.StatusOK, applyResponse{
			Ok: true, DryRun: true, AppliedAt: appliedAt, Plan: plan, Applied: applyResult.Applied,
		})
		return
	}

	if plan.Summary.Destructive > 0 && !body.Yes {
		writeJSON(w, http.StatusBadRequest, map[string][]string{
			"errors": {"Apply requires yes: true when destructive operations are present."},
		})
		return
	}

	if plan.Summary.HasDrift {
		result, err := h.manager.ImportConfig(domain.ExportedConfig{
			Version: "1", Rules: applyResult.Rules,
		}, "replace")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string][]string{"errors": {err.Error()}})
			return
		}
		// Invariant (Resilience-C): apply must never report ok:true when the
		// underlying import reports errors. Every currently-reachable import error
		// path is pre-blocked before this point — duplicate listen bindings via the
		// plan engine's detectDuplicateKeys (→ Summary.HasErrors), invalid desired
		// rules via plan validation, and persist failures throw (→ 500) — so this
		// is a belt-and-suspenders guard against future drift, mirroring TS api.ts.
		// Surface the import errors through the existing Plan.Errors field; no
		// applied counts.
		if len(result.Errors) > 0 {
			writeJSON(w, http.StatusOK, applyImportErrorResponse(plan, result.Errors, appliedAt, zeroCounts))
			return
		}
	}

	writeJSON(w, http.StatusOK, applyResponse{
		Ok: true, DryRun: false, AppliedAt: appliedAt, Plan: plan, Applied: applyResult.Applied,
	})
}

// applyImportErrorResponse builds the ok:false config-apply response that
// surfaces import-level errors through the existing Plan.Errors field, with no
// applied counts (Resilience-C invariant). Extracted so the construction is unit
// testable even though the handler branch that calls it is structurally
// unreachable today — every reachable import-error path is pre-blocked by the
// plan engine (duplicate bindings, invalid rules) or throws (persist).
func applyImportErrorResponse(plan configplan.Response, errs []string, appliedAt string, zeroCounts map[string]int) applyResponse {
	for _, msg := range errs {
		plan.Errors = append(plan.Errors, configplan.PlanError{Code: "IMPORT_ERROR", Message: msg})
	}
	plan.Summary.HasErrors = true
	return applyResponse{
		Ok: false, DryRun: false, AppliedAt: appliedAt, Plan: plan, Applied: zeroCounts,
	}
}
