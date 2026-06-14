import { Controller, Get, Inject } from "@nestjs/common";
import { HealthService, type HealthStatus } from "./health.service.js";

/**
 * Transport adapter for the scaffold liveness probe. Controllers in the NestJS
 * migration are HTTP adapters only — all behaviour lives in the service.
 *
 * The dependency is wired with an explicit `@Inject` token so the scaffold does
 * not rely on emitted decorator metadata (`design:paramtypes`), which the
 * esbuild-based test transform does not produce. This keeps DI identical
 * between `tsc` builds and Vitest runs.
 */
@Controller()
export class HealthController {
  constructor(@Inject(HealthService) private readonly health: HealthService) {}

  @Get("health")
  getHealth(): HealthStatus {
    return this.health.getHealth();
  }
}
