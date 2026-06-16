import { Inject, Injectable } from "@nestjs/common";
import type { ActivityEvent, ActivityEventType, ActivitySeverity } from "@portier/shared";
import type { ActivityListParams } from "../../activity/activity-store.js";
import { ACTIVITY_STORE, type ActivityClearer, type ActivityReader } from "./activity.reader.js";

const DEFAULT_LIMIT = 100;
const MAX_LIMIT = 500;

/** Coerces the raw `limit` query value with the SAME rule as the Express route. */
export function parseActivityLimit(rawLimit: string | undefined): number {
  const limit = Number(rawLimit);
  return Number.isInteger(limit) && limit > 0 ? Math.min(limit, MAX_LIMIT) : DEFAULT_LIMIT;
}

/**
 * Behaviour for `GET /api/activity` (maps the raw query params to the store's
 * `ActivityListParams` with the same coercion as the Express route and returns
 * the matching domain events; the controller maps them to the response DTO) and
 * `DELETE /api/activity` (clears the in-memory log). The list path never throws;
 * the clear path has no input and always succeeds.
 */
@Injectable()
export class ActivityService {
  constructor(@Inject(ACTIVITY_STORE) private readonly store: ActivityReader & ActivityClearer) {}

  list(
    rawLimit?: string,
    rawRuleId?: string,
    rawType?: string,
    rawSeverity?: string
  ): ActivityEvent[] {
    const params: ActivityListParams = {
      limit: parseActivityLimit(rawLimit),
      ruleId: typeof rawRuleId === "string" ? rawRuleId : undefined,
      type: typeof rawType === "string" ? (rawType as ActivityEventType) : undefined,
      severity: typeof rawSeverity === "string" ? (rawSeverity as ActivitySeverity) : undefined,
    };
    return this.store.list(params);
  }

  clear(): void {
    this.store.clear();
  }
}
