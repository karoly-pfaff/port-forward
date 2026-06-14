import type { ActivityEvent } from "@portier/shared";
import type { ActivityListParams } from "../../../activity/activity-store.js";

/**
 * Narrow read-only view of the activity store the GET endpoint needs. The real
 * domain `ActivityStore` satisfies it; tests inject a fake. Keeping the
 * dependency to this interface (not the concrete class) is the DI seam the other
 * runtime-dependent endpoints reuse. Lives in `activity.reader.ts` alongside the
 * other feature readers (`status.reader.ts`, `forwards.reader.ts`, …).
 */
export interface ActivityReader {
  list(params: ActivityListParams): ActivityEvent[];
}

/** Narrow write capability for `DELETE /api/activity` (clears the in-memory log). */
export interface ActivityClearer {
  clear(): void;
}

/**
 * Injection token for the activity store. Unlike the pure-read `*_READER` tokens,
 * the bound value satisfies BOTH `ActivityReader` (GET) and `ActivityClearer`
 * (DELETE) — each consumer narrows to what it needs — so it is named for the
 * store it provides rather than a single capability.
 */
export const ACTIVITY_STORE = "ACTIVITY_STORE";
