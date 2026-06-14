import type { RuntimeInfo } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { toRuntimeInfoResponseDto } from "./runtime.response.dto.js";

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

describe("toRuntimeInfoResponseDto", () => {
  it("preserves the runtime info shape byte-for-byte without mutating the source", () => {
    const snapshot = structuredClone(RUNTIME);

    const dto = toRuntimeInfoResponseDto(RUNTIME);

    expect(dto).toEqual(RUNTIME);
    expect(dto).not.toBe(RUNTIME); // fresh object
    expect(RUNTIME).toEqual(snapshot); // source untouched
  });
});
