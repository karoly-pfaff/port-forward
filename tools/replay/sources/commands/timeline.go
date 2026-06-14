package commands

import (
	"flag"
	"fmt"
	"io"
	"os"

	"portier/replay/sources/core"
)

// runTimeline handles `replay timeline --from <file-or-dir> [--out <file>]`.
func runTimeline(jsonOut bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("timeline", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	from := fs.String("from", "", "path to a workflow artifact (run/plan report, history export, or report bundle directory)")
	out := fs.String("out", "", "optional file to write the timeline JSON to")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	if *from == "" {
		fmt.Fprintln(stderr, "Error: --from is required")
		return 2
	}

	timeline, err := core.TimelineInput(*from)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}

	data, err := core.MarshalTimeline(timeline)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	if jsonOut {
		if _, err := stdout.Write(data); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
	} else {
		core.RenderTimelineHuman(stdout, timeline)
	}

	if *out != "" {
		if err := os.WriteFile(*out, data, 0o644); err != nil {
			fmt.Fprintf(stderr, "Error: writing %s: %v\n", *out, err)
			return 1
		}
		if !jsonOut {
			fmt.Fprintf(stdout, "Replay timeline written to %s\n", *out)
		}
	}

	return 0
}
