package manager

import (
	"fmt"
	"strings"
	"time"

	"portier/service/sources/activity"
	"portier/service/sources/config"
	"portier/service/sources/connections"
	"portier/service/sources/domain"
	"portier/service/sources/forwarders"
	"portier/service/sources/recovery"
	"portier/service/sources/validation"
)

type Manager struct {
	rules       []domain.ForwardRule
	runtime     map[string]runtimeState
	store       *config.Store
	activity    *activity.Store
	onStartLog  func(rule domain.ForwardRule)
	onEventLog  forwarders.LogFunc
	tcpRegistry *connections.TcpConnectionRegistry
	udpRegistry *connections.UdpSessionRegistry
	recovery    *recovery.State
}

type runtimeState struct {
	running   bool
	startedAt string
	lastError string
	forwarder forwarders.Forwarder
}

func New(rules []domain.ForwardRule) (*Manager, error) {
	return NewWithStore(nil, rules)
}

func NewWithStore(store *config.Store, rules []domain.ForwardRule) (*Manager, error) {
	// Persisted duplicate listen bindings are NOT rejected at construction
	// (v1.17 Slice 3, R-1): a config that an older Portier accepted must still
	// load so the management API can come up and the operator can fix it. The
	// duplicate is handled at autostart (StartEnabled skips conflicting enabled
	// rules), and create/update/import still reject NEW duplicates strictly.
	copied := make([]domain.ForwardRule, len(rules))
	copy(copied, rules)
	return &Manager{
		rules:       copied,
		runtime:     make(map[string]runtimeState),
		store:       store,
		tcpRegistry: connections.NewTcpConnectionRegistry(),
		udpRegistry: connections.NewUdpSessionRegistry(),
	}, nil
}

// NewFromConfig builds a Manager from the persisted config, recovering from
// config-load failures instead of failing startup. A malformed, schema-invalid,
// or unreadable config no longer aborts the process: the recovery loader
// classifies the failure (quarantining the bad file where safe), the manager
// starts with no active rules, and RecoveryState() reports the condition while
// blocking writes so the bad config cannot be silently overwritten.
//
// A schema-valid config that contains semantic conflicts (e.g. persisted
// duplicate listen bindings) also loads successfully now (v1.17 Slice 3): the
// rules are preserved and the conflict is handled at autostart, not by failing
// construction.
func NewFromConfig(configPath string) (*Manager, error) {
	store := config.NewStore(configPath)
	rules, state := recovery.LoadConfig(configPath)
	manager, err := NewWithStore(&store, rules)
	if err != nil {
		return nil, err
	}
	manager.recovery = state
	return manager, nil
}

// RecoveryState returns the active startup recovery state, or nil when the
// config loaded normally. Internal accessor for later API/UI/CLI surfacing
// (Slice 5) and for tests.
func (m *Manager) RecoveryState() *recovery.State {
	return m.recovery
}

func (m *Manager) SetStartLogger(logger func(rule domain.ForwardRule)) {
	m.onStartLog = logger
}

func (m *Manager) SetEventLogger(logger forwarders.LogFunc) {
	m.onEventLog = logger
}

func (m *Manager) SetActivityStore(store *activity.Store) {
	m.activity = store
}

// RuleStartOutcome describes one enabled rule that did not autostart, with a
// concise operator-safe reason. Used for failed binds and skipped conflicts.
type RuleStartOutcome struct {
	RuleID   string
	RuleName string
	Error    string
}

// StartEnabledResult summarizes a boot-time autostart pass. It is informational
// (for logging/diagnostics); StartEnabled is non-fatal, so there is no error.
type StartEnabledResult struct {
	// Attempted is the number of enabled rules considered for autostart.
	Attempted int
	// Started is the number that started successfully.
	Started int
	// Failed lists enabled rules whose forwarder failed to bind/start.
	Failed []RuleStartOutcome
	// Skipped lists enabled rules not started because they share a listen
	// binding with another enabled rule (persisted duplicate conflict).
	Skipped []RuleStartOutcome
}

