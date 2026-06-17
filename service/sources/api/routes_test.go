package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Every API route is served through the chi-backed API router; anything chi
// cannot match (chi NotFound / MethodNotAllowed) is routed to writeAPINotFound,
// the generic /api 404 envelope. These tests pin the chi integration mechanics —
// the registered route set, the 404-not-405 method-mismatch behavior, and the
// unknown-path fallback — complementing the per-endpoint behavior tests and the
// router dispatch contract (router_contract_test.go, which drives ServeHTTP end
// to end).

// TestModularRoutesRegistered asserts the exact set of routes mounted onto the
// chi router. Each entry is a (method, path) pair; /api/activity intentionally
// appears twice (GET + DELETE) — the first multi-method same-path module.
func TestModularRoutesRegistered(t *testing.T) {
	h := newTestHandler(t, "", "missing")

	want := map[string]bool{
		"GET /api/health":                         true,
		"GET /api/runtime":                        true,
		"GET /api/ports/advisory":                 true,
		"GET /api/activity":                       true,
		"DELETE /api/activity":                    true,
		"GET /api/status":                         true,
		"GET /api/connections":                    true,
		"GET /api/forwards":                       true,
		"POST /api/forwards":                      true,
		"POST /api/forwards/reorder":              true,
		"PATCH /api/forwards/{id}":                true,
		"DELETE /api/forwards/{id}":               true,
		"POST /api/forwards/{id}/start":           true,
		"POST /api/forwards/{id}/stop":            true,
		"POST /api/forwards/{id}/diagnose":        true,
		"POST /api/forwards/groups/{group}/start": true,
		"POST /api/forwards/groups/{group}/stop":  true,
		"GET /api/config/export":                  true,
		"POST /api/config/import":                 true,
		"POST /api/config/plan":                   true,
		"POST /api/config/apply":                  true,
	}
	got := map[string]bool{}
	for _, route := range h.modularRoutes() {
		got[route.method+" "+route.path] = true
	}

	if len(got) != len(want) {
		t.Fatalf("modular routes = %d (%v), want %d", len(got), got, len(want))
	}
	for key := range want {
		if !got[key] {
			t.Fatalf("missing modular route %q (have %v)", key, got)
		}
	}
}

// isNotFoundEnvelope reports whether the recorded response is the generic /api
// 404 JSON envelope (the contract for an unmatched route or a method mismatch).
func isNotFoundEnvelope(t *testing.T, rec *httptest.ResponseRecorder) bool {
	t.Helper()
	if rec.Code != http.StatusNotFound {
		return false
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		return false
	}
	var body struct {
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		return false
	}
	return len(body.Errors) == 1 && body.Errors[0] == notFoundMessage
}

// TestAPIRouterServesRegisteredRoutes confirms each registered route is matched
// and served by the chi router (not the 404 fallback).
func TestAPIRouterServesRegisteredRoutes(t *testing.T) {
	h := newTestHandler(t, "", "missing")

	cases := []struct {
		method string
		path   string
		status int
	}{
		{http.MethodGet, "/api/health", http.StatusOK},
		{http.MethodGet, "/api/runtime", http.StatusOK},
		{http.MethodGet, "/api/ports/advisory", http.StatusBadRequest}, // missing args, but reached the handler
		{http.MethodGet, "/api/activity", http.StatusOK},
		{http.MethodDelete, "/api/activity", http.StatusNoContent},
		{http.MethodGet, "/api/status", http.StatusOK},
		{http.MethodGet, "/api/connections", http.StatusOK},
		{http.MethodGet, "/api/forwards", http.StatusOK},
		{http.MethodGet, "/api/config/export", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.status {
				t.Fatalf("%s %s: status = %d, want %d", tc.method, tc.path, rec.Code, tc.status)
			}
			if isNotFoundEnvelope(t, rec) {
				t.Fatalf("%s %s: served the 404 fallback, want the registered handler", tc.method, tc.path)
			}
		})
	}
}

// TestAPIRouterMethodMismatchReturns404Envelope pins that a wrong method on a
// known path returns the generic /api 404 JSON envelope, NOT a 405. chi's
// MethodNotAllowed is routed to writeAPINotFound, which emits the envelope.
func TestAPIRouterMethodMismatchReturns404Envelope(t *testing.T) {
	h := newTestHandler(t, "", "missing")

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/health"},
		{http.MethodDelete, "/api/runtime"},
		{http.MethodPut, "/api/ports/advisory"},
		{http.MethodPost, "/api/activity"},
		{http.MethodPost, "/api/status"},
		{http.MethodPost, "/api/connections"},
		// Forwards/config routes: a wrong method on a known path must still produce
		// the generic envelope (chi MethodNotAllowed → writeAPINotFound, never 405).
		{http.MethodPut, "/api/forwards/abc"},              // no PUT on {id}
		{http.MethodGet, "/api/forwards/reorder"},          // reorder is POST-only
		{http.MethodGet, "/api/forwards/abc/start"},        // lifecycle is POST-only
		{http.MethodGet, "/api/forwards/groups/web/start"}, // group action is POST-only
		{http.MethodPut, "/api/config/export"},             // export is GET-only
		{http.MethodGet, "/api/config/import"},             // import is POST-only
		{http.MethodGet, "/api/config/apply"},              // apply is POST-only
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if !isNotFoundEnvelope(t, rec) {
				t.Fatalf("%s %s: status = %d, want the 404 envelope (never 405)", tc.method, tc.path, rec.Code)
			}
		})
	}
}

