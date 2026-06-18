// Package recovery implements the Go service startup config-load recovery
// foundation (v1.17 Slice 2). It classifies how a persisted config failed to
// load, preserves/quarantines bad config files where safe, and produces an
// internal recovery State the runtime carries so startup can continue with no
// active rules instead of exiting before the management API binds.
//
// Scope (this slice): config-load recovery only. Per-rule autostart bind-failure
// recovery (Slice 3), TypeScript/NestJS parity (Slice 4), and API/UI/CLI
// surfacing of the recovery State (Slice 5) are intentionally out of scope. The
// State shape is internal; nothing here is wired into the public REST contract.
//
// See docs/recovery.md for the policy this implements.
package recovery

import (
	"errors"
	"fmt"
	"os"
	"time"

	"portier/service/sources/config"
	"portier/service/sources/domain"
)

// Reason is the machine-stable classification of a startup recovery condition.
type Reason string

const (
	// ReasonNone means no recovery condition: config loaded normally (or was
	// simply absent). A State is only produced for the non-None reasons.
	ReasonNone Reason = ""
	// ReasonUnreadable: the config file exists but could not be read
	// (permission/lock/IO). The bytes may be valid, so it is left in place and
	// NOT quarantined.
	ReasonUnreadable Reason = "unreadable"
	// ReasonMalformed: the file was read but is not a valid config container
	// (invalid JSON, or not a rules array / {"rules": [...]} object).
	ReasonMalformed Reason = "malformed"
	// ReasonSchemaInvalid: the container decoded but at least one rule failed
	// validation. The whole file is rejected; valid rules are NOT salvaged.
	ReasonSchemaInvalid Reason = "schema-invalid"
	// ReasonUnsupportedVersion is reserved for a future versioned config
	// envelope; the current bare-array format carries no version, so this slice
	// does not produce it (see docs/recovery.md, Open questions).
	ReasonUnsupportedVersion Reason = "unsupported-version"
	// ReasonDuplicateBinding is reserved for persisted duplicate listen bindings,
	// handled with autostart recovery in Slice 3; this slice does not produce it.
	ReasonDuplicateBinding Reason = "duplicate-binding"
)

// State describes an active startup recovery condition. It is internal to the
// runtime for now (carried by the manager so it can block writes); Slice 5 will
// decide how, if at all, to surface it on the public API. A nil *State means no
// recovery is active.
type State struct {
	// Reason is the machine-stable classification.
	Reason Reason
	// Message is an operator-safe summary. It never contains config file
	// contents; the quarantine path (if any) is carried separately.
	Message string
	// ConfigPath is the config path Portier tried to load.
	ConfigPath string
	// QuarantinePath is where the bad config was moved, or "" when nothing was
	// quarantined (file unreadable, or quarantine itself failed).
	QuarantinePath string
	// WritesBlocked reports that rule persistence must be refused while this
	// recovery is active, so a fresh empty config cannot overwrite the bad one.
	WritesBlocked bool
	// DetectedAt is when the recovery condition was classified.
	DetectedAt time.Time
}

// Active reports whether s describes an active recovery condition. It is
// nil-safe so callers can hold a `*State` that is nil in the normal case.
func (s *State) Active() bool {
	return s != nil && s.Reason != ReasonNone
}

// LoadConfig loads the persisted config at configPath, recovering from load
// failures instead of returning a fatal error. It returns the rules to run with
// (empty when recovery is active) and a recovery *State, which is nil when the
// config loaded normally or was simply missing.
//
// It never returns an error: every config-load outcome is either a normal load
// or a recovered one. This is what keeps the management API reachable.
func LoadConfig(configPath string) ([]domain.ForwardRule, *State) {
	return loadConfigAt(configPath, time.Now())
}

// loadConfigAt is LoadConfig with an injected clock so quarantine naming and
// DetectedAt are deterministic under test.
func loadConfigAt(configPath string, now time.Time) ([]domain.ForwardRule, *State) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Missing config is the normal first-run state: empty rules, no
			// recovery, writes allowed.
			return []domain.ForwardRule{}, nil
		}
		// Unreadable: the bytes may be perfectly good — do not quarantine (a
		// copy/rename may also fail or lose data). Preserve in place, block
		// writes so a later save cannot clobber a file we never read.
		return []domain.ForwardRule{}, &State{
			Reason:        ReasonUnreadable,
			Message:       "Configuration file could not be read; started with no active rules. The original file was left untouched.",
			ConfigPath:    configPath,
			WritesBlocked: true,
			DetectedAt:    now,
		}
	}

	rules, parseErr := config.Parse(raw)
	if parseErr == nil {
		return rules, nil
	}

	reason := ReasonMalformed
	summary := "Configuration file is not valid; started with no active rules."
	switch {
	case errors.Is(parseErr, config.ErrSchemaInvalid):
		reason = ReasonSchemaInvalid
		summary = "Configuration file contains an invalid rule; started with no active rules."
	case errors.Is(parseErr, config.ErrMalformed):
		reason = ReasonMalformed
		summary = "Configuration file could not be parsed; started with no active rules."
	}

	state := &State{
		Reason:        reason,
		ConfigPath:    configPath,
		WritesBlocked: true,
		DetectedAt:    now,
	}

	// The file was readable but bad — quarantine it so the user's data is
	// preserved and a fresh empty config cannot silently replace it.
	quarantinePath, quarantineErr := quarantine(configPath, now)
	if quarantineErr != nil {
		// Quarantine failed: keep recovery active, leave the original in place,
		// do not overwrite it. Startup still continues with empty rules.
		state.Message = summary + " The original file could not be quarantined and was left in place."
		return []domain.ForwardRule{}, state
	}

	state.QuarantinePath = quarantinePath
	state.Message = summary + " The original file was quarantined."
	return []domain.ForwardRule{}, state
}

// quarantine renames the bad config to a unique, timestamped name in the same
// directory (same filesystem → atomic rename; preserves bytes). It never
// overwrites an existing quarantine. On success the original path no longer
// exists, so a later save cannot append to or partially overwrite the bad file.
func quarantine(configPath string, now time.Time) (string, error) {
	stamp := now.UTC().Format("2006-01-02T150405Z")
	base := configPath + ".corrupt-" + stamp

	candidate := base
	for attempt := 1; ; attempt++ {
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			break
		}
		candidate = fmt.Sprintf("%s-%d", base, attempt)
	}

	if err := os.Rename(configPath, candidate); err != nil {
		return "", err
	}
	return candidate, nil
}
