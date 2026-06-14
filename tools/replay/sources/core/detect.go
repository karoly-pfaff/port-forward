package core

// Input detection for the offline replay tool. Detection is based on the JSON
// shape / manifest shape of an existing Portier workflow artifact — never on the
// filename. The replay tool is strictly offline and read-only: it parses only the
// provided artifact (a single JSON report file, or a support-report bundle
// directory whose manifest it reads), and it NEVER reads the config/policy/
// baseline/report files that an artifact refers to, contacts the runtime, executes
// a workflow, mutates inputs, or uploads anything.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Source kinds the replay tool recognizes.
const (
	SourceWorkflowRunReport     = "workflow-run-report"
	SourceWorkflowPlanReport    = "workflow-plan-report"
	SourceWorkflowHistoryExport = "workflow-history-export"
	SourceWorkflowReportBundle  = "workflow-report-bundle"
)

// Step / result status values used by the workflow artifacts.
const (
	statusValid   = "valid"
	statusInvalid = "invalid"
	statusPassed  = "passed"
	statusFailed  = "failed"
	statusSkipped = "skipped"
)

// historyExportSource is the `source` value stamped into a workflow history
// export by `workflow history export`.
const historyExportSource = "workflow-history"

// reportBundleManifestType is the `type` value of a workflow support-report
// bundle's manifest.json (written by `workflow report`).
const reportBundleManifestType = "workflow-report"

// Input is the detected, normalized description of a replay artifact. It carries
// only the small signals the plan builder needs — the source kind, the workflow
// name and result label (when the artifact has them), and lightweight content
// counts. It never holds raw configs, secrets, or referenced-file contents.
type Input struct {
	Kind     string // one of the Source* constants
	Workflow string // workflow name (empty for a history export)
	Result   string // result label (empty for a history export)

	Steps    int  // step count (run / plan reports)
	Runs     int  // run count (history export)
	HasCodes bool // whether explainable codes are present (run / plan reports)
}

// rawArtifact is the lenient on-disk shape used to detect a single-file artifact.
// Unknown fields are tolerated; detection keys off the fields below.
type rawArtifact struct {
	SchemaVersion *int                       `json:"schemaVersion"` // plan + history export
	Source        string                     `json:"source"`        // history export
	Name          string                     `json:"name"`          // plan
	Workflow      string                     `json:"workflow"`      // run
	Result        string                     `json:"result"`        // run + plan
	Steps         []rawStep                  `json:"steps"`         // run + plan
	Runs          []json.RawMessage          `json:"runs"`          // history export
	RunCount      *int                       `json:"runCount"`      // history export
	Explanations  map[string]json.RawMessage `json:"explanations"`  // run + plan (--explain)
}

// rawStep is the lenient shape of one step; only the status is needed.
type rawStep struct {
	Status string `json:"status"`
}

// rawManifest is the lenient shape of a support-report bundle's manifest.json.
type rawManifest struct {
	SchemaVersion *int   `json:"schemaVersion"`
	Type          string `json:"type"`
	Workflow      string `json:"workflow"`
	Result        string `json:"result"`
}

// DetectInput resolves an artifact path (a file or a bundle directory) into a
// detected Input. A directory is treated as a support-report bundle and only its
// manifest.json is read; a file is parsed as a single-file JSON artifact. It reads
// nothing else and contacts nothing.
func DetectInput(path string) (Input, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Input{}, err
	}
	if info.IsDir() {
		return detectBundle(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Input{}, err
	}
	return DetectJSON(data)
}

// detectBundle detects a support-report bundle by reading ONLY its manifest.json.
// It does not read report.json/explanations.json (their inspection is a plan
// capability, not a detection requirement) and never reads any referenced file.
func detectBundle(dir string) (Input, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Input{}, fmt.Errorf("not a workflow report bundle (no readable manifest.json): %w", err)
	}
	return DetectBundleManifest(data)
}

// DetectJSON detects a single-file workflow artifact (run report, plan report, or
// history export) from its JSON shape. It is pure (no filesystem access) so it is
// independently testable.
func DetectJSON(data []byte) (Input, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return Input{}, fmt.Errorf("input file is empty")
	}
	var raw rawArtifact
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return Input{}, fmt.Errorf("not a valid Portier artifact (invalid JSON): %w", err)
	}

	switch {
	case raw.Source == historyExportSource && (raw.Runs != nil || raw.RunCount != nil):
		return Input{
			Kind: SourceWorkflowHistoryExport,
			Runs: len(raw.Runs),
		}, nil

	case raw.SchemaVersion != nil && (raw.Result == statusValid || raw.Result == statusInvalid):
		if len(raw.Steps) == 0 {
			return Input{}, fmt.Errorf("not a workflow plan report (no steps)")
		}
		return Input{
			Kind:     SourceWorkflowPlanReport,
			Workflow: raw.Name,
			Result:   raw.Result,
			Steps:    len(raw.Steps),
			HasCodes: len(raw.Explanations) > 0 || hasStepWithStatus(raw.Steps, statusInvalid),
		}, nil

	case raw.SchemaVersion == nil && (raw.Result == statusPassed || raw.Result == statusFailed):
		if len(raw.Steps) == 0 {
			return Input{}, fmt.Errorf("not a workflow run report (no steps)")
		}
		return Input{
			Kind:     SourceWorkflowRunReport,
			Workflow: raw.Workflow,
			Result:   raw.Result,
			Steps:    len(raw.Steps),
			HasCodes: len(raw.Explanations) > 0 || hasStepWithStatus(raw.Steps, statusFailed, statusSkipped),
		}, nil

	default:
		return Input{}, fmt.Errorf("unsupported artifact shape (expected a workflow run report, plan report, or history export)")
	}
}

// DetectBundleManifest detects a support-report bundle from its manifest.json
// bytes. It is pure so it is independently testable.
func DetectBundleManifest(data []byte) (Input, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return Input{}, fmt.Errorf("bundle manifest is empty")
	}
	var m rawManifest
	if err := json.Unmarshal(trimmed, &m); err != nil {
		return Input{}, fmt.Errorf("not a valid workflow report bundle manifest: %w", err)
	}
	if m.SchemaVersion == nil || m.Type != reportBundleManifestType {
		return Input{}, fmt.Errorf("unsupported bundle manifest (expected a workflow report manifest)")
	}
	return Input{
		Kind:     SourceWorkflowReportBundle,
		Workflow: m.Workflow,
		Result:   m.Result,
	}, nil
}

// hasStepWithStatus reports whether any step has one of the given statuses.
func hasStepWithStatus(steps []rawStep, statuses ...string) bool {
	for _, s := range steps {
		for _, want := range statuses {
			if s.Status == want {
				return true
			}
		}
	}
	return false
}
