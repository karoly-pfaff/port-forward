package commands_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"portier/cli/sources/client"
	"portier/cli/sources/commands"
	"portier/cli/sources/doctor"
	"portier/cli/sources/version"
)

// doctorServerConfig controls the responses of the live-doctor test server.
type doctorServerConfig struct {
	runtimeVersion string           // version reported by /api/runtime
	statusCode     int              // non-zero overrides /api/status with this status code
	statuses       []map[string]any // body for /api/status (ignored if statusCode set)
	exportCode     int              // non-zero overrides /api/config/export with this status code
	exportRules    []map[string]any // rules for /api/config/export
	recovery       map[string]any   // when set, the recovery block on /api/runtime
}

// makeDoctorServer returns an httptest server serving /api/runtime, /api/status,
// and /api/config/export per the given config.
func makeDoctorServer(t *testing.T, cfg doctorServerConfig) *httptest.Server {
	t.Helper()
	rv := cfg.runtimeVersion
	if rv == "" {
		rv = version.Version
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/runtime":
			recovery := cfg.recovery
			if recovery == nil {
				recovery = map[string]any{"active": false}
			}
			json.NewEncoder(w).Encode(map[string]any{
				"name": "Portier", "version": rv, "runtime": "go",
				"platform": "linux", "arch": "amd64", "uptimeSeconds": 10,
				"startedAt": "2026-01-01T00:00:00Z", "managementHost": "127.0.0.1",
				"managementPort": 47831, "configPath": "/tmp/rules.json",
				"staticDir": "web", "serviceMode": false, "pid": 1234,
				"recovery": recovery,
			})
		case "/api/status":
			if cfg.statusCode != 0 {
				http.Error(w, `{"errors":["boom"]}`, cfg.statusCode)
				return
			}
			json.NewEncoder(w).Encode(cfg.statuses)
		case "/api/config/export":
			if cfg.exportCode != 0 {
				http.Error(w, `{"errors":["boom"]}`, cfg.exportCode)
				return
			}
			rules := cfg.exportRules
			if rules == nil {
				rules = []map[string]any{}
			}
			json.NewEncoder(w).Encode(map[string]any{
				"version": "1", "exportedAt": "2026-01-01T00:00:00Z", "rules": rules,
			})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

func statusEntry(id, health string) map[string]any {
	return map[string]any{"ruleId": id, "running": health == "healthy", "health": health, "bytesIn": 0, "bytesOut": 0}
}

// runDoctorJSON runs `doctor --json` against c and decodes the report.
func runDoctorJSON(t *testing.T, c *client.Client) (doctor.Report, int) {
	t.Helper()
	var out, errBuf strings.Builder
	code := commands.RunDoctor(c, true, nil, &out, &errBuf)
	var report doctor.Report
	if err := json.Unmarshal([]byte(out.String()), &report); err != nil {
		t.Fatalf("decoding doctor JSON: %v\noutput:\n%s", err, out.String())
	}
	return report, code
}

func doctorCodes(report doctor.Report) []string {
	codes := make([]string, len(report.Checks))
	for i, c := range report.Checks {
		codes[i] = c.Code
	}
	return codes
}

func doctorHasCode(report doctor.Report, code string) bool {
	for _, c := range report.Checks {
		if c.Code == code {
			return true
		}
	}
	return false
}

func doctorSeverityOf(report doctor.Report, code string) doctor.Severity {
	for _, c := range report.Checks {
		if c.Code == code {
			return c.Severity
		}
	}
	return ""
}

func TestDoctor_AllHealthy(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses:    []map[string]any{statusEntry("r1", "healthy"), statusEntry("r2", "healthy")},
		exportRules: []map[string]any{{"id": "r1"}, {"id": "r2"}},
	})
	defer srv.Close()
	report, code := runDoctorJSON(t, client.New(srv.URL))

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	want := []string{
		"runtime.reachable", "runtime.version", "runtime.status_read",
		"rules.present", "rules.health_ok", "config.export_read",
	}
	if got := doctorCodes(report); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("codes = %v, want %v", got, want)
	}
	if report.Summary.Error != 0 || report.Summary.Warning != 0 {
		t.Errorf("summary = %+v, want no warnings/errors", report.Summary)
	}
}

