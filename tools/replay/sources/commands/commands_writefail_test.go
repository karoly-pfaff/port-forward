package commands

import (
	"bytes"
	"fmt"
	"testing"
)

// failWriter fails every Write — used to exercise the stdout-write-failure error
// path in the --json output of each command (a real error case: the process can't
// write its JSON result to stdout).
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("stdout write failed") }

func TestPlan_JSONStdoutWriteFailure(t *testing.T) {
	path := writeTemp(t, "run.json", runReportJSON)
	if code := Run([]string{"--json", "plan", "--from", path}, failWriter{}, &bytes.Buffer{}); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

func TestAnalyze_JSONStdoutWriteFailure(t *testing.T) {
	path := writeTemp(t, "run.json", runReportJSON)
	if code := Run([]string{"--json", "analyze", "--from", path}, failWriter{}, &bytes.Buffer{}); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

func TestTimeline_JSONStdoutWriteFailure(t *testing.T) {
	path := writeTemp(t, "run.json", runReportJSON)
	if code := Run([]string{"--json", "timeline", "--from", path}, failWriter{}, &bytes.Buffer{}); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}
