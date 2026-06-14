package core

// Offline timeline reconstruction for the replay tool. `timeline` reconstructs a
// deterministic, ordered view of what a saved Portier workflow artifact says
// happened — bracketed by clearly-marked SYNTHETIC lifecycle events (start/result)
// around the artifact's own saved events.
//
// Like the rest of the replay tool it is strictly offline and read-only: it parses
// only the supplied `--from` JSON file (or, for a support-report bundle directory,
// the bundle's own manifest.json / report.json) and NEVER executes workflows,
// contacts the runtime, reads the config/policy/baseline/report files an artifact
// refers to, mutates inputs, applies/imports, enforces policy, runs shell commands,
// uploads, or collects logs/environment/process data. It never infers timestamps
// the artifact does not contain — ordering is by saved order only.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TimelineSchemaVersion is the schema version of the replay timeline. It is a
// local tool schema (not a REST/API contract), independent of the workflow
// artifact schemas it analyzes and distinct from the plan/analysis schemas.
const TimelineSchemaVersion = 1

// Synthetic lifecycle event statuses.
const (
	eventStatusStarted       = "started"
	eventStatusReconstructed = "reconstructed"
)

// Lifecycle event types (synthetic reconstruction markers, per artifact kind).
const (
	lifecycleWorkflow = "workflow.lifecycle"
	lifecyclePlan     = "plan.lifecycle"
	lifecycleHistory  = "history.lifecycle"
	lifecycleBundle   = "bundle.lifecycle"
	typeHistoryRun    = "workflow.run"
)

// TimelineEvent is one ordered event. Synthetic is true for the reconstruction
// lifecycle markers (start/result) the replay tool adds, false for events that
// come straight from the saved artifact. ExitCode/CreatedAt/Workflow/Message/Codes/
// RunSummary are populated only where the artifact provides them.
type TimelineEvent struct {
	Order      int                 `json:"order"`
	ID         string              `json:"id"`
	Type       string              `json:"type"`
	Status     string              `json:"status"`
	Synthetic  bool                `json:"synthetic"`
	ExitCode   *int                `json:"exitCode,omitempty"`
	CreatedAt  string              `json:"createdAt,omitempty"`
	Workflow   string              `json:"workflow,omitempty"`
	Message    string              `json:"message,omitempty"`
	Codes      []string            `json:"codes,omitempty"`
	RunSummary *TimelineRunSummary `json:"runSummary,omitempty"`
}

// TimelineRunSummary is the per-run step tally carried by history run events.
type TimelineRunSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// TimelineSummary counts events. Total is every event (incl. synthetic); Synthetic
// counts the lifecycle markers; passed/failed/skipped and valid/invalid count the
// non-synthetic saved events by status. All fields are always present.
type TimelineSummary struct {
	Total     int `json:"total"`
	Passed    int `json:"passed"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped"`
	Valid     int `json:"valid"`
	Invalid   int `json:"invalid"`
	Synthetic int `json:"synthetic"`
}

// Timeline is the deterministic reconstructed timeline of one artifact. Workflow/
// Result are omitted when the artifact has none (e.g. a history export has no single
// result). Events is always present.
type Timeline struct {
	SchemaVersion int             `json:"schemaVersion"`
	Source        string          `json:"source"`
	Workflow      string          `json:"workflow,omitempty"`
	Result        string          `json:"result,omitempty"`
	Events        []TimelineEvent `json:"events"`
	Summary       TimelineSummary `json:"summary"`
}

// --- lenient raw shapes (timeline-specific; richer than detection/analysis) ---

type timelineRawReport struct {
	Workflow     string                     `json:"workflow"`
	Name         string                     `json:"name"`
	Result       string                     `json:"result"`
	Steps        []timelineRawStep          `json:"steps"`
	Explanations map[string]json.RawMessage `json:"explanations"`
}

type timelineRawStep struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Status   string          `json:"status"`
	ExitCode *int            `json:"exitCode"`
	Message  string          `json:"message"`
	Code     string          `json:"code"`   // plan: validation code
	Report   json.RawMessage `json:"report"` // run: embedded policy/compare report
}

type timelineRawHistory struct {
	Runs []timelineRawHistoryRun `json:"runs"`
}

