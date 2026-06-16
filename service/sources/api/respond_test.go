package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"portier/service/sources/manager"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]any{"name": "Portier", "ok": true})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["name"] != "Portier" || body["ok"] != true {
		t.Fatalf("body = %v, want {name:Portier, ok:true}", body)
	}
}

func TestDecodeRequest_ValidJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/forwards", strings.NewReader(`{"name":"R1"}`))

	var target struct {
		Name string `json:"name"`
	}
	if ok := decodeRequest(rec, req, &target); !ok {
		t.Fatalf("decodeRequest returned false for valid JSON; body=%s", rec.Body.String())
	}
	if target.Name != "R1" {
		t.Fatalf("target.Name = %q, want R1", target.Name)
	}
}

func TestDecodeRequest_MalformedJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/forwards", strings.NewReader(`{not json`))

	var target map[string]any
	if ok := decodeRequest(rec, req, &target); ok {
		t.Fatal("decodeRequest returned true for malformed JSON")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertErrorEnvelope(t, rec)
}

func TestDecodeRequest_EmptyBody(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/forwards", strings.NewReader(""))

	var target map[string]any
	if ok := decodeRequest(rec, req, &target); ok {
		t.Fatal("decodeRequest returned true for empty body")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	errs := assertErrorEnvelope(t, rec)
	if len(errs) != 1 || errs[0] != "request body must be JSON." {
		t.Fatalf("errors = %v, want [request body must be JSON.]", errs)
	}
}

func TestReadBody_EnforcesSizeLimit(t *testing.T) {
	// A body larger than the 1 MB limit must be truncated to exactly 1 MB.
	oversized := strings.Repeat("a", 1_000_050)
	req := httptest.NewRequest(http.MethodPost, "/api/forwards", strings.NewReader(oversized))

	raw, err := readBody(req)
	if err != nil {
		t.Fatalf("readBody: %v", err)
	}
	if len(raw) != 1_000_000 {
		t.Fatalf("readBody returned %d bytes, want 1000000 (1 MB limit)", len(raw))
	}
}

func TestWriteManagerError_Mappings(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantErrors []string
	}{
		{
			name:       "validation -> 400",
			err:        manager.ValidationError{Errors: []string{"name is required.", "listenPort is invalid."}},
			wantStatus: http.StatusBadRequest,
			wantErrors: []string{"name is required.", "listenPort is invalid."},
		},
		{
			name:       "conflict -> 409",
			err:        manager.ConflictError{Message: "duplicate listen binding."},
			wantStatus: http.StatusConflict,
			wantErrors: []string{"duplicate listen binding."},
		},
		{
			name:       "not found -> 404",
			err:        manager.NotFoundError{Message: "rule abc was not found."},
			wantStatus: http.StatusNotFound,
			wantErrors: []string{"rule abc was not found."},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeManagerError(rec, tc.err)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("content-type = %q, want application/json", ct)
			}
			errs := assertErrorEnvelope(t, rec)
			if len(errs) != len(tc.wantErrors) {
				t.Fatalf("errors = %v, want %v", errs, tc.wantErrors)
			}
			for i, want := range tc.wantErrors {
				if errs[i] != want {
					t.Fatalf("errors[%d] = %q, want %q", i, errs[i], want)
				}
			}
		})
	}
}

// assertErrorEnvelope decodes the recorded body as {"errors": [...]} and returns
// the messages, failing if the shape is wrong.
func assertErrorEnvelope(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	var body struct {
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error envelope: %v (body=%s)", err, rec.Body.String())
	}
	return body.Errors
}
