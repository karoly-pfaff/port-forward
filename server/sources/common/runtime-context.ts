import { Global, Module, type DynamicModule } from "@nestjs/common";
import type { ActivityStore } from "../activity/activity-store.js";
import type { ForwardManager } from "../forward-manager.js";
import type { RuntimeInfoReader } from "../api/runtime/runtime.reader.js";
import type { StaticFallback } from "../static/static-serving.js";

/**
 * The live runtime dependencies the NestJS server binds when it is the active
 * server (`sources/index.ts`). The single `ForwardManager` instance satisfies every
 * reader/writer interface the feature providers need (status/forwards/config/
 * connections/diagnostics), so each token resolves to it; the `ActivityStore`,
 * `RuntimeInfoReader`, and `StaticFallback` back the remaining tokens.
 *
 * When no runtime is supplied (`null`) — OpenAPI generation and tests — the feature
 * providers fall back to their isolated empty/in-memory defaults, so the app builds
 * without a live forwarding runtime and tests override the specific tokens they seed.
 */
export interface AppRuntime {
  manager: ForwardManager;
  activity: ActivityStore;
  runtimeInfoReader: RuntimeInfoReader;
  staticFallback: StaticFallback;
  staticClientDir?: string;
}

/** Injection token carrying the live `AppRuntime` (or `null` in test/OpenAPI/doc mode). */
export const APP_RUNTIME = "APP_RUNTIME";

/**
 * Global module providing `APP_RUNTIME`. It is `@Global()` so every feature module's
 * provider factory can inject it without importing this module explicitly; the root
 * module (`AppModule`/`createLiveAppModule`) registers it exactly once via `forRoot`.
 */
@Global()
@Module({})
export class RuntimeContextModule {
  static forRoot(runtime: AppRuntime | null): DynamicModule {
    return {
      module: RuntimeContextModule,
      providers: [{ provide: APP_RUNTIME, useValue: runtime }],
      exports: [APP_RUNTIME],
    };
  }
}
