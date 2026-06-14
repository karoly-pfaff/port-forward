import { Controller, Get, Inject, Query } from "@nestjs/common";
import type { PortAdvisory } from "@portier/shared";
import { ApiBadRequestException } from "../../common/api-errors.js";
import { PortsService } from "./ports.service.js";

/**
 * Transport adapter for `GET /api/ports/advisory`. Extracts the query params and
 * maps the service result to the HTTP contract: a `200` advisory array, or — on
 * invalid input — a `400` via `ApiBadRequestException`. The `{ errors: [...] }`
 * envelope shape is owned by the shared error layer (`ApiErrorEnvelopeFilter` /
 * `toApiError`), so no envelope logic lives here.
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
      throw new ApiBadRequestException(result.errors);
    }
    return result.advisories;
  }
}