// StartEnabled autostarts enabled rules and is non-fatal (v1.17 Slice 3, R-1):
// one rule's bind failure, or a persisted duplicate binding, never aborts the
// boot pass — every other enabled rule is still attempted and the management API
// still comes up. A rule that fails to bind is left enabled-but-stopped with its
// `lastError` (via StartRule); enabled rules that share a listen binding are
// skipped (not autostarted) and marked enabled-but-stopped with a conflict
// `lastError`. The returned summary is for logging; rule state carries the truth.
func (m *Manager) StartEnabled() StartEnabledResult {
	result := StartEnabledResult{}
	conflicts := m.conflictingEnabledBindings()
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}
		result.Attempted++

		if message, isConflict := conflicts[rule.ID]; isConflict {
			m.markRuleStartFailure(rule, message)
			result.Skipped = append(result.Skipped, RuleStartOutcome{RuleID: rule.ID, RuleName: rule.Name, Error: message})
			continue
		}

		if _, err := m.StartRule(rule.ID); err != nil {
			result.Failed = append(result.Failed, RuleStartOutcome{RuleID: rule.ID, RuleName: rule.Name, Error: err.Error()})
			continue
		}
		result.Started++
	}
	return result
}

// conflictingEnabledBindings returns, for each enabled rule that shares a listen
// binding (protocol + listenHost + listenPort) with another enabled rule, a
// deterministic operator-safe message naming the conflict. Disabled rules never
// contribute a conflict (they will not bind), so an enabled rule that only
// shares its binding with disabled rules autostarts normally. The message lists
// the conflicting rule names in stable rule order.
func (m *Manager) conflictingEnabledBindings() map[string]string {
	groups := make(map[string][]domain.ForwardRule)
	for _, rule := range m.rules {
		if rule.Enabled {
			key := listenKey(rule)
			groups[key] = append(groups[key], rule)
		}
	}

	messages := make(map[string]string)
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		names := make([]string, 0, len(group))
		for _, rule := range group {
			names = append(names, fmt.Sprintf("%q", rule.Name))
		}
		first := group[0]
		message := fmt.Sprintf(
			"Listen binding %s %s:%d is claimed by %d enabled rules (%s); not autostarted to avoid a conflict.",
			strings.ToUpper(string(first.Protocol)),
			first.ListenHost,
			first.ListenPort,
			len(group),
			strings.Join(names, ", "),
		)
		for _, rule := range group {
			messages[rule.ID] = message
		}
	}
	return messages
}

// markRuleStartFailure records an enabled rule as stopped/error with lastError
// WITHOUT attempting to bind, and emits the existing rule.error activity event.
// Used for autostart conflict-skips; bind failures take the StartRule path,
// which already sets lastError and emits rule.error.
func (m *Manager) markRuleStartFailure(rule domain.ForwardRule, message string) {
	state := m.runtime[rule.ID]
	state.running = false
	state.startedAt = ""
	state.lastError = message
	state.forwarder = nil
	m.runtime[rule.ID] = state
	m.emitRuleEvent(activity.EventRuleError, activity.SeverityError, rule, message)
}

func (m *Manager) StopAll() {
	for _, rule := range m.ListRules() {
		_, _ = m.StopRule(rule.ID)
	}
}

func (m *Manager) ListRules() []domain.ForwardRule {
	copied := make([]domain.ForwardRule, len(m.rules))
	copy(copied, m.rules)
	return copied
}

func (m *Manager) ListStatus() []domain.ForwardStatus {
	statuses := make([]domain.ForwardStatus, 0, len(m.rules))
	for _, rule := range m.rules {
		statuses = append(statuses, m.statusForRule(rule))
	}
	return statuses
}

// ListActivity returns activity events with optional filters. Returns empty slice when no store is set.
func (m *Manager) ListActivity(params activity.ListParams) []activity.ActivityEvent {
	if m.activity == nil {
		return []activity.ActivityEvent{}
	}
	return m.activity.List(params)
}

// ClearActivity clears the in-memory activity log. No-op when no store is set.
func (m *Manager) ClearActivity() {
	if m.activity != nil {
		m.activity.Clear()
	}
}

// GetLiveTCPConnections returns a snapshot of all currently active TCP connections.
func (m *Manager) GetLiveTCPConnections() []connections.TcpConnectionInfo {
	return m.tcpRegistry.Snapshot(time.Now())
}

