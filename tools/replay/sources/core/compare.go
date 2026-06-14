package core

// Offline comparison of two saved Portier workflow artifacts. `compare` parses two
// artifacts and reports deterministic differences (changed result, added/removed
// emitted codes, changed step/run sets, distribution deltas, …) using ONLY the
// artifacts' own contents.
//
// Like the rest of the replay tool it is strictly offline and read-only: it parses
// only the supplied `--left`/`--right` JSON files (or, for a support-report bundle
// directory, the bundle's own manifest.json / report.json) and NEVER executes
// workflows, contacts the runtime, reads the config/policy/baseline/report files an
// artifact refers to, mutates inputs, applies/imports, enforces policy, runs shell
// commands, uploads, or collects logs/environment/process data.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// CompareSchemaVersion is the schema version of the replay comparison. It is a
// local tool schema (not a REST/API contract), distinct from the plan/analysis/
// timeline schemas.
const CompareSchemaVersion = 1

// CompareSide describes one artifact in a comparison.
type CompareSide struct {
	Source   string `json:"source"`
	Workflow string `json:"workflow,omitempty"`
	Result   string `json:"result,omitempty"`
}

// CompareChange is one detected difference. Left/Right carry scalar before/after
// values; Added/Removed carry set differences. Only the relevant fields are set.
type CompareChange struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Message string   `json:"message"`
	Left    string   `json:"left,omitempty"`
	Right   string   `json:"right,omitempty"`
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

// CompareSet is a full set diff for one dimension (e.g. emitted codes). All three
// slices are always present (`[]` when empty).
type CompareSet struct {
	Added     []string `json:"added"`
	Removed   []string `json:"removed"`
	Unchanged []string `json:"unchanged"`
}

// CompareSummary is the high-level comparison verdict.
type CompareSummary struct {
	SameKind    bool `json:"sameKind"`
	Changed     bool `json:"changed"`
	ChangeCount int  `json:"changeCount"`
}

// Compare is the deterministic comparison of two artifacts. Sets is omitted for a
// mixed-kind (limited) comparison.
type Compare struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Left          CompareSide           `json:"left"`
	Right         CompareSide           `json:"right"`
	Summary       CompareSummary        `json:"summary"`
	Changes       []CompareChange       `json:"changes"`
	Sets          map[string]CompareSet `json:"sets,omitempty"`
	Insights      []string              `json:"insights"`
}

// compareFacts is the offline data compare needs about one artifact, parsed once.
// It reuses the timeline raw structs and the shared StepSummary/RunSummary types.
type compareFacts struct {
	Source        string
	Workflow      string
	Result        string
	Steps         StepSummary
	Runs          RunSummary
	Codes         []string       // sorted unique emitted codes
	Workflows     map[string]int // workflow-name -> run count (history)
	FailedSteps   []string       // sorted (run/bundle)
	SkippedSteps  []string       // sorted (run)
	InvalidSteps  []string       // sorted (plan)
	FailedRuns    []string       // sorted (history)
	MostRecentRun string         // history runs[0] id
}

// CompareInputs parses two artifact paths (files or bundle directories) and builds
// their deterministic comparison. It reads only those artifacts (and, for a bundle
// directory, only the bundle's own manifest/report files) and contacts nothing.
func CompareInputs(left, right string) (Compare, error) {
	lf, err := extractFacts(left)
	if err != nil {
		return Compare{}, err
	}
	rf, err := extractFacts(right)
	if err != nil {
		return Compare{}, err
	}
	return BuildCompare(lf, rf), nil
}

