package api

// Coverage Slice D — high-value API error paths. These exercise request
// rejection, body-read failures, the generic manager-error → 500 mapping, and
// the diagnostic bind helpers. api_test.go is a white-box (`package api`) test,
// so the unexported handlers/helpers are called directly where that is the
// clearest way to assert behavior; the persist-failure cases go through real
// HTTP so the status code is the asserted contract.

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"portier/service/sources/activity"
	"portier/service/sources/app"
	"portier/service/sources/config"
	"portier/service/sources/manager"
	"portier/service/sources/options"
)

// errReader is an io.Reader that always fails, simulating a client that
// disconnects mid-body so io.ReadAll returns an error.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("simulated read failure") }

// persistFailingHandler builds a Handler whose manager persists to a path under
// a regular file, so any Save (os.MkdirAll) fails with a generic *PathError —
// the clean, cross-platform way to drive the manager's non-typed error path
// without a production seam. NewWithStore does not call Load, so construction
// succeeds and only mutations fail.
func persistFailingHandler(t *testing.T) *Handler {
	t.Helper()
	blockFile := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(blockFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write block file: %v", err)
	}
	store := config.NewStore(filepath.Join(blockFile, "forwards.json"))
	m, err := manager.NewWithStore(&store, nil)
	if err != nil {
		t.Fatalf("NewWithStore: %v", err)
	}
	m.SetActivityStore(&activity.Store{})
	return NewHandler(app.New(m, options.Options{Host: "127.0.0.1", Port: 47831}, time.Now(), "test"))
}

const validRuleBody = `{"name":"R1","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48088,"targetHost":"127.0.0.1","targetPort":9000,"enabled":false}`

// --- readBody ---

func TestReadBody_EmptyBodyRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/forwards", strings.NewReader(""))
	if _, err := readBody(req); err == nil {
		t.Fatal("expected error for empty body")
	}
}

func TestReadBody_ReadErrorPropagates(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/forwards", errReader{})
	if _, err := readBody(req); err == nil {
		t.Fatal("expected error from failing body reader")
	}
}

// A request whose body fails to read must surface as 400 through the handler
// (covers decodeRequest's readBody-error branch end to end).
func TestCreateForward_BodyReadError_Returns400(t *testing.T) {
	h := newTestHandler(t, "", "missing")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/forwards", errReader{})
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// configApply reads the body directly; a failing reader must yield 400.
func TestConfigApply_BodyReadError_Returns400(t *testing.T) {
	h := newTestHandler(t, "", "missing")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/config/apply", errReader{})
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// --- decodeRequest malformed JSON ---

func TestCreateForward_MalformedJSON_Returns400(t *testing.T) {
	h := newTestHandler(t, "", "missing")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/forwards", strings.NewReader("{not json"))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// --- generic manager error → 500 (writeManagerError default + createForward) ---

func TestWriteManagerError_GenericErrorReturns500(t *testing.T) {
	rec := httptest.NewRecorder()
	writeManagerError(rec, errors.New("boom"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	// S-1: the generic 500 must redact the internal error, not echo it.
	if strings.Contains(body, "boom") {
		t.Fatalf("generic 500 must not echo the internal error, got: %s", body)
	}
	if !strings.Contains(body, internalErrorMessage) {
		t.Fatalf("generic 500 should return %q, got: %s", internalErrorMessage, body)
	}
}

func TestCreateForward_PersistFailureReturns500(t *testing.T) {
	srv := httptest.NewServer(persistFailingHandler(t))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/api/forwards", "application/json", strings.NewReader(validRuleBody))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	assertRedacted500(t, resp)
}

// assertRedacted500 verifies an unexpected 500 returns the fixed message and does
// not leak the internal *PathError text (the temp path injected by persistFailingHandler).
func assertRedacted500(t *testing.T, resp *http.Response) {
	t.Helper()
	raw, _ := io.ReadAll(resp.Body)
	body := string(raw)
	if strings.Contains(body, "notadir") {
		t.Fatalf("500 body leaked the internal path/error, got: %s", body)
	}
	if !strings.Contains(body, internalErrorMessage) {
		t.Fatalf("500 body should be %q, got: %s", internalErrorMessage, body)
	}
}

// configApply with drift must surface a manager ImportConfig persist failure as
// 500 (covers configApply's ImportConfig-error branch).
func TestConfigApply_ImportFailureReturns500(t *testing.T) {
	srv := httptest.NewServer(persistFailingHandler(t))
	defer srv.Close()
	body := `{"desired":{"rules":[{"name":"R1","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48089,"targetHost":"127.0.0.1","targetPort":9000,"enabled":false}]}}`
	resp, err := http.Post(srv.URL+"/api/config/apply", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	assertRedacted500(t, resp)
}

// --- updateForward error branches ---

func TestUpdateForward_UnknownRuleReturns404(t *testing.T) {
	h := newTestHandler(t, "", "missing")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/forwards/does-not-exist", strings.NewReader(`{"name":"X"}`))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestUpdateForward_BodyReadError_Returns400(t *testing.T) {
	h := newTestHandler(t, "", "missing")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/forwards/some-id", errReader{})
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// --- importConfig malformed body ---

func TestImportConfig_MalformedBody_Returns400(t *testing.T) {
	h := newTestHandler(t, "", "missing")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader("{not json"))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// --- NewHandler option defaults ---

func TestNewHandler_AppliesDefaults(t *testing.T) {
	// app.New applies the defaults (nil Manager, zero StartedAt, empty Version);
	// NewHandler exposes them through the app container (app.App is the single
	// dependency container — there is no separate manager bridge).
	h := NewHandler(app.New(nil, options.Options{}, time.Time{}, ""))
	if h.app.Manager == nil {
		t.Fatal("nil Manager should default to a fresh manager")
	}
	if h.app.Version == "" {
		t.Fatal("empty Version should default to the build version")
	}
	if h.app.StartedAt.IsZero() {
		t.Fatal("zero StartedAt should default to time.Now()")
	}
}

// --- diagnostic bind helpers ---

func TestTryTCPBind_SuccessAndFailure(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()
	occupied := ln.Addr().String()

	if status, _ := tryTCPBind(occupied); status != "fail" {
		t.Fatalf("bind to occupied %s = %q, want fail", occupied, status)
	}
	if status, _ := tryTCPBind("127.0.0.1:0"); status != "pass" {
		t.Fatalf("bind to ephemeral = %q, want pass", status)
	}
}

func TestTryUDPBind_SuccessAndFailure(t *testing.T) {
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve udp port: %v", err)
	}
	defer pc.Close()
	occupied := pc.LocalAddr().String()

	if status, _ := tryUDPBind(occupied); status != "fail" {
		t.Fatalf("udp bind to occupied %s = %q, want fail", occupied, status)
	}
	if status, _ := tryUDPBind("127.0.0.1:0"); status != "pass" {
		t.Fatalf("udp bind to ephemeral = %q, want pass", status)
	}
}

// readBody must enforce the 1 MB limit boundary without error for an exactly
// readable body (guards the LimitReader path).
func TestReadBody_AcceptsValidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/forwards", strings.NewReader(validRuleBody))
	raw, err := readBody(req)
	if err != nil {
		t.Fatalf("readBody valid: %v", err)
	}
	if !strings.Contains(string(raw), "R1") {
		t.Fatalf("body not read back: %s", raw)
	}
}
