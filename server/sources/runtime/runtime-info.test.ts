import { PORTIER_DEFAULT_HOST, PORTIER_DEFAULT_PORT } from "@portier/shared";
import { describe, expect, it } from "vitest";
import {
  buildRuntimeInfo,
  normalizeArch,
  normalizePlatform,
  type RuntimeInfoOptions,
} from "./runtime-info.js";

const STARTED_AT = new Date("2026-06-14T12:00:00.000Z");
const NOW = new Date("2026-06-14T12:00:42.000Z"); // 42s later

const fullInfo: RuntimeInfoOptions = {
  version: "9.9.9-test",
  managementHost: "127.0.0.1",
  managementPort: 47831,
  configPath: "/test/data/forwards.json",
  staticDir: "/test/web",
  serviceMode: true,
  startedAt: STARTED_AT,
};

describe("normalizePlatform", () => {
  it("maps known platforms and falls back to unknown", () => {
    expect(normalizePlatform("win32")).toBe("windows");
    expect(normalizePlatform("darwin")).toBe("macos");
    expect(normalizePlatform("linux")).toBe("linux");
    expect(normalizePlatform("freebsd")).toBe("unknown");
    expect(normalizePlatform("")).toBe("unknown");
  });
});

describe("normalizeArch", () => {
  it("maps known archs and falls back to unknown", () => {
    expect(normalizeArch("x64")).toBe("x64");
    expect(normalizeArch("arm64")).toBe("arm64");
    expect(normalizeArch("ia32")).toBe("unknown");
    expect(normalizeArch("")).toBe("unknown");
  });
});

describe("buildRuntimeInfo", () => {
  it("builds the full response from provided runtime info", () => {
    expect(
      buildRuntimeInfo({
        runtimeInfo: fullInfo,
        startedAt: STARTED_AT,
        now: NOW,
        pid: 4242,
        platform: "linux",
        arch: "arm64",
      })
    ).toEqual({
      name: "Portier",
      version: "9.9.9-test",
      runtime: "node",
      platform: "linux",
      arch: "arm64",
      uptimeSeconds: 42,
      startedAt: "2026-06-14T12:00:00.000Z",
      managementHost: "127.0.0.1",
      managementPort: 47831,
      configPath: "/test/data/forwards.json",
      staticDir: "/test/web",
      serviceMode: true,
      pid: 4242,
      recovery: { active: false },
    });
  });

  it("applies the historical defaults when no runtime info is provided", () => {
    expect(
      buildRuntimeInfo({
        runtimeInfo: undefined,
        startedAt: STARTED_AT,
        now: NOW,
        pid: 7,
        platform: "win32",
        arch: "x64",
      })
    ).toEqual({
      name: "Portier",
      version: "unknown",
      runtime: "node",
      platform: "windows",
      arch: "x64",
      uptimeSeconds: 42,
      startedAt: "2026-06-14T12:00:00.000Z",
      managementHost: PORTIER_DEFAULT_HOST,
      managementPort: PORTIER_DEFAULT_PORT,
      configPath: "",
      staticDir: "",
      serviceMode: false,
      pid: 7,
      recovery: { active: false },
    });
  });

  it("floors uptime to whole seconds and never goes negative for now === startedAt", () => {
    const sameInstant = buildRuntimeInfo({
      runtimeInfo: fullInfo,
      startedAt: STARTED_AT,
      now: STARTED_AT,
      pid: 1,
      platform: "linux",
      arch: "x64",
    });
    expect(sameInstant.uptimeSeconds).toBe(0);

    const subSecond = buildRuntimeInfo({
      runtimeInfo: fullInfo,
      startedAt: STARTED_AT,
      now: new Date(STARTED_AT.getTime() + 1999), // 1.999s
      pid: 1,
      platform: "linux",
      arch: "x64",
    });
    expect(subSecond.uptimeSeconds).toBe(1);
  });

  it("preserves the contract field order in JSON serialization", () => {
    const json = JSON.stringify(
      buildRuntimeInfo({
        runtimeInfo: fullInfo,
        startedAt: STARTED_AT,
        now: NOW,
        pid: 4242,
        platform: "linux",
        arch: "arm64",
      })
    );
    expect(Object.keys(JSON.parse(json) as Record<string, unknown>)).toEqual([
      "name",
      "version",
      "runtime",
      "platform",
      "arch",
      "uptimeSeconds",
      "startedAt",
      "managementHost",
      "managementPort",
      "configPath",
      "staticDir",
      "serviceMode",
      "pid",
      "recovery",
    ]);
  });
});