// GetLiveUDPSessions returns a snapshot of all non-expired UDP sessions.
func (m *Manager) GetLiveUDPSessions() []connections.UdpSessionInfo {
	return m.udpRegistry.Snapshot(time.Now())
}

func (m *Manager) ExportConfig() domain.ExportedConfig {
	rules := m.ListRules()
	cfg := domain.ExportedConfig{
		Version:    "1",
		ExportedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Rules:      rules,
	}
	m.emitActivity(activity.ActivityEventInput{
		Type:     activity.EventConfigExported,
		Severity: activity.SeverityInfo,
		Message:  fmt.Sprintf("Config exported: %d rule(s).", len(rules)),
		Details:  map[string]any{"ruleCount": len(rules)},
	})
	return cfg
}

func (m *Manager) CreateRule(input validation.ForwardRuleInput) (domain.ForwardRule, error) {
	if input.ID == nil {
		id := domain.NewRuleID()
		input.ID = &id
	}

	rule, errors := validation.ValidateForwardRuleInput(input)
	if len(errors) > 0 {
		return domain.ForwardRule{}, ValidationError{Errors: errors}
	}
	if err := m.ensureNoDuplicate(rule, ""); err != nil {
		return domain.ForwardRule{}, err
	}

	m.rules = append(m.rules, rule)
	if err := m.persist(); err != nil {
		m.rules = m.rules[:len(m.rules)-1]
		return domain.ForwardRule{}, err
	}

	m.emitRuleEvent(activity.EventRuleCreated, activity.SeveritySuccess, rule, fmt.Sprintf(`Rule "%s" created.`, rule.Name))

	if rule.Enabled {
		if _, err := m.StartRule(rule.ID); err != nil {
			return domain.ForwardRule{}, err
		}
	}
	return rule, nil
}

func (m *Manager) UpdateRule(ruleID string, patch validation.ForwardRulePatch) (domain.ForwardRule, error) {
	index := m.indexOf(ruleID)
	if index < 0 {
		return domain.ForwardRule{}, NotFoundError{Message: fmt.Sprintf("Forward rule %s was not found.", ruleID)}
	}

	existing := m.rules[index]
	wasRunning := m.runtime[existing.ID].running
	next, errors := validation.ApplyPatch(existing, patch)
	if len(errors) > 0 {
		return domain.ForwardRule{}, ValidationError{Errors: errors}
	}
	next.ID = existing.ID

	if err := m.ensureNoDuplicate(next, existing.ID); err != nil {
		return domain.ForwardRule{}, err
	}

	needsRestart := wasRunning && ForwardingFieldsChanged(existing, next)
	if needsRestart {
		m.stopRuntime(existing)
		m.emitRuleEvent(activity.EventRuleStopped, activity.SeverityInfo, existing, fmt.Sprintf(`Rule "%s" stopped.`, existing.Name))
	}

	previous := m.rules[index]
	m.rules[index] = next
	if err := m.persist(); err != nil {
		m.rules[index] = previous
		if needsRestart {
			_, _ = m.StartRule(previous.ID)
		}
		return domain.ForwardRule{}, err
	}

	m.emitRuleEvent(activity.EventRuleUpdated, activity.SeverityInfo, next, fmt.Sprintf(`Rule "%s" updated.`, next.Name))

	if needsRestart {
		if _, err := m.StartRule(next.ID); err != nil {
			return domain.ForwardRule{}, err
		}
	}
	return next, nil
}

func (m *Manager) DeleteRule(ruleID string) error {
	index := m.indexOf(ruleID)
	if index < 0 {
		return NotFoundError{Message: fmt.Sprintf("Forward rule %s was not found.", ruleID)}
	}

	rule := m.rules[index]
	wasRunning := m.runtime[rule.ID].running

	previousRules := m.ListRules()
	previousRuntime := m.copyRuntime()
	m.stopRuntime(rule)

	if wasRunning {
		m.emitRuleEvent(activity.EventRuleStopped, activity.SeverityInfo, rule, fmt.Sprintf(`Rule "%s" stopped.`, rule.Name))
	}

	m.rules = append(m.rules[:index], m.rules[index+1:]...)
	delete(m.runtime, ruleID)
	if err := m.persist(); err != nil {
		m.rules = previousRules
		m.runtime = previousRuntime
		return err
	}

	m.emitRuleEvent(activity.EventRuleDeleted, activity.SeverityWarning, rule, fmt.Sprintf(`Rule "%s" deleted.`, rule.Name))
	return nil
}

