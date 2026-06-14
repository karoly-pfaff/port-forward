import { Controller, Get, Inject } from "@nestjs/common";
import type { ForwardRuleResponse } from "@portier/shared";
import { ForwardsService } from "./forwards.service.js";

/**
 * Transport adapter for `GET /api/forwards`. Delegates to the service and returns
 * the `ForwardRuleResponse[]` array verbatim (matching the existing Express
 * route). Read-only and always `200`; only the list read is migrated — the
 * write/lifecycle routes under `/api/forwards/...` stay with Express.
 */
@Controller("api/forwards")
export class ForwardsController {
  constructor(@Inject(ForwardsService) private readonly forwards: ForwardsService) {}

  @Get()
  list(): ForwardRuleResponse[] {
    return this.forwards.list();
  }
}