type timelineRawHistoryRun struct {
	ID        string             `json:"id"`
	CreatedAt string             `json:"createdAt"`
	Workflow  string             `json:"workflow"`
	Result    string             `json:"result"`
	Summary   TimelineRunSummary `json:"summary"`
	Codes     []string           `json:"codes"`
}

type timelineBundleReport struct {
	Workflow string               `json:"workflow"`
	Result   string               `json:"result"`
	Steps    []timelineBundleStep `json:"steps"`
}

type timelineBundleStep struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Status   string   `json:"status"`
	ExitCode *int     `json:"exitCode"`
	Message  string   `json:"message"`
	Codes    []string `json:"codes"`
}

// TimelineInput resolves an artifact path (a file or a bundle directory) into a
// deterministic Timeline. A directory is treated as a support-report bundle (only
// its own manifest/report files are read); a file is parsed as a single-file JSON
// artifact. It reads nothing else and contacts nothing.
func TimelineInput(path string) (Timeline, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Timeline{}, err
	}
	if info.IsDir() {
		return timelineBundleDir(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Timeline{}, err
	}
	return TimelineJSON(data)
}

// TimelineJSON classifies a single-file artifact (reusing detection) and builds its
// timeline. It is pure (no filesystem access) so it is independently testable.
func TimelineJSON(data []byte) (Timeline, error) {
	in, err := DetectJSON(data)
	if err != nil {
		return Timeline{}, err
	}
	switch in.Kind {
	case SourceWorkflowRunReport:
		return timelineRunReport(data)
	case SourceWorkflowPlanReport:
		return timelinePlanReport(data)
	case SourceWorkflowHistoryExport:
		return timelineHistoryExport(data)
	default:
		// DetectJSON never classifies a bundle from bytes; this is defensive.
		return Timeline{}, fmt.Errorf("unsupported artifact for timeline")
	}
}

func timelineRunReport(data []byte) (Timeline, error) {
	var raw timelineRawReport
	if err := json.Unmarshal(data, &raw); err != nil {
		return Timeline{}, fmt.Errorf("not a valid workflow run report: %w", err)
	}
	t := newTimeline(SourceWorkflowRunReport, raw.Workflow, raw.Result)
	t.add(lifecycleStart(lifecycleWorkflow, "workflow-start", "Workflow replay timeline reconstructed from saved report."))
	for _, s := range raw.Steps {
		ev := TimelineEvent{ID: s.ID, Type: s.Type, Status: s.Status, ExitCode: s.ExitCode, Message: s.Message}
		switch s.Status {
		case statusFailed:
			ev.Codes = embeddedFindingCodes(s.Report)
		case statusSkipped:
			ev.Codes = []string{codeDependencyFailed}
		}
		t.add(ev)
	}
	// A run report always carries a passed/failed result (DetectJSON guarantees it).
	t.add(lifecycleEnd(lifecycleWorkflow, "workflow-result", raw.Result, "Workflow result: "+raw.Result+"."))
	t.finishSummary()
	return t, nil
}

func timelinePlanReport(data []byte) (Timeline, error) {
	var raw timelineRawReport
	if err := json.Unmarshal(data, &raw); err != nil {
		return Timeline{}, fmt.Errorf("not a valid workflow plan report: %w", err)
	}
	t := newTimeline(SourceWorkflowPlanReport, raw.Name, raw.Result)
	t.add(lifecycleStart(lifecyclePlan, "plan-start", "Plan replay timeline reconstructed from saved report."))
	for _, s := range raw.Steps {
		ev := TimelineEvent{ID: s.ID, Type: s.Type, Status: s.Status, Message: s.Message}
		if s.Status == statusInvalid && s.Code != "" {
			ev.Codes = []string{s.Code}
		}
		t.add(ev)
	}
	// A plan report always carries a valid/invalid result (DetectJSON guarantees it).
	t.add(lifecycleEnd(lifecyclePlan, "plan-result", raw.Result, "Plan result: "+raw.Result+"."))
	t.finishSummary()
	return t, nil
}