func (m *Manager) ReorderRules(ids []string) error {
	for _, id := range ids {
		if m.indexOf(id) < 0 {
			return NotFoundError{Message: fmt.Sprintf("Rule %s was not found.", id)}
		}
	}

	previousRules := m.ListRules()
	nextRules := make([]domain.ForwardRule, 0, len(m.rules))
	added := make(map[string]bool)
	for _, id := range ids {
		index := m.indexOf(id)
		if index >= 0 && !added[id] {
			nextRules = append(nextRules, m.rules[index])
			added[id] = true
		}
	}
	for _, rule := range m.rules {
		if !added[rule.ID] {
			nextRules = append(nextRules, rule)
		}
	}

	m.rules = nextRules
	if err := m.persist(); err != nil {
		m.rules = previousRules
		return err
	}
	return nil
}

func (m *Manager) ImportConfig(config domain.ExportedConfig, mode string) (domain.ImportResult, error) {
	if config.Version != "1" {
		return domain.ImportResult{}, ValidationError{Errors: []string{"config must be a valid Portier config object with version 1 and a rules array."}}
	}
	if mode != "replace" && mode != "merge" {
		return domain.ImportResult{}, ValidationError{Errors: []string{"mode must be replace or merge."}}
	}

	validated, validationErrors := validateImportRules(config.Rules)
	if len(validationErrors) > 0 {
		m.emitActivity(activity.ActivityEventInput{
			Type:     activity.EventConfigImportFailed,
			Severity: activity.SeverityError,
			Message:  fmt.Sprintf("Config import rejected: %d invalid rule(s).", len(validationErrors)),
			Details:  map[string]any{"errors": strings.Join(validationErrors, "; ")},
		})
		return domain.ImportResult{Imported: 0, Skipped: 0, Errors: validationErrors}, nil
	}
	if err := ensureNoDuplicateBindings(validated); err != nil {
		m.emitActivity(activity.ActivityEventInput{
			Type:     activity.EventConfigImportFailed,
			Severity: activity.SeverityError,
			Message:  fmt.Sprintf("Config import rejected: %s", err.Error()),
		})
		return domain.ImportResult{Imported: 0, Skipped: 0, Errors: []string{err.Error()}}, nil
	}

	previousRules := m.ListRules()
	previousRuntime := m.copyRuntime()
	imported := 0
	skipped := 0
	startAfterPersist := make([]domain.ForwardRule, 0)

	if mode == "replace" {
		m.StopAll()
		m.rules = make([]domain.ForwardRule, len(validated))
		copy(m.rules, validated)
		m.runtime = make(map[string]runtimeState)
		imported = len(validated)
		startAfterPersist = append(startAfterPersist, validated...)
	} else {
		nextRules := m.ListRules()
		for _, rule := range validated {
			candidate := rule
			if m.hasIDIn(candidate.ID, nextRules) {
				candidate.ID = domain.NewRuleID()
			}
			if conflict := findDuplicateBinding(candidate, nextRules, ""); conflict != nil {
				conflictMsg := fmt.Sprintf(
					`Rule "%s" conflicts with existing rule "%s" on %s %s:%d.`,
					candidate.Name,
					conflict.Name,
					strings.ToUpper(string(candidate.Protocol)),
					candidate.ListenHost,
					candidate.ListenPort,
				)
				m.emitActivity(activity.ActivityEventInput{
					Type:     activity.EventConfigImportFailed,
					Severity: activity.SeverityWarning,
					Message:  fmt.Sprintf("Config merge conflict: %s", conflictMsg),
				})
				return domain.ImportResult{
					Imported: 0,
					Skipped:  skipped + 1,
					Errors:   []string{conflictMsg},
				}, nil
			}
			nextRules = append(nextRules, candidate)
			startAfterPersist = append(startAfterPersist, candidate)
			imported++
		}
		m.rules = nextRules
	}

	if err := m.persist(); err != nil {
		m.rules = previousRules
		m.runtime = previousRuntime
		return domain.ImportResult{}, err
	}
	for _, rule := range startAfterPersist {
		if rule.Enabled {
			_, _ = m.StartRule(rule.ID)
		}
	}

	m.emitActivity(activity.ActivityEventInput{
		Type:     activity.EventConfigImported,
		Severity: activity.SeveritySuccess,
		Message:  fmt.Sprintf("Config imported (%s): %d rule(s) added, %d skipped.", mode, imported, skipped),
		Details:  map[string]any{"mode": mode, "imported": imported, "skipped": skipped},
	})

	return domain.ImportResult{Imported: imported, Skipped: skipped, Errors: []string{}}, nil
}

