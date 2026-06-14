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

// HistoryFilters describes the optional, AND-combined filters for listing the
// workflow run history (v1.12 Slice 3). A zero value means "no filter". All
// matching is over the compact local history metadata only — it never reads raw
// reports, configs, or any external data. The JSON tags double as the `filters`
// object emitted under --json (only active fields appear).
type HistoryFilters struct {
	Result   string `json:"result,omitempty"`   // exact run result ("passed"/"failed")
	Workflow string `json:"workflow,omitempty"` // exact workflow name
	Code     string `json:"code,omitempty"`     // run whose codes contain this code
	Limit    int    `json:"limit,omitempty"`    // keep at most the newest N matches
}

// hasAny reports whether any filter is active.
func (f HistoryFilters) hasAny() bool {
	return f.Result != "" || f.Workflow != "" || f.Code != "" || f.Limit > 0
}

// ValidHistoryResult reports whether result is an accepted `--result` filter
// value. Only the run-result values the store can actually contain are accepted
// (an invalid plan is never recorded, so only passed/failed exist).
func ValidHistoryResult(result string) bool {
	return result == runResultPassed || result == runResultFailed
}

// ApplyHistoryFilters returns the runs matching every active filter, preserving
// the input order (newest first). The limit is applied AFTER filtering, so it
// returns the newest N matches. It returns a non-nil slice (possibly empty) and
// never mutates the input. An unmatched filter simply yields an empty slice.
func ApplyHistoryFilters(runs []HistoryRun, f HistoryFilters) []HistoryRun {
	out := make([]HistoryRun, 0, len(runs))
	for _, r := range runs {
		if f.Result != "" && r.Result != f.Result {
			continue
		}
		if f.Workflow != "" && r.Workflow != f.Workflow {
			continue
		}
		if f.Code != "" && !containsString(r.Codes, f.Code) {
			continue
		}
		out = append(out, r)
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out
}

// HistoryListView is the result of listing the history with optional filters. It
// is the JSON contract for `workflow history list` under --json: the matched runs
// (newest first), the active filters (omitted when none), and the shown vs
// total-stored counts. It carries only compact history metadata.
type HistoryListView struct {
	SchemaVersion int             `json:"schemaVersion"`
	Filters       *HistoryFilters `json:"filters,omitempty"`
	Shown         int             `json:"shown"`
	TotalStored   int             `json:"totalStored"`
	Runs          []HistoryRun    `json:"runs"`
}

// BuildHistoryListView applies the filters to a loaded history and builds the
// list view. `filters` is included only when at least one filter is active
// (deterministic: the key is present iff filtering is in effect, and within it
// only active fields appear). It does not mutate h.
func BuildHistoryListView(h History, f HistoryFilters) HistoryListView {
	filtered := ApplyHistoryFilters(h.Runs, f)
	view := HistoryListView{
		SchemaVersion: HistorySchemaVersion,
		Shown:         len(filtered),
		TotalStored:   len(h.Runs),
		Runs:          filtered,
	}
	if f.hasAny() {
		ff := f
		view.Filters = &ff
	}
	return view
}

// HistoryResultCounts counts runs by result (the fixed run-result values).
type HistoryResultCounts struct {
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

// HistoryStepCounts counts steps by status across the in-scope runs.
type HistoryStepCounts struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// WorkflowCount is a per-workflow-name run count.
type WorkflowCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// StepTypeCount is a per-step-type step count.
type StepTypeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// CodeCount is the number of RUNS containing a given emitted code (codes are
// deduped per run, so this counts runs, not total occurrences).
type CodeCount struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

// HistoryStats is the deterministic summary of the (optionally filtered) local
// workflow history (v1.12 Slice 4). It is computed from compact history metadata
// only — never raw reports or referenced files. Grouped counts are arrays sorted
// by count descending then key ascending; the fixed result/step buckets are
// objects with stable keys.
type HistoryStats struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Filters       *HistoryFilters     `json:"filters,omitempty"`
	TotalStored   int                 `json:"totalStored"`
	Shown         int                 `json:"shown"`
	Results       HistoryResultCounts `json:"results"`
	Steps         HistoryStepCounts   `json:"steps"`
	Workflows     []WorkflowCount     `json:"workflows"`
	StepTypes     []StepTypeCount     `json:"stepTypes"`
	Codes         []CodeCount         `json:"codes"`
}

// BuildHistoryStats applies the filters and summarizes the matching runs. It
// reads only compact history metadata and never mutates h. Grouped counts are
// sorted by count descending then key ascending; `filters` is included only when
// at least one filter is active (mirroring BuildHistoryListView). Grouped slices
// are always non-nil (`[]`) for a stable shape.
func BuildHistoryStats(h History, f HistoryFilters) HistoryStats {
	filtered := ApplyHistoryFilters(h.Runs, f)
	stats := HistoryStats{
		SchemaVersion: HistorySchemaVersion,
		TotalStored:   len(h.Runs),
		Shown:         len(filtered),
		Workflows:     []WorkflowCount{},
		StepTypes:     []StepTypeCount{},
		Codes:         []CodeCount{},
	}
	if f.hasAny() {
		ff := f
		stats.Filters = &ff
	}

	workflows := map[string]int{}
	stepTypes := map[string]int{}
	codes := map[string]int{}
	for _, r := range filtered {
		switch r.Result {
		case runResultPassed:
			stats.Results.Passed++
		case runResultFailed:
			stats.Results.Failed++
		}
		workflows[r.Workflow]++
		for _, s := range r.Steps {
			switch s.Status {
			case runStatusPassed:
				stats.Steps.Passed++
			case runStatusFailed:
				stats.Steps.Failed++
			case runStatusSkipped:
				stats.Steps.Skipped++
			}
			stepTypes[s.Type]++
		}
		for _, c := range r.Codes {
			codes[c]++ // codes are deduped per run → counts runs containing c
		}
	}

	for name, count := range workflows {
		stats.Workflows = append(stats.Workflows, WorkflowCount{Name: name, Count: count})
	}
	for typ, count := range stepTypes {
		stats.StepTypes = append(stats.StepTypes, StepTypeCount{Type: typ, Count: count})
	}
	for code, count := range codes {
		stats.Codes = append(stats.Codes, CodeCount{Code: code, Count: count})
	}
	sortCounts(stats.Workflows, func(w WorkflowCount) (int, string) { return w.Count, w.Name })
	sortCounts(stats.StepTypes, func(s StepTypeCount) (int, string) { return s.Count, s.Type })
	sortCounts(stats.Codes, func(c CodeCount) (int, string) { return c.Count, c.Code })
	return stats
}

// sortCounts sorts items by count descending, then by key ascending — a
// deterministic order for the grouped stats tables.
func sortCounts[T any](items []T, key func(T) (int, string)) {
	sort.Slice(items, func(i, j int) bool {
		ci, ki := key(items[i])
		cj, kj := key(items[j])
		if ci != cj {
			return ci > cj
		}
		return ki < kj
	})
}

// containsString reports whether list contains s.
func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// HistoryExportSchemaVersion is the schema version of the workflow-history export
// snapshot (v1.12 Slice 2). It is independent of the on-disk HistorySchemaVersion.
const HistoryExportSchemaVersion = 1

// HistorySafety is the explicit, self-documenting statement of what a workflow
// history export deliberately excludes. Every field is always false — the export
// carries compact metadata only, never raw configs/policies, full reports, logs,
// environment, process data, runtime URLs, or tokens. The flags exist so a reader
// (human or tool) can confirm the snapshot's safety boundary without inspecting it.
type HistorySafety struct {
	ContainsRawConfigs  bool `json:"containsRawConfigs"`
	ContainsRawPolicies bool `json:"containsRawPolicies"`
	ContainsFullReports bool `json:"containsFullReports"`
	ContainsLogs        bool `json:"containsLogs"`
	ContainsEnvironment bool `json:"containsEnvironment"`
	ContainsProcessData bool `json:"containsProcessData"`
	ContainsRuntimeURLs bool `json:"containsRuntimeUrls"`
	ContainsTokens      bool `json:"containsTokens"`
}

// HistoryExport is the deterministic, compact snapshot of the local workflow
// history produced by `workflow history export`. It reuses the same compact
// HistoryRun entries as the store (newest first) plus snapshot metadata and an
// explicit safety statement. It is built locally from the existing history — it
// never carries raw configs, policies, full reports, secrets, or runtime data.
type HistoryExport struct {
	SchemaVersion int           `json:"schemaVersion"`
	CreatedAt     string        `json:"createdAt"`
	Source        string        `json:"source"`
	RunCount      int           `json:"runCount"`
	Runs          []HistoryRun  `json:"runs"`
	Safety        HistorySafety `json:"safety"`
}

// BuildHistoryExport builds the export snapshot from a loaded History. It copies
// the compact run entries verbatim (preserving the store's newest-first order and
// the already-deduped+sorted codes), stamps createdAt (injected for tests), and
// attaches the all-false safety statement. An empty/missing history yields a valid
// snapshot with runCount 0 and an empty runs array. It does not mutate h.
func BuildHistoryExport(h History, createdAt time.Time) HistoryExport {
	runs := h.Runs
	if runs == nil {
		runs = []HistoryRun{} // emit [] (not null) for a stable shape
	}
	return HistoryExport{
		SchemaVersion: HistoryExportSchemaVersion,
		CreatedAt:     createdAt.UTC().Format(time.RFC3339),
		Source:        "workflow-history",
		RunCount:      len(runs),
		Runs:          runs,
		Safety: HistorySafety{
			ContainsRawConfigs:  false,
			ContainsRawPolicies: false,
			ContainsFullReports: false,
			ContainsLogs:        false,
			ContainsEnvironment: false,
			ContainsProcessData: false,
			ContainsRuntimeURLs: false,
			ContainsTokens:      false,
		},
	}
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