// BuildCompare builds the comparison from two extracted facts. A mixed-kind
// comparison is deliberately limited (source comparison + a clear marker), never a
// crash and never a false claim of semantic equivalence.
func BuildCompare(l, r compareFacts) Compare {
	c := Compare{
		SchemaVersion: CompareSchemaVersion,
		Left:          CompareSide{Source: l.Source, Workflow: l.Workflow, Result: l.Result},
		Right:         CompareSide{Source: r.Source, Workflow: r.Workflow, Result: r.Result},
		Changes:       []CompareChange{},
		Insights:      []string{},
	}
	c.Summary.SameKind = l.Source == r.Source

	if !c.Summary.SameKind {
		c.Changes = append(c.Changes, CompareChange{
			ID:      "kind",
			Type:    "kind.mismatch",
			Message: "Artifacts are different kinds; detailed comparison is limited.",
			Left:    l.Source,
			Right:   r.Source,
		})
		c.Insights = append(c.Insights, "Use replay analyze on each artifact for deeper single-artifact analysis.")
		c.Summary.ChangeCount = len(c.Changes)
		c.Summary.Changed = true
		return c
	}

	var changes []CompareChange
	var sets map[string]CompareSet
	var insights []string
	switch l.Source {
	case SourceWorkflowRunReport:
		changes, sets, insights = compareRunReports(l, r)
	case SourceWorkflowPlanReport:
		changes, sets, insights = comparePlanReports(l, r)
	case SourceWorkflowHistoryExport:
		changes, sets, insights = compareHistory(l, r)
	case SourceWorkflowReportBundle:
		changes, sets, insights = compareBundles(l, r)
	}

	c.Changes = changes
	if c.Changes == nil {
		c.Changes = []CompareChange{}
	}
	if len(sets) > 0 {
		c.Sets = sets
	}
	c.Summary.ChangeCount = len(c.Changes)
	c.Summary.Changed = c.Summary.ChangeCount > 0
	if len(changes) == 0 {
		insights = append(insights, "No meaningful differences detected.")
	}
	c.Insights = insights
	return c
}

// --- per-kind comparisons ---

func compareRunReports(l, r compareFacts) ([]CompareChange, map[string]CompareSet, []string) {
	var changes []CompareChange
	var insights []string
	sets := map[string]CompareSet{}

	stringChange(&changes, "workflow", "Workflow", l.Workflow, r.Workflow)
	if l.Result != r.Result {
		msg := fmt.Sprintf("Result changed from %s to %s.", l.Result, r.Result)
		changes = append(changes, CompareChange{ID: "result", Type: "result.changed", Message: msg, Left: l.Result, Right: r.Result})
		insights = append(insights, msg)
	}
	intChange(&changes, "steps.total", "Step total", l.Steps.Total, r.Steps.Total)
	intChange(&changes, "steps.passed", "Passed step count", l.Steps.Passed, r.Steps.Passed)
	intChange(&changes, "steps.failed", "Failed step count", l.Steps.Failed, r.Steps.Failed)
	intChange(&changes, "steps.skipped", "Skipped step count", l.Steps.Skipped, r.Steps.Skipped)

	codes := diffSets(l.Codes, r.Codes)
	sets["codes"] = codes
	addSetChanges(&changes, "codes", "New emitted codes appeared.", "Some emitted codes are gone.", codes)
	if len(codes.Added) > 0 {
		insights = append(insights, "New emitted codes appeared.")
	}

	failed := diffSets(l.FailedSteps, r.FailedSteps)
	sets["failedSteps"] = failed
	addSetChanges(&changes, "failedSteps", "New failed steps appeared.", "Some failed steps are resolved.", failed)
	if len(failed.Added) > 0 || len(failed.Removed) > 0 {
		insights = append(insights, "Failed steps changed.")
	}

	skipped := diffSets(l.SkippedSteps, r.SkippedSteps)
	sets["skippedSteps"] = skipped
	addSetChanges(&changes, "skippedSteps", "New skipped steps appeared.", "Some skipped steps are gone.", skipped)

	return changes, sets, insights
}

func comparePlanReports(l, r compareFacts) ([]CompareChange, map[string]CompareSet, []string) {
	var changes []CompareChange
	var insights []string
	sets := map[string]CompareSet{}

	stringChange(&changes, "workflow", "Workflow", l.Workflow, r.Workflow)
	if l.Result != r.Result {
		changes = append(changes, CompareChange{
			ID: "result", Type: "result.changed",
			Message: fmt.Sprintf("Plan result changed from %s to %s.", l.Result, r.Result),
			Left:    l.Result, Right: r.Result,
		})
		switch {
		case l.Result == statusInvalid && r.Result == statusValid:
			insights = append(insights, "Plan became valid.")
		case l.Result == statusValid && r.Result == statusInvalid:
			insights = append(insights, "Plan became invalid.")
		}
	}
	intChange(&changes, "steps.total", "Step total", l.Steps.Total, r.Steps.Total)
	intChange(&changes, "steps.valid", "Valid step count", l.Steps.Valid, r.Steps.Valid)
	intChange(&changes, "steps.invalid", "Invalid step count", l.Steps.Invalid, r.Steps.Invalid)

	codes := diffSets(l.Codes, r.Codes)
	sets["codes"] = codes
	addSetChanges(&changes, "codes", "New validation codes appeared.", "Some validation codes are gone.", codes)

	invalid := diffSets(l.InvalidSteps, r.InvalidSteps)
	sets["invalidSteps"] = invalid
	addSetChanges(&changes, "invalidSteps", "New invalid steps appeared.", "Some invalid steps are now valid.", invalid)
	if len(invalid.Added) > 0 || len(invalid.Removed) > 0 {
		insights = append(insights, "Invalid step set changed.")
	}

	return changes, sets, insights
}

