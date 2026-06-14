import { Controller, Get, Inject, Query } from "@nestjs/common";
import { ActivityService, type ActivityListResult } from "./activity.service.js";

/**
 * Transport adapter for `GET /api/activity`. Extracts the optional query params
 * and delegates to the service. No filtering/limit logic lives here; the endpoint
 * is read-only and always returns `200 { events: [...] }`.
 *
 * Only the read (`GET`) is migrated — `DELETE /api/activity` (a mutation) stays
 * with Express and is deferred.
 */
@Controller("api/activity")
export class ActivityController {
  constructor(@Inject(ActivityService) private readonly activity: ActivityService) {}

  @Get()
  list(
    @Query("limit") limit?: string,
    @Query("ruleId") ruleId?: string,
    @Query("type") type?: string,
    @Query("severity") severity?: string
  ): ActivityListResult {
    return this.activity.list(limit, ruleId, type, severity);
  }
}
