import "reflect-metadata";
import { Module } from "@nestjs/common";
import { APP_FILTER } from "@nestjs/core";
import { HealthModule } from "./health/health.module.js";
import { ApiNotFoundFilter } from "./common/api-not-found.filter.js";

/**
 * Root module of the NestJS server scaffold (v1.14).
 *
 * This app does NOT replace the existing Express TypeScript server yet — it is
 * an incremental, reversible migration foundation. It currently exposes only a
 * minimal, contract-safe surface: a `/health` liveness probe and a contract-
 * shaped `/api/*` 404. The global `ApiNotFoundFilter` is registered here (rather
 * than in `main.ts`) so it is active in test applications too.
 */
@Module({
  imports: [HealthModule],
  providers: [{ provide: APP_FILTER, useClass: ApiNotFoundFilter }],
})
export class AppModule {}
