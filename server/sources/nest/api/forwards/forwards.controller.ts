import { Controller, Get, Inject } from "@nestjs/common";
import { ForwardsService } from "./forwards.service.js";
import { toForwardsListResponseDto, type ForwardsListResponseDto } from "./forwards-list.response.dto.js";

/**
 * Transport adapter for `GET /api/forwards`. No request input (no DTO needed).
 * Delegates to the service and maps the domain rule-response list to
 * `ForwardsListResponseDto` at the HTTP boundary (matching the existing Express
 * route). Read-only and always `200`; only the list read is migrated — the
 * write/lifecycle routes under `/api/forwards/...` stay with Express.
 */
@Controller("api/forwards")
export class ForwardsController {
  constructor(@Inject(ForwardsService) private readonly forwards: ForwardsService) {}

  @Get()
  list(): ForwardsListResponseDto {
    return toForwardsListResponseDto(this.forwards.list());
  }
}
