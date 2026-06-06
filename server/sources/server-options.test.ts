import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { DEFAULT_MANAGEMENT_HOST, DEFAULT_MANAGEMENT_PORT, resolveServerOptions } from "./server-options.js";

describe("resolveServerOptions", () => {
  it("uses service-safe defaults", () => {
    const cwd = "C:\\portier";

    expect(resolveServerOptions([], {}, cwd)).toEqual({
      service: false,
      configPath: resolve(cwd, "data/forwards.json"),
      host: DEFAULT_MANAGEMENT_HOST,
      port: DEFAULT_MANAGEMENT_PORT,
      staticClientDir: resolve(cwd, "client", "build"),
      staticClientDirSource: "default"
    });
  });

  it("allows environment variables to configure the service", () => {
    const cwd = "C:\\portier";

    expect(
      resolveServerOptions(
        [],
        {
          PORTIER_CONFIG: "rules.json",
          PORTIER_HOST: "0.0.0.0",
          PORTIER_PORT: "5000",
          PORTIER_STATIC_DIR: "web"
        },
        cwd
      )
    ).toEqual({
      service: false,
      configPath: resolve(cwd, "rules.json"),
      host: "0.0.0.0",
      port: 5000,
      staticClientDir: resolve(cwd, "web"),
      staticClientDirSource: "env"
    });
  });

  it("lets CLI flags override environment variables", () => {
    const cwd = "C:\\portier";

    expect(
      resolveServerOptions(
        [
          "--service",
          "--config",
          "cli.json",
          "--host",
          "127.0.0.2",
          "--port",
          "5001",
          "--static-dir",
          "static"
        ],
        {
          PORTIER_CONFIG: "env.json",
          PORTIER_HOST: "0.0.0.0",
          PORTIER_PORT: "5000",
          PORTIER_STATIC_DIR: "web"
        },
        cwd
      )
    ).toMatchObject({
      service: true,
      configPath: resolve(cwd, "cli.json"),
      host: "127.0.0.2",
      port: 5001,
      staticClientDir: resolve(cwd, "static"),
      staticClientDirSource: "cli"
    });
  });

  it("defaults static files relative to the packaged executable", () => {
    expect(resolveServerOptions([], {}, "C:\\work", "C:\\Tools\\Portier\\portier-server.exe")).toMatchObject({
      staticClientDir: resolve("C:\\Tools\\Portier", "client", "build"),
      staticClientDirSource: "default"
    });
  });

  it("rejects invalid ports", () => {
    expect(() => resolveServerOptions(["--port", "99999"], {}, "C:\\portier")).toThrow("Invalid port");
  });
});
