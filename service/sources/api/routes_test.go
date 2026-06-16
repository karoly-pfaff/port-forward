package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The migrated health/runtime endpoints are served through the modular route
// table; everything else still flows through the legacy ordered serveAPI
// dispatch. These tests pin the skeleton itself (the route table + the
// exact-method dispatch that preserves 404-not-405), complementing the
// behavior tests (TestHealthEndpoint / TestRuntimeEndpoint) and the Slice 1
// router dispatch contract.

func TestModularRoutesRegistered(t *testing.T) {
	h := newTestHandler(t, "", "missing")

	// Each entry is a (method, path) pair; /api/activity intentionally appears
	// twice (GET + DELETE) — the first multi-method same-path modular route.
	want := map[string]bool{
		"GET /api/health":         true,
		"GET /api/runtime":        true,
		"GET /api/ports/advisory": true,
		"GET /api/activity":       true,
		"DELETE /api/activity":    true,
		"GET /api/status":         true,
		"GET /api/connections":    true,
	}
	got := map[string]bool{}
	for _, route := range h.routes {
		got[route.method+" "+route.path] = true
	}

	if len(h.routes) != len(want) {
		t.Fatalf("modular routes = %d (%v), want %d", len(h.routes), got, len(want))
	}
	for key := range want {
		if !got[key] {
			t.Fatalf("missing modular route %q (have %v)", key, got)
		}
	}
}

func TestDispatchModular(t *testing.T) {
	h := newTestHandler(t, "", "missing")

	cases := []struct {
		name    string
		method  string
		path    string
		handled bool
	}{
		{"health matched", http.MethodGet, "/api/health", true},
		{"runtime matched", http.MethodGet, "/api/runtime", true},
		{"ports advisory matched", http.MethodGet, "/api/ports/advisory", true},
		{"activity GET matched", http.MethodGet, "/api/activity", true},
		{"activity DELETE matched", http.MethodDelete, "/api/activity", true},
		{"status matched", http.MethodGet, "/api/status", true},
		{"connections matched", http.MethodGet, "/api/connections", true},
		// Wrong method must NOT be handled here — it falls through to the legacy
		// dispatch and its generic 404 envelope (404-not-405).
		{"health wrong method falls through", http.MethodPost, "/api/health", false},
		{"runtime wrong method falls through", http.MethodDelete, "/api/runtime", false},
		{"ports advisory wrong method falls through", http.MethodPost, "/api/ports/advisory", false},
		{"activity wrong method falls through", http.MethodPost, "/api/activity", false},
		{"activity subpath falls through", http.MethodGet, "/api/activity/extra", false},
		{"status wrong method falls through", http.MethodPost, "/api/status", false},
		{"status subpath falls through", http.MethodGet, "/api/status/extra", false},
		{"connections wrong method falls through", http.MethodPost, "/api/connections", false},
		{"connections subpath falls through", http.MethodGet, "/api/connections/extra", false},
		// A non-migrated path is never claimed by the modular table.
		{"forwards not migrated", http.MethodGet, "/api/forwards", false},
		{"config export not migrated", http.MethodGet, "/api/config/export", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if got := h.dispatchModular(rec, req); got != tc.handled {
				t.Fatalf("dispatchModular(%s %s) = %v, want %v", tc.method, tc.path, got, tc.handled)
			}
		})
	}
}
