import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ActivityEvent, ForwardRuleResponse, ForwardStatus, RuleDiagnosticsResult, RuntimeInfo } from "@portier/shared";
import { buildDiagnosticsBundle, buildDiagnosticsFilename, downloadJson } from "./diagnosticsExport.js";
import type { DiagnosisEntry } from "../forwards/ForwardRuleList.js";
import * as portierApi from "../../api/portierApi.js";

vi.mock("../../api/portierApi.js", () => ({
  fetchRuntimeInfo: vi.fn(),
  fetchForwardRules: vi.fn(),
  fetchForwardStatus: vi.fn(),
  fetchActivity: vi.fn()
}));

const testRuntime: RuntimeInfo = {
  name: "Portier",
  version: "1.2.0",
  runtime: "go",
  platform: "windows",
  arch: "x64",
  uptimeSeconds: 60,
  startedAt: "2026-01-01T00:00:00.000Z",
  managementHost: "127.0.0.1",
  managementPort: 47831,
  configPath: "C:\\ProgramData\\Portier\\rules.json",
  staticDir: "C:\\Program Files\\Portier\\web",
  serviceMode: true,
  pid: 999
};

const testRule: ForwardRuleResponse = {
  id: "r1",
  name: "Test Rule",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: 48001,
  targetHost: "127.0.0.1",
  targetPort: 3000,
  enabled: true,
  advisories: []
};

const testStatus: ForwardStatus = {
  ruleId: "r1",
  running: true, health: "healthy",
  bytesIn: 0,
  bytesOut: 0
};

const testActivity: ActivityEvent = {
  id: "a1",
  timestamp: "2026-01-01T00:00:00.000Z",
  type: "rule.started",
  severity: "success",
  ruleId: "r1",
  ruleName: "Test Rule",
  protocol: "tcp",
  message: "Rule started"
};

const testDiagnosticsResult: RuleDiagnosticsResult = {
  ruleId: "r1",
  ruleName: "Test Rule",
  protocol: "tcp",
  summary: { status: "pass", message: "All checks passed" },
  checks: [],
  diagnosedAt: "2026-01-01T00:00:00.000Z"
};

