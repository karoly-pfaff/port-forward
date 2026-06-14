// Command replay is a standalone, offline analysis utility for existing Portier
// workflow artifacts. It reads workflow run/plan JSON reports, workflow history
// exports, and workflow support-report bundles, and either reports what offline
// replay/analysis each saved artifact can support (`plan`) or produces deterministic
// analysis from the artifact contents (`analyze`).
//
// It is strictly offline and read-only. It NEVER executes workflows, contacts the
// Portier runtime, reads the config/policy/baseline/report files an artifact refers
// to, mutates runtime/config/history/report files, applies/imports configs,
// enforces policy, schedules jobs, runs shell commands, uploads anything, or
// collects logs/environment/process data.
//
// It is a separate tool beside the Portier CLI — not a `portier` subcommand. The
// binary is built as `replay` (`replay.exe` on Windows). Dispatch and command
// parsing live in the commands package; domain logic lives in the core package.
package main

import (
	"os"

	"portier/replay/sources/commands"
)

func main() {
	os.Exit(commands.Run(os.Args[1:], os.Stdout, os.Stderr))
}
