package static

// Coverage Slice D — static asset serving. Covers ServeClient's "requested asset
// exists" branch (previously only the index.html SPA-fallback was exercised) and
// HasClient detection.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeStaticDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>index</html>"), 0o600); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log('app')"), 0o600); err != nil {
		t.Fatalf("write app.js: %v", err)
	}
	return dir
}

func TestServeClient_ServesExistingAsset(t *testing.T) {
	dir := writeStaticDir(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	ServeClient(rec, req, dir)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "console.log('app')" {
		t.Fatalf("served body = %q, want the asset contents", body)
	}
}

func TestServeClient_FallsBackToIndexForUnknownPath(t *testing.T) {
	dir := writeStaticDir(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/some/spa/route", nil)
	ServeClient(rec, req, dir)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "<html>index</html>" {
		t.Fatalf("SPA fallback body = %q, want index.html", body)
	}
}

func TestServeClient_RootServesIndex(t *testing.T) {
	dir := writeStaticDir(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ServeClient(rec, req, dir)

	if rec.Code != http.StatusOK || rec.Body.String() != "<html>index</html>" {
		t.Fatalf("root request did not serve index.html (code %d, body %q)", rec.Code, rec.Body.String())
	}
}

func TestHasClient(t *testing.T) {
	dir := writeStaticDir(t)
	if !HasClient(dir) {
		t.Fatal("HasClient should be true when index.html exists")
	}
	if HasClient(filepath.Join(dir, "does-not-exist")) {
		t.Fatal("HasClient should be false when index.html is absent")
	}
}
