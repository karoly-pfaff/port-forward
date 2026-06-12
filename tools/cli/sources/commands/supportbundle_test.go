package commands_test

// Black-box tests for `portier support-bundle` (v1.9 Slice 6): it collects
// deterministic doctor artifacts into a directory, reuses the doctor report
// JSON schema, still produces a useful bundle when the runtime is unreachable,
// and never mutates the runtime/config.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/client"
	"portier/cli/sources/commands"
)

func readJSONFile(t *testing.T, path string, into any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatalf("decoding %s: %v\n%s", path, err, data)
	}
}

func fileExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

func TestSupportBundle_CreatesExpectedFiles(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses:    []map[string]any{statusEntry("r1", "healthy")},
		exportRules: []map[string]any{{"id": "r1"}},
	})
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "bundle")

	var stdout, stderr strings.Builder
	code := commands.RunSupportBundle(client.New(srv.URL), false, []string{"--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", code, stderr.String())
	}

	for _, name := range []string{"manifest.json", "doctor.json", "doctor.txt", "explanations.json", "runtime.json", "config-export.json"} {
		if !fileExists(t, filepath.Join(out, name)) {
			t.Errorf("expected bundle file %s to exist", name)
		}
	}
	if !strings.Contains(stdout.String(), "Support bundle written to") {
		t.Errorf("human stdout should confirm the bundle:\n%s", stdout.String())
	}
}

func TestSupportBundle_ManifestShape(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "bundle")

	var stdout, stderr strings.Builder
	commands.RunSupportBundle(client.New(srv.URL), false, []string{"--out", out}, &stdout, &stderr)

	var manifest map[string]any
	readJSONFile(t, filepath.Join(out, "manifest.json"), &manifest)

	if manifest["schemaVersion"].(float64) != 1 {
		t.Errorf("schemaVersion = %v, want 1", manifest["schemaVersion"])
	}
	for _, key := range []string{"generatedAt", "cliVersion", "source", "generatedBy", "runtimeUrl", "result", "artifacts"} {
		if _, ok := manifest[key]; !ok {
			t.Errorf("manifest missing %q: %v", key, manifest)
		}
	}
	if manifest["result"] != "passed" {
		t.Errorf("result = %v, want passed", manifest["result"])
	}
	arts, _ := manifest["artifacts"].([]any)
	if len(arts) == 0 {
		t.Errorf("artifacts list is empty: %v", manifest["artifacts"])
	}
}

func TestSupportBundle_DoctorJSONSchema(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "bundle")

	var stdout, stderr strings.Builder
	commands.RunSupportBundle(client.New(srv.URL), false, []string{"--out", out}, &stdout, &stderr)

	var doctor map[string]any
	readJSONFile(t, filepath.Join(out, "doctor.json"), &doctor)
	for _, key := range []string{"checks", "summary", "strict", "result"} {
		if _, ok := doctor[key]; !ok {
			t.Errorf("doctor.json missing %q (must match doctor --json schema): %v", key, doctor)
		}
	}
}

func TestSupportBundle_DoctorTxtContent(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "bundle")

	var stdout, stderr strings.Builder
	commands.RunSupportBundle(client.New(srv.URL), false, []string{"--out", out}, &stdout, &stderr)

	data, err := os.ReadFile(filepath.Join(out, "doctor.txt"))
	if err != nil {
		t.Fatalf("reading doctor.txt: %v", err)
	}
	s := string(data)
	for _, want := range []string{"Portier Doctor", "Summary:", "Result: passed"} {
		if !strings.Contains(s, want) {
			t.Errorf("doctor.txt missing %q\n%s", want, s)
		}
	}
}

func TestSupportBundle_ExplanationsIncludeAllCodes(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "bundle")

	var stdout, stderr strings.Builder
	commands.RunSupportBundle(client.New(srv.URL), false, []string{"--out", out}, &stdout, &stderr)

	var list []commands.Explanation
	readJSONFile(t, filepath.Join(out, "explanations.json"), &list)
	if len(list) != 20 {
		t.Errorf("explanations.json has %d entries, want 20", len(list))
	}
}

func TestSupportBundle_UnreachableStillCreatesBundle_Exit1(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{})
	url := srv.URL
	srv.Close() // unreachable

	out := filepath.Join(t.TempDir(), "bundle")
	var stdout, stderr strings.Builder
	code := commands.RunSupportBundle(client.New(url), false, []string{"--out", out}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (doctor report has runtime.unreachable error)", code)
	}

	// A useful bundle still exists: manifest + doctor report + explanations.
	for _, name := range []string{"manifest.json", "doctor.json", "doctor.txt", "explanations.json"} {
		if !fileExists(t, filepath.Join(out, name)) {
			t.Errorf("expected %s to exist even when runtime unreachable", name)
		}
	}
	// runtime.json / config-export.json are omitted with a manifest warning.
	if fileExists(t, filepath.Join(out, "runtime.json")) {
		t.Errorf("runtime.json should be omitted when unreachable")
	}
	var manifest map[string]any
	readJSONFile(t, filepath.Join(out, "manifest.json"), &manifest)
	warnings, _ := manifest["warnings"].([]any)
	if len(warnings) == 0 {
		t.Errorf("manifest should record warnings for the omitted artifacts: %v", manifest)
	}
	var doctor map[string]any
	readJSONFile(t, filepath.Join(out, "doctor.json"), &doctor)
	checks, _ := doctor["checks"].([]any)
	if len(checks) != 1 {
		t.Fatalf("unreachable doctor.json should have exactly 1 check, got %v", doctor["checks"])
	}
	first, _ := checks[0].(map[string]any)
	if first["code"] != "runtime.unreachable" {
		t.Errorf("first check = %v, want runtime.unreachable", first["code"])
	}
}

