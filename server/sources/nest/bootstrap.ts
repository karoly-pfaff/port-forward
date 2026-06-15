import "reflect-metadata";
import type { INestApplication } from "@nestjs/common";
import { createNestApp } from "./app.factory.js";
import type { NestListenOptions } from "./nest-options.js";

/**
 * Builds the NestJS app and starts listening on the given options.
 *
 * Takes the listen options explicitly (rather than reading `process.env`) so it
 * can be started on an ephemeral loopback port in tests — keeping the startup
 * glue fully covered. Returns the running app so the caller owns `close()`.
 */
export async function bootstrap(listen: NestListenOptions): Promise<INestApplication> {
  const app = await createNestApp();
  await app.listen(listen.port, listen.host);
  console.log(
    `Portier NestJS server listening on http://${listen.host}:${listen.port} ` +
      `(shadow mode — the existing TypeScript server remains the active runtime).`
  );
  return app;
}

/** Reports a startup failure without crashing the process abruptly. */
export function reportBootstrapFailure(error: unknown): void {
  console.error("Portier NestJS server failed to start.", error);
  process.exitCode = 1;
}
