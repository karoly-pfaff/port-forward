export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

export function formatTimestamp(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleTimeString(undefined, { hour12: false, hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

export function formatUdpModeLabel(mode: string | undefined): string {
  if (mode === "bidirectional-last-client") return "Bidir – last";
  if (mode === "bidirectional-multi-client") return "Bidir – multi";
  return mode ?? "one-way";
}

export function formatDurationMs(ms: number): string {
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const rs = s % 60;
  if (m < 60) return `${m}m ${String(rs).padStart(2, "0")}s`;
  const h = Math.floor(m / 60);
  const rm = m % 60;
  return `${h}h ${String(rm).padStart(2, "0")}m`;
}

export function formatEndpoint(address: string, port: number): string {
  return `${address}:${port}`;
}

export function formatTimestampOrNever(value: string | null): string {
  if (value === null) return "Never";
  return formatTimestamp(value);
}