func TestSupportBundle_ExistingEmptyDir_Used(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	out := t.TempDir() // already exists and is empty

	var stdout, stderr strings.Builder
	code := commands.RunSupportBundle(client.New(srv.URL), false, []string{"--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (existing empty dir is used)\nstderr: %s", code, stderr.String())
	}
	if !fileExists(t, filepath.Join(out, "manifest.json")) {
		t.Errorf("bundle should be written into the existing empty dir")
	}
}

func TestSupportBundle_MissingOut_Exit2(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	var stdout, stderr strings.Builder
	code := commands.RunSupportBundle(client.New(srv.URL), false, []string{}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "requires --out") {
		t.Errorf("stderr should explain --out is required: %q", stderr.String())
	}
}

func TestSupportBundle_NonEmptyDir_Exit1(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	out := t.TempDir() // exists and we make it non-empty
	if err := os.WriteFile(filepath.Join(out, "existing.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seeding dir: %v", err)
	}
	var stdout, stderr strings.Builder
	code := commands.RunSupportBundle(client.New(srv.URL), false, []string{"--out", out}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (refuse non-empty dir)", code)
	}
	if !strings.Contains(stderr.String(), "not empty") {
		t.Errorf("stderr should explain the dir is not empty: %q", stderr.String())
	}
}

func TestSupportBundle_OutIsFile_Exit1(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	f := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}
	var stdout, stderr strings.Builder
	code := commands.RunSupportBundle(client.New(srv.URL), false, []string{"--out", f}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (--out is a file)", code)
	}
	if !strings.Contains(stderr.String(), "not a directory") {
		t.Errorf("stderr should explain --out is not a directory: %q", stderr.String())
	}
}

func TestSupportBundle_MkdirFailure_Exit1(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	// Parent path is a regular file → MkdirAll of a child fails on all platforms.
	parent := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(parent, []byte("x"), 0o644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}
	out := filepath.Join(parent, "bundle")
	var stdout, stderr strings.Builder
	code := commands.RunSupportBundle(client.New(srv.URL), false, []string{"--out", out}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (mkdir failure)", code)
	}
}

func TestSupportBundle_JSONSummary(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "bundle")

	var stdout, stderr strings.Builder
	code := commands.RunSupportBundle(client.New(srv.URL), true, []string{"--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var summary map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &summary); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	for _, key := range []string{"out", "artifacts", "strict", "result"} {
		if _, ok := summary[key]; !ok {
			t.Errorf("summary missing %q: %v", key, summary)
		}
	}
	if summary["result"] != "passed" {
		t.Errorf("result = %v, want passed", summary["result"])
	}
}

func TestSupportBundle_StrictWarning_Exit1(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "warning")}})
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "bundle")

	var stdout, stderr strings.Builder
	code := commands.RunSupportBundle(client.New(srv.URL), false, []string{"--strict", "--out", out}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (warning under --strict)", code)
	}
	var manifest map[string]any
	readJSONFile(t, filepath.Join(out, "manifest.json"), &manifest)
	if manifest["result"] != "failed" || manifest["strict"] != true {
		t.Errorf("manifest result/strict = %v/%v, want failed/true", manifest["result"], manifest["strict"])
	}
}

func TestSupportBundle_JSONEncodeFailure_Exit1(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "bundle")

	var stderr strings.Builder
	code := commands.RunSupportBundle(client.New(srv.URL), true, []string{"--out", out}, failingWriter{}, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Error encoding JSON") {
		t.Errorf("stderr missing encode-error message: %q", stderr.String())
	}
}

func TestSupportBundle_Help_Exit0(t *testing.T) {
	var stdout, stderr strings.Builder
	code := commands.RunSupportBundle(nil, false, []string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "support-bundle") {
		t.Errorf("help missing usage: %q", stdout.String())
	}
}

func TestSupportBundle_UnexpectedArg_Exit2(t *testing.T) {
	var stdout, stderr strings.Builder
	code := commands.RunSupportBundle(nil, false, []string{"--out", "x", "extra"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestSupportBundle_BadFlag_Exit2(t *testing.T) {
	var stdout, stderr strings.Builder
	code := commands.RunSupportBundle(nil, false, []string{"--nope"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}
