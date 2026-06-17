package api

// Go service API inventory — the declared HTTP API surface of the Go runtime,
// compared against the canonical server-generated OpenAPI artifact
// (server/build/api/openapi.json) to catch drift as the v1.15 router
// modularization moves handlers around. The NestJS/server OpenAPI build artifact
// stays the single source of truth; this is a read-only drift gate, not a
// competing API document.
//
// This lives in a _test.go file deliberately: it is a dev/CI validation concern,
// so it adds zero code to the production service binary. The pure comparison
// logic below is exercised by the unit tests in this file under ordinary
// `go test`; the actual cross-artifact assertion (reading the real OpenAPI file)
// lives in openapi_inventory_test.go behind the `openapi_inventory` build tag so
// `go test ./...` never requires the server artifact.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// routeInventoryEntry is one declared Go HTTP route. Canonical routes are part of
// the canonical OpenAPI document; Additive routes are Go-only and must carry a
// reason (they are explicitly whitelisted as not appearing in OpenAPI).
type routeInventoryEntry struct {
	Method      string
	Path        string
	Statuses    []int
	Canonical   bool
	Additive    bool
	AdditiveWhy string
}

// canonical builds a canonical (OpenAPI-documented) route entry. Paths use the
// OpenAPI placeholder form ({id}, {group}) so they compare directly against the
// OpenAPI path keys, even though the Go router matches them by segment.
func canonical(method, path string, statuses ...int) routeInventoryEntry {
	return routeInventoryEntry{Method: method, Path: path, Statuses: statuses, Canonical: true}
}

// routeInventory is the declared Go API surface. Statuses are the *documented
// contract* status codes (matching the OpenAPI operation responses); the
// universal generic-500 no-leak path (an unexpected/persist failure surfaced by
// writeManagerError) is intentionally not enumerated here because OpenAPI does
// not model it for any operation — see the durable Slice-4 rule.
func routeInventory() []routeInventoryEntry {
	return []routeInventoryEntry{
		canonical("GET", "/api/forwards", 200),
		canonical("POST", "/api/forwards", 201, 400, 409),
		canonical("PATCH", "/api/forwards/{id}", 200, 400, 404, 409),
		canonical("DELETE", "/api/forwards/{id}", 204, 404),
		canonical("POST", "/api/forwards/{id}/start", 200, 404),
		canonical("POST", "/api/forwards/{id}/stop", 200, 404),
		canonical("POST", "/api/forwards/{id}/diagnose", 200, 404),
		canonical("POST", "/api/forwards/reorder", 200, 400, 404),
		canonical("POST", "/api/forwards/groups/{group}/start", 200, 400, 404),
		canonical("POST", "/api/forwards/groups/{group}/stop", 200, 400, 404),
		canonical("GET", "/api/status", 200),
		canonical("GET", "/api/runtime", 200),
		canonical("GET", "/api/activity", 200),
		canonical("DELETE", "/api/activity", 204),
		canonical("GET", "/api/config/export", 200),
		canonical("POST", "/api/config/plan", 200, 400),
		canonical("POST", "/api/config/import", 200, 400, 422),
		canonical("POST", "/api/config/apply", 200, 400),
		canonical("GET", "/api/connections", 200),
		canonical("GET", "/api/ports/advisory", 200, 400),
		{
			Method:      "GET",
			Path:        "/api/health",
			Statuses:    []int{200},
			Additive:    true,
			AdditiveWhy: "Go-native liveness probe; intentionally outside the canonical /api OpenAPI surface (NestJS documents /health instead).",
		},
	}
}

// apiRouteRef names a method+path with a reason; used for OpenAPI-only routes
// that the Go runtime intentionally does not serve.
type apiRouteRef struct {
	Method string
	Path   string
	Reason string
}

