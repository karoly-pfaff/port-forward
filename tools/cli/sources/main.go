package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"portier/cli/sources/client"
	"portier/cli/sources/commands"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("portier", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // suppress default usage output; we print our own

	flagURL := fs.String("url", "", "full management API URL")
	flagHost := fs.String("host", "", "management host (default: 127.0.0.1)")
	flagPort := fs.Int("port", 0, "management port (default: 47831)")
	flagJSON := fs.Bool("json", false, "output as machine-readable JSON")
	flagVersion := fs.Bool("version", false, "show CLI version and exit")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			commands.PrintHelp(os.Stdout)
			return 0
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		commands.PrintHelp(os.Stderr)
		return 2
	}

	if *flagVersion {
		commands.PrintVersion(os.Stdout)
		return 0
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		commands.PrintHelp(os.Stdout)
		return 0
	}

	switch remaining[0] {
	case "help":
		commands.PrintHelp(os.Stdout)
		return 0
	case "version":
		commands.PrintVersion(os.Stdout)
		return 0
	case "runtime":
		managementURL, err := commands.ResolveURL(*flagURL, *flagHost, *flagPort)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 2
		}
		c := client.New(managementURL)
		return commands.RunRuntime(c, *flagJSON, os.Stdout, os.Stderr)
	case "list":
		managementURL, err := commands.ResolveURL(*flagURL, *flagHost, *flagPort)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 2
		}
		c := client.New(managementURL)
		return commands.RunList(c, *flagJSON, os.Stdout, os.Stderr)
	case "status":
		managementURL, err := commands.ResolveURL(*flagURL, *flagHost, *flagPort)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 2
		}
		c := client.New(managementURL)
		return commands.RunStatus(c, *flagJSON, os.Stdout, os.Stderr)
	case "activity":
		managementURL, err := commands.ResolveURL(*flagURL, *flagHost, *flagPort)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 2
		}
		c := client.New(managementURL)
		return commands.RunActivity(c, *flagJSON, remaining[1:], os.Stdout, os.Stderr)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n\n", remaining[0])
		commands.PrintHelp(os.Stderr)
		return 2
	}
}
