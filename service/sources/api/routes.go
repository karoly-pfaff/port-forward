package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// modularRoute is one explicit method+path → handler registration for an
// endpoint served by a feature route module (v1.15). The per-feature registrars
// return these and buildAPIRouter mounts them onto the chi API router.
//
// Routes are registered as exact static patterns (or chi {param} patterns for
// the forwards id/group routes). A request whose path is not a registered
// pattern (chi NotFound) OR whose method is not registered for an existing
// pattern (chi MethodNotAllowed) is routed to writeAPINotFound, which emits the
// generic /api 404 envelope. Routing a method mismatch there is what preserves
// the established 404-not-405 behavior (chi's default MethodNotAllowed would
// emit a 405). Static routes take chi precedence over the {id} param route, so
// reorder/group/config paths are never misrouted as a rule id.
type modularRoute struct {
	method  string
	path    string
	handler http.HandlerFunc
}

// modularRoutes composes the per-feature route registrars. As of Slice 12 every
// API route is registered here; there is no legacy ordered dispatch left.
func (h *Handler) modularRoutes() []modularRoute {
	var routes []modularRoute
	routes = append(routes, h.healthRoutes()...)
	routes = append(routes, h.runtimeRoutes()...)
	routes = append(routes, h.portsRoutes()...)
	routes = append(routes, h.activityRoutes()...)
	routes = append(routes, h.statusRoutes()...)
	routes = append(routes, h.connectionsRoutes()...)
	routes = append(routes, h.forwardsRoutes()...)
	routes = append(routes, h.configRoutes()...)
	return routes
}

// buildAPIRouter constructs the chi router that owns every API route. Each
// modular route is mounted as an exact method+pattern (or {param} pattern for
// the forwards id/group routes); anything chi cannot match (unknown path →
// NotFound, or a wrong method on a known path → MethodNotAllowed) is routed to
// writeAPINotFound so the generic /api 404 envelope (404-not-405) is preserved.
// The chi router only ever serves requests under /api — the top-level ServeHTTP
// keeps owning the API/static boundary.
func (h *Handler) buildAPIRouter() http.Handler {
	router := chi.NewRouter()
	for _, route := range h.modularRoutes() {
		router.Method(route.method, route.path, route.handler)
	}
	router.NotFound(writeAPINotFound)
	router.MethodNotAllowed(writeAPINotFound)
	return router
}