func (m *Manager) StartRule(ruleID string) (domain.ForwardStatus, error) {
	rule, ok := m.ruleByID(ruleID)
	if !ok {
		return domain.ForwardStatus{}, NotFoundError{Message: fmt.Sprintf("Forward rule %s was not found.", ruleID)}
	}

	state := m.runtime[ruleID]
	if state.running {
		return m.statusForRule(rule), nil
	}

	forwarder := forwarders.NewForwarder(rule, m.onEventLog, m.activityEventFunc(), m.tcpRegistry, m.udpRegistry)
	if forwarder == nil {
		// Unrecognized protocol: no runtime to start (unchanged no-op behavior).
		return m.statusForRule(rule), nil
	}

	if err := forwarder.Start(); err != nil {
		state.running = false
		state.startedAt = ""
		state.lastError = err.Error()
		state.forwarder = nil
		m.runtime[ruleID] = state
		m.emitRuleEvent(activity.EventRuleError, activity.SeverityError, rule, fmt.Sprintf(`Rule "%s" failed to start: %s`, rule.Name, err.Error()))
		return m.statusForRule(rule), err
	}

	state.running = true
	state.startedAt = forwarder.Status().StartedAt
	state.lastError = ""
	state.forwarder = forwarder
	m.runtime[ruleID] = state
	m.emitRuleEvent(activity.EventRuleStarted, activity.SeveritySuccess, rule, fmt.Sprintf(`Rule "%s" started.`, rule.Name))
	return m.statusForRule(rule), nil
}

func (m *Manager) StopRule(ruleID string) (domain.ForwardStatus, error) {
	rule, ok := m.ruleByID(ruleID)
	if !ok {
		return domain.ForwardStatus{}, NotFoundError{Message: fmt.Sprintf("Forward rule %s was not found.", ruleID)}
	}

	wasRunning := m.runtime[ruleID].running
	m.stopRuntime(rule)

	if wasRunning {
		m.emitRuleEvent(activity.EventRuleStopped, activity.SeverityInfo, rule, fmt.Sprintf(`Rule "%s" stopped.`, rule.Name))
	}

	return m.statusForRule(rule), nil
}

// StartGroup starts every rule whose normalized group equals `group`, in rule
// order. Behaviour mirrors single-rule StartRule (POST /api/forwards/:id/start):
// an already-running rule is skipped (idempotent) and `Enabled`/autostart is NOT
// a precondition. Returns one result per matched rule; an empty slice means no
// rule matched. Does not mutate rule definitions, order, or metadata.
func (m *Manager) StartGroup(group string) []domain.GroupActionResult {
	results := make([]domain.GroupActionResult, 0)
	for _, rule := range m.rules {
		if rule.Group == nil || *rule.Group != group {
			continue
		}
		if m.runtime[rule.ID].running {
			results = append(results, domain.GroupActionResult{RuleID: rule.ID, RuleName: rule.Name, Status: "skipped", Reason: "already_running"})
			continue
		}
		if _, err := m.StartRule(rule.ID); err != nil {
			results = append(results, domain.GroupActionResult{RuleID: rule.ID, RuleName: rule.Name, Status: "failed", Reason: err.Error()})
			continue
		}
		results = append(results, domain.GroupActionResult{RuleID: rule.ID, RuleName: rule.Name, Status: "started"})
	}
	return results
}

