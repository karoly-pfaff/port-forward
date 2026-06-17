package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"portier/service/sources/app"
	"portier/service/sources/configplan"
	"portier/service/sources/domain"
	"portier/service/sources/manager"
	"portier/service/sources/static"
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

func applyImportErrorResponse(plan configplan.Response, errs []string, appliedAt string, zeroCounts map[string]int) applyResponse {
	for _, msg := range errs {
		plan.Errors = append(plan.Errors, configplan.PlanError{Code: "IMPORT_ERROR", Message: msg})
	}
	plan.Summary.HasErrors = true
	return applyResponse{
		Ok: false, DryRun: false, AppliedAt: appliedAt, Plan: plan, Applied: zeroCounts,
	}
}
