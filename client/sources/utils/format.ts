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
