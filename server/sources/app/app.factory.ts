import "reflect-metadata";
import type { INestApplication, LogLevel } from "@nestjs/common";
import { NestFactory } from "@nestjs/core";
import type { NestExpressApplication } from "@nestjs/platform-express";
import { AppModule, createLiveAppModule } from "./app.module.js";
import type { AppRuntime } from "./runtime-context.js";
import { configureStaticAssets } from "../static/static-serving.js";

/** Default NestJS log levels for the live server runtime. */
export const DEFAULT_SERVER_LOG_LEVELS: LogLevel[] = ["error", "warn", "log"];

export interface CreateNestAppOptions {
  /**
   * NestJS logger option. When omitted it defaults to {@link DEFAULT_SERVER_LOG_LEVELS}
   * for a live runtime and `false` (silent) otherwise (OpenAPI generation, tests).
   */
  logger?: LogLevel[] | false;
}

/** Resolves the NestJS logger option: explicit override, else enabled for a live runtime, else silent. */
export function resolveLoggerOption(
  runtime: AppRuntime | undefined,
  options: CreateNestAppOptions
): LogLevel[] | false {
  if (options.logger !== undefined) {
    return options.logger;
  }
  return runtime ? DEFAULT_SERVER_LOG_LEVELS : false;
}

/**
 * Builds the NestJS application without starting an HTTP listener.
 *
 * With a live `runtime` (the server entry, `sources/index.ts`), every provider binds
 * to the real `ForwardManager`/`ActivityStore`/runtime info, static asset serving is
 * wired from `runtime.staticClientDir`, and the NestJS logger is enabled. Without one
 * (OpenAPI generation, tests), the app builds with empty/in-memory defaults and a
 * silent logger. The caller owns `init()`/`listen()`/`close()`.
 */
export async function createNestApp(
  runtime?: AppRuntime,
  options: CreateNestAppOptions = {}
): Promise<INestApplication> {
  const app = await NestFactory.create<NestExpressApplication>(
    runtime ? createLiveAppModule(runtime) : AppModule,
    { logger: resolveLoggerOption(runtime, options) }
  );
  if (runtime?.staticClientDir) {
    configureStaticAssets(app, runtime.staticClientDir);
  }
  return app;
}
