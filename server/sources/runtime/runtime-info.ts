import type { RuntimeInfo } from "@portier/shared";
import { PORTIER_DEFAULT_HOST, PORTIER_DEFAULT_PORT } from "@portier/shared";

/**
 * Static runtime metadata the host process knows at startup. `startedAt` is the
 * process start time; everything else describes how the management server was
 * launched. Resolved once at app creation (not per request).
 */
export interface RuntimeInfoOptions {
  version: string;
  managementHost: string;
  managementPort: number;
  configPath: string;
  staticDir: string;
  serviceMode: boolean;
  startedAt: Date;
}

/**
 * All inputs `buildRuntimeInfo` needs. The volatile/process-dependent values
 * (`now`, `pid`, `platform`, `arch`) are passed in explicitly so the builder
 * stays pure and deterministic — the caller owns the live clock and process. The
 * the NestJS service feeds this builder, so the runtime
 * info shape cannot drift between the two implementations.
 */
export interface RuntimeInfoInput {
  /** Static runtime metadata, or `undefined` when none is wired (defaults applied). */
  runtimeInfo: RuntimeInfoOptions | undefined;
  /** Resolved process start time (used for `startedAt` and `uptimeSeconds`). */
  startedAt: Date;
  /** Current wall-clock time (used only to compute `uptimeSeconds`). */
  now: Date;
  /** Process id (`process.pid`). */
  pid: number;
  /** Raw platform string (`process.platform`); normalized for the response. */
  platform: string;
  /** Raw architecture string (`process.arch`); normalized for the response. */
  arch: string;
}

export function normalizePlatform(platform: string): RuntimeInfo["platform"] {
  if (platform === "win32") return "windows";
  if (platform === "darwin") return "macos";
  if (platform === "linux") return "linux";
  return "unknown";
}

export function normalizeArch(arch: string): RuntimeInfo["arch"] {
  if (arch === "x64") return "x64";
  if (arch === "arm64") return "arm64";
  return "unknown";
}

/**
 * Builds the `GET /api/runtime` response for the Node runtime. Pure: identical
 * inputs always produce identical output (including field order, which JSON
 * serialization preserves), so it can be parity-tested deterministically. The
 * defaults (`"unknown"` version, the management host/port, empty paths) mirror
 * the historical behavior exactly.
 */
export function buildRuntimeInfo(input: RuntimeInfoInput): RuntimeInfo {
  const info = input.runtimeInfo;
  return {
    name: "Portier",
    version: info?.version ?? "unknown",
    runtime: "node",
    platform: normalizePlatform(input.platform),
    arch: normalizeArch(input.arch),
    uptimeSeconds: Math.floor((input.now.getTime() - input.startedAt.getTime()) / 1000),
    startedAt: input.startedAt.toISOString(),
    managementHost: info?.managementHost ?? PORTIER_DEFAULT_HOST,
    managementPort: info?.managementPort ?? PORTIER_DEFAULT_PORT,
    configPath: info?.configPath ?? "",
    staticDir: info?.staticDir ?? "",
    serviceMode: info?.serviceMode ?? false,
    pid: input.pid,
  };
}
