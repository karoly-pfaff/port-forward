/**
 * Generic live wall-clock provider, shared across endpoints with volatile
 * timestamp fields. Introduced for `GET /api/runtime` (`uptimeSeconds`, v1.14
 * Slice 9) and reused by `GET /api/config/export` (`exportedAt`, Slice 10); the
 * still-deferred `GET /api/connections` (`generatedAt`) reuses it next. Keeping
 * the clock here (not per-feature) lets a test pin time deterministically by
 * overriding one token, so volatile endpoints stay byte-for-byte parity-testable.
 */
export interface ClockReader {
  now(): Date;
}

/** Injection token for the clock reader. */
export const CLOCK_READER = "CLOCK_READER";

/** Production clock: the real wall clock. */
export const defaultClockReader: ClockReader = {
  now: () => new Date(),
};
