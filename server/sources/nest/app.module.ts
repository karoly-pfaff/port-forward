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
 * Root module of the NestJS server.
 *
 * The Express TypeScript server (`sources/index.ts` + `sources/api.ts`) is the
 * default active runtime; this NestJS app serves the same `/api` surface in shadow
 * mode under `npm run start:nest`. It composes the feature modules — a `/health`
 * liveness probe, the `/api` read and write/lifecycle/config routes, and a
 * contract-shaped `/api/*` error envelope — and registers the global
 * `ApiErrorEnvelopeFilter` here (rather than in `main.ts`) so it is active in test
 * applications too. The `STATIC_FALLBACK` token (consumed by that filter for the
 * non-API SPA fallback) defaults to `disabledStaticFallback`: no static dir is wired
 * by default, so static serving is off until a directory is supplied; the API stays
 * fully usable either way.
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
