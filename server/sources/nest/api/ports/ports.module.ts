import { Module } from "@nestjs/common";
import { PortsController } from "./ports.controller.js";
import { PortsService } from "./ports.service.js";

/**
 * The first migrated read-only API feature: `GET /api/ports/advisory`.
 * Composes its controller (transport) and service (behaviour).
 */
@Module({
  controllers: [PortsController],
  providers: [PortsService],
})
export class PortsModule {}
