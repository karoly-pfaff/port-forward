package api

import (
	"net/http"
	"strings"

	"portier/service/sources/app"
	"portier/service/sources/manager"
	"portier/service/sources/static"
)

const notFoundMessage = "API route was not found."

// Handler is the service HTTP entry point. It holds the explicit dependency
// container (app.App) and the chi API router (v1.15). As of Slice 12 every API
// route is served by the chi router; an unmatched request returns the generic
// /api 404 envelope via writeAPINotFound.
type Handler struct {
	app             *app.App
	staticAvailable bool

	// apiRouter is the chi router owning all API routes (v1.15 Slice 10–12). It
	// only ever serves requests under /api; its NotFound/MethodNotAllowed handlers
	// emit the generic /api 404 envelope (writeAPINotFound) — never a 405.
	apiRouter http.Handler

	// manager is the bridge the feature route handlers (forwards, config) use to
	// reach the rule manager (v1.15 Slice 2). It mirrors app.Manager; a later
	// cleanup slice may have each handler read app.Manager directly and remove
	// this field.
	manager *manager.Manager
}

// NewHandler builds the HTTP handler from the explicit app dependency container.
// This is the preferred construction path (v1.15); main.go and tests build an
// app.App via app.New and pass it here.
func NewHandler(application *app.App) *Handler {
	h := &Handler{
		app:             application,
		staticAvailable: static.HasClient(application.Options.StaticDir),
		manager:         application.Manager,
	}
	h.apiRouter = h.buildAPIRouter()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The top-level API/static boundary stays owned by Portier (not chi): /api
	// and /api/* go to the chi-backed API router, everything else to static
	// serving (or a plain 404 when there is no client build).
	if isAPIPath(r.URL.Path) {
		h.apiRouter.ServeHTTP(w, r)
		return
	}

	if h.staticAvailable {
		static.ServeClient(w, r, h.app.Options.StaticDir)
		return
	}

	http.NotFound(w, r)
}

func isAPIPath(route string) bool {
	return route == "/api" || strings.HasPrefix(route, "/api/")
}

// writeAPINotFound emits the generic /api 404 JSON envelope. It is wired to the
// chi router's NotFound and MethodNotAllowed handlers (routes.go), so an unknown
// API path OR a wrong method on a known API path returns this envelope rather
// than a 405 — the established 404-not-405 contract. (v1.15 Slice 12 retired the
// former serveLegacyAPI ordered dispatch; all API routes are now chi-owned.)
func writeAPINotFound(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string][]string{
		"errors": {notFoundMessage},
	})
}
