package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestBuildOutputName verifies the tool builds as `replay` (or `replay.exe` where
// the Go toolchain appends the extension) and never as the old `portier-replay`.
func TestBuildOutputName(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not available")
	}
	dir := t.TempDir()
	outBase := filepath.Join(dir, "replay")

	// `go test` runs with the working directory set to this package's directory
	// (sources), which holds main.go, so "." builds the replay command.
	cmd := exec.Command(goBin, "build", "-o", outBase, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	candidates := []string{outBase}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, outBase+".exe")
	}
	found := false
	for _, c := range candidates {
		if _, statErr := os.Stat(c); statErr == nil {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a build output named replay (or replay.exe) in %s", dir)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "portier-replay")); statErr == nil {
		t.Error("build must not produce a stale portier-replay binary")
	}
}
