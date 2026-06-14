import { Controller, Get, Inject } from "@nestjs/common";
import { ApiOkResponse, ApiOperation, ApiTags } from "@nestjs/swagger";
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
@ApiTags("health")
@Controller()
export class HealthController {
  constructor(@Inject(HealthService) private readonly health: HealthService) {}

  @Get("health")
  @ApiOperation({ summary: "Liveness probe", description: "Scaffold liveness probe (outside the frozen /api contract)." })
  @ApiOkResponse({
    description: "The server is alive.",
    schema: {
      type: "object",
      properties: {
        ok: { type: "boolean", example: true },
        server: { type: "string", enum: ["node"] },
        name: { type: "string", example: "Portier" },
      },
      required: ["ok", "server", "name"],
    },
  })
  getHealth(): HealthStatus {
    return this.health.getHealth();
  }
}
