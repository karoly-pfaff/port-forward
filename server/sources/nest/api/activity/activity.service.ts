import { Inject, Injectable } from "@nestjs/common";
import type { ActivityEvent, ActivityEventType, ActivitySeverity } from "@portier/shared";
import type { ActivityListParams } from "../../../activity/activity-store.js";

/**
 * Narrow read-only view of the activity store the controller needs. The real
 * domain `ActivityStore` satisfies it; tests inject a fake. Keeping the
 * dependency to this interface (not the concrete class) is the DI seam future
 * runtime-dependent endpoints will reuse.
 */
export interface ActivityReader {
  list(params: ActivityListParams): ActivityEvent[];
}

/** Injection token for the activity reader. */
export const ACTIVITY_STORE = "ACTIVITY_STORE";

/** Response body for `GET /api/activity`. */
export interface ActivityListResult {
  events: ActivityEvent[];
}

const DEFAULT_LIMIT = 100;
const MAX_LIMIT = 500;

/** Coerces the raw `limit` query value with the SAME rule as the Express route. */
export function parseActivityLimit(rawLimit: string | undefined): number {
  const limit = Number(rawLimit);
  return Number.isInteger(limit) && limit > 0 ? Math.min(limit, MAX_LIMIT) : DEFAULT_LIMIT;
}

/**
 * Behaviour for `GET /api/activity`: maps the raw query params to the store's
 * `ActivityListParams` (identical coercion to the existing Express route) and
 * delegates filtering/limiting to the injected store. Read-only; never throws.
 */
@Injectable()
export class ActivityService {
  constructor(@Inject(ACTIVITY_STORE) private readonly store: ActivityReader) {}

  list(
    rawLimit?: string,
    rawRuleId?: string,
    rawType?: string,
    rawSeverity?: string
  ): ActivityListResult {
    const params: ActivityListParams = {
      limit: parseActivityLimit(rawLimit),
      ruleId: typeof rawRuleId === "string" ? rawRuleId : undefined,
      type: typeof rawType === "string" ? (rawType as ActivityEventType) : undefined,
      severity: typeof rawSeverity === "string" ? (rawSeverity as ActivitySeverity) : undefined,
    };
    return { events: this.store.list(params) };
  }
}
