package static

// Coverage Slice D — static asset serving. Covers ServeClient's "requested asset
// exists" branch (previously only the index.html SPA-fallback was exercised) and
// HasClient detection.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestServeClient_RejectsTraversalAttempt(t *testing.T) {
	dir := writeStaticDir(t)
	// A secret sibling of the static dir that must never be served.
	secret := filepath.Join(filepath.Dir(dir), "secret.txt")
	if err := os.WriteFile(secret, []byte("TOP-SECRET"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	for _, target := range []string{"/../secret.txt", "/..%2fsecret.txt", "/%2e%2e/secret.txt", "/foo/../../secret.txt"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, target, nil)
		ServeClient(rec, req, dir)

		// The secret outside dir must never be served, by any mechanism
		// (path.Clean containment, the within() guard, or ServeFile's own
		// ".." rejection).
		if body := rec.Body.String(); strings.Contains(body, "TOP-SECRET") {
			t.Fatalf("traversal %q leaked the secret file (body %q)", target, body)
		}
	}
}

func TestWithin(t *testing.T) {
	dir := filepath.Join("base", "web")
	inside := []string{
		filepath.Join(dir, "app.js"),
		filepath.Join(dir, "assets", "main.css"),
	}
	for _, c := range inside {
		if !within(dir, c) {
			t.Errorf("within(%q, %q) = false, want true", dir, c)
		}
	}
	outside := []string{
		filepath.Join("base", "secret.txt"),           // ../secret.txt
		filepath.Join(filepath.Dir(dir), "..", "etc"), // escapes above base
	}
	for _, c := range outside {
		if within(dir, c) {
			t.Errorf("within(%q, %q) = true, want false", dir, c)
		}
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
