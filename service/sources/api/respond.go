package api

// Shared HTTP request/response/error plumbing for the service API layer. These
// helpers are used by the (currently monolithic) serveAPI handlers and by the
// migrated feature route modules; keeping them in one small file lets future
// feature route files reuse them without depending on api.go's handler bodies.
// Behavior here is contract-covered — preserve status codes, body strings, the
// {"errors": [...]} envelope, content type, the request body size limit, and the
// manager-error → status mapping exactly.

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"portier/service/sources/manager"
)

// writeJSON writes body as JSON with the given status and the JSON content type.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// decodeRequest reads and JSON-decodes the request body into target. On a read
// or decode failure it writes a 400 {"errors": [...]} envelope and returns false.
func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	raw, err := readBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {err.Error()}})
		return false
	}
	if err := json.Unmarshal(raw, target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {err.Error()}})
		return false
	}
	return true
}

// readBody reads the request body up to a 1 MB limit and rejects an empty body.
func readBody(r *http.Request) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1_000_000))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, errors.New("request body must be JSON.")
	}
	return raw, nil
}

// writeManagerError maps a manager error to its HTTP status and {"errors": [...]}
// envelope: validation → 400, conflict → 409, not found → 404, and any other
// (unexpected) error → 500 with only its message (no raw internal leakage).
func writeManagerError(w http.ResponseWriter, err error) {
	var validationError manager.ValidationError
	if errors.As(err, &validationError) {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": validationError.Errors})
		return
	}

	var conflictError manager.ConflictError
	if errors.As(err, &conflictError) {
		writeJSON(w, http.StatusConflict, map[string][]string{"errors": {conflictError.Message}})
		return
	}

	var notFoundError manager.NotFoundError
	if errors.As(err, &notFoundError) {
		writeJSON(w, http.StatusNotFound, map[string][]string{"errors": {notFoundError.Message}})
		return
	}

	writeJSON(w, http.StatusInternalServerError, map[string][]string{"errors": {err.Error()}})
}
