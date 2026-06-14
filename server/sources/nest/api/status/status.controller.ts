import { Controller, Get, Inject } from "@nestjs/common";
import type { ForwardStatus } from "@portier/shared";
import { StatusService } from "./status.service.js";

/**
 * Transport adapter for `GET /api/status`. Delegates to the service and returns
 * the `ForwardStatus[]` array verbatim (matching the existing Express route).
 * Read-only and always `200`; no status logic lives here.
 */
@Controller("api/status")
export class StatusController {
  constructor(@Inject(StatusService) private readonly status: StatusService) {}

  @Get()
  list(): ForwardStatus[] {
    return this.status.list();
  }
}