func compareHistory(l, r compareFacts) ([]CompareChange, map[string]CompareSet, []string) {
	var changes []CompareChange
	var insights []string
	sets := map[string]CompareSet{}

	intChange(&changes, "runs.total", "Run count", l.Runs.Total, r.Runs.Total)
	intChange(&changes, "runs.passed", "Passed run count", l.Runs.Passed, r.Runs.Passed)
	intChange(&changes, "runs.failed", "Failed run count", l.Runs.Failed, r.Runs.Failed)

	// Workflow distribution deltas, sorted by workflow name ascending.
	for _, name := range sortedUnionKeys(l.Workflows, r.Workflows) {
		lc, rc := l.Workflows[name], r.Workflows[name]
		if lc != rc {
			changes = append(changes, CompareChange{
				ID: "workflows." + name, Type: "workflows.delta",
				Message: fmt.Sprintf("Workflow %q run count changed from %d to %d.", name, lc, rc),
				Left:    strconv.Itoa(lc), Right: strconv.Itoa(rc),
			})
		}
	}

	codes := diffSets(l.Codes, r.Codes)
	sets["codes"] = codes
	addSetChanges(&changes, "codes", "New codes appeared in history.", "Some codes no longer appear in history.", codes)

	failedRuns := diffSets(l.FailedRuns, r.FailedRuns)
	sets["failedRuns"] = failedRuns
	addSetChanges(&changes, "failedRuns", "New failed runs appeared.", "Some failed runs are gone.", failedRuns)

	if l.MostRecentRun != r.MostRecentRun {
		changes = append(changes, CompareChange{
			ID: "mostRecentRun", Type: "mostRecentRun.changed",
			Message: fmt.Sprintf("Most recent run changed from %s to %s.", valOr(l.MostRecentRun), valOr(r.MostRecentRun)),
			Left:    l.MostRecentRun, Right: r.MostRecentRun,
		})
	}

	if l.Runs.Total == 0 && r.Runs.Total == 0 {
		insights = append(insights, "History export contains no comparable runs.")
	}
	switch {
	case r.Runs.Failed > l.Runs.Failed:
		insights = append(insights, "Failed run count increased.")
	case r.Runs.Failed < l.Runs.Failed:
		insights = append(insights, "Failed run count decreased.")
	}
	if len(codes.Added) > 0 {
		insights = append(insights, "New codes appeared in history.")
	}

	return changes, sets, insights
}

func compareBundles(l, r compareFacts) ([]CompareChange, map[string]CompareSet, []string) {
	var changes []CompareChange
	var insights []string
	sets := map[string]CompareSet{}

	stringChange(&changes, "workflow", "Workflow", l.Workflow, r.Workflow)
	if l.Result != r.Result {
		msg := fmt.Sprintf("Result changed from %s to %s.", valOr(l.Result), valOr(r.Result))
		changes = append(changes, CompareChange{ID: "result", Type: "result.changed", Message: msg, Left: l.Result, Right: r.Result})
		insights = append(insights, msg)
	}
	intChange(&changes, "steps.total", "Step total", l.Steps.Total, r.Steps.Total)
	intChange(&changes, "steps.passed", "Passed step count", l.Steps.Passed, r.Steps.Passed)
	intChange(&changes, "steps.failed", "Failed step count", l.Steps.Failed, r.Steps.Failed)
	intChange(&changes, "steps.skipped", "Skipped step count", l.Steps.Skipped, r.Steps.Skipped)
	intChange(&changes, "steps.valid", "Valid step count", l.Steps.Valid, r.Steps.Valid)
	intChange(&changes, "steps.invalid", "Invalid step count", l.Steps.Invalid, r.Steps.Invalid)

	codes := diffSets(l.Codes, r.Codes)
	sets["codes"] = codes
	addSetChanges(&changes, "codes", "New emitted codes appeared.", "Some emitted codes are gone.", codes)
	if len(codes.Added) > 0 {
		insights = append(insights, "New emitted codes appeared.")
	}

	return changes, sets, insights
}

