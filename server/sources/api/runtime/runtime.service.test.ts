import { describe, expect, it } from "vitest";
import type { RuntimeInfoOptions } from "../../runtime/runtime-info.js";
import { RuntimeService } from "./runtime.service.js";
import type { ClockReader, ProcessReader, RuntimeInfoReader } from "./runtime.reader.js";

const STARTED_AT = new Date("2026-06-14T12:00:00.000Z");
const NOW = new Date("2026-06-14T12:00:30.000Z"); // 30s later

function service(opts: {
  info?: RuntimeInfoOptions;
  now?: Date;
  startedAt?: Date;
  pid?: number;
  platform?: string;
  arch?: string;
}): RuntimeService {
  const clock: ClockReader = { now: () => opts.now ?? NOW };
  const proc: ProcessReader = {
    pid: () => opts.pid ?? 1234,
    platform: () => opts.platform ?? "linux",
    arch: () => opts.arch ?? "x64",
  };
  const runtimeInfo: RuntimeInfoReader = {
    options: () => opts.info,
    startedAt: () => opts.startedAt ?? STARTED_AT,
  };
  return new RuntimeService(clock, proc, runtimeInfo);
}

describe("RuntimeService.get", () => {
  it("composes the readers into the shared runtime-info shape", () => {
    const info: RuntimeInfoOptions = {
      version: "9.9.9-test",
      managementHost: "127.0.0.1",
      managementPort: 47831,
      configPath: "/test/data/forwards.json",
      staticDir: "/test/web",
      serviceMode: false,
      startedAt: STARTED_AT,
    };

    expect(service({ info }).get()).toEqual({
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
      recovery: { active: false },
    });
  });

  it("applies defaults when no runtime info is wired (default case)", () => {
    const result = service({ pid: 99, platform: "darwin", arch: "arm64" }).get();
    expect(result.version).toBe("unknown");
    expect(result.runtime).toBe("node");
    expect(result.platform).toBe("macos");
    expect(result.arch).toBe("arm64");
    expect(result.configPath).toBe("");
    expect(result.staticDir).toBe("");
    expect(result.serviceMode).toBe(false);
    expect(result.pid).toBe(99);
    expect(result.uptimeSeconds).toBe(30);
    expect(result.recovery).toEqual({ active: false });
  });

  it("passes through an active recovery block from the runtime info", () => {
    const info: RuntimeInfoOptions = {
      version: "9.9.9-test",
      managementHost: "127.0.0.1",
      managementPort: 47831,
      configPath: "/test/data/forwards.json",
      staticDir: "/test/web",
      serviceMode: false,
      startedAt: STARTED_AT,
      recovery: {
        active: true,
        reason: "malformed",
        message: "Configuration file could not be parsed; started with no active rules. The original file was quarantined.",
        configPath: "/test/data/forwards.json",
        quarantinePath: "/test/data/forwards.json.corrupt-2026-06-14T120000Z",
        writesBlocked: true,
        detectedAt: "2026-06-14T12:00:00.000Z",
      },
    };
    expect(service({ info }).get().recovery).toEqual(info.recovery);
  });
});
