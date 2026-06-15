import "reflect-metadata";
import { Module } from "@nestjs/common";
import { APP_FILTER } from "@nestjs/core";
import { HealthModule } from "./health/health.module.js";
import { PortsModule } from "./api/ports/ports.module.js";
import { ActivityModule } from "./api/activity/activity.module.js";
import { StatusModule } from "./api/status/status.module.js";
import { ForwardsModule } from "./api/forwards/forwards.module.js";
import { RuntimeModule } from "./api/runtime/runtime.module.js";
import { ConfigModule } from "./api/config/config.module.js";
import { ConnectionsModule } from "./api/connections/connections.module.js";
import { ApiErrorEnvelopeFilter } from "./common/api-error-envelope.filter.js";
import { STATIC_FALLBACK, disabledStaticFallback } from "./static/static-serving.js";

/**
 * Root module of the NestJS server scaffold (v1.14).
 *
 * This app does NOT replace the existing Express TypeScript server yet — it is
 * an incremental, reversible migration foundation. It exposes a minimal,
 * contract-safe surface: a `/health` liveness probe, the migrated read-only API
 * routes (`GET /api/ports/advisory`, `GET /api/activity`, `GET /api/status`,
 * `GET /api/forwards`, `GET /api/runtime`, `GET /api/config/export`,
 * `GET /api/connections`) plus `DELETE /api/activity`, and a contract-shaped
 * `/api/*` error envelope for everything not yet migrated. The
 * global `ApiErrorEnvelopeFilter` is registered here (rather than in `main.ts`)
 * so it is active in test applications too. The `STATIC_FALLBACK` token (consumed
 * by that filter for the non-API SPA fallback) defaults to `disabledStaticFallback`
 * — the scaffold wires no static dir, so static serving is off until a test (or a
 * future runtime switch) supplies a directory; the API stays fully usable either way.
 */
@Module({
  imports: [
    HealthModule,
    PortsModule,
    ActivityModule,
    StatusModule,
    ForwardsModule,
    RuntimeModule,
    ConfigModule,
    ConnectionsModule,
  ],
  providers: [
    { provide: STATIC_FALLBACK, useValue: disabledStaticFallback },
    { provide: APP_FILTER, useClass: ApiErrorEnvelopeFilter },
  ],
})
export class AppModule {}
