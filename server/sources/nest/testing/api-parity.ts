import http from "node:http";
import type { AddressInfo } from "node:net";
import type { INestApplication } from "@nestjs/common";

/**
 * Small, local API parity harness comparing the Express and NestJS runtimes.
 *
 * It boots an HTTP handler (the Express app) or a NestJS app on an ephemeral
 * loopback port, captures `{ status, body }` for a request, and deterministically
 * compares two responses. Its purpose is to make migrating an endpoint safer:
 * a test can fire the same request at the existing Express route and the new
 * Nest route and assert they behave identically. It is intentionally tiny — not
 * a framework — and lives only under the server test sources.
 */

export interface ApiResponse {
  status: number;
  body: unknown;
}

export interface ParityServer {
  baseUrl: string;
  close: () => Promise<void>;
}

/** Recursively sorts object keys so semantically-equal values stringify identically. */
function sortKeys(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(sortKeys);
  }
  if (value !== null && typeof value === "object") {
    const source = value as Record<string, unknown>;
    return Object.keys(source)
      .sort()
      .reduce<Record<string, unknown>>((acc, key) => {
        acc[key] = sortKeys(source[key]);
        return acc;
      }, {});
  }
  return value;
}

/** Deterministic JSON: object keys sorted recursively so key order never causes a false mismatch. */
export function stableStringify(value: unknown): string {
  return JSON.stringify(sortKeys(value));
}

/**
 * Returns the reasons two API responses differ (empty array = identical). Pure,
 * so parity assertions can be unit-tested without booting a server.
 */
export function diffApiResponses(expected: ApiResponse, actual: ApiResponse): string[] {
  const reasons: string[] = [];
  if (expected.status !== actual.status) {
    reasons.push(`status mismatch: ${expected.status} vs ${actual.status}`);
  }
  if (stableStringify(expected.body) !== stableStringify(actual.body)) {
    reasons.push(
      `body mismatch: ${stableStringify(expected.body)} vs ${stableStringify(actual.body)}`
    );
  }
  return reasons;
}

/** Boots a raw HTTP handler (e.g. the Express app) on an ephemeral loopback port. */
export async function startHandlerServer(handler: http.RequestListener): Promise<ParityServer> {
  const server = http.createServer(handler);
  const port = await new Promise<number>((resolve) => {
    server.listen(0, "127.0.0.1", () => {
      resolve((server.address() as AddressInfo).port);
    });
  });
  return {
    baseUrl: `http://127.0.0.1:${port}`,
    close: () => new Promise<void>((resolve) => server.close(() => resolve())),
  };
}

/** Boots a NestJS application on an ephemeral loopback port. */
export async function startNestServer(app: INestApplication): Promise<ParityServer> {
  await app.listen(0, "127.0.0.1");
  return {
    baseUrl: await app.getUrl(),
    close: () => app.close(),
  };
}

/** Fetches `path` and captures the status and parsed JSON body (null for an empty body). */
export async function fetchApi(
  baseUrl: string,
  path: string,
  init?: RequestInit
): Promise<ApiResponse> {
  const response = await fetch(`${baseUrl}${path}`, init);
  const text = await response.text();
  return { status: response.status, body: text === "" ? null : JSON.parse(text) };
}
