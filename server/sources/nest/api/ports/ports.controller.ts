import { BadRequestException, Controller, Get, Inject, Query } from "@nestjs/common";
import type { PortAdvisory } from "@portier/shared";
import { PortsService } from "./ports.service.js";

/**
 * Transport adapter for `GET /api/ports/advisory`. Extracts the query params and
 * maps the service result to the HTTP contract: a `200` advisory array, or a
 * `400` with the `{ errors: [...] }` envelope (matching the existing Express
 * route and the Go service). No advisory logic lives here.
 */
@Controller("api/ports")
export class PortsController {
  constructor(@Inject(PortsService) private readonly ports: PortsService) {}

  @Get("advisory")
  getAdvisory(
    @Query("port") port?: string,
    @Query("purpose") purpose?: string,
    @Query("listenHost") listenHost?: string
  ): PortAdvisory[] {
    const result = this.ports.resolveAdvisories(port, purpose, listenHost);
    if (!result.ok) {
      throw new BadRequestException({ errors: result.errors });
    }
    return result.advisories;
  }
}
