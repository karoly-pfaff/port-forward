package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// These tests pin the routing contract through Handler.ServeHTTP end to end,
// capturing the order- and method-sensitive decisions that are easy to break if
// route registration changes, independently of the per-endpoint behavior already
// covered in api_test.go:
//
//   - a known path with an unsupported method returns the JSON 404 envelope
//     (NOT 405) — chi's default MethodNotAllowed would emit a 405, so this must
//     stay locked;
//   - route precedence: static routes (/api/forwards, /api/forwards/reorder, and
//     the /api/forwards/groups/{group}/... actions) win over the
//     /api/forwards/{id} param route, so reorder and group actions are never
//     matched as a rule id;
//   - every unmatched /api route (including bare /api) returns the JSON 404
//     envelope, while a non-API path with no static client returns a plain 404.
//
// All cases run against an empty in-memory manager (missing config), so no
// sockets are opened and routing is exercised in isolation.

const apiNotFoundBody = "API route was not found."

// routeCase is one method+path probe and its expected dispatched status.
type routeCase struct {
	name   string
	method string
	path   string
	status int
	// envelope is true when the response body must be the JSON {errors:[...]}
	// 404 envelope (the /api unmatched-route contract).
	envelope bool
}

func TestRouterDispatchContract(t *testing.T) {
	handler := newTestHandler(t, "", "missing")

	cases := []routeCase{
		// Matched read endpoints (success dispatch; bodies covered elsewhere).
		{"health", http.MethodGet, "/api/health", http.StatusOK, false},
		{"runtime", http.MethodGet, "/api/runtime", http.StatusOK, false},
		{"forwards list", http.MethodGet, "/api/forwards", http.StatusOK, false},
		{"status", http.MethodGet, "/api/status", http.StatusOK, false},
		{"connections", http.MethodGet, "/api/connections", http.StatusOK, false},
		{"config export", http.MethodGet, "/api/config/export", http.StatusOK, false},
		{"activity list", http.MethodGet, "/api/activity", http.StatusOK, false},
		{"activity clear", http.MethodDelete, "/api/activity", http.StatusNoContent, false},

		// Method-not-allowed on a known path -> JSON 404 envelope, never 405.
		{"health wrong method", http.MethodPost, "/api/health", http.StatusNotFound, true},
		{"runtime wrong method", http.MethodDelete, "/api/runtime", http.StatusNotFound, true},
		{"status wrong method", http.MethodPost, "/api/status", http.StatusNotFound, true},
		{"connections wrong method", http.MethodPost, "/api/connections", http.StatusNotFound, true},
		{"config export wrong method", http.MethodPost, "/api/config/export", http.StatusNotFound, true},
		{"activity wrong method", http.MethodPut, "/api/activity", http.StatusNotFound, true},

		// Route precedence: exact /api/forwards/reorder wins over the id prefix;
		// a wrong method on it falls through to the id prefix -> unknown id 404.
		{"reorder null ids", http.MethodPost, "/api/forwards/reorder", http.StatusBadRequest, false},
		{"reorder wrong method", http.MethodGet, "/api/forwards/reorder", http.StatusNotFound, true},

		// Group prefix is matched before the id prefix.
		{"group wrong method", http.MethodGet, "/api/forwards/groups/web/start", http.StatusNotFound, true},
		{"group unknown action", http.MethodPost, "/api/forwards/groups/web/restart", http.StatusNotFound, true},
		// A valid group with no matching rules is owned by serveGroupAction and
		// returns a group-specific 404 message (not the generic envelope); the
		// message is asserted by TestGroupNoMatch404. Here we only lock the
		// dispatch decision: it stays a 404, never a 405 or a fallthrough.
		{"group no match", http.MethodPost, "/api/forwards/groups/ghost/start", http.StatusNotFound, false},

		// Id prefix: unknown id sub-actions and methods.
		{"forward by id GET", http.MethodGet, "/api/forwards/abc", http.StatusNotFound, true},
		{"forward unknown subaction", http.MethodPost, "/api/forwards/abc/restart", http.StatusNotFound, true},
		{"forward by id empty", http.MethodGet, "/api/forwards/", http.StatusNotFound, true},

		// Unmatched /api routes (including bare /api) -> JSON 404 envelope.
		{"bare api", http.MethodGet, "/api", http.StatusNotFound, true},
		{"unknown api", http.MethodGet, "/api/nope", http.StatusNotFound, true},
		{"unknown nested api", http.MethodPost, "/api/config/nope", http.StatusNotFound, true},
		{"ports advisory missing args", http.MethodGet, "/api/ports/advisory", http.StatusBadRequest, false},

		// Non-API path with no static client -> plain http.NotFound (no envelope).
		{"non-api root", http.MethodGet, "/", http.StatusNotFound, false},
		{"non-api path", http.MethodGet, "/dashboard", http.StatusNotFound, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("%s %s: status = %d, want %d", tc.method, tc.path, rec.Code, tc.status)
			}

			if tc.envelope {
				if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
					t.Fatalf("%s %s: content-type = %q, want application/json", tc.method, tc.path, ct)
				}
				var body struct {
					Errors []string `json:"errors"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("%s %s: decode envelope: %v", tc.method, tc.path, err)
				}
				if len(body.Errors) != 1 || body.Errors[0] != apiNotFoundBody {
					t.Fatalf("%s %s: errors = %v, want [%q]", tc.method, tc.path, body.Errors, apiNotFoundBody)
				}
			}
		})
	}
}

// TestRouterStaticVsAPIBoundary pins that the static client only intercepts
// non-API routes: with a static client present, /api unmatched routes still
// return the JSON 404 envelope (never the SPA index), while a non-API route
// falls back to the served client.
func TestRouterStaticVsAPIBoundary(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<html><body>Portier UI</body></html>"), 0o600); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	handler := newTestHandler(t, staticDir, "missing")

	// Unmatched /api route still returns the JSON envelope, not index.html.
	apiRec := httptest.NewRecorder()
	handler.ServeHTTP(apiRec, httptest.NewRequest(http.MethodGet, "/api/nope", nil))
	if apiRec.Code != http.StatusNotFound {
		t.Fatalf("/api/nope status = %d, want 404", apiRec.Code)
	}
	if ct := apiRec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("/api/nope content-type = %q, want application/json", ct)
	}

	// Non-API route falls back to the served client (SPA index, 200).
	spaRec := httptest.NewRecorder()
	handler.ServeHTTP(spaRec, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if spaRec.Code != http.StatusOK {
		t.Fatalf("/dashboard status = %d, want 200", spaRec.Code)
	}
}
