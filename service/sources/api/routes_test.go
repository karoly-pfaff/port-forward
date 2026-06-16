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

	want := map[string]string{
		"/api/health":         http.MethodGet,
		"/api/runtime":        http.MethodGet,
		"/api/ports/advisory": http.MethodGet,
	}
	got := map[string]string{}
	for _, route := range h.routes {
		got[route.path] = route.method
	}

	if len(h.routes) != len(want) {
		t.Fatalf("modular routes = %d (%v), want %d", len(h.routes), got, len(want))
	}
	for path, method := range want {
		if got[path] != method {
			t.Fatalf("route %s = %q, want %q", path, got[path], method)
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
		// Wrong method must NOT be handled here — it falls through to the legacy
		// dispatch and its generic 404 envelope (404-not-405).
		{"health wrong method falls through", http.MethodPost, "/api/health", false},
		{"runtime wrong method falls through", http.MethodDelete, "/api/runtime", false},
		{"ports advisory wrong method falls through", http.MethodPost, "/api/ports/advisory", false},
		// A non-migrated path is never claimed by the modular table.
		{"status not migrated", http.MethodGet, "/api/status", false},
		{"forwards not migrated", http.MethodGet, "/api/forwards", false},
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
