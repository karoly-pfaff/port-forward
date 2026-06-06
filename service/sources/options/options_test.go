package options

import (
	"path/filepath"
	"testing"
)

func TestResolveUsesDefaults(t *testing.T) {
	cwd := t.TempDir()

	got, err := Resolve(nil, Env{}, cwd)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	if got.Service {
		t.Fatal("expected service mode to default to false")
	}
	if got.ConfigPath != filepath.Join(cwd, DefaultConfigPath) {
		t.Fatalf("config path = %q, want %q", got.ConfigPath, filepath.Join(cwd, DefaultConfigPath))
	}
	if got.Host != DefaultHost {
		t.Fatalf("host = %q, want %q", got.Host, DefaultHost)
	}
	if got.Port != DefaultPort {
		t.Fatalf("port = %d, want %d", got.Port, DefaultPort)
	}
	if got.StaticDir != filepath.Join(cwd, DefaultStaticDir) {
		t.Fatalf("static dir = %q, want %q", got.StaticDir, filepath.Join(cwd, DefaultStaticDir))
	}
	if got.StaticDirSource != "default" {
		t.Fatalf("static dir source = %q, want default", got.StaticDirSource)
	}
}

func TestResolveUsesEnvironment(t *testing.T) {
	cwd := t.TempDir()

	got, err := Resolve(nil, Env{
		"PORTIER_CONFIG":     "rules.json",
		"PORTIER_HOST":       "0.0.0.0",
		"PORTIER_PORT":       "5000",
		"PORTIER_STATIC_DIR": "web",
	}, cwd)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	if got.ConfigPath != filepath.Join(cwd, "rules.json") {
		t.Fatalf("config path = %q", got.ConfigPath)
	}
	if got.Host != "0.0.0.0" {
		t.Fatalf("host = %q", got.Host)
	}
	if got.Port != 5000 {
		t.Fatalf("port = %d", got.Port)
	}
	if got.StaticDir != filepath.Join(cwd, "web") {
		t.Fatalf("static dir = %q", got.StaticDir)
	}
	if got.StaticDirSource != "env" {
		t.Fatalf("static dir source = %q, want env", got.StaticDirSource)
	}
}

func TestResolveLetsCLIOverrideEnvironment(t *testing.T) {
	cwd := t.TempDir()

	got, err := Resolve([]string{
		"--service",
		"--config", "cli.json",
		"--host", "127.0.0.2",
		"--port", "5001",
		"--static-dir", "static",
	}, Env{
		"PORTIER_CONFIG":     "env.json",
		"PORTIER_HOST":       "0.0.0.0",
		"PORTIER_PORT":       "5000",
		"PORTIER_STATIC_DIR": "web",
	}, cwd)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	if !got.Service {
		t.Fatal("expected service mode from CLI")
	}
	if got.ConfigPath != filepath.Join(cwd, "cli.json") {
		t.Fatalf("config path = %q", got.ConfigPath)
	}
	if got.Host != "127.0.0.2" {
		t.Fatalf("host = %q", got.Host)
	}
	if got.Port != 5001 {
		t.Fatalf("port = %d", got.Port)
	}
	if got.StaticDir != filepath.Join(cwd, "static") {
		t.Fatalf("static dir = %q", got.StaticDir)
	}
	if got.StaticDirSource != "cli" {
		t.Fatalf("static dir source = %q, want cli", got.StaticDirSource)
	}
}

func TestResolveRejectsInvalidPort(t *testing.T) {
	if _, err := Resolve([]string{"--port", "99999"}, Env{}, t.TempDir()); err == nil {
		t.Fatal("expected invalid port error")
	}
}