func TestDoctor_Unreachable(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{})
	url := srv.URL
	srv.Close() // close so the connection is refused

	report, code := runDoctorJSON(t, client.New(url))
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (doctor ran and found an error)", code)
	}
	if got := doctorCodes(report); len(got) != 1 || got[0] != "runtime.unreachable" {
		t.Errorf("codes = %v, want [runtime.unreachable]", got)
	}
	if report.Checks[0].Severity != "error" {
		t.Errorf("severity = %q, want error", report.Checks[0].Severity)
	}
}

func TestDoctor_VersionMismatch_Warns(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		runtimeVersion: "9.9.9-different",
		statuses:       []map[string]any{statusEntry("r1", "healthy")},
	})
	defer srv.Close()
	report, code := runDoctorJSON(t, client.New(srv.URL))

	if code != 0 {
		t.Errorf("exit code = %d, want 0 (version mismatch is a warning)", code)
	}
	if sev := doctorSeverityOf(report, "runtime.version"); sev != "warning" {
		t.Errorf("runtime.version severity = %q, want warning", sev)
	}
}

func TestDoctor_VersionMatch_Info(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		runtimeVersion: version.Version,
		statuses:       []map[string]any{statusEntry("r1", "healthy")},
	})
	defer srv.Close()
	report, _ := runDoctorJSON(t, client.New(srv.URL))

	if sev := doctorSeverityOf(report, "runtime.version"); sev != "info" {
		t.Errorf("runtime.version severity = %q, want info", sev)
	}
}

func TestDoctor_RecoveryActive_Warns(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses: []map[string]any{statusEntry("r1", "healthy")},
		recovery: map[string]any{
			"active": true, "reason": "malformed",
			"message": "bad config", "configPath": "/tmp/rules.json",
			"quarantinePath": "/tmp/rules.json.corrupt-x", "writesBlocked": true,
			"detectedAt": "2026-01-01T00:00:00Z",
		},
	})
	defer srv.Close()
	report, code := runDoctorJSON(t, client.New(srv.URL))

	if code != 0 {
		t.Errorf("exit code = %d, want 0 (recovery is a warning)", code)
	}
	if !doctorHasCode(report, "config.recovery_active") {
		t.Errorf("missing config.recovery_active; got %v", doctorCodes(report))
	}
	if doctorSeverityOf(report, "config.recovery_active") != "warning" {
		t.Errorf("config.recovery_active should be a warning")
	}
}

func TestDoctor_RecoveryActive_StrictExit1(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses: []map[string]any{statusEntry("r1", "healthy")},
		recovery: map[string]any{"active": true, "reason": "unreadable", "writesBlocked": true},
	})
	defer srv.Close()
	var out, errBuf strings.Builder
	code := commands.RunDoctor(client.New(srv.URL), false, []string{"--strict"}, &out, &errBuf)
	if code != 1 {
		t.Errorf("strict exit = %d, want 1 (recovery warning fails under --strict)", code)
	}
}

func TestDoctor_RecoveryInactive_NoCheck(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses: []map[string]any{statusEntry("r1", "healthy")},
	})
	defer srv.Close()
	report, code := runDoctorJSON(t, client.New(srv.URL))

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if doctorHasCode(report, "config.recovery_active") {
		t.Errorf("inactive recovery must not emit a check; got %v", doctorCodes(report))
	}
}

func TestDoctor_NoRules_Warns(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{}})
	defer srv.Close()
	report, code := runDoctorJSON(t, client.New(srv.URL))

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !doctorHasCode(report, "rules.none") {
		t.Errorf("missing rules.none; got %v", doctorCodes(report))
	}
	if doctorSeverityOf(report, "rules.none") != "warning" {
		t.Errorf("rules.none should be a warning")
	}
	// No health checks when there are no rules.
	if doctorHasCode(report, "rules.health_ok") || doctorHasCode(report, "rules.present") {
		t.Errorf("no rules → no rules.present / health checks; got %v", doctorCodes(report))
	}
}

