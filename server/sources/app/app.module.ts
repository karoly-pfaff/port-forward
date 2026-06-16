import "reflect-metadata";
import { Module, type DynamicModule, type Provider } from "@nestjs/common";
import { APP_FILTER } from "@nestjs/core";
import { HealthModule } from "../health/health.module.js";
import { PortsModule } from "../api/ports/ports.module.js";
import { ActivityModule } from "../api/activity/activity.module.js";
import { StatusModule } from "../api/status/status.module.js";
import { ForwardsModule } from "../api/forwards/forwards.module.js";
import { RuntimeModule } from "../api/runtime/runtime.module.js";
import { ConfigModule } from "../api/config/config.module.js";
import { ConnectionsModule } from "../api/connections/connections.module.js";
import { ApiErrorEnvelopeFilter } from "../common/api-error-envelope.filter.js";
import { APP_RUNTIME, RuntimeContextModule, type AppRuntime } from "../common/runtime-context.js";
import { STATIC_FALLBACK, disabledStaticFallback, type StaticFallback } from "../static/static-serving.js";

/**
 * Root module of the NestJS server (the default TypeScript server runtime).
 *
 * The feature modules — a `/health` liveness probe, the `/api` read/write/lifecycle/
 * config routes, and a contract-shaped `/api/*` error envelope — source their live
 * dependencies from the global `RuntimeContextModule` (`APP_RUNTIME`): the active
 * server (`sources/index.ts`) builds the app with `createLiveAppModule(runtime)` so
 * every provider resolves to the live `ForwardManager`/`ActivityStore`/runtime info/
 * static fallback. When no runtime is supplied — the static `AppModule` used by
 * OpenAPI generation and tests — `APP_RUNTIME` is `null` and the providers fall back
 * to empty/in-memory defaults (tests override the specific tokens they seed).
 */

const FEATURE_MODULES = [
  HealthModule,
  PortsModule,
  ActivityModule,
  StatusModule,
  ForwardsModule,
  RuntimeModule,
  ConfigModule,
  ConnectionsModule,
];

/** Root-level providers shared by the static and live root modules. */
function rootProviders(): Provider[] {
  return [
    {
      provide: STATIC_FALLBACK,
      useFactory: (rt: AppRuntime | null): StaticFallback => rt?.staticFallback ?? disabledStaticFallback,
      inject: [APP_RUNTIME],
    },
    { provide: APP_FILTER, useClass: ApiErrorEnvelopeFilter },
  ];
}

/**
 * Static root module: no live runtime (`APP_RUNTIME = null`). Used by OpenAPI
 * generation and tests (which seed via `overrideProvider`).
 */
@Module({
  imports: [RuntimeContextModule.forRoot(null), ...FEATURE_MODULES],
  providers: rootProviders(),
})
export class AppModule {}

/**
 * Empty host class for the live dynamic root module. A distinct class (not
 * `AppModule`) so Nest does not merge `AppModule`'s static `@Module` metadata —
 * which would double-register `RuntimeContextModule` and the feature modules.
 */
@Module({})
export class LiveAppModule {}

/**
 * Live root module: binds the supplied `AppRuntime` so every feature provider
 * resolves to the live `ForwardManager`/`ActivityStore`/runtime info/static fallback.
 * Used by the active server entry (`sources/index.ts`).
 */
export function createLiveAppModule(runtime: AppRuntime): DynamicModule {
  return {
    module: LiveAppModule,
    imports: [RuntimeContextModule.forRoot(runtime), ...FEATURE_MODULES],
    providers: rootProviders(),
  };
}
