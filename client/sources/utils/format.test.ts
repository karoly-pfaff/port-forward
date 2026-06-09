import { describe, expect, it } from "vitest";
import {
  formatBytes,
  formatDurationMs,
  formatEndpoint,
  formatTimestampOrNever,
  formatUdpModeLabel,
} from "./format.js";

describe("formatBytes", () => {
  it("returns bytes for values under 1 KB", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(1)).toBe("1 B");
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(1023)).toBe("1023 B");
  });

  it("returns KB for values in the kilobyte range", () => {
    expect(formatBytes(1024)).toBe("1.0 KB");
    expect(formatBytes(2048)).toBe("2.0 KB");
    expect(formatBytes(1536)).toBe("1.5 KB");
  });

  it("returns MB for values at 1 MB and above", () => {
    expect(formatBytes(1024 * 1024)).toBe("1.0 MB");
    expect(formatBytes(2 * 1024 * 1024)).toBe("2.0 MB");
    expect(formatBytes(1.5 * 1024 * 1024)).toBe("1.5 MB");
  });
});

describe("formatDurationMs", () => {
  it("returns 0s for zero", () => {
    expect(formatDurationMs(0)).toBe("0s");
  });

  it("returns seconds for durations under 60s", () => {
    expect(formatDurationMs(1000)).toBe("1s");
    expect(formatDurationMs(30000)).toBe("30s");
    expect(formatDurationMs(59000)).toBe("59s");
  });

  it("returns minutes and zero-padded seconds for 60s–59m59s", () => {
    expect(formatDurationMs(60000)).toBe("1m 00s");
    expect(formatDurationMs(65000)).toBe("1m 05s");
    expect(formatDurationMs(90000)).toBe("1m 30s");
    expect(formatDurationMs(3599000)).toBe("59m 59s");
  });

  it("returns hours and zero-padded minutes for 1h and above", () => {
    expect(formatDurationMs(3600000)).toBe("1h 00m");
    expect(formatDurationMs(3660000)).toBe("1h 01m");
    expect(formatDurationMs(7200000)).toBe("2h 00m");
    expect(formatDurationMs(7260000)).toBe("2h 01m");
  });

  it("ignores sub-second precision", () => {
    expect(formatDurationMs(1500)).toBe("1s");
    expect(formatDurationMs(61999)).toBe("1m 01s");
  });
});

describe("formatEndpoint", () => {
  it("formats address and port with a colon", () => {
    expect(formatEndpoint("127.0.0.1", 3000)).toBe("127.0.0.1:3000");
    expect(formatEndpoint("192.168.1.1", 48001)).toBe("192.168.1.1:48001");
    expect(formatEndpoint("0.0.0.0", 80)).toBe("0.0.0.0:80");
  });
});

describe("formatTimestampOrNever", () => {
  it("returns 'Never' for null", () => {
    expect(formatTimestampOrNever(null)).toBe("Never");
  });

  it("returns a non-empty string for a valid ISO timestamp", () => {
    const result = formatTimestampOrNever("2026-01-01T12:00:00.000Z");
    expect(result).not.toBe("Never");
    expect(typeof result).toBe("string");
    expect(result.length).toBeGreaterThan(0);
  });
});

describe("formatUdpModeLabel", () => {
  it("labels bidirectional-last-client", () => {
    expect(formatUdpModeLabel("bidirectional-last-client")).toBe("Bidir – last");
  });

  it("labels bidirectional-multi-client", () => {
    expect(formatUdpModeLabel("bidirectional-multi-client")).toBe("Bidir – multi");
  });

  it("returns 'one-way' for undefined", () => {
    expect(formatUdpModeLabel(undefined)).toBe("one-way");
  });

  it("returns the mode string as-is for one-way", () => {
    expect(formatUdpModeLabel("one-way")).toBe("one-way");
  });
});