func TestDoctor_HealthWarning(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses: []map[string]any{statusEntry("r1", "healthy"), statusEntry("r2", "warning")},
	})
	defer srv.Close()
	report, code := runDoctorJSON(t, client.New(srv.URL))

	if code != 0 {
		t.Errorf("exit code = %d, want 0 (warning health alone is not an error)", code)
	}
	if !doctorHasCode(report, "rules.health_warning") {
		t.Errorf("missing rules.health_warning; got %v", doctorCodes(report))
	}
	if doctorHasCode(report, "rules.health_ok") {
		t.Errorf("rules.health_ok must not appear alongside a warning")
	}
}

func TestDoctor_HealthError_Exit1(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses: []map[string]any{statusEntry("r1", "healthy"), statusEntry("r2", "error")},
	})
	defer srv.Close()
	report, code := runDoctorJSON(t, client.New(srv.URL))

	if code != 1 {
		t.Errorf("exit code = %d, want 1 (error health is an error-severity check)", code)
	}
	if !doctorHasCode(report, "rules.health_error") {
		t.Errorf("missing rules.health_error; got %v", doctorCodes(report))
	}
	if doctorSeverityOf(report, "rules.health_error") != "error" {
		t.Errorf("rules.health_error should be error severity")
	}
}

func TestDoctor_MixedHealth_BothReported(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses: []map[string]any{statusEntry("r1", "warning"), statusEntry("r2", "error")},
	})
	defer srv.Close()
	report, code := runDoctorJSON(t, client.New(srv.URL))

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !doctorHasCode(report, "rules.health_error") || !doctorHasCode(report, "rules.health_warning") {
		t.Errorf("mixed fleet should report both health_error and health_warning; got %v", doctorCodes(report))
	}
	// error before warning (deterministic order).
	codes := doctorCodes(report)
	ei, wi := indexOf(codes, "rules.health_error"), indexOf(codes, "rules.health_warning")
	if ei > wi {
		t.Errorf("rules.health_error (%d) should come before rules.health_warning (%d): %v", ei, wi, codes)
	}
}

func TestDoctor_StatusFailed_Exit1(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statusCode: http.StatusInternalServerError})
	defer srv.Close()
	report, code := runDoctorJSON(t, client.New(srv.URL))

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !doctorHasCode(report, "runtime.status_failed") {
		t.Errorf("missing runtime.status_failed; got %v", doctorCodes(report))
	}
	// A status failure does not stop the read-only config-export check.
	if !doctorHasCode(report, "config.export_read") {
		t.Errorf("config export check should still run after a status failure; got %v", doctorCodes(report))
	}
	if doctorHasCode(report, "rules.present") || doctorHasCode(report, "rules.none") {
		t.Errorf("no rules checks when status read failed; got %v", doctorCodes(report))
	}
}

func TestDoctor_ConfigExportFailed_Exit1(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses:   []map[string]any{statusEntry("r1", "healthy")},
		exportCode: http.StatusInternalServerError,
	})
	defer srv.Close()
	report, code := runDoctorJSON(t, client.New(srv.URL))

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if doctorSeverityOf(report, "config.export_failed") != "error" {
		t.Errorf("missing/incorrect config.export_failed; got %v", doctorCodes(report))
	}
}

func TestDoctor_JSONShape(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	var out, errBuf strings.Builder
	commands.RunDoctor(client.New(srv.URL), true, nil, &out, &errBuf)

	var raw map[string]any
	if err := json.Unmarshal([]byte(out.String()), &raw); err != nil {
		t.Fatalf("decoding JSON: %v", err)
	}
	if _, ok := raw["checks"]; !ok {
		t.Errorf("JSON missing \"checks\": %v", raw)
	}
	summary, ok := raw["summary"].(map[string]any)
	if !ok {
		t.Fatalf("JSON missing \"summary\" object: %v", raw)
	}
	for _, key := range []string{"info", "warning", "error"} {
		if _, ok := summary[key]; !ok {
			t.Errorf("summary missing %q: %v", key, summary)
		}
	}
}

func TestDoctor_HumanOutput(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses: []map[string]any{statusEntry("r1", "warning"), statusEntry("r2", "error")},
	})
	defer srv.Close()
	var out, errBuf strings.Builder
	code := commands.RunDoctor(client.New(srv.URL), false, nil, &out, &errBuf)

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	s := out.String()
	for _, want := range []string{"Portier Doctor", "[INFO]", "[WARN]", "[ERROR]", "Summary:"} {
		if !strings.Contains(s, want) {
			t.Errorf("human output missing %q\noutput:\n%s", want, s)
		}
	}
}