// openAPIOnlyPaths are routes the canonical OpenAPI documents but the Go runtime
// intentionally does not serve. /health is the NestJS liveness probe; the Go
// runtime's liveness lives at /api/health (the Go-only additive entry above).
func openAPIOnlyPaths() []apiRouteRef {
	return []apiRouteRef{
		{Method: "GET", Path: "/health", Reason: "NestJS liveness probe; the Go runtime exposes /api/health instead."},
	}
}

// apiSurface maps path -> uppercase method -> sorted, unique status codes.
type apiSurface map[string]map[string][]int

func httpMethods() map[string]bool {
	return map[string]bool{
		"GET": true, "POST": true, "PUT": true, "PATCH": true,
		"DELETE": true, "HEAD": true, "OPTIONS": true, "TRACE": true,
	}
}

// parseOpenAPISurface extracts the path/method/status surface from an OpenAPI 3
// document. Non-method path-item keys (e.g. "parameters") and non-numeric
// response keys (e.g. "default") are ignored.
func parseOpenAPISurface(doc []byte) (apiSurface, error) {
	var parsed struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(doc, &parsed); err != nil {
		return nil, fmt.Errorf("parse OpenAPI document: %w", err)
	}

	methods := httpMethods()
	surface := apiSurface{}
	for path, ops := range parsed.Paths {
		for method, raw := range ops {
			upper := strings.ToUpper(method)
			if !methods[upper] {
				continue
			}
			var op struct {
				Responses map[string]json.RawMessage `json:"responses"`
			}
			if err := json.Unmarshal(raw, &op); err != nil {
				return nil, fmt.Errorf("parse operation %s %s: %w", upper, path, err)
			}
			var statuses []int
			for code := range op.Responses {
				n, err := strconv.Atoi(code)
				if err != nil {
					continue
				}
				statuses = append(statuses, n)
			}
			statuses = sortedUniqueInts(statuses)
			if surface[path] == nil {
				surface[path] = map[string][]int{}
			}
			surface[path][upper] = statuses
		}
	}
	return surface, nil
}

