import type { RuntimeInfo } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { RuntimeController } from "./runtime.controller.js";
import type { RuntimeService } from "./runtime.service.js";

const RUNTIME: RuntimeInfo = {
  name: "Portier",
  version: "9.9.9-test",
  runtime: "node",
  platform: "linux",
  arch: "x64",
  uptimeSeconds: 30,
  startedAt: "2026-06-14T12:00:00.000Z",
  managementHost: "127.0.0.1",
  managementPort: 47831,
  configPath: "/test/data/forwards.json",
  staticDir: "/test/web",
  serviceMode: false,
  pid: 1234,
};

describe("RuntimeController.get", () => {
  it("delegates to the service and maps the result to the response DTO", () => {
    const controller = new RuntimeController({ get: () => RUNTIME } as unknown as RuntimeService);

    const result = controller.get();

    expect(result).toEqual(RUNTIME); // byte-for-byte
    expect(result).not.toBe(RUNTIME); // mapped copy
  });
});
