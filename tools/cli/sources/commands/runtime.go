package commands

import (
	"errors"
	"fmt"
	"io"

	"portier/cli/sources/client"
	"portier/cli/sources/output"
)

// RunRuntime executes the `portier runtime` command.
// Returns an exit code: 0 success, 1 API/general error, 3 connection failure.
func RunRuntime(c *client.Client, jsonOutput bool, stdout, stderr io.Writer) int {
	info, err := c.GetRuntime()
	if err != nil {
		var connErr *client.ConnectionError
		if errors.As(err, &connErr) {
			fmt.Fprintf(stderr, "Error: %s\n", connErr)
			fmt.Fprintln(stderr, "Hint: Is the Portier service running?")
			return 3
		}
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, info); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	printRuntimeHuman(info, stdout)
	return 0
}

func printRuntimeHuman(info *client.RuntimeInfo, w io.Writer) {
	output.PrintField(w, "Name", info.Name)
	output.PrintField(w, "Version", info.Version)
	output.PrintField(w, "Runtime", info.Runtime)
	output.PrintField(w, "Platform", fmt.Sprintf("%s/%s", info.Platform, info.Arch))
	output.PrintField(w, "Uptime", fmt.Sprintf("%ds", info.UptimeSeconds))
	output.PrintField(w, "Management URL", fmt.Sprintf("http://%s:%d", info.ManagementHost, info.ManagementPort))
	output.PrintField(w, "Config path", info.ConfigPath)
	output.PrintField(w, "Static dir", info.StaticDir)
	output.PrintField(w, "Service mode", fmt.Sprintf("%v", info.ServiceMode))
}
