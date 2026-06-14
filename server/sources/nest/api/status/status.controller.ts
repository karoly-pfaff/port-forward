import { Controller, Get, Inject } from "@nestjs/common";
import { ApiOkResponse, ApiOperation, ApiTags } from "@nestjs/swagger";
import { ForwardStatusDto } from "../../common/api-schemas.js";
import { StatusService } from "./status.service.js";
import { toStatusListResponseDto, type StatusListResponseDto } from "./status-list.response.dto.js";

/**
 * Transport adapter for `GET /api/status`. No request input (no DTO needed).
 * Delegates to the service and maps the domain status list to
 * `StatusListResponseDto` at the HTTP boundary (matching the existing Express
 * route). Read-only and always `200`; no status logic lives here.
 */
@ApiTags("status")
@Controller("api/status")
export class StatusController {
  constructor(@Inject(StatusService) private readonly status: StatusService) {}

  @Get()
  @ApiOperation({ summary: "Per-rule status", description: "Returns the current status of every forward rule." })
  @ApiOkResponse({ type: ForwardStatusDto, isArray: true, description: "Status for each rule." })
  list(): StatusListResponseDto {
    return toStatusListResponseDto(this.status.list());
  }
}
