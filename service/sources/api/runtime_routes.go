package api

import (
	"net/http"
	"os"
	"runtime"
	"time"
)

// runtimeRoutes registers the runtime-info endpoint.
func (h *Handler) runtimeRoutes() []modularRoute {
	return []modularRoute{
		{method: http.MethodGet, path: "/api/runtime", handler: h.handleRuntime},
	}
}

// handleRuntime reports service runtime info. Dependencies come from the app
// container (options, start time, version); the response shape is unchanged.
func (h *Handler) handleRuntime(w http.ResponseWriter, _ *http.Request) {
	opts := h.app.Options
	uptimeSeconds := int64(time.Since(h.app.StartedAt).Seconds())
	writeJSON(w, http.StatusOK, map[string]any{
		"name":           "Portier",
		"version":        h.app.Version,
		"runtime":        "go",
		"platform":       normalizePlatform(),
		"arch":           normalizeArch(),
		"uptimeSeconds":  uptimeSeconds,
		"startedAt":      h.app.StartedAt.UTC().Format(time.RFC3339),
		"managementHost": opts.Host,
		"managementPort": opts.Port,
		"configPath":     opts.ConfigPath,
		"staticDir":      opts.StaticDir,
		"serviceMode":    opts.Service,
		"pid":            os.Getpid(),
	})
}

func normalizePlatform() string {
	return internalNormalizePlatform(runtime.GOOS)
}

func internalNormalizePlatform(goos string) string {
	switch goos {
	case "windows":
		return "windows"
	case "darwin":
		return "macos"
	case "linux":
		return "linux"
	default:
		return "unknown"
	}
}

func normalizeArch() string {
	return internalNormalizeArch(runtime.GOARCH)
}

func internalNormalizeArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	default:
		return "unknown"
	}
}