func sortedUniqueInts(in []int) []int {
	seen := map[int]bool{}
	out := make([]int, 0, len(in))
	for _, n := range in {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}

func methodPathKey(method, path string) string {
	return strings.ToUpper(method) + " " + path
}

// inventoryConsistencyErrors validates the inventory is internally well-formed
// independently of OpenAPI: exactly one of Canonical/Additive, additive routes
// carry a reason, methods are known, paths are absolute, statuses are non-empty,
// and (method, path) pairs are unique.
func inventoryConsistencyErrors(entries []routeInventoryEntry) []string {
	methods := httpMethods()
	var errs []string
	seen := map[string]bool{}
	for _, e := range entries {
		key := methodPathKey(e.Method, e.Path)
		if e.Method != strings.ToUpper(e.Method) || !methods[e.Method] {
			errs = append(errs, fmt.Sprintf("%s: invalid HTTP method %q", key, e.Method))
		}
		if !strings.HasPrefix(e.Path, "/") {
			errs = append(errs, fmt.Sprintf("%s: path must be absolute", key))
		}
		if e.Canonical == e.Additive {
			errs = append(errs, fmt.Sprintf("%s: must be exactly one of Canonical or Additive", key))
		}
		if e.Additive && strings.TrimSpace(e.AdditiveWhy) == "" {
			errs = append(errs, fmt.Sprintf("%s: additive route must explain why (AdditiveWhy)", key))
		}
		if len(e.Statuses) == 0 {
			errs = append(errs, fmt.Sprintf("%s: at least one documented status is required", key))
		}
		if seen[key] {
			errs = append(errs, fmt.Sprintf("%s: duplicate inventory entry", key))
		}
		seen[key] = true
	}
	sort.Strings(errs)
	return errs
}

// compareInventory diffs the Go inventory against the OpenAPI surface and returns
// developer-friendly, order-independent messages (empty == in sync). It reports
// routes/methods missing in Go, extra in Go, status drift, mislabeled additive
// routes, and stale OpenAPI-only exclusions.
func compareInventory(entries []routeInventoryEntry, surface apiSurface, openAPIOnly []apiRouteRef) []string {
	goCanonical := map[string]map[string][]int{}
	goAdditive := map[string]bool{}
	for _, e := range entries {
		key := methodPathKey(e.Method, e.Path)
		if e.Additive {
			goAdditive[key] = true
			continue
		}
		if goCanonical[e.Path] == nil {
			goCanonical[e.Path] = map[string][]int{}
		}
		goCanonical[e.Path][strings.ToUpper(e.Method)] = sortedUniqueInts(e.Statuses)
	}

	excluded := map[string]bool{}
	for _, ref := range openAPIOnly {
		excluded[methodPathKey(ref.Method, ref.Path)] = true
	}

	var diffs []string

	// Every documented (non-excluded) OpenAPI operation must exist in Go, with
	// matching statuses.
	for path, ops := range surface {
		for method, oapiStatuses := range ops {
			key := methodPathKey(method, path)
			if excluded[key] {
				continue
			}
			goStatuses, ok := goCanonical[path][method]
			if !ok {
				diffs = append(diffs, "missing in Go: "+key+" (documented in OpenAPI)")
				continue
			}
			diffs = append(diffs, statusDiffs(key, oapiStatuses, goStatuses)...)
		}
	}

	// Every Go canonical operation must be documented in OpenAPI (and not be an
	// OpenAPI-only-excluded route).
	for path, ops := range goCanonical {
		for method := range ops {
			key := methodPathKey(method, path)
			if excluded[key] {
				diffs = append(diffs, "marked OpenAPI-only-excluded but also in Go canonical inventory: "+key)
				continue
			}
			if _, ok := surface[path][method]; !ok {
				diffs = append(diffs, "extra in Go: "+key+" (canonical but not documented in OpenAPI)")
			}
		}
	}

	// Additive (Go-only) routes must NOT be documented in OpenAPI; if they are,
	// they should be Canonical instead.
	for key := range goAdditive {
		method, path := splitMethodPath(key)
		if _, ok := surface[path][method]; ok {
			diffs = append(diffs, "additive route is documented in OpenAPI: "+key+" (mark it Canonical, not Additive)")
		}
	}

	// OpenAPI-only exclusions must still exist in OpenAPI (catch stale entries).
	for _, ref := range openAPIOnly {
		if _, ok := surface[ref.Path][strings.ToUpper(ref.Method)]; !ok {
			diffs = append(diffs, "stale OpenAPI-only exclusion: "+methodPathKey(ref.Method, ref.Path)+" is no longer in OpenAPI")
		}
	}

	sort.Strings(diffs)
	return diffs
}

func splitMethodPath(key string) (method, path string) {
	parts := strings.SplitN(key, " ", 2)
	if len(parts) != 2 {
		return key, ""
	}
	return parts[0], parts[1]
}

func statusDiffs(key string, oapi, goStatuses []int) []string {
	goSet := map[int]bool{}
	for _, s := range goStatuses {
		goSet[s] = true
	}
	oapiSet := map[int]bool{}
	for _, s := range oapi {
		oapiSet[s] = true
	}
	var diffs []string
	for _, s := range oapi {
		if !goSet[s] {
			diffs = append(diffs, fmt.Sprintf("status missing in Go: %s %d (documented in OpenAPI)", key, s))
		}
	}
	for _, s := range goStatuses {
		if !oapiSet[s] {
			diffs = append(diffs, fmt.Sprintf("extra status in Go: %s %d (not documented in OpenAPI)", key, s))
		}
	}
	return diffs
}

// findFileUpwards walks up from start looking for the relative path rel,
// returning the first match. It is the basis for locating the canonical OpenAPI
// artifact from a test working directory.
func findFileUpwards(start, rel string) (string, bool) {
	dir := start
	for {
		candidate := filepath.Join(dir, rel)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// canonicalOpenAPIRelPath is the canonical primary OpenAPI artifact location,
// relative to the repository root.
func canonicalOpenAPIRelPath() string {
	return filepath.Join("server", "build", "api", "openapi.json")
}

// locateCanonicalOpenAPI finds server/build/api/openapi.json at or above startDir,
// failing with an actionable message (never falling back to docs/api/openapi.json).
func locateCanonicalOpenAPI(startDir string) (string, error) {
	rel := canonicalOpenAPIRelPath()
	if p, ok := findFileUpwards(startDir, rel); ok {
		return p, nil
	}
	return "", fmt.Errorf("canonical OpenAPI artifact (%s) not found at or above %s; run `npm run generate:apidoc` first", rel, startDir)
}

// --- unit tests (pure logic; no artifact required) ---

func TestRouteInventoryIsConsistent(t *testing.T) {
	if errs := inventoryConsistencyErrors(routeInventory()); len(errs) > 0 {
		t.Fatalf("route inventory is inconsistent:\n  %s", strings.Join(errs, "\n  "))
	}
}

// TestRouteInventoryMatchesModularRoutes ties the static route inventory to the
// LIVE chi route registration (h.modularRoutes()): every registered route must
// have an inventory entry and vice versa, on the exact (method, path) pair. This
// is an always-on guard (ordinary `go test`, no build tag) so that adding or
// removing a chi route without updating the inventory fails immediately — before
// the OpenAPI cross-artifact gate. The chi {id}/{group} patterns already use the
// same placeholder form as the inventory's canonical paths, so they compare
// directly.
func TestRouteInventoryMatchesModularRoutes(t *testing.T) {
	h := newTestHandler(t, "", "missing")

	registered := map[string]bool{}
	for _, route := range h.modularRoutes() {
		registered[route.method+" "+route.path] = true
	}

	inventory := map[string]bool{}
	for _, entry := range routeInventory() {
		inventory[entry.Method+" "+entry.Path] = true
	}

	for key := range registered {
		if !inventory[key] {
			t.Errorf("chi route %q has no routeInventory() entry (update the inventory)", key)
		}
	}
	for key := range inventory {
		if !registered[key] {
			t.Errorf("routeInventory() entry %q is not registered on the chi router", key)
		}
	}
}

func TestInventoryConsistencyDetectsProblems(t *testing.T) {
	bad := []routeInventoryEntry{
		{Method: "get", Path: "api/x", Statuses: nil, Canonical: true, Additive: true}, // bad method, rel path, no status, both flags
		{Method: "GET", Path: "/api/y", Statuses: []int{200}, Additive: true},          // additive without reason
		{Method: "GET", Path: "/api/z", Statuses: []int{200}, Canonical: true},         // ok
		{Method: "GET", Path: "/api/z", Statuses: []int{200}, Canonical: true},         // duplicate
	}
	errs := inventoryConsistencyErrors(bad)
	if len(errs) == 0 {
		t.Fatal("expected consistency errors")
	}
	joined := strings.Join(errs, "\n")
	for _, want := range []string{"invalid HTTP method", "path must be absolute", "exactly one of Canonical or Additive", "additive route must explain", "at least one documented status", "duplicate inventory entry"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing error %q in:\n%s", want, joined)
		}
	}
}

func TestParseOpenAPISurface(t *testing.T) {
	doc := []byte(`{
      "paths": {
        "/api/forwards": {
          "get": { "responses": { "200": {} } },
          "post": { "responses": { "201": {}, "400": {}, "409": {} } },
          "parameters": []
        },
        "/api/config/import": {
          "post": { "responses": { "200": {}, "400": {}, "422": {}, "default": {} } }
        }
      }
    }`)
	surface, err := parseOpenAPISurface(doc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := surface["/api/forwards"]["GET"]; len(got) != 1 || got[0] != 200 {
		t.Fatalf("GET /api/forwards statuses = %v, want [200]", got)
	}
	if got := surface["/api/forwards"]["POST"]; !eqInts(got, []int{201, 400, 409}) {
		t.Fatalf("POST /api/forwards statuses = %v, want [201 400 409]", got)
	}
	// "parameters" is ignored; "default" (non-numeric) is dropped.
	if _, ok := surface["/api/forwards"]["PARAMETERS"]; ok {
		t.Fatal("non-method key 'parameters' should be ignored")
	}
	if got := surface["/api/config/import"]["POST"]; !eqInts(got, []int{200, 400, 422}) {
		t.Fatalf("POST /api/config/import statuses = %v, want [200 400 422]", got)
	}
}

func TestCompareInventory_Match(t *testing.T) {
	entries := []routeInventoryEntry{
		canonical("GET", "/api/status", 200),
		canonical("POST", "/api/forwards", 201, 400, 409),
		{Method: "GET", Path: "/api/health", Statuses: []int{200}, Additive: true, AdditiveWhy: "go-native"},
	}
	surface := apiSurface{
		"/api/status":   {"GET": {200}},
		"/api/forwards": {"POST": {201, 400, 409}},
		"/health":       {"GET": {200}},
	}
	excluded := []apiRouteRef{{Method: "GET", Path: "/health", Reason: "nest liveness"}}
	if diffs := compareInventory(entries, surface, excluded); len(diffs) != 0 {
		t.Fatalf("expected no diffs, got:\n  %s", strings.Join(diffs, "\n  "))
	}
}

func TestCompareInventory_MissingGoRoute(t *testing.T) {
	entries := []routeInventoryEntry{canonical("GET", "/api/status", 200)}
	surface := apiSurface{
		"/api/status":      {"GET": {200}},
		"/api/connections": {"GET": {200}},
	}
	diffs := compareInventory(entries, surface, nil)
	if !containsSubstr(diffs, "missing in Go: GET /api/connections") {
		t.Fatalf("expected missing-in-Go diff, got:\n  %s", strings.Join(diffs, "\n  "))
	}
}

func TestCompareInventory_ExtraGoRoute(t *testing.T) {
	entries := []routeInventoryEntry{
		canonical("GET", "/api/status", 200),
		canonical("GET", "/api/secret", 200), // canonical but not documented
	}
	surface := apiSurface{"/api/status": {"GET": {200}}}
	diffs := compareInventory(entries, surface, nil)
	if !containsSubstr(diffs, "extra in Go: GET /api/secret") {
		t.Fatalf("expected extra-in-Go diff, got:\n  %s", strings.Join(diffs, "\n  "))
	}
}

func TestCompareInventory_MethodMismatch(t *testing.T) {
	entries := []routeInventoryEntry{canonical("GET", "/api/x", 200)}
	surface := apiSurface{"/api/x": {"POST": {200}}}
	diffs := compareInventory(entries, surface, nil)
	if !containsSubstr(diffs, "missing in Go: POST /api/x") {
		t.Fatalf("expected missing POST diff, got:\n  %s", strings.Join(diffs, "\n  "))
	}
	if !containsSubstr(diffs, "extra in Go: GET /api/x") {
		t.Fatalf("expected extra GET diff, got:\n  %s", strings.Join(diffs, "\n  "))
	}
}

func TestCompareInventory_WhitelistedAdditiveAllowed(t *testing.T) {
	entries := []routeInventoryEntry{
		{Method: "GET", Path: "/api/health", Statuses: []int{200}, Additive: true, AdditiveWhy: "go-native"},
	}
	if diffs := compareInventory(entries, apiSurface{}, nil); len(diffs) != 0 {
		t.Fatalf("whitelisted additive route should not fail, got:\n  %s", strings.Join(diffs, "\n  "))
	}
}

func TestCompareInventory_UnwhitelistedGoOnlyFails(t *testing.T) {
	// A Go-only route NOT marked additive (Canonical) and absent from OpenAPI fails.
	entries := []routeInventoryEntry{canonical("GET", "/api/health", 200)}
	diffs := compareInventory(entries, apiSurface{}, nil)
	if !containsSubstr(diffs, "extra in Go: GET /api/health") {
		t.Fatalf("expected unwhitelisted Go-only route to fail, got:\n  %s", strings.Join(diffs, "\n  "))
	}
}

func TestCompareInventory_AdditiveMislabeled(t *testing.T) {
	entries := []routeInventoryEntry{
		{Method: "GET", Path: "/api/status", Statuses: []int{200}, Additive: true, AdditiveWhy: "x"},
	}
	surface := apiSurface{"/api/status": {"GET": {200}}}
	diffs := compareInventory(entries, surface, nil)
	if !containsSubstr(diffs, "additive route is documented in OpenAPI: GET /api/status") {
		t.Fatalf("expected mislabeled-additive diff, got:\n  %s", strings.Join(diffs, "\n  "))
	}
}

func TestCompareInventory_StatusMismatch(t *testing.T) {
	entries := []routeInventoryEntry{canonical("POST", "/api/forwards", 201, 400)} // missing 409, no extras
	surface := apiSurface{"/api/forwards": {"POST": {201, 400, 409}}}
	diffs := compareInventory(entries, surface, nil)
	if !containsSubstr(diffs, "status missing in Go: POST /api/forwards 409") {
		t.Fatalf("expected status-missing diff, got:\n  %s", strings.Join(diffs, "\n  "))
	}

	entries2 := []routeInventoryEntry{canonical("POST", "/api/forwards", 201, 400, 409, 500)} // extra 500
	diffs2 := compareInventory(entries2, surface, nil)
	if !containsSubstr(diffs2, "extra status in Go: POST /api/forwards 500") {
		t.Fatalf("expected extra-status diff, got:\n  %s", strings.Join(diffs2, "\n  "))
	}
}

func TestCompareInventory_OpenAPIOnlyExcluded(t *testing.T) {
	// /health documented in OpenAPI, not served by Go, but excluded -> no diff.
	entries := []routeInventoryEntry{canonical("GET", "/api/status", 200)}
	surface := apiSurface{
		"/api/status": {"GET": {200}},
		"/health":     {"GET": {200}},
	}
	excluded := []apiRouteRef{{Method: "GET", Path: "/health", Reason: "nest liveness"}}
	if diffs := compareInventory(entries, surface, excluded); len(diffs) != 0 {
		t.Fatalf("excluded OpenAPI-only route should not fail, got:\n  %s", strings.Join(diffs, "\n  "))
	}

	// A stale exclusion (not in OpenAPI) is reported.
	stale := []apiRouteRef{{Method: "GET", Path: "/gone", Reason: "x"}}
	diffs := compareInventory(entries, apiSurface{"/api/status": {"GET": {200}}}, stale)
	if !containsSubstr(diffs, "stale OpenAPI-only exclusion: GET /gone") {
		t.Fatalf("expected stale-exclusion diff, got:\n  %s", strings.Join(diffs, "\n  "))
	}
}

func TestLocateCanonicalOpenAPI(t *testing.T) {
	// Found: create the artifact in a nested tree and locate it from a subdir.
	root := t.TempDir()
	artifactDir := filepath.Join(root, "server", "build", "api")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	artifact := filepath.Join(artifactDir, "openapi.json")
	if err := os.WriteFile(artifact, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	startDir := filepath.Join(root, "service", "sources", "api")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start: %v", err)
	}
	got, err := locateCanonicalOpenAPI(startDir)
	if err != nil {
		t.Fatalf("locate: %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(artifact) {
		t.Fatalf("located %q, want %q", got, artifact)
	}

	// Missing: an isolated temp dir whose ancestors lack the artifact.
	_, err = locateCanonicalOpenAPI(t.TempDir())
	if err == nil {
		t.Fatal("expected error when artifact is missing")
	}
	if !strings.Contains(err.Error(), "npm run generate:apidoc") {
		t.Fatalf("error should point to `npm run generate:apidoc`, got: %v", err)
	}
}

// --- test helpers ---

func eqInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsSubstr(lines []string, want string) bool {
	for _, l := range lines {
		if strings.Contains(l, want) {
			return true
		}
	}
	return false
}
