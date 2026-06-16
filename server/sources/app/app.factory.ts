import "reflect-metadata";
import type { INestApplication } from "@nestjs/common";
import { NestFactory } from "@nestjs/core";
import type { NestExpressApplication } from "@nestjs/platform-express";
import { AppModule, createLiveAppModule } from "./app.module.js";
import type { AppRuntime } from "../common/runtime-context.js";
import { configureStaticAssets } from "../static/static-serving.js";

/**
 * Builds the NestJS application without starting an HTTP listener.
 *
 * With a live `runtime` (the active server entry, `sources/index.ts`), every
 * provider binds to the real `ForwardManager`/`ActivityStore`/runtime info, and
 * static asset serving is wired from `runtime.staticClientDir`. Without one (OpenAPI
 * generation, tests), the app builds with empty/in-memory defaults. The caller owns
 * `init()`/`listen()`/`close()`.
 */
export async function createNestApp(runtime?: AppRuntime): Promise<INestApplication> {
  const app = await NestFactory.create<NestExpressApplication>(
    runtime ? createLiveAppModule(runtime) : AppModule,
    { logger: false }
  );
  if (runtime?.staticClientDir) {
    configureStaticAssets(app, runtime.staticClientDir);
  }
  return app;
}
