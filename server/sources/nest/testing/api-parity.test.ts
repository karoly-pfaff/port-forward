import http from "node:http";
import { afterEach, describe, expect, it } from "vitest";
import {
  diffApiResponses,
  fetchApi,
  stableStringify,
  startHandlerServer,
  type ParityServer,
} from "./api-parity.js";

describe("stableStringify", () => {
  it("sorts object keys recursively so key order is irrelevant", () => {
    const a = stableStringify({ b: 1, a: { d: 4, c: 3 } });
    const b = stableStringify({ a: { c: 3, d: 4 }, b: 1 });
    expect(a).toBe(b);
  });

  it("preserves array order and handles primitives and null", () => {
    expect(stableStringify([3, 1, 2])).toBe("[3,1,2]");
    expect(stableStringify(5)).toBe("5");
    expect(stableStringify("x")).toBe('"x"');
    expect(stableStringify(null)).toBe("null");
  });
});

describe("diffApiResponses", () => {
  it("returns no reasons for identical responses (ignoring key order)", () => {
    expect(
      diffApiResponses(
        { status: 200, body: { a: 1, b: 2 } },
        { status: 200, body: { b: 2, a: 1 } }
      )
    ).toEqual([]);
  });

  it("reports a status mismatch", () => {
    const reasons = diffApiResponses({ status: 200, body: null }, { status: 400, body: null });
    expect(reasons).toHaveLength(1);
    expect(reasons[0]).toContain("status mismatch");
  });

  it("reports a body mismatch", () => {
    const reasons = diffApiResponses({ status: 200, body: { a: 1 } }, { status: 200, body: { a: 2 } });
    expect(reasons).toHaveLength(1);
    expect(reasons[0]).toContain("body mismatch");
  });

  it("reports both a status and a body mismatch", () => {
    const reasons = diffApiResponses({ status: 200, body: 1 }, { status: 500, body: 2 });
    expect(reasons).toHaveLength(2);
  });
});

describe("startHandlerServer + fetchApi", () => {
  let server: ParityServer | undefined;

  afterEach(async () => {
    if (server) {
      await server.close();
      server = undefined;
    }
  });

  it("boots a handler and parses a JSON body and an empty body", async () => {
    const handler: http.RequestListener = (request, response) => {
      if (request.url === "/empty") {
        response.statusCode = 204;
        response.end();
        return;
      }
      response.setHeader("content-type", "application/json");
      response.end(JSON.stringify({ hello: "world" }));
    };
    server = await startHandlerServer(handler);

    expect(await fetchApi(server.baseUrl, "/json")).toEqual({ status: 200, body: { hello: "world" } });
    expect(await fetchApi(server.baseUrl, "/empty")).toEqual({ status: 204, body: null });
  });
});
