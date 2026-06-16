import { Module } from "@nestjs/common";
import { HealthController } from "./health.controller.js";
import { HealthService } from "./health.service.js";

/**
 * Composes the NestJS server health feature: its controller (HTTP surface) and its
 * service (behaviour). Feature modules keep concerns isolated — new endpoints arrive as their own
 * modules rather than one growing controller.
 */
@Module({
  controllers: [HealthController],
  providers: [HealthService],
})
export class HealthModule {}
