import { afterEach, describe, expect, it, vi } from "vitest";
import type { ExportedConfig, ForwardRule, ForwardRuleResponse, ForwardStatus } from "@portier/shared";
import {
  clearActivity,
  deleteForwardRule,
  diagnoseForwardRule,
  exportConfig,
  fetchActivity,
  fetchForwardRules,
  fetchForwardStatus,
  fetchLiveConnections,
  fetchPortAdvisories,
  fetchRuntimeInfo,
  importConfig,
  reorderForwardRules,
  saveForwardRule,
  setForwardRuleRunning
} from "./portierApi.js";

function mockFetchOk(jsonData: unknown): void {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(jsonData)
    })
  );
}

function mockFetchError(status: number, errors?: string[]): void {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: false,
      status,
      json: errors
        ? () => Promise.resolve({ errors })
        : () => Promise.reject(new Error("no json"))
    })
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

const rule: ForwardRule = {
  id: "r1",
  name: "Test",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: 48001,
  targetHost: "127.0.0.1",
  targetPort: 3000,
  enabled: true
};

describe("fetchForwardRules", () => {
  it("returns rules from GET /api/forwards", async () => {
    const data: ForwardRuleResponse[] = [{ ...rule, advisories: [] }];
    mockFetchOk(data);
    const result = await fetchForwardRules();
    expect(result).toEqual(data);
    expect(vi.mocked(fetch)).toHaveBeenCalledWith("/api/forwards");
  });
});

describe("fetchForwardStatus", () => {
  it("returns statuses from GET /api/status", async () => {
    const data: ForwardStatus[] = [{ ruleId: "r1", running: false, bytesIn: 0, bytesOut: 0 }];
    mockFetchOk(data);
    const result = await fetchForwardStatus();
    expect(result).toEqual(data);
    expect(vi.mocked(fetch)).toHaveBeenCalledWith("/api/status");
  });
});

describe("saveForwardRule", () => {
  it("POSTs to /api/forwards when id is undefined", async () => {
    mockFetchOk({ ...rule, advisories: [] });
    await saveForwardRule(undefined, rule);
    expect(vi.mocked(fetch)).toHaveBeenCalledWith("/api/forwards", expect.objectContaining({ method: "POST" }));
  });

  it("PATCHes /api/forwards/:id when id is provided", async () => {
    mockFetchOk({ ...rule, advisories: [] });
    await saveForwardRule("r1", rule);
    expect(vi.mocked(fetch)).toHaveBeenCalledWith("/api/forwards/r1", expect.objectContaining({ method: "PATCH" }));
  });
});

describe("deleteForwardRule", () => {
  it("DELETEs /api/forwards/:id", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({ ok: true, status: 204, json: () => Promise.resolve(null) })
    );
    await deleteForwardRule(rule);
    expect(vi.mocked(fetch)).toHaveBeenCalledWith("/api/forwards/r1", expect.objectContaining({ method: "DELETE" }));
  });
});

describe("setForwardRuleRunning", () => {
  it("POSTs start endpoint when running=true", async () => {
    const status: ForwardStatus = { ruleId: "r1", running: true, bytesIn: 0, bytesOut: 0 };
    mockFetchOk(status);
    const result = await setForwardRuleRunning(rule, true);
    expect(result.running).toBe(true);
    expect(vi.mocked(fetch)).toHaveBeenCalledWith("/api/forwards/r1/start", expect.objectContaining({ method: "POST" }));
  });

  it("POSTs stop endpoint when running=false", async () => {
    const status: ForwardStatus = { ruleId: "r1", running: false, bytesIn: 0, bytesOut: 0 };
    mockFetchOk(status);
    await setForwardRuleRunning(rule, false);
    expect(vi.mocked(fetch)).toHaveBeenCalledWith("/api/forwards/r1/stop", expect.objectContaining({ method: "POST" }));
  });
});

describe("reorderForwardRules", () => {
  it("POSTs /api/forwards/reorder with ids", async () => {
    const data: ForwardRuleResponse[] = [{ ...rule, advisories: [] }];
    mockFetchOk(data);
    const result = await reorderForwardRules(["r1"]);
    expect(result).toEqual(data);
    expect(vi.mocked(fetch)).toHaveBeenCalledWith("/api/forwards/reorder", expect.objectContaining({ method: "POST" }));
  });
});