func TestDoctor_JSONEncodeFailure_Exit1(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	var errBuf strings.Builder
	code := commands.RunDoctor(client.New(srv.URL), true, nil, failingWriter{}, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "Error encoding JSON") {
		t.Errorf("stderr missing encode-error message: %q", errBuf.String())
	}
}

func TestDoctor_UnexpectedArg_Exit2(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{})
	defer srv.Close()
	var out, errBuf strings.Builder
	code := commands.RunDoctor(client.New(srv.URL), false, []string{"extra"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "takes no arguments") {
		t.Errorf("stderr missing usage message: %q", errBuf.String())
	}
}

func TestDoctor_Help_Exit0(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunDoctor(nil, false, []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "portier doctor") {
		t.Errorf("help output missing usage: %q", out.String())
	}
}

func TestDoctor_BadFlag_Exit2(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunDoctor(nil, false, []string{"--nope"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

// --- strict mode (v1.9 Slice 4) ---

func TestDoctor_WarningOnly_NormalExit0(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses: []map[string]any{statusEntry("r1", "healthy"), statusEntry("r2", "warning")},
	})
	defer srv.Close()
	var out, errBuf strings.Builder
	code := commands.RunDoctor(client.New(srv.URL), false, nil, &out, &errBuf)
	if code != 0 {
		t.Errorf("normal exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Result: passed") {
		t.Errorf("normal warning-only output should report Result: passed\n%s", out.String())
	}
}

func TestDoctor_WarningOnly_StrictExit1(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses: []map[string]any{statusEntry("r1", "healthy"), statusEntry("r2", "warning")},
	})
	defer srv.Close()
	var out, errBuf strings.Builder
	code := commands.RunDoctor(client.New(srv.URL), false, []string{"--strict"}, &out, &errBuf)
	if code != 1 {
		t.Errorf("strict exit = %d, want 1", code)
	}
	s := out.String()
	if !strings.Contains(s, "Strict mode: warnings are treated as failures.") {
		t.Errorf("strict output missing strict note\n%s", s)
	}
	if !strings.Contains(s, "Result: failed") {
		t.Errorf("strict warning-only output should report Result: failed\n%s", s)
	}
}

func TestDoctor_Error_StrictAndNormalExit1(t *testing.T) {
	cfg := doctorServerConfig{
		statuses: []map[string]any{statusEntry("r1", "error")},
	}
	srv := makeDoctorServer(t, cfg)
	defer srv.Close()
	var out, errBuf strings.Builder
	if code := commands.RunDoctor(client.New(srv.URL), false, nil, &out, &errBuf); code != 1 {
		t.Errorf("normal exit = %d, want 1", code)
	}
	out.Reset()
	errBuf.Reset()
	if code := commands.RunDoctor(client.New(srv.URL), false, []string{"--strict"}, &out, &errBuf); code != 1 {
		t.Errorf("strict exit = %d, want 1", code)
	}
}

func TestDoctor_AllInfo_StrictExit0(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses:    []map[string]any{statusEntry("r1", "healthy")},
		exportRules: []map[string]any{{"id": "r1"}},
	})
	defer srv.Close()
	var out, errBuf strings.Builder
	code := commands.RunDoctor(client.New(srv.URL), false, []string{"--strict"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("strict exit = %d, want 0 (all info, no warnings)", code)
	}
}

func TestDoctor_StrictJSON_Fields(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses: []map[string]any{statusEntry("r1", "warning")},
	})
	defer srv.Close()
	var out, errBuf strings.Builder
	code := commands.RunDoctor(client.New(srv.URL), true, []string{"--strict"}, &out, &errBuf)
	if code != 1 {
		t.Errorf("strict JSON exit = %d, want 1", code)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(out.String()), &raw); err != nil {
		t.Fatalf("decoding JSON: %v\n%s", err, out.String())
	}
	if raw["strict"] != true {
		t.Errorf("strict field = %v, want true", raw["strict"])
	}
	if raw["result"] != "failed" {
		t.Errorf("result field = %v, want failed", raw["result"])
	}
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}