func timelineHistoryExport(data []byte) (Timeline, error) {
	var raw timelineRawHistory
	if err := json.Unmarshal(data, &raw); err != nil {
		return Timeline{}, fmt.Errorf("not a valid workflow history export: %w", err)
	}
	t := newTimeline(SourceWorkflowHistoryExport, "", "")
	t.add(lifecycleStart(lifecycleHistory, "history-start", "History replay timeline reconstructed from saved export (newest first)."))
	for _, r := range raw.Runs {
		summary := r.Summary
		ev := TimelineEvent{
			ID:         r.ID,
			Type:       typeHistoryRun,
			Status:     r.Result,
			CreatedAt:  r.CreatedAt,
			Workflow:   r.Workflow,
			Codes:      dedupeStrings(r.Codes),
			RunSummary: &summary,
		}
		t.add(ev)
	}
	t.add(lifecycleEnd(lifecycleHistory, "history-end", eventStatusReconstructed,
		fmt.Sprintf("Reconstructed %d run(s) from saved history.", len(raw.Runs))))
	t.finishSummary()
	return t, nil
}

// timelineBundleDir builds a timeline from a support-report bundle by reading ONLY
// its own manifest.json (required) and report.json. No other file — and in
// particular no file the report refers to — is read. A missing/malformed report.json
// yields a minimal start/result timeline.
func timelineBundleDir(dir string) (Timeline, error) {
	manifestData, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Timeline{}, fmt.Errorf("not a workflow report bundle (no readable manifest.json): %w", err)
	}
	if _, err := DetectBundleManifest(manifestData); err != nil {
		return Timeline{}, err
	}
	var man analyzeBundleManifest
	_ = json.Unmarshal(manifestData, &man) // shape already validated by DetectBundleManifest

	var rep timelineBundleReport
	if data, err := os.ReadFile(filepath.Join(dir, "report.json")); err == nil {
		_ = json.Unmarshal(data, &rep) // tolerate a missing/malformed report; manifest suffices
	}

	workflow := firstNonEmpty(man.Workflow, rep.Workflow)
	result := firstNonEmpty(man.Result, rep.Result)
	t := newTimeline(SourceWorkflowReportBundle, workflow, result)
	t.add(lifecycleStart(lifecycleBundle, "bundle-start", "Bundle replay timeline reconstructed from saved report."))
	for _, s := range rep.Steps {
		t.add(TimelineEvent{ID: s.ID, Type: s.Type, Status: s.Status, ExitCode: s.ExitCode, Message: s.Message, Codes: dedupeStrings(s.Codes)})
	}
	endMsg := "Bundle timeline reconstructed."
	if result != "" {
		endMsg = "Bundle result: " + result + "."
	}
	t.add(lifecycleEnd(lifecycleBundle, "bundle-result", endStatus(result), endMsg))
	t.finishSummary()
	return t, nil
}

// --- helpers ---

// newTimeline returns a Timeline with a non-nil Events slice (serializes `[]`).
func newTimeline(source, workflow, result string) Timeline {
	return Timeline{
		SchemaVersion: TimelineSchemaVersion,
		Source:        source,
		Workflow:      workflow,
		Result:        result,
		Events:        []TimelineEvent{},
	}
}

// add appends an event, assigning its 1-based order.
func (t *Timeline) add(ev TimelineEvent) {
	ev.Order = len(t.Events) + 1
	t.Events = append(t.Events, ev)
}

// finishSummary tallies the events into the summary (synthetic events count only
// toward Total and Synthetic; saved events count toward their status bucket).
func (t *Timeline) finishSummary() {
	for _, ev := range t.Events {
		t.Summary.Total++
		if ev.Synthetic {
			t.Summary.Synthetic++
			continue
		}
		switch ev.Status {
		case statusPassed:
			t.Summary.Passed++
		case statusFailed:
			t.Summary.Failed++
		case statusSkipped:
			t.Summary.Skipped++
		case statusValid:
			t.Summary.Valid++
		case statusInvalid:
			t.Summary.Invalid++
		}
	}
}

// lifecycleStart builds the synthetic opening event.
func lifecycleStart(typ, id, message string) TimelineEvent {
	return TimelineEvent{ID: id, Type: typ, Status: eventStatusStarted, Synthetic: true, Message: message}
}

// lifecycleEnd builds the synthetic closing event.
func lifecycleEnd(typ, id, status, message string) TimelineEvent {
	return TimelineEvent{ID: id, Type: typ, Status: status, Synthetic: true, Message: message}
}

// endStatus is the closing synthetic event's status: the artifact result if present,
// otherwise "reconstructed" (e.g. a support bundle whose manifest/report carry no
// result).
func endStatus(result string) string {
	if result == "" {
		return eventStatusReconstructed
	}
	return result
}