// TestAPIRouterUnknownPathReturns404Envelope pins that an unknown API path (chi
// NotFound) is routed to writeAPINotFound and ends in the 404 envelope.
func TestAPIRouterUnknownPathReturns404Envelope(t *testing.T) {
	h := newTestHandler(t, "", "missing")

	paths := []string{
		"/api/nope",
		"/api/status/extra",
		"/api/connections/extra",
		"/api/activity/extra",
		"/api/forwards/extra/extra",
		"/api/forwards/abc/restart", // unknown {id} subaction
		"/api/config/export/extra",  // exact config route is not a prefix
		"/api/config/unknown",       // unknown config subpath
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if !isNotFoundEnvelope(t, rec) {
				t.Fatalf("GET %s: status = %d, want the 404 envelope", path, rec.Code)
			}
		})
	}
}

// TestAPIRouterConfigRoutes confirms the config routes are served by chi. The
// full export/import/plan/apply response bodies are covered by api_test.go; here
// we only lock the routing decisions.
func TestAPIRouterConfigRoutes(t *testing.T) {
	h := newTestHandler(t, "", "missing")

	// GET /api/config/export is served (200), not the generic 404.
	exportRec := httptest.NewRecorder()
	h.ServeHTTP(exportRec, httptest.NewRequest(http.MethodGet, "/api/config/export", nil))
	if exportRec.Code != http.StatusOK || isNotFoundEnvelope(t, exportRec) {
		t.Fatalf("GET /api/config/export: status = %d, want 200 (chi export)", exportRec.Code)
	}

	// POST /api/config/plan with an empty body reaches configPlan, which rejects it
	// with 400 (not the generic route-not-found envelope).
	planRec := httptest.NewRecorder()
	h.ServeHTTP(planRec, httptest.NewRequest(http.MethodPost, "/api/config/plan", nil))
	if planRec.Code != http.StatusBadRequest || isNotFoundEnvelope(t, planRec) {
		t.Fatalf("POST /api/config/plan: status = %d (envelope=%v), want 400 from configPlan", planRec.Code, isNotFoundEnvelope(t, planRec))
	}

	// POST /api/config/import with an empty body reaches importConfig (400 on the
	// missing-JSON body), proving it is chi-owned and reachable.
	importRec := httptest.NewRecorder()
	h.ServeHTTP(importRec, httptest.NewRequest(http.MethodPost, "/api/config/import", nil))
	if importRec.Code != http.StatusBadRequest || isNotFoundEnvelope(t, importRec) {
		t.Fatalf("POST /api/config/import: status = %d (envelope=%v), want 400 from importConfig", importRec.Code, isNotFoundEnvelope(t, importRec))
	}

	// POST /api/config/apply with an empty body reaches configApply (400 on the
	// missing-JSON body).
	applyRec := httptest.NewRecorder()
	h.ServeHTTP(applyRec, httptest.NewRequest(http.MethodPost, "/api/config/apply", nil))
	if applyRec.Code != http.StatusBadRequest || isNotFoundEnvelope(t, applyRec) {
		t.Fatalf("POST /api/config/apply: status = %d (envelope=%v), want 400 from configApply", applyRec.Code, isNotFoundEnvelope(t, applyRec))
	}
}

// TestAPIRouterForwardsPrecedence proves the chi forwards routes do not misroute
// reorder or group actions as an {id}, and that the write/lifecycle routes reach
// their handlers (not the 404 fallback). The full CRUD/lifecycle/group response
// bodies are covered by api_test.go / group_test.go; here we only lock the
// routing decisions.
func TestAPIRouterForwardsPrecedence(t *testing.T) {
	h := newTestHandler(t, "", "missing")

	// POST /api/forwards (create) is now chi-owned; an empty body is rejected with
	// 400 by createForward (proving GET /api/forwards does not shadow the POST).
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/api/forwards", nil))
	if createRec.Code != http.StatusBadRequest || isNotFoundEnvelope(t, createRec) {
		t.Fatalf("POST /api/forwards: status = %d, want 400 from createForward (not misrouted)", createRec.Code)
	}

	// POST /api/forwards/reorder reaches reorderForwards (400 on a missing ids
	// body) — the static reorder route wins over the {id} param route, so reorder
	// is NOT treated as a rule id.
	reorderRec := httptest.NewRecorder()
	h.ServeHTTP(reorderRec, httptest.NewRequest(http.MethodPost, "/api/forwards/reorder", nil))
	if reorderRec.Code != http.StatusBadRequest || isNotFoundEnvelope(t, reorderRec) {
		t.Fatalf("POST /api/forwards/reorder: status = %d, want 400 from reorderForwards (not misrouted as id)", reorderRec.Code)
	}

	// POST /api/forwards/groups/ghost/start reaches the group handler (the static
	// groups route wins over {id}); an unknown group returns the group-specific
	// 404 message, NOT the generic route-not-found envelope and NOT misrouted.
	groupRec := httptest.NewRecorder()
	h.ServeHTTP(groupRec, httptest.NewRequest(http.MethodPost, "/api/forwards/groups/ghost/start", nil))
	if groupRec.Code != http.StatusNotFound || isNotFoundEnvelope(t, groupRec) {
		t.Fatalf("POST /api/forwards/groups/ghost/start: status = %d (envelope=%v), want a group-specific 404 (not misrouted as id)", groupRec.Code, isNotFoundEnvelope(t, groupRec))
	}
}
