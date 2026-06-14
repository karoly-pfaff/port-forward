import { Injectable } from "@nestjs/common";
import { getPortAdvisories, type PortAdvisory } from "@portier/shared";
import type { PortsAdvisoryQueryDto } from "./ports-advisory.query.dto.js";

/**
 * Behaviour for `GET /api/ports/advisory`. Input validation/coercion is owned by
 * `PortsAdvisoryQueryDto` + `ApiValidationPipe`; this service simply computes the
 * advisories from the validated query via the shared `getPortAdvisories`, so the
 * two runtimes cannot drift.
 */
@Injectable()
export class PortsService {
  getAdvisories(query: PortsAdvisoryQueryDto): PortAdvisory[] {
    return getPortAdvisories({
      port: query.port,
      listenHost: query.listenHost,
      purpose: query.purpose,
    });
  }
}
