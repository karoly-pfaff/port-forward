import "reflect-metadata";
import type { INestApplication } from "@nestjs/common";
import { afterEach, describe, expect, it, vi } from "vitest";
import { bootstrap, reportBootstrapFailure } from "./bootstrap.js";

describe("bootstrap", () => {
  let app: INestApplication | undefined;

  afterEach(async () => {
    if (app) {
      await app.close();
      app = undefined;
    }
    vi.restoreAllMocks();
  });

  it("creates and starts the Nest app on the given loopback options", async () => {
    const log = vi.spyOn(console, "log").mockImplementation(() => undefined);

    app = await bootstrap({ host: "127.0.0.1", port: 0 });
    const url = await app.getUrl();
    const response = await fetch(`${url}/health`);

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ ok: true, server: "node", name: "Portier" });
    expect(url).toContain("127.0.0.1");
    expect(log).toHaveBeenCalledOnce();
  });
});

describe("reportBootstrapFailure", () => {
  it("logs the error and sets a failing exit code", () => {
    const previousExitCode = process.exitCode;
    const error = vi.spyOn(console, "error").mockImplementation(() => undefined);

    try {
      reportBootstrapFailure(new Error("boom"));

      expect(process.exitCode).toBe(1);
      expect(error).toHaveBeenCalledWith(
        "Portier NestJS scaffold failed to start.",
        expect.any(Error)
      );
    } finally {
      process.exitCode = previousExitCode;
      error.mockRestore();
    }
  });
});