// StopGroup stops every running rule whose normalized group equals `group`, in
// rule order. A rule that is not running is skipped. Mirrors single-rule StopRule.
func (m *Manager) StopGroup(group string) []domain.GroupActionResult {
	results := make([]domain.GroupActionResult, 0)
	for _, rule := range m.rules {
		if rule.Group == nil || *rule.Group != group {
			continue
		}
		if !m.runtime[rule.ID].running {
			results = append(results, domain.GroupActionResult{RuleID: rule.ID, RuleName: rule.Name, Status: "skipped", Reason: "not_running"})
			continue
		}
		if _, err := m.StopRule(rule.ID); err != nil {
			results = append(results, domain.GroupActionResult{RuleID: rule.ID, RuleName: rule.Name, Status: "failed", Reason: err.Error()})
			continue
		}
		results = append(results, domain.GroupActionResult{RuleID: rule.ID, RuleName: rule.Name, Status: "stopped"})
	}
	return results
}

func (m *Manager) stopRuntime(rule domain.ForwardRule) {
	state := m.runtime[rule.ID]
	if state.forwarder != nil {
		state.forwarder.Stop()
		state.lastError = state.forwarder.Status().LastError
	}
	state.running = false
	state.startedAt = ""
	state.forwarder = nil
	if state.lastError == "" {
		delete(m.runtime, rule.ID)
		return
	}
	m.runtime[rule.ID] = state
}

func ForwardingFieldsChanged(a domain.ForwardRule, b domain.ForwardRule) bool {
	return a.Protocol != b.Protocol ||
		a.ListenHost != b.ListenHost ||
		a.ListenPort != b.ListenPort ||
		a.TargetHost != b.TargetHost ||
		a.TargetPort != b.TargetPort ||
		udpModeValue(a.UdpMode) != udpModeValue(b.UdpMode)
}

func (m *Manager) statusForRule(rule domain.ForwardRule) domain.ForwardStatus {
	state := m.runtime[rule.ID]

	var status domain.ForwardStatus
	if state.forwarder != nil {
		// The forwarder tracks runtime counters but not the rule's Enabled flag;
		// the manager is the single place that derives Health.
		status = state.forwarder.Status()
	} else {
		status = domain.ForwardStatus{
			RuleID:   rule.ID,
			Running:  state.running,
			BytesIn:  0,
			BytesOut: 0,
		}
		if state.running {
			status.StartedAt = state.startedAt
		}
		if state.lastError != "" {
			status.LastError = state.lastError
		}

		if rule.Protocol == domain.ProtocolTCP {
			zero := 0
			status.ActiveConnections = &zero
		}

		if rule.Protocol == domain.ProtocolUDP {
			packetsIn := int64(0)
			packetsOut := int64(0)
			status.PacketsIn = &packetsIn
			status.PacketsOut = &packetsOut

			if rule.UdpMode != nil && *rule.UdpMode == domain.UdpModeBidirectionalMulti {
				zero := 0
				status.ActiveUdpSessions = &zero
			}
		}
	}

	status.Health = domain.DeriveRuleHealth(rule.Enabled, status.Running, status.LastError)
	return status
}

func (m *Manager) persist() error {
	if m.recovery.Active() && m.recovery.WritesBlocked {
		return RecoveryError{Message: "Configuration is in recovery mode; rule changes are blocked until the configuration is repaired."}
	}
	if m.store == nil {
		return nil
	}
	return m.store.Save(m.rules)
}

func (m *Manager) ensureNoDuplicate(rule domain.ForwardRule, ignoreRuleID string) error {
	if conflict := findDuplicateBinding(rule, m.rules, ignoreRuleID); conflict != nil {
		return ConflictError{Message: fmt.Sprintf(
			"A %s rule is already listening on %s:%d.",
			strings.ToUpper(string(rule.Protocol)),
			rule.ListenHost,
			rule.ListenPort,
		)}
	}
	return nil
}

func ensureNoDuplicateBindings(rules []domain.ForwardRule) error {
	seen := make(map[string]domain.ForwardRule)
	for _, rule := range rules {
		key := listenKey(rule)
		if existing, ok := seen[key]; ok {
			return fmt.Errorf(
				"a %s rule is already listening on %s:%d (rules %q and %q)",
				rule.Protocol,
				rule.ListenHost,
				rule.ListenPort,
				existing.Name,
				rule.Name,
			)
		}
		seen[key] = rule
	}
	return nil
}

