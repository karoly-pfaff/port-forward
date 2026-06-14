import "reflect-metadata";
import { Module } from "@nestjs/common";
import { APP_FILTER } from "@nestjs/core";
import { HealthModule } from "./health/health.module.js";
import { PortsModule } from "./api/ports/ports.module.js";
import { ActivityModule } from "./api/activity/activity.module.js";
import { StatusModule } from "./api/status/status.module.js";
import { ForwardsModule } from "./api/forwards/forwards.module.js";
import { ApiErrorEnvelopeFilter } from "./common/api-error-envelope.filter.js";

/**
 * Root module of the NestJS server scaffold (v1.14).
 *
 * This app does NOT replace the existing Express TypeScript server yet — it is
 * an incremental, reversible migration foundation. It exposes a minimal,
 * contract-safe surface: a `/health` liveness probe, the migrated read-only API
 * routes (`GET /api/ports/advisory`, `GET /api/activity`, `GET /api/status`,
 * `GET /api/forwards`), and a contract-shaped `/api/*` error envelope for
 * everything not yet migrated. The global `ApiErrorEnvelopeFilter` is registered
 * here (rather than in `main.ts`) so it is active in test applications too.
 */
@Module({
  imports: [HealthModule, PortsModule, ActivityModule, StatusModule, ForwardsModule],
  providers: [{ provide: APP_FILTER, useClass: ApiErrorEnvelopeFilter }],
})
export class AppModule {}