describe("buildDiagnosticsFilename", () => {
  it("returns a filename matching portier-diagnostics-*.json pattern", () => {
    const filename = buildDiagnosticsFilename();
    expect(filename).toMatch(/^portier-diagnostics-\d{8}-\d{6}\.json$/);
  });

  it("does not contain characters invalid on Windows", () => {
    const filename = buildDiagnosticsFilename();
    expect(filename).not.toMatch(/[\\/:*?"<>|]/);
  });
});

describe("buildDiagnosticsBundle — full success", () => {
  beforeEach(() => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue(testRuntime);
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([testRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([testStatus]);
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([testActivity]);
  });

  it("returns schemaVersion 1", async () => {
    const bundle = await buildDiagnosticsBundle(new Map());
    expect(bundle.schemaVersion).toBe("1");
  });

  it("includes an exportedAt ISO timestamp", async () => {
    const bundle = await buildDiagnosticsBundle(new Map());
    expect(bundle.exportedAt).toBeTruthy();
    expect(() => new Date(bundle.exportedAt)).not.toThrow();
  });

  it("includes app name and version from runtime", async () => {
    const bundle = await buildDiagnosticsBundle(new Map());
    expect(bundle.app.name).toBe("Portier");
    expect(bundle.app.version).toBe("1.2.0");
  });

  it("includes runtime info", async () => {
    const bundle = await buildDiagnosticsBundle(new Map());
    expect(bundle.runtime).toEqual(testRuntime);
  });

  it("includes rules from fresh fetch", async () => {
    const bundle = await buildDiagnosticsBundle(new Map());
    expect(bundle.rules).toEqual([testRule]);
  });

  it("includes statuses from fresh fetch", async () => {
    const bundle = await buildDiagnosticsBundle(new Map());
    expect(bundle.statuses).toEqual([testStatus]);
  });

  it("includes activity events", async () => {
    const bundle = await buildDiagnosticsBundle(new Map());
    expect(bundle.activity.included).toBe(true);
    expect(bundle.activity.events).toEqual([testActivity]);
    expect(bundle.activity.note).toBeTruthy();
  });

  it("fetches activity with limit 100", async () => {
    await buildDiagnosticsBundle(new Map());
    expect(portierApi.fetchActivity).toHaveBeenCalledWith({ limit: 100 });
  });

  it("includes metadata with source and generatedBy", async () => {
    const bundle = await buildDiagnosticsBundle(new Map());
    expect(bundle.metadata.source).toBe("client");
    expect(bundle.metadata.generatedBy).toBe("settings");
    expect(bundle.metadata.managementUrl).toContain("127.0.0.1");
  });

  it("has no errors field when all fetches succeed", async () => {
    const bundle = await buildDiagnosticsBundle(new Map());
    expect(bundle.errors).toBeUndefined();
  });

  it("does not include forbidden fields", async () => {
    const bundle = await buildDiagnosticsBundle(new Map());
    const json = JSON.stringify(bundle);
    expect(json).not.toContain("env");
    expect(json).not.toContain("HOME");
    expect(json).not.toContain("node_modules");
  });
});

describe("buildDiagnosticsBundle — diagnostics from UI state", () => {
  beforeEach(() => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue(testRuntime);
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([testRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([testStatus]);
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([testActivity]);
  });

  it("includes done diagnostics from diagnosisMap", async () => {
    const diagnosisMap = new Map<string, DiagnosisEntry>([
      ["r1", { state: "done", result: testDiagnosticsResult }]
    ]);
    const bundle = await buildDiagnosticsBundle(diagnosisMap);
    expect(bundle.diagnostics["r1"]).toEqual(testDiagnosticsResult);
    expect(bundle.diagnosticsNote).toBeUndefined();
  });

  it("excludes pending diagnosis entries", async () => {
    const diagnosisMap = new Map<string, DiagnosisEntry>([
      ["r1", { state: "pending" }]
    ]);
    const bundle = await buildDiagnosticsBundle(diagnosisMap);
    expect(bundle.diagnostics["r1"]).toBeUndefined();
  });

  it("excludes error diagnosis entries", async () => {
    const diagnosisMap = new Map<string, DiagnosisEntry>([
      ["r1", { state: "error", message: "failed" }]
    ]);
    const bundle = await buildDiagnosticsBundle(diagnosisMap);
    expect(bundle.diagnostics["r1"]).toBeUndefined();
  });

  it("sets diagnosticsNote when no done entries exist", async () => {
    const bundle = await buildDiagnosticsBundle(new Map());
    expect(bundle.diagnosticsNote).toBe("No rule diagnostics had been run in this UI session.");
  });

  it("does not set diagnosticsNote when diagnostics are present", async () => {
    const diagnosisMap = new Map<string, DiagnosisEntry>([
      ["r1", { state: "done", result: testDiagnosticsResult }]
    ]);
    const bundle = await buildDiagnosticsBundle(diagnosisMap);
    expect(bundle.diagnosticsNote).toBeUndefined();
  });
});

describe("buildDiagnosticsBundle — partial failure", () => {
  it("includes errors array when runtime fetch fails", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockRejectedValue(new Error("runtime down"));
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([testRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([testStatus]);
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([testActivity]);

    const bundle = await buildDiagnosticsBundle(new Map());
    expect(bundle.runtime).toBeNull();
    expect(bundle.rules).toEqual([testRule]);
    expect(bundle.errors).toBeDefined();
    expect(bundle.errors).toEqual(expect.arrayContaining([
      expect.objectContaining({ source: "runtime" })
    ]));
  });

  it("includes rules/statuses/activity even when runtime fails", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockRejectedValue(new Error("no runtime"));
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([testRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([testStatus]);
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([testActivity]);

    const bundle = await buildDiagnosticsBundle(new Map());
    expect(bundle.rules).toEqual([testRule]);
    expect(bundle.statuses).toEqual([testStatus]);
    expect(bundle.activity.events).toEqual([testActivity]);
  });

  it("accumulates errors for each failed source", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockRejectedValue(new Error("runtime err"));
    vi.mocked(portierApi.fetchForwardRules).mockRejectedValue(new Error("rules err"));
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    const bundle = await buildDiagnosticsBundle(new Map());
    expect(bundle.errors).toHaveLength(2);
    const sources = bundle.errors!.map((e) => e.source);
    expect(sources).toContain("runtime");
    expect(sources).toContain("rules");
  });

  it("returns empty arrays for failed rules and statuses", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue(testRuntime);
    vi.mocked(portierApi.fetchForwardRules).mockRejectedValue(new Error("fetch failed"));
    vi.mocked(portierApi.fetchForwardStatus).mockRejectedValue(new Error("fetch failed"));
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    const bundle = await buildDiagnosticsBundle(new Map());
    expect(bundle.rules).toEqual([]);
    expect(bundle.statuses).toEqual([]);
  });

  it("uses fallback management URL when runtime is unavailable", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockRejectedValue(new Error("no runtime"));
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    const bundle = await buildDiagnosticsBundle(new Map());
    expect(bundle.metadata.managementUrl).toBe("http://127.0.0.1:47831");
  });
});

describe("downloadJson", () => {
  beforeEach(() => {
    Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn(() => "blob:test") });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
  });

  it("creates a blob URL and triggers a click", () => {
    downloadJson("test.json", { foo: "bar" });
    expect(URL.createObjectURL).toHaveBeenCalled();
    expect(HTMLAnchorElement.prototype.click).toHaveBeenCalled();
  });

  it("revokes the object URL after download", () => {
    downloadJson("test.json", { foo: "bar" });
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:test");
  });

  it("sets the correct download filename on the anchor", () => {
    const anchors: HTMLAnchorElement[] = [];
    vi.spyOn(document, "createElement").mockImplementation((tag) => {
      const el = Object.getPrototypeOf(document).createElement.call(document, tag) as HTMLAnchorElement;
      if (tag === "a") anchors.push(el);
      return el;
    });

    downloadJson("portier-diagnostics-20260101-120000.json", {});
    expect(anchors[0]?.download).toBe("portier-diagnostics-20260101-120000.json");
  });
});
