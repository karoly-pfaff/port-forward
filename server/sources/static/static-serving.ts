import { existsSync } from "node:fs";
import { join } from "node:path";
import type { NestExpressApplication } from "@nestjs/platform-express";
import type { Request, Response } from "express";

/**
 * Static client serving for the NestJS server.
 *
 * Serves the web client with SPA-fallback semantics: a static
 * directory is served only when it actually contains an `index.html`; real assets
 * are served by the platform's static middleware (`express.static` under the hood);
 * any unmatched **non-API** GET/HEAD route falls back to `index.html` (SPA routing);
 * and a missing/absent static directory is allowed — the API stays fully usable.
 *
 * The split: `configureStaticAssets` registers asset serving
 * (pre-router, like `express.static`), and the SPA fallback runs at the unmatched-route
 * stage via the shared error filter (Nest surfaces unmatched routes as
 * `NotFoundException`, so the filter is the single place that already decides what an
 * unmatched route returns — `/api/*` → the error envelope, non-API → the SPA index).
 * Static serving never touches the `/api/*` error-envelope behavior.
 */

/** The SPA entry file; a static dir is only usable when it contains this file. */
export const STATIC_INDEX_FILE = "index.html";

/** Injection token for the SPA static-fallback strategy consumed by the error filter. */
export const STATIC_FALLBACK = "STATIC_FALLBACK";

/** A static dir is usable only if it has an index.html. */
export function hasStaticClient(staticDir: string): boolean {
  return existsSync(join(staticDir, STATIC_INDEX_FILE));
}

/** The resolved static-serving state for a (possibly undefined/missing) directory. */
export interface ResolvedStatic {
  /** True when `staticDir` exists and contains an `index.html`. */
  enabled: boolean;
  /** The configured directory (or null when none was supplied). */
  staticDir: string | null;
  /** The absolute `index.html` path when enabled (else null). */
  indexPath: string | null;
}

/** Resolves whether static serving is enabled for a (possibly undefined/missing) dir. */
export function resolveStaticOptions(staticDir?: string): ResolvedStatic {
  if (staticDir && hasStaticClient(staticDir)) {
    return { enabled: true, staticDir, indexPath: join(staticDir, STATIC_INDEX_FILE) };
  }
  return { enabled: false, staticDir: staticDir ?? null, indexPath: null };
}

/**
 * SPA fallback strategy: decides whether a non-API not-found request should be
 * answered with the SPA index (and serves it). Injected into the error filter so the
 * filter delegates the static decision instead of embedding filesystem logic.
 */
export interface StaticFallback {
  /**
   * Serves the SPA index for a non-API GET/HEAD route and returns true; returns
   * false when static serving is disabled or the method is not GET/HEAD (the caller
   * then keeps the default not-found behavior). Only called by the filter for
   * non-API unmatched routes, so it does not re-check the path.
   */
  handle(request: Request, response: Response): boolean;
}

/** The default: static serving disabled — the filter keeps its default 404. */
export const disabledStaticFallback: StaticFallback = { handle: () => false };

/** Builds a SPA fallback that serves `<dir>/index.html` for non-API GET/HEAD not-founds. */
export function createStaticFallback(staticDir: string): StaticFallback {
  const resolved = resolveStaticOptions(staticDir);
  if (!resolved.enabled || resolved.indexPath === null) {
    return disabledStaticFallback;
  }
  const indexPath = resolved.indexPath;
  return {
    handle(request: Request, response: Response): boolean {
      if (request.method !== "GET" && request.method !== "HEAD") {
        return false;
      }
      response.sendFile(indexPath);
      return true;
    },
  };
}

/**
 * Registers real asset serving on a Nest (express-platform) app — the equivalent of
 * the platform's `express.static(dir)`. Call before `app.init()/listen()`. Returns the
 * resolved static state (for logging / wiring the fallback). When disabled (no dir or
 * no `index.html`) the app serves no static assets and the API stays fully usable.
 */
export function configureStaticAssets(app: NestExpressApplication, staticDir?: string): ResolvedStatic {
  const resolved = resolveStaticOptions(staticDir);
  if (resolved.enabled && resolved.staticDir) {
    app.useStaticAssets(resolved.staticDir);
  }
  return resolved;
}
