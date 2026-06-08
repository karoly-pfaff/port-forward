package commands_test

import (
	"fmt"
	"testing"

	"portier/cli/sources/commands"
)

func TestResolveURL_Default(t *testing.T) {
	t.Setenv("PORTIER_URL", "")
	got, err := commands.ResolveURL("", "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != commands.DefaultURL {
		t.Errorf("got %q, want %q", got, commands.DefaultURL)
	}
}

func TestResolveURL_FlagURL(t *testing.T) {
	t.Setenv("PORTIER_URL", "")
	got, err := commands.ResolveURL("http://10.0.0.1:9000", "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://10.0.0.1:9000" {
		t.Errorf("got %q, want %q", got, "http://10.0.0.1:9000")
	}
}

func TestResolveURL_FlagURLWinsOverHostPort(t *testing.T) {
	t.Setenv("PORTIER_URL", "")
	got, err := commands.ResolveURL("http://10.0.0.1:9000", "192.168.1.1", 8888)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://10.0.0.1:9000" {
		t.Errorf("got %q, want %q", got, "http://10.0.0.1:9000")
	}
}

func TestResolveURL_FlagURLWinsOverEnv(t *testing.T) {
	t.Setenv("PORTIER_URL", "http://env.example.com:1234")
	got, err := commands.ResolveURL("http://10.0.0.1:9000", "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://10.0.0.1:9000" {
		t.Errorf("got %q, want %q", got, "http://10.0.0.1:9000")
	}
}

func TestResolveURL_HostOnly(t *testing.T) {
	t.Setenv("PORTIER_URL", "")
	got, err := commands.ResolveURL("", "192.168.1.5", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := fmt.Sprintf("http://192.168.1.5:%d", commands.DefaultPort)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveURL_PortOnly(t *testing.T) {
	t.Setenv("PORTIER_URL", "")
	got, err := commands.ResolveURL("", "", 9000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := fmt.Sprintf("http://%s:9000", commands.DefaultHost)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveURL_HostAndPort(t *testing.T) {
	t.Setenv("PORTIER_URL", "")
	got, err := commands.ResolveURL("", "192.168.1.5", 9000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://192.168.1.5:9000" {
		t.Errorf("got %q, want %q", got, "http://192.168.1.5:9000")
	}
}

func TestResolveURL_EnvVar(t *testing.T) {
	t.Setenv("PORTIER_URL", "http://10.0.0.5:9999")
	got, err := commands.ResolveURL("", "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://10.0.0.5:9999" {
		t.Errorf("got %q, want %q", got, "http://10.0.0.5:9999")
	}
}

func TestResolveURL_HostPortWinsOverEnv(t *testing.T) {
	t.Setenv("PORTIER_URL", "http://10.0.0.5:9999")
	got, err := commands.ResolveURL("", "192.168.1.1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := fmt.Sprintf("http://192.168.1.1:%d", commands.DefaultPort)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveURL_InvalidFlagURL(t *testing.T) {
	t.Setenv("PORTIER_URL", "")
	_, err := commands.ResolveURL("ftp://127.0.0.1:47831", "", 0)
	if err == nil {
		t.Fatal("expected error for unsupported scheme, got nil")
	}
}

func TestResolveURL_InvalidEnvURL(t *testing.T) {
	t.Setenv("PORTIER_URL", "ftp://bad-scheme")
	_, err := commands.ResolveURL("", "", 0)
	if err == nil {
		t.Fatal("expected error for invalid PORTIER_URL scheme, got nil")
	}
}

func TestResolveURL_MissingHost(t *testing.T) {
	t.Setenv("PORTIER_URL", "")
	_, err := commands.ResolveURL("http://", "", 0)
	if err == nil {
		t.Fatal("expected error for URL missing host, got nil")
	}
}
