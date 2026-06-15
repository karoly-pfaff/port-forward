import "reflect-metadata";
import type { INestApplication } from "@nestjs/common";
import { NestFactory } from "@nestjs/core";
import { AppModule } from "./app.module.js";

/**
 * Builds the NestJS server application without starting an HTTP listener.
 *
 * Shared by `main.ts` (the executable bootstrap) and the integration tests so
 * both exercise the same composition. The caller owns `listen()`/`close()`.
 */
export async function createNestApp(): Promise<INestApplication> {
  return NestFactory.create(AppModule, { logger: false });
}