describe("fetchPortAdvisories", () => {
  it("fetches with port and purpose", async () => {
    mockFetchOk([]);
    await fetchPortAdvisories({ port: 48001, purpose: "forward" });
    const url = (vi.mocked(fetch).mock.calls[0][0] as string);
    expect(url).toContain("port=48001");
    expect(url).toContain("purpose=forward");
    expect(url).not.toContain("listenHost");
  });

  it("includes listenHost when provided", async () => {
    mockFetchOk([]);
    await fetchPortAdvisories({ port: 48001, listenHost: "0.0.0.0", purpose: "forward" });
    const url = (vi.mocked(fetch).mock.calls[0][0] as string);
    expect(url).toContain("listenHost=0.0.0.0");
  });
});

describe("fetchActivity", () => {
  it("fetches /api/activity with no params when called with empty params", async () => {
    mockFetchOk({ events: [] });
    const result = await fetchActivity();
    expect(result).toEqual([]);
    expect(vi.mocked(fetch)).toHaveBeenCalledWith("/api/activity");
  });

  it("includes all provided params in query string", async () => {
    mockFetchOk({ events: [] });
    await fetchActivity({ limit: 10, ruleId: "r1", type: "rule.created", severity: "success" });
    const url = (vi.mocked(fetch).mock.calls[0][0] as string);
    expect(url).toContain("limit=10");
    expect(url).toContain("ruleId=r1");
    expect(url).toContain("type=rule.created");
    expect(url).toContain("severity=success");
  });
});

describe("clearActivity", () => {
  it("DELETEs /api/activity", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({ ok: true, status: 204, json: () => Promise.resolve(null) })
    );
    await clearActivity();
    expect(vi.mocked(fetch)).toHaveBeenCalledWith("/api/activity", expect.objectContaining({ method: "DELETE" }));
  });
});

describe("diagnoseForwardRule", () => {
  it("POSTs /api/forwards/:id/diagnose", async () => {
    const data = { ruleId: "r1", ruleName: "Test", protocol: "tcp", summary: { status: "pass", message: "All good" }, checks: [], diagnosedAt: "" };
    mockFetchOk(data);
    await diagnoseForwardRule("r1");
    expect(vi.mocked(fetch)).toHaveBeenCalledWith("/api/forwards/r1/diagnose", expect.objectContaining({ method: "POST" }));
  });
});

describe("fetchLiveConnections", () => {
  it("fetches /api/connections", async () => {
    const data = { generatedAt: "", tcpConnections: [], udpSessions: [], ruleSummaries: [] };
    mockFetchOk(data);
    const result = await fetchLiveConnections();
    expect(result).toEqual(data);
    expect(vi.mocked(fetch)).toHaveBeenCalledWith("/api/connections");
  });
});

describe("fetchRuntimeInfo", () => {
  it("fetches /api/runtime", async () => {
    const data = { name: "Portier", version: "1.4.0", runtime: "go" };
    mockFetchOk(data);
    await fetchRuntimeInfo();
    expect(vi.mocked(fetch)).toHaveBeenCalledWith("/api/runtime");
  });
});

describe("exportConfig", () => {
  it("fetches /api/config/export", async () => {
    const data: ExportedConfig = { version: "1", exportedAt: "", rules: [] };
    mockFetchOk(data);
    const result = await exportConfig();
    expect(result).toEqual(data);
    expect(vi.mocked(fetch)).toHaveBeenCalledWith("/api/config/export");
  });
});

describe("importConfig", () => {
  it("POSTs /api/config/import with mode and config", async () => {
    const config: ExportedConfig = { version: "1", exportedAt: "", rules: [] };
    mockFetchOk({ result: { imported: 0, skipped: 0, errors: [] }, rules: [] });
    await importConfig(config, "replace");
    expect(vi.mocked(fetch)).toHaveBeenCalledWith("/api/config/import", expect.objectContaining({ method: "POST" }));
  });
});

describe("ensureOk (error handling)", () => {
  it("throws error message from response body errors array", async () => {
    mockFetchError(400, ["Name is required."]);
    await expect(fetchForwardRules()).rejects.toThrow("Name is required.");
  });

  it("throws status-based error when json() rejects", async () => {
    mockFetchError(500);
    await expect(fetchForwardRules()).rejects.toThrow("Request failed with 500.");
  });
});