func findDuplicateBinding(candidate domain.ForwardRule, rules []domain.ForwardRule, ignoreRuleID string) *domain.ForwardRule {
	key := listenKey(candidate)
	for _, rule := range rules {
		if rule.ID == ignoreRuleID {
			continue
		}
		if listenKey(rule) == key {
			conflict := rule
			return &conflict
		}
	}
	return nil
}

func listenKey(rule domain.ForwardRule) string {
	return fmt.Sprintf("%s:%s:%d", rule.Protocol, rule.ListenHost, rule.ListenPort)
}

func (m *Manager) indexOf(ruleID string) int {
	for index, rule := range m.rules {
		if rule.ID == ruleID {
			return index
		}
	}
	return -1
}

func (m *Manager) ruleByID(ruleID string) (domain.ForwardRule, bool) {
	index := m.indexOf(ruleID)
	if index < 0 {
		return domain.ForwardRule{}, false
	}
	return m.rules[index], true
}

func (m *Manager) hasIDIn(ruleID string, rules []domain.ForwardRule) bool {
	for _, rule := range rules {
		if rule.ID == ruleID {
			return true
		}
	}
	return false
}

func (m *Manager) copyRuntime() map[string]runtimeState {
	copied := make(map[string]runtimeState, len(m.runtime))
	for key, value := range m.runtime {
		copied[key] = value
	}
	return copied
}

func validateImportRules(rules []domain.ForwardRule) ([]domain.ForwardRule, []string) {
	validated := make([]domain.ForwardRule, 0, len(rules))
	errors := make([]string, 0)
	for _, raw := range rules {
		rule, ruleErrors := validation.ValidateForwardRuleInput(validation.InputFromRule(raw))
		if len(ruleErrors) > 0 {
			name := raw.Name
			if name == "" {
				name = "?"
			}
			errors = append(errors, fmt.Sprintf(`Rule "%s": %s`, name, strings.Join(ruleErrors, " ")))
			continue
		}
		validated = append(validated, rule)
	}
	return validated, errors
}

// emitActivity records an activity event if the activity store is configured.
func (m *Manager) emitActivity(input activity.ActivityEventInput) {
	if m.activity != nil {
		m.activity.Add(input)
	}
}

// emitRuleEvent records a rule-scoped activity event, populating RuleID, RuleName,
// and Protocol from the given rule. It is the single emission path for rule-scoped
// events (created/updated/deleted/started/stopped/error) so their payload shape
// cannot drift between call sites. Config-level events (export/import) are not
// rule-scoped and continue to call emitActivity directly with their own details.
func (m *Manager) emitRuleEvent(eventType, severity string, rule domain.ForwardRule, message string) {
	id, name, proto := rule.ID, rule.Name, string(rule.Protocol)
	m.emitActivity(activity.ActivityEventInput{
		Type:     eventType,
		Severity: severity,
		RuleID:   &id,
		RuleName: &name,
		Protocol: &proto,
		Message:  message,
	})
}

// activityEventFunc returns an EventFunc that records events to the activity store,
// or nil when no store is configured.
func (m *Manager) activityEventFunc() activity.EventFunc {
	if m.activity == nil {
		return nil
	}
	store := m.activity
	return func(input activity.ActivityEventInput) {
		store.Add(input)
	}
}

func udpModeValue(mode *domain.UdpMode) string {
	if mode == nil {
		return ""
	}
	return string(*mode)
}

type ValidationError struct {
	Errors []string
}

func (e ValidationError) Error() string {
	return strings.Join(e.Errors, " ")
}

type ConflictError struct {
	Message string
}

func (e ConflictError) Error() string {
	return e.Message
}

type NotFoundError struct {
	Message string
}

func (e NotFoundError) Error() string {
	return e.Message
}

// RecoveryError is returned when a mutating operation is refused because the
// runtime is in config-load recovery mode (writes blocked). It carries no API
// schema change: the existing error envelope surfaces the message. Slice 5 will
// decide on a dedicated status code / surfacing.
type RecoveryError struct {
	Message string
}

func (e RecoveryError) Error() string {
	return e.Message
}
