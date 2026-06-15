/**
 * Generic live wall-clock provider, shared across every endpoint with a volatile
 * timestamp field — `GET /api/runtime` (`uptimeSeconds`), `GET /api/config/export`
 * (`exportedAt`), `GET /api/connections` (`generatedAt`), `POST /api/config/plan`
 * (`generatedAt`), `POST /api/forwards/:id/diagnose` (`diagnosedAt`), and
 * `POST /api/config/apply` (`appliedAt` + the embedded plan's `generatedAt`).
 * Keeping the clock here (not per-feature) lets a test pin time deterministically by
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
