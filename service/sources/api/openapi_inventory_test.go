//go:build openapi_inventory

// This cross-artifact gate is guarded by the `openapi_inventory` build tag so it
// does NOT run under ordinary `go test ./...` (which must not require the server
// OpenAPI artifact). Run it via `npm run validate:openapi:go`, which invokes
//
//	go -C service test -count=1 -tags openapi_inventory \
//	  -run TestGoAPIInventoryMatchesOpenAPI ./sources/api/
//
// after `npm run generate:apidoc` has produced server/build/api/openapi.json.
// The pure comparison logic it calls lives (and is unit-tested) in
// route_inventory_test.go.

package api

import (
	"os"
	"strings"
	"testing"
)

// TestGoAPIInventoryMatchesOpenAPI verifies the Go runtime's declared API surface
// (routeInventory) matches the canonical server-generated OpenAPI artifact. The
// NestJS/server OpenAPI document stays the single source of truth; this only
// reads it. It does not generate OpenAPI and never falls back to
// docs/api/openapi.json.
func TestGoAPIInventoryMatchesOpenAPI(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	artifact, err := locateCanonicalOpenAPI(cwd)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("read canonical OpenAPI %s: %v", artifact, err)
	}
	surface, err := parseOpenAPISurface(data)
	if err != nil {
		t.Fatalf("parse canonical OpenAPI %s: %v", artifact, err)
	}

	entries := routeInventory()
	if errs := inventoryConsistencyErrors(entries); len(errs) > 0 {
		t.Fatalf("Go route inventory is internally inconsistent:\n  %s", strings.Join(errs, "\n  "))
	}

	diffs := compareInventory(entries, surface, openAPIOnlyPaths())
	if len(diffs) > 0 {
		t.Fatalf(
			"Go API inventory drifted from the canonical OpenAPI artifact:\n  %s\n\n"+
				"Canonical artifact: %s\n"+
				"If the OpenAPI changed intentionally, update the Go handlers and the inventory in\n"+
				"service/sources/api/route_inventory_test.go to match. The OpenAPI document remains\n"+
				"the source of truth (regenerate with `npm run generate:apidoc`).",
			strings.Join(diffs, "\n  "), artifact)
	}
}
