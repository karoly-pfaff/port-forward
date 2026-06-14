import { Controller, Delete, Get, HttpCode, Inject, Query } from "@nestjs/common";
import { ActivityService } from "./activity.service.js";
import { toActivityListResponseDto, type ActivityListResponseDto } from "./activity-list.response.dto.js";

/**
 * Transport adapter for `/api/activity`.
 *
 * - `GET`: no DTO is needed for the query — its `limit`/filters are pure
 *   silent coercion-with-fallback (always `200`, never rejects), so a DTO would
 *   add ceremony without validation value and risk parity drift; the coercion
 *   stays endpoint-local in the service. The response IS mapped to an explicit
 *   `ActivityListResponseDto` at the HTTP boundary.
 * - `DELETE`: no request input (no request DTO) and a `204` with an empty body —
 *   there is no JSON body, so there is no response DTO (the absent body IS the
 *   response, matching the existing Express route exactly). `@HttpCode(204)` is
 *   required because NestJS defaults `DELETE` to `200`.
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
  ): ActivityListResponseDto {
    return toActivityListResponseDto(this.activity.list(limit, ruleId, type, severity));
  }

  @Delete()
  @HttpCode(204)
  clear(): void {
    this.activity.clear();
  }
}
