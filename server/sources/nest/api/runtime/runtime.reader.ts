import type { RuntimeInfoOptions } from "../../../runtime-info.js";

/**
 * Volatile/process-and-time providers for `GET /api/runtime`.
 *
 * Runtime info is the first endpoint with inherently volatile fields
 * (`uptimeSeconds`, plus process `pid`/`platform`/`arch`). To keep the service
 * pure and deterministically parity-testable, those values come from narrow
 * injected readers instead of being read inline. Production binds readers over
 * the real clock/process; tests override them with fixed values.
 *
 * The clock provider (`ClockReader`/`CLOCK_READER`) was promoted to
 * `common/clock.reader.ts` once `GET /api/config/export` (Slice 10) also needed
 * it; it is re-exported here so the runtime feature's imports stay stable.
 */

export { type ClockReader, CLOCK_READER, defaultClockReader } from "../../common/clock.reader.js";

/** Process-metadata provider (`pid`/`platform`/`arch`). */
export interface ProcessReader {
  pid(): number;
  platform(): string;
  arch(): string;
}

/** Injection token for the process reader. */
export const PROCESS_READER = "PROCESS_READER";

/** Production process metadata: the real `process`. */
export const defaultProcessReader: ProcessReader = {
  pid: () => process.pid,
  platform: () => process.platform,
  arch: () => process.arch,
};

/**
 * Static runtime metadata provider. The scaffold has no runtime wired, so the
 * default returns no options (defaults are applied by the builder) and a start
 * time captured once at construction — mirroring Express, where
 * `runtimeStartedAt` is resolved a single time at app creation. When the NestJS
 * server becomes the active runtime this is bound to the real launch metadata.
 */
export interface RuntimeInfoReader {
  options(): RuntimeInfoOptions | undefined;
  startedAt(): Date;
}

/** Injection token for the runtime-info reader. */
export const RUNTIME_INFO_READER = "RUNTIME_INFO_READER";

/** Builds the scaffold default reader: no options, a fixed-at-construction start time. */
export function createDefaultRuntimeInfoReader(): RuntimeInfoReader {
  const fallbackStartedAt = new Date();
  return {
    options: () => undefined,
    startedAt: () => fallbackStartedAt,
  };
}
