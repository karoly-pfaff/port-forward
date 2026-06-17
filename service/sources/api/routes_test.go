package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The migrated feature routes are served through the chi-backed API router
// (v1.15 Slice 10); everything else falls through (chi NotFound /
// MethodNotAllowed) to the legacy ordered serveLegacyAPI dispatch. These tests
// pin the chi integration mechanics — the registered route set, the
// 404-not-405 method-mismatch behavior, and the legacy fall-through for
// unmigrated routes — complementing the per-endpoint behavior tests and the
// Slice 1 router dispatch contract (which drives ServeHTTP end to end).

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

// TestAPIRouterServesMigratedRoutes confirms each migrated route is matched and
// served by the chi router (not the legacy 404 fallback).
func TestAPIRouterServesMigratedRoutes(t *testing.T) {
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
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.status {
				t.Fatalf("%s %s: status = %d, want %d", tc.method, tc.path, rec.Code, tc.status)
			}
			if isNotFoundEnvelope(t, rec) {
				t.Fatalf("%s %s: served the legacy 404 envelope, want the migrated handler", tc.method, tc.path)
			}
		})
	}
}

// TestAPIRouterMethodMismatchReturns404Envelope pins that a wrong method on a
// migrated path returns the generic /api 404 JSON envelope, NOT a 405. chi's
// MethodNotAllowed is delegated to serveLegacyAPI, which falls through to the
// envelope when no legacy route owns the path.
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
		// Forwards write/lifecycle/group routes: a wrong method on a known path
		// must still produce the generic envelope (chi MethodNotAllowed → legacy).
		{http.MethodPut, "/api/forwards/abc"},              // no PUT on {id}
		{http.MethodGet, "/api/forwards/reorder"},          // reorder is POST-only
		{http.MethodGet, "/api/forwards/abc/start"},        // lifecycle is POST-only
		{http.MethodGet, "/api/forwards/groups/web/start"}, // group action is POST-only
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
// NotFound) is delegated to the legacy dispatch and ends in the 404 envelope.
func TestAPIRouterUnknownPathReturns404Envelope(t *testing.T) {
	h := newTestHandler(t, "", "missing")

	paths := []string{
		"/api/nope",
		"/api/status/extra",
		"/api/connections/extra",
		"/api/activity/extra",
		"/api/forwards/extra/extra",
		"/api/forwards/abc/restart", // unknown {id} subaction
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

// TestAPIRouterConfigRoutesUseLegacy confirms the still-unmigrated config routes
// are reached through the chi NotFound fallback (they are not registered on chi).
// We assert the request reaches the legacy handler by checking a route-specific
// outcome rather than the generic 404 envelope.
func TestAPIRouterConfigRoutesUseLegacy(t *testing.T) {
	h := newTestHandler(t, "", "missing")

	// GET /api/config/export is served (200), not the generic 404 — config routes
	// still flow through serveLegacyAPI.
	exportRec := httptest.NewRecorder()
	h.ServeHTTP(exportRec, httptest.NewRequest(http.MethodGet, "/api/config/export", nil))
	if exportRec.Code != http.StatusOK {
		t.Fatalf("GET /api/config/export: status = %d, want 200 (legacy export)", exportRec.Code)
	}

	// POST /api/config/plan with an empty body reaches the legacy configPlan, which
	// rejects it with 400 (not the generic route-not-found envelope).
	planRec := httptest.NewRecorder()
	h.ServeHTTP(planRec, httptest.NewRequest(http.MethodPost, "/api/config/plan", nil))
	if planRec.Code != http.StatusBadRequest || isNotFoundEnvelope(t, planRec) {
		t.Fatalf("POST /api/config/plan: status = %d (envelope=%v), want 400 from legacy configPlan", planRec.Code, isNotFoundEnvelope(t, planRec))
	}
}

// TestAPIRouterForwardsPrecedence proves the chi forwards routes do not misroute
// reorder or group actions as an {id}, and that the write/lifecycle routes reach
// their handlers (not the legacy 404 fallback). The full CRUD/lifecycle/group
// response bodies are covered by api_test.go / group_test.go; here we only lock
// the routing decisions.
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
