package api

import (
	"encoding/json"
	"net/http"
	"time"

	"portier/service/sources/configplan"
	"portier/service/sources/domain"
)

// configRoutes registers the config export/import/plan/apply endpoints. These
// are exact static paths; a wrong method or unknown /api/config/... subpath does
// not match here and is routed (chi NotFound/MethodNotAllowed → writeAPINotFound)
// to the generic /api 404 envelope, never a 405.
func (h *Handler) configRoutes() []modularRoute {
	return []modularRoute{
		{method: http.MethodGet, path: "/api/config/export", handler: h.handleConfigExport},
		{method: http.MethodPost, path: "/api/config/import", handler: h.importConfig},
		{method: http.MethodPost, path: "/api/config/plan", handler: h.configPlan},
		{method: http.MethodPost, path: "/api/config/apply", handler: h.configApply},
	}
}

// handleConfigExport returns the exported config snapshot. Read-only, no request
// input.
func (h *Handler) handleConfigExport(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.app.Manager.ExportConfig())
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
		CurrentRules: h.app.Manager.ListRules(),
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

	result, err := h.app.Manager.ImportConfig(domain.ExportedConfig{
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
	writeJSON(w, http.StatusOK, map[string]any{"result": result, "rules": rulesToResponses(h.app.Manager.ListRules())})
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
		CurrentRules: h.app.Manager.ListRules(),
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
		result, err := h.app.Manager.ImportConfig(domain.ExportedConfig{
			Version: "1", Rules: applyResult.Rules,
		}, "replace")
		if err != nil {
			writeInternalError(w)
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

func applyImportErrorResponse(plan configplan.Response, errs []string, appliedAt string, zeroCounts map[string]int) applyResponse {
	for _, msg := range errs {
		plan.Errors = append(plan.Errors, configplan.PlanError{Code: "IMPORT_ERROR", Message: msg})
	}
	plan.Summary.HasErrors = true
	return applyResponse{
		Ok: false, DryRun: false, AppliedAt: appliedAt, Plan: plan, Applied: zeroCounts,
	}
}
