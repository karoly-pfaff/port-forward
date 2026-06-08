package output

import (
	"strings"
	"testing"
)

func TestFormatBytes_SubKB(t *testing.T) {
	got := FormatBytes(500)
	if got != "500 B" {
		t.Errorf("FormatBytes(500) = %q, want %q", got, "500 B")
	}
}

func TestFormatBytes_Zero(t *testing.T) {
	got := FormatBytes(0)
	if got != "0 B" {
		t.Errorf("FormatBytes(0) = %q, want %q", got, "0 B")
	}
}

func TestFormatBytes_KB(t *testing.T) {
	got := FormatBytes(1024)
	if got != "1.0 KB" {
		t.Errorf("FormatBytes(1024) = %q, want %q", got, "1.0 KB")
	}
}

func TestFormatBytes_MB(t *testing.T) {
	got := FormatBytes(1024 * 1024)
	if got != "1.0 MB" {
		t.Errorf("FormatBytes(1024*1024) = %q, want %q", got, "1.0 MB")
	}
}

func TestFormatBytes_GB(t *testing.T) {
	got := FormatBytes(1024 * 1024 * 1024)
	if got != "1.0 GB" {
		t.Errorf("FormatBytes(1024*1024*1024) = %q, want %q", got, "1.0 GB")
	}
}

func TestFormatTimestamp_RFC3339(t *testing.T) {
	got := FormatTimestamp("2026-01-15T12:30:00Z")
	// Should be formatted as local time, not the original string
	if got == "2026-01-15T12:30:00Z" {
		t.Errorf("FormatTimestamp should reformat RFC3339 timestamps, got original string back")
	}
	// Should contain the date portion
	if !strings.Contains(got, "2026-01-15") {
		t.Errorf("FormatTimestamp result %q does not contain expected date", got)
	}
}

func TestFormatTimestamp_RFC3339Nano(t *testing.T) {
	got := FormatTimestamp("2026-01-15T12:30:00.123456789Z")
	// Should be formatted as local time
	if got == "2026-01-15T12:30:00.123456789Z" {
		t.Errorf("FormatTimestamp should reformat RFC3339Nano timestamps, got original string back")
	}
	if !strings.Contains(got, "2026-01-15") {
		t.Errorf("FormatTimestamp result %q does not contain expected date", got)
	}
}

func TestFormatTimestamp_Invalid(t *testing.T) {
	invalid := "not-a-timestamp"
	got := FormatTimestamp(invalid)
	if got != invalid {
		t.Errorf("FormatTimestamp(%q) = %q, want unchanged %q", invalid, got, invalid)
	}
}
