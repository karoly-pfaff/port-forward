import { Controller, Delete, Get, HttpCode, Inject, Query } from "@nestjs/common";
import { ApiNoContentResponse, ApiOkResponse, ApiOperation, ApiQuery, ApiTags } from "@nestjs/swagger";
import { ActivityService } from "./activity.service.js";
import { ActivityListResponseDto, toActivityListResponseDto } from "./activity-list.response.dto.js";

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
 *   response, matching the documented `/api` contract). `@HttpCode(204)` is
 *   required because NestJS defaults `DELETE` to `200`.
 */
@ApiTags("activity")
@Controller("api/activity")
export class ActivityController {
  constructor(@Inject(ActivityService) private readonly activity: ActivityService) {}

  @Get()
  @ApiOperation({ summary: "List activity events", description: "Returns recent activity events, newest first." })
  @ApiQuery({ name: "limit", required: false, type: Number, description: "Max events (1–500, default 100)." })
  @ApiQuery({ name: "ruleId", required: false, type: String, description: "Filter by rule id." })
  @ApiQuery({ name: "type", required: false, type: String, description: "Filter by event type." })
  @ApiQuery({ name: "severity", required: false, type: String, description: "Filter by severity." })
  @ApiOkResponse({ type: ActivityListResponseDto, description: "Matching activity events." })
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
  @ApiOperation({ summary: "Clear the activity log", description: "Clears the in-memory activity log." })
  @ApiNoContentResponse({ description: "Activity log cleared. No response body." })
  clear(): void {
    this.activity.clear();
  }
}
