package api

import (
	"net/http"
	"os"
	"runtime"
	"time"

	"portier/service/sources/recovery"
)

// runtimeRoutes registers the runtime-info endpoint.
func (h *Handler) runtimeRoutes() []modularRoute {
	return []modularRoute{
		{method: http.MethodGet, path: "/api/runtime", handler: h.handleRuntime},
	}
}

// handleRuntime reports service runtime info, including the v1.17 config-recovery
// block. Dependencies come from the app container (options, start time, version);
// all pre-existing fields are unchanged.
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
		"recovery":       recoveryResponse(h.app.Manager.RecoveryState()),
	})
}

// recoveryResponse maps the internal startup recovery state to the additive
// `recovery` block on GET /api/runtime. Always returns `{ "active": false }`
// during normal operation; when active it exposes the operator-safe reason,
// message, paths, write-block, and detection time (kebab reason + RFC3339 time,
// matching `startedAt`). It is nil-safe. Only file-level config-load recovery
// sets this; per-rule autostart/duplicate failures stay rule-level (lastError).
func recoveryResponse(state *recovery.State) map[string]any {
	if !state.Active() {
		return map[string]any{"active": false}
	}
	return map[string]any{
		"active":         true,
		"reason":         string(state.Reason),
		"message":        state.Message,
		"configPath":     state.ConfigPath,
		"quarantinePath": state.QuarantinePath,
		"writesBlocked":  state.WritesBlocked,
		"detectedAt":     state.DetectedAt.UTC().Format(time.RFC3339),
	}
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
