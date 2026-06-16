package app

import (
	"testing"
	"time"

	"portier/service/sources/manager"
	"portier/service/sources/options"
)

func TestNew_AppliesDefaults(t *testing.T) {
	a := New(nil, options.Options{}, time.Time{}, "")

	if a.Manager == nil {
		t.Fatal("nil Manager should default to a fresh manager")
	}
	if a.StartedAt.IsZero() {
		t.Fatal("zero StartedAt should default to time.Now()")
	}
	if a.Version == "" {
		t.Fatal("empty Version should default to the build version")
	}
}

func TestNew_PreservesProvidedValues(t *testing.T) {
	m, err := manager.New(nil)
	if err != nil {
		t.Fatalf("manager.New: %v", err)
	}
	started := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	opts := options.Options{Host: "127.0.0.1", Port: 47831, ConfigPath: "rules.json", StaticDir: "web", Service: true}

	a := New(m, opts, started, "9.9.9")

	if a.Manager != m {
		t.Fatal("provided Manager should be preserved")
	}
	if !a.StartedAt.Equal(started) {
		t.Fatalf("StartedAt = %v, want %v", a.StartedAt, started)
	}
	if a.Version != "9.9.9" {
		t.Fatalf("Version = %q, want 9.9.9", a.Version)
	}
	if a.Options != opts {
		t.Fatalf("Options = %+v, want %+v", a.Options, opts)
	}
}
