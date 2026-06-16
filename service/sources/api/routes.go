package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// modularRoute is one explicit method+path → handler registration for an
// endpoint that has been migrated into a feature route module (v1.15). The
// per-feature registrars return these and buildAPIRouter mounts them onto the
// chi API router (v1.15 Slice 10).
//
// Migrated routes are registered as exact static patterns. A request whose path
// is not a registered pattern (chi NotFound) OR whose method is not registered
// for an existing pattern (chi MethodNotAllowed) falls through to the legacy
// ordered serveLegacyAPI dispatch — which serves the still-unmigrated routes and
// ends in the generic /api 404 envelope. Routing a method mismatch through that
// fallback is what preserves the established 404-not-405 behavior (chi's default
// MethodNotAllowed would emit a 405). Migrated paths are exact patterns with no
// overlapping prefix route, so they cannot shadow the legacy /api/forwards/...
// id/group prefix routes.
type modularRoute struct {
	method  string
	path    string
	handler http.HandlerFunc
}

// modularRoutes composes the per-feature route registrars. As later v1.15 slices
// migrate features out of the ordered serveLegacyAPI dispatch, add their
// registrar here.
func (h *Handler) modularRoutes() []modularRoute {
	var routes []modularRoute
	routes = append(routes, h.healthRoutes()...)
	routes = append(routes, h.runtimeRoutes()...)
	routes = append(routes, h.portsRoutes()...)
	routes = append(routes, h.activityRoutes()...)
	routes = append(routes, h.statusRoutes()...)
	routes = append(routes, h.connectionsRoutes()...)
	routes = append(routes, h.forwardsRoutes()...)
	return routes
}

// buildAPIRouter constructs the chi router that owns the migrated API routes.
// Each modular route is mounted as an exact method+pattern; anything chi cannot
// match (unknown path → NotFound, or a wrong method on a known path →
// MethodNotAllowed) is delegated to the legacy ordered dispatch so unmigrated
// routes keep working and the generic /api 404 envelope (404-not-405) is
// preserved. The chi router only ever serves requests under /api — the
// top-level ServeHTTP keeps owning the API/static boundary.
func (h *Handler) buildAPIRouter() http.Handler {
	router := chi.NewRouter()
	for _, route := range h.modularRoutes() {
		router.Method(route.method, route.path, route.handler)
	}
	router.NotFound(h.serveLegacyAPI)
	router.MethodNotAllowed(h.serveLegacyAPI)
	return router
}