// --- facts extraction (single parse per artifact) ---

func extractFacts(path string) (compareFacts, error) {
	info, err := os.Stat(path)
	if err != nil {
		return compareFacts{}, err
	}
	if info.IsDir() {
		return bundleFacts(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return compareFacts{}, err
	}
	return factsFromJSON(data)
}

func factsFromJSON(data []byte) (compareFacts, error) {
	in, err := DetectJSON(data)
	if err != nil {
		return compareFacts{}, err
	}
	switch in.Kind {
	case SourceWorkflowRunReport:
		return runReportFacts(data)
	case SourceWorkflowPlanReport:
		return planReportFacts(data)
	case SourceWorkflowHistoryExport:
		return historyFacts(data)
	default:
		return compareFacts{}, fmt.Errorf("unsupported artifact for compare")
	}
}

func runReportFacts(data []byte) (compareFacts, error) {
	var raw timelineRawReport
	if err := json.Unmarshal(data, &raw); err != nil {
		return compareFacts{}, fmt.Errorf("not a valid workflow run report: %w", err)
	}
	f := compareFacts{Source: SourceWorkflowRunReport, Workflow: raw.Workflow, Result: raw.Result}
	codeSet := map[string]bool{}
	for _, s := range raw.Steps {
		tallyStep(&f.Steps, s.Status)
		switch s.Status {
		case statusFailed:
			f.FailedSteps = append(f.FailedSteps, s.ID)
			for _, c := range embeddedFindingCodes(s.Report) {
				codeSet[c] = true
			}
		case statusSkipped:
			f.SkippedSteps = append(f.SkippedSteps, s.ID)
			codeSet[codeDependencyFailed] = true
		}
	}
	for _, k := range sortedMapKeys(raw.Explanations) {
		codeSet[k] = true
	}
	f.Codes = sortedSetKeys(codeSet)
	sort.Strings(f.FailedSteps)
	sort.Strings(f.SkippedSteps)
	return f, nil
}

func planReportFacts(data []byte) (compareFacts, error) {
	var raw timelineRawReport
	if err := json.Unmarshal(data, &raw); err != nil {
		return compareFacts{}, fmt.Errorf("not a valid workflow plan report: %w", err)
	}
	f := compareFacts{Source: SourceWorkflowPlanReport, Workflow: raw.Name, Result: raw.Result}
	codeSet := map[string]bool{}
	for _, s := range raw.Steps {
		tallyStep(&f.Steps, s.Status)
		if s.Status == statusInvalid {
			f.InvalidSteps = append(f.InvalidSteps, s.ID)
			if s.Code != "" {
				codeSet[s.Code] = true
			}
		}
	}
	for _, k := range sortedMapKeys(raw.Explanations) {
		codeSet[k] = true
	}
	f.Codes = sortedSetKeys(codeSet)
	sort.Strings(f.InvalidSteps)
	return f, nil
}

func historyFacts(data []byte) (compareFacts, error) {
	var raw timelineRawHistory
	if err := json.Unmarshal(data, &raw); err != nil {
		return compareFacts{}, fmt.Errorf("not a valid workflow history export: %w", err)
	}
	f := compareFacts{Source: SourceWorkflowHistoryExport, Workflows: map[string]int{}}
	codeSet := map[string]bool{}
	for _, run := range raw.Runs {
		f.Runs.Total++
		switch run.Result {
		case statusPassed:
			f.Runs.Passed++
		case statusFailed:
			f.Runs.Failed++
			f.FailedRuns = append(f.FailedRuns, run.ID)
		}
		if run.Workflow != "" {
			f.Workflows[run.Workflow]++
		}
		seen := map[string]bool{}
		for _, c := range run.Codes {
			if seen[c] {
				continue
			}
			seen[c] = true
			codeSet[c] = true
		}
	}
	if len(raw.Runs) > 0 {
		f.MostRecentRun = raw.Runs[0].ID // export is newest-first
	}
	f.Codes = sortedSetKeys(codeSet)
	sort.Strings(f.FailedRuns)
	return f, nil
}

// bundleFacts reads ONLY the bundle's own manifest.json (required) and report.json.
func bundleFacts(dir string) (compareFacts, error) {
	manifestData, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return compareFacts{}, fmt.Errorf("not a workflow report bundle (no readable manifest.json): %w", err)
	}
	if _, err := DetectBundleManifest(manifestData); err != nil {
		return compareFacts{}, err
	}
	var man analyzeBundleManifest
	_ = json.Unmarshal(manifestData, &man)

	var rep timelineBundleReport
	if data, err := os.ReadFile(filepath.Join(dir, "report.json")); err == nil {
		_ = json.Unmarshal(data, &rep)
	}

	f := compareFacts{
		Source:   SourceWorkflowReportBundle,
		Workflow: firstNonEmpty(man.Workflow, rep.Workflow),
		Result:   firstNonEmpty(man.Result, rep.Result),
	}
	codeSet := map[string]bool{}
	for _, s := range rep.Steps {
		tallyStep(&f.Steps, s.Status)
		if s.Status == statusFailed || s.Status == statusInvalid {
			f.FailedSteps = append(f.FailedSteps, s.ID)
		}
		for _, c := range s.Codes {
			codeSet[c] = true
		}
	}
	f.Codes = sortedSetKeys(codeSet)
	sort.Strings(f.FailedSteps)
	return f, nil
}

