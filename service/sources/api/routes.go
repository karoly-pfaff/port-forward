package api

import "net/http"

// modularRoute is one explicit method+path → handler registration for an
// endpoint that has been migrated into a feature route module (v1.15). The
// route table is consulted (exact method + exact path) BEFORE the legacy
// ordered serveAPI dispatch.
//
// Matching requires an exact method, which is what preserves the established
// 404-not-405 behavior: a request to a migrated path with the wrong method does
// not match here, falls through to the legacy ordered dispatch, and ends at the
// generic /api 404 envelope — exactly as before. Migrated paths are exact
// matches with no prefix routes overlapping them, so checking them first does
// not change exact-vs-prefix precedence for any other route.
type modularRoute struct {
	method  string
	path    string
	handler http.HandlerFunc
}

// modularRoutes composes the per-feature route registrars. As later v1.15 slices
// migrate features out of the ordered serveAPI dispatch, add their registrar
// here.
func (h *Handler) modularRoutes() []modularRoute {
	var routes []modularRoute
	routes = append(routes, h.healthRoutes()...)
	routes = append(routes, h.runtimeRoutes()...)
	routes = append(routes, h.portsRoutes()...)
	routes = append(routes, h.activityRoutes()...)
	return routes
}

// dispatchModular runs the first migrated route whose method and path match the
// request and reports whether it handled it. A false result means the request
// is left to the legacy ordered serveAPI dispatch.
func (h *Handler) dispatchModular(w http.ResponseWriter, r *http.Request) bool {
	for _, route := range h.routes {
		if r.Method == route.method && r.URL.Path == route.path {
			route.handler(w, r)
			return true
		}
	}
	return false
}
