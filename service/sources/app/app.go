// Package app holds the explicit dependency container for the Portier native
// service HTTP layer. App bundles the already-resolved dependencies the handler
// and (over the v1.15 modularization) the per-feature route modules need; it has
// no behavior and no global state. Route modules read from *App — they never
// reach back into it to mutate process state. Keeping it a plain struct keeps
// wiring explicit in main.go and avoids a service locator / DI framework.
package app

import (
	"time"

	"portier/service/sources/manager"
	"portier/service/sources/options"
	"portier/service/sources/version"
)

// App is the explicit dependency container for the service HTTP layer.
type App struct {
	// Manager owns rule state, forwarding lifecycle, activity, and persistence.
	Manager *manager.Manager
	// Options are the resolved service options (host/port/config/static/service).
	Options options.Options
	// StartedAt is the process start time used to report runtime uptime.
	StartedAt time.Time
	// Version is the reported service version.
	Version string
}

// New builds an App, applying the same defaults the API handler historically
// applied so the composition root and tests can pass zero values safely: a nil
// Manager becomes a fresh empty in-memory manager, a zero StartedAt becomes now,
// and an empty Version becomes the build version.
func New(forwardManager *manager.Manager, opts options.Options, startedAt time.Time, ver string) *App {
	if forwardManager == nil {
		forwardManager, _ = manager.New(nil)
	}
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	if ver == "" {
		ver = version.Version
	}
	return &App{
		Manager:   forwardManager,
		Options:   opts,
		StartedAt: startedAt,
		Version:   ver,
	}
}
