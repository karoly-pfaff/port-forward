package commands_test

import (
	"strings"
	"testing"

	"portier/cli/sources/commands"
)

func TestPrintHelp_ContainsRuntimeCommand(t *testing.T) {
	var buf strings.Builder
	commands.PrintHelp(&buf)
	if !strings.Contains(buf.String(), "runtime") {
		t.Error("help output does not mention 'runtime' command")
	}
}

func TestPrintHelp_ContainsDefaultURL(t *testing.T) {
	var buf strings.Builder
	commands.PrintHelp(&buf)
	if !strings.Contains(buf.String(), commands.DefaultURL) {
		t.Errorf("help output does not contain default URL %q", commands.DefaultURL)
	}
}

func TestPrintHelp_ContainsGlobalFlags(t *testing.T) {
	var buf strings.Builder
	commands.PrintHelp(&buf)
	out := buf.String()
	for _, flag := range []string{"--url", "--host", "--port", "--json"} {
		if !strings.Contains(out, flag) {
			t.Errorf("help output does not mention global flag %q", flag)
		}
	}
}

func TestPrintHelp_ContainsPortierURL(t *testing.T) {
	var buf strings.Builder
	commands.PrintHelp(&buf)
	if !strings.Contains(buf.String(), "PORTIER_URL") {
		t.Error("help output does not mention PORTIER_URL environment variable")
	}
}

func TestPrintHelp_ContainsExitCodes(t *testing.T) {
	var buf strings.Builder
	commands.PrintHelp(&buf)
	out := buf.String()
	for _, code := range []string{"0", "1", "2", "3"} {
		if !strings.Contains(out, code) {
			t.Errorf("help output does not mention exit code %q", code)
		}
	}
}

func TestPrintVersion_ContainsCLIName(t *testing.T) {
	var buf strings.Builder
	commands.PrintVersion(&buf)
	out := buf.String()
	if !strings.Contains(out, "Portier CLI") {
		t.Errorf("version output does not contain 'Portier CLI': %q", out)
	}
}

func TestPrintVersion_ContainsVersion(t *testing.T) {
	var buf strings.Builder
	commands.PrintVersion(&buf)
	out := buf.String()
	if strings.TrimSpace(out) == "Portier CLI" {
		t.Error("version output contains no version string beyond the name")
	}
}
