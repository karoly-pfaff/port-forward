import "reflect-metadata";
import { bootstrap, reportBootstrapFailure } from "./bootstrap.js";
import { resolveNestListenOptions } from "./nest-options.js";

/**
 * Executable entry point for the NestJS server (`npm run start:nest`).
 *
 * This is NOT the default active Portier server — running it does not replace the
 * Express runtime started by `sources/index.ts`; it serves the `/api` surface in
 * shadow mode.
 *
 * This file is intentionally logic-free wiring: it only resolves listen options
 * from the environment and starts the app. All of its constituents
 * (`resolveNestListenOptions`, `bootstrap`, `reportBootstrapFailure`) are
 * unit-covered; this entry is excluded from coverage like `sources/index.ts`,
 * because importing it would start a real listener as a side effect.
 */
void bootstrap(resolveNestListenOptions(process.env)).catch(reportBootstrapFailure);