// --- helpers ---

// diffSets returns the added/removed/unchanged diff of two sorted-unique sets, all
// slices non-nil (serialize as `[]`).
func diffSets(left, right []string) CompareSet {
	lset := toBoolSet(left)
	rset := toBoolSet(right)
	added := []string{}
	removed := []string{}
	unchanged := []string{}
	for _, v := range right {
		if lset[v] {
			unchanged = append(unchanged, v)
		} else {
			added = append(added, v)
		}
	}
	for _, v := range left {
		if !rset[v] {
			removed = append(removed, v)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(unchanged)
	return CompareSet{Added: added, Removed: removed, Unchanged: unchanged}
}

// addSetChanges appends an "<idBase>.added" and/or "<idBase>.removed" change for a
// non-empty set diff.
func addSetChanges(changes *[]CompareChange, idBase, addedMsg, removedMsg string, set CompareSet) {
	if len(set.Added) > 0 {
		*changes = append(*changes, CompareChange{ID: idBase + ".added", Type: idBase + ".added", Message: addedMsg, Added: set.Added})
	}
	if len(set.Removed) > 0 {
		*changes = append(*changes, CompareChange{ID: idBase + ".removed", Type: idBase + ".removed", Message: removedMsg, Removed: set.Removed})
	}
}

// intChange appends a scalar count change when l != r.
func intChange(changes *[]CompareChange, id, label string, l, r int) {
	if l != r {
		*changes = append(*changes, CompareChange{
			ID: id, Type: id + ".changed",
			Message: fmt.Sprintf("%s changed from %d to %d.", label, l, r),
			Left:    strconv.Itoa(l), Right: strconv.Itoa(r),
		})
	}
}

// stringChange appends a scalar string change when l != r.
func stringChange(changes *[]CompareChange, id, label, l, r string) {
	if l != r {
		*changes = append(*changes, CompareChange{
			ID: id, Type: id + ".changed",
			Message: fmt.Sprintf("%s changed from %s to %s.", label, valOr(l), valOr(r)),
			Left:    l, Right: r,
		})
	}
}

func toBoolSet(list []string) map[string]bool {
	m := make(map[string]bool, len(list))
	for _, v := range list {
		m[v] = true
	}
	return m
}

func sortedSetKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedUnionKeys(a, b map[string]int) []string {
	m := map[string]bool{}
	for k := range a {
		m[k] = true
	}
	for k := range b {
		m[k] = true
	}
	return sortedSetKeys(m)
}

func valOr(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
