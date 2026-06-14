import { Controller, Get, Inject, Query } from "@nestjs/common";
import type { PortAdvisory } from "@portier/shared";
import { ApiValidationPipe } from "../../common/api-validation.pipe.js";
import { PortsAdvisoryQueryDto } from "./ports-advisory.query.dto.js";
import { PortsService } from "./ports.service.js";

/**
 * Transport adapter for `GET /api/ports/advisory`. The `ApiValidationPipe`
 * validates/coerces the query into a `PortsAdvisoryQueryDto` (throwing the
 * shared `400 { errors: [...] }` envelope on failure); the controller then just
 * delegates to the service. No validation or envelope logic lives here.
 */
@Controller("api/ports")
export class PortsController {
  constructor(@Inject(PortsService) private readonly ports: PortsService) {}

  @Get("advisory")
  getAdvisory(@Query(new ApiValidationPipe(PortsAdvisoryQueryDto)) query: PortsAdvisoryQueryDto): PortAdvisory[] {
    return this.ports.getAdvisories(query);
  }
}
