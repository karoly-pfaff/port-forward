package workflow

// Local workflow run history (v1.12 Slice 1): an OPT-IN, local, bounded record of
// `workflow run` outcomes. It stores only compact, non-sensitive metadata — run
// id, time, workflow name, result, summary counts, compact per-step metadata
// (id/type/status/exitCode), and the deduped+sorted explanation/finding codes a
// run emitted. It NEVER stores raw configs, raw policies, full embedded policy
// reports, file contents, logs, environment variables, process data, secrets,
// runtime URLs, or auth tokens. History is read/written locally only; this code
// never contacts the runtime and adds no telemetry, upload, or background
// collection. The storage path is injected (NewHistoryStore) so it stays testable
// and platform-agnostic.

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"portier/cli/sources/output"
)

// HistorySchemaVersion is the schema version of the workflow-history file.
const HistorySchemaVersion = 1

// HistoryFileName is the default file name for the workflow-history store.
const HistoryFileName = "workflow-history.json"

// historyMaxRuns is the bounded retention limit: the store keeps at most the
// latest historyMaxRuns runs, dropping the oldest when appending beyond it.
const historyMaxRuns = 100

// HistoryStep is the compact metadata kept for one step of a recorded run. It
// deliberately excludes the step message and any embedded policy/compare report
// (which could carry rule host/port detail) — only identity, status, and exit
// code are kept.
type HistoryStep struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	ExitCode int    `json:"exitCode"`
}

// HistoryRun is one recorded workflow run: its id, time, workflow name, result,
// summary counts, compact step metadata, and the deduped+sorted codes the run
// emitted (empty when none). It is the compact projection of a WorkflowRun — not
// a copy of the run report — so it never carries raw configs, reports, or secrets.
type HistoryRun struct {
	ID        string             `json:"id"`
	CreatedAt string             `json:"createdAt"`
	Workflow  string             `json:"workflow"`
	Result    string             `json:"result"`
	Summary   WorkflowRunSummary `json:"summary"`
	Steps     []HistoryStep      `json:"steps"`
	Codes     []string           `json:"codes,omitempty"`
}

// History is the on-disk workflow-history document: a schema version and the
// recorded runs, newest first.
type History struct {
	SchemaVersion int          `json:"schemaVersion"`
	Runs          []HistoryRun `json:"runs"`
}

// idSanitizer collapses any run of non-alphanumeric characters in a workflow name
// into a single hyphen so a run id stays a safe, readable token.
var idSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

// sanitizeWorkflowName lowercases a workflow name and reduces it to a hyphenated
// token suitable for a run id. An empty/whitespace name becomes "workflow".
func sanitizeWorkflowName(name string) string {
	s := idSanitizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "workflow"
	}
	return s
}

// NewRunID builds a run id from a UTC timestamp, the workflow name, and a short
// caller-provided suffix (random in production, fixed in tests). The shape is
// "<YYYYMMDDThhmmssZ>-<sanitized-workflow>-<suffix>", e.g.
// "20260614T101530Z-policy-baseline-check-a1b2c3d4". The suffix is injected so id
// generation is deterministic under test.
func NewRunID(createdAt time.Time, workflow, suffix string) string {
	ts := createdAt.UTC().Format("20060102T150405Z")
	id := ts + "-" + sanitizeWorkflowName(workflow)
	if suffix != "" {
		id += "-" + suffix
	}
	return id
}

// ProjectRun builds the compact history entry for a completed run. It copies only
// non-sensitive metadata: identity, time, workflow name, result, summary counts,
// compact per-step metadata, and the deduped+sorted emitted codes. The per-step
// embedded report and message are intentionally dropped.
func ProjectRun(run WorkflowRun, id string, createdAt time.Time) HistoryRun {
	steps := make([]HistoryStep, len(run.Steps))
	for i, s := range run.Steps {
		steps[i] = HistoryStep{
			ID:       s.ID,
			Type:     s.Type,
			Status:   s.Status,
			ExitCode: s.ExitCode,
		}
	}
	return HistoryRun{
		ID:        id,
		CreatedAt: createdAt.UTC().Format(time.RFC3339),
		Workflow:  run.Workflow,
		Result:    run.Result,
		Summary:   run.Summary,
		Steps:     steps,
		Codes:     sortedDedupe(ExplanationCodesForRun(run)),
	}
}

// sortedDedupe returns the unique values of in, sorted. It returns nil when empty
// so the Codes field stays omitted (compact) for runs with no codes.
func sortedDedupe(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// HistoryStore reads and writes the local workflow-history file at a fixed path.
// The path is injected so it stays testable and platform-agnostic; the store owns
// load, bounded append (retention), and clear. It never contacts the runtime.
type HistoryStore struct {
	path string
}

// NewHistoryStore returns a history store backed by the file at path.
func NewHistoryStore(path string) *HistoryStore {
	return &HistoryStore{path: path}
}

// Path returns the store's backing file path.
func (s *HistoryStore) Path() string { return s.path }

// Load reads the history file. A missing file is not an error — it returns an
// empty history (with the current schema version). A present-but-malformed file
// returns an error so the caller can surface it rather than silently discarding
// recorded runs.
func (s *HistoryStore) Load() (History, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return History{SchemaVersion: HistorySchemaVersion}, nil
		}
		return History{}, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return History{SchemaVersion: HistorySchemaVersion}, nil
	}
	var h History
	if err := json.Unmarshal(data, &h); err != nil {
		return History{}, err
	}
	if h.SchemaVersion == 0 {
		h.SchemaVersion = HistorySchemaVersion
	}
	return h, nil
}

// Append records a run at the front of the history (newest first), enforces the
// bounded retention limit (dropping the oldest beyond historyMaxRuns), and writes
// the file back. The parent directory is created if missing. A load failure
// (malformed existing file) or write failure is returned so the command layer can
// apply the documented history-write-failure exit behavior.
func (s *HistoryStore) Append(run HistoryRun) error {
	h, err := s.Load()
	if err != nil {
		return err
	}
	h.SchemaVersion = HistorySchemaVersion
	h.Runs = append([]HistoryRun{run}, h.Runs...)
	if len(h.Runs) > historyMaxRuns {
		h.Runs = h.Runs[:historyMaxRuns]
	}
	return s.write(h)
}

// Clear removes the history file. A missing file is treated as success (already
// empty). It never contacts the runtime.
func (s *HistoryStore) Clear() error {
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// write serializes the history as deterministic pretty JSON (reusing the shared
// output helper), creating the parent directory if needed.
func (s *HistoryStore) write(h History) error {
	if dir := filepath.Dir(s.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return output.WritePrettyJSON(s.path, h)
}
