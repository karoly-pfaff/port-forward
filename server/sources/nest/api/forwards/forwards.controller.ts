import { Controller, Get, Inject } from "@nestjs/common";
import { ApiOkResponse, ApiOperation, ApiTags } from "@nestjs/swagger";
import { ForwardRuleResponseDto } from "../../common/api-schemas.js";
import { ForwardsService } from "./forwards.service.js";
import { toForwardsListResponseDto, type ForwardsListResponseDto } from "./forwards-list.response.dto.js";

/**
 * Transport adapter for `GET /api/forwards`. No request input (no DTO needed).
 * Delegates to the service and maps the domain rule-response list to
 * `ForwardsListResponseDto` at the HTTP boundary (matching the existing Express
 * route). Read-only and always `200`; only the list read is migrated — the
 * write/lifecycle routes under `/api/forwards/...` stay with Express.
 */
@ApiTags("forwards")
@Controller("api/forwards")
export class ForwardsController {
  constructor(@Inject(ForwardsService) private readonly forwards: ForwardsService) {}

  @Get()
  @ApiOperation({ summary: "List forward rules", description: "Returns every forward rule with its port advisories." })
  @ApiOkResponse({ type: ForwardRuleResponseDto, isArray: true, description: "All forward rules." })
  list(): ForwardsListResponseDto {
    return toForwardsListResponseDto(this.forwards.list());
  }
}
