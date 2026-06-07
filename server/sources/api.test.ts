import http from "node:http";
import net from "node:net";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { describe, expect, it } from "vitest";
import type { ForwardRule } from "@portier/shared";
import { createApp, type RuntimeInfoOptions } from "./api.js";
import { ForwardManager, type RuleStore } from "./forward-manager.js";
import { ActivityStore } from "./activity/activity-store.js";
import { getFreeTcpPort, getFreeUdpPort } from "./test-helpers.js";

class MemoryStore implements RuleStore {
  constructor(private rules: ForwardRule[] = []) {}

  async load(): Promise<ForwardRule[]> {
    return this.rules;
  }

  async save(rules: ForwardRule[]): Promise<void> {
    this.rules = rules;
  }
}

interface WithServerOptions {
  activity?: ActivityStore;
  rules?: ForwardRule[];
  staticClientDir?: string;
  runtimeInfo?: RuntimeInfoOptions;
}

async function withServer(
  handler: (port: number, manager: ForwardManager) => Promise<void>,
  options?: WithServerOptions
): Promise<void> {
  const store = new MemoryStore(options?.rules ?? []);
  const manager = new ForwardManager(store, options?.activity);
  if (options?.rules?.length) {
    await manager.loadAndStartEnabled();
  }
  const app = createApp(manager, {
    activity: options?.activity,
    staticClientDir: options?.staticClientDir,
    runtimeInfo: options?.runtimeInfo
  });
  const server = http.createServer(app);
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  try {
    const address = server.address();
    if (!address || typeof address === "string") throw new Error("Expected a TCP address.");
    await handler(address.port, manager);
  } finally {
    await manager.stopAll();
    await new Promise<void>((resolve, reject) =>
      server.close((err) => (err ? reject(err) : resolve()))
    );
  }
}

const baseRule: ForwardRule = {
  id: "r1",
  name: "Test Rule",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: 49901,
  targetHost: "127.0.0.1",
  targetPort: 49902,
  enabled: false
};

describe("GET /api/activity", () => {
  it("returns an empty events array when no activity store is provided", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/activity`);
      expect(response.status).toBe(200);
      expect(await response.json()).toEqual({ events: [] });
    });
  });

  it("returns events from the activity store", async () => {
    const activity = new ActivityStore();
    activity.add({
      type: "rule.started",
      severity: "success",
      ruleId: "r1",
      ruleName: "Test",
      protocol: "tcp",
      message: "Rule started."
    });

    await withServer(
      async (port) => {
        const response = await fetch(`http://127.0.0.1:${port}/api/activity`);
        expect(response.status).toBe(200);
        const body = (await response.json()) as { events: unknown[] };
        expect(body.events).toHaveLength(1);
        expect((body.events[0] as { type: string }).type).toBe("rule.started");
      },
      { activity }
    );
  });

  it("respects the limit query param", async () => {
    const activity = new ActivityStore();
    for (let i = 0; i < 20; i++) {
      activity.add({ type: "rule.started", severity: "info", message: `Event ${i}` });
    }

    await withServer(
      async (port) => {
        const response = await fetch(`http://127.0.0.1:${port}/api/activity?limit=5`);
        const body = (await response.json()) as { events: unknown[] };
        expect(body.events).toHaveLength(5);
      },
      { activity }
    );
  });

  it("filters by severity via query param", async () => {
    const activity = new ActivityStore();
    activity.add({ type: "rule.started", severity: "success", message: "OK." });
    activity.add({ type: "rule.error", severity: "error", message: "Fail." });

    await withServer(
      async (port) => {
        const response = await fetch(`http://127.0.0.1:${port}/api/activity?severity=error`);
        const body = (await response.json()) as { events: Array<{ severity: string }> };
        expect(body.events).toHaveLength(1);
        expect(body.events[0].severity).toBe("error");
      },
      { activity }
    );
  });
});

describe("DELETE /api/activity", () => {
  it("returns 204 with no activity store", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/activity`, { method: "DELETE" });
      expect(response.status).toBe(204);
    });
  });

  it("clears the activity store and returns 204", async () => {
    const activity = new ActivityStore();
    activity.add({ type: "rule.started", severity: "success", message: "Started." });

    await withServer(
      async (port) => {
        const before = await fetch(`http://127.0.0.1:${port}/api/activity`);
        const beforeBody = (await before.json()) as { events: unknown[] };
        expect(beforeBody.events).toHaveLength(1);

        const del = await fetch(`http://127.0.0.1:${port}/api/activity`, { method: "DELETE" });
        expect(del.status).toBe(204);

        const after = await fetch(`http://127.0.0.1:${port}/api/activity`);
        const afterBody = (await after.json()) as { events: unknown[] };
        expect(afterBody.events).toHaveLength(0);
      },
      { activity }
    );
  });

  it("does not affect forwarding rules", async () => {
    const activity = new ActivityStore();
    activity.add({ type: "rule.created", severity: "info", message: "Created." });

    await withServer(
      async (port) => {
        await fetch(`http://127.0.0.1:${port}/api/activity`, { method: "DELETE" });

        const rules = await fetch(`http://127.0.0.1:${port}/api/forwards`);
        expect(rules.status).toBe(200);
        const body = (await rules.json()) as unknown[];
        expect(Array.isArray(body)).toBe(true);
      },
      { activity }
    );
  });
});

describe("port advisory API", () => {
  it("returns advisories for a requested port, host, and purpose", async () => {
    const manager = new ForwardManager(new MemoryStore());
    const server = http.createServer(createApp(manager));
    await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));

    try {
      const address = server.address();
      if (!address || typeof address === "string") {
        throw new Error("Expected a TCP address.");
      }

      const response = await fetch(
        `http://127.0.0.1:${address.port}/api/ports/advisory?port=48001&listenHost=0.0.0.0&purpose=forward`
      );

      expect(response.status).toBe(200);
      expect(await response.json()).toEqual([
        {
          code: "LAN_EXPOSURE",
          severity: "warning",
          message: "Listening on 0.0.0.0 exposes this forwarded port on the LAN."
        }
      ]);
    } finally {
      await new Promise<void>((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
      });
    }
  });
});

describe("GET /api/config/export", () => {
  it("returns current rules as an ExportedConfig", async () => {
    await withServer(
      async (port) => {
        const response = await fetch(`http://127.0.0.1:${port}/api/config/export`);
        expect(response.status).toBe(200);
        const body = (await response.json()) as { version: string; rules: unknown[]; exportedAt: string };
        expect(body.version).toBe("1");
        expect(body.rules).toHaveLength(1);
        expect(body.exportedAt).toBeTruthy();
      },
      { rules: [baseRule] }
    );
  });
});

describe("POST /api/config/import", () => {
  it("rejects an invalid mode", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/config/import`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ mode: "bad", config: { version: "1", rules: [] } })
      });
      expect(response.status).toBe(400);
    });
  });

  it("merges imported rules with existing ones", async () => {
    await withServer(async (port) => {
      const newRule: ForwardRule = {
        id: "r2",
        name: "Imported",
        protocol: "tcp",
        listenHost: "127.0.0.1",
        listenPort: 49903,
        targetHost: "127.0.0.1",
        targetPort: 49904,
        enabled: false
      };
      const response = await fetch(`http://127.0.0.1:${port}/api/config/import`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          mode: "merge",
          config: { version: "1", exportedAt: new Date().toISOString(), rules: [newRule] }
        })
      });
      expect(response.status).toBe(200);
      const body = (await response.json()) as { result: { imported: number }; rules: unknown[] };
      expect(body.result.imported).toBe(1);
      expect(body.rules).toHaveLength(2); // existing r1 + imported r2
    }, { rules: [baseRule] });
  });

  it("rejects import with invalid rules (no partial import)", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/config/import`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          mode: "merge",
          config: {
            version: "1",
            exportedAt: new Date().toISOString(),
            rules: [{ name: "", protocol: "invalid" }] // invalid rule
          }
        })
      });
      expect(response.status).toBe(422);
      const body = (await response.json()) as { errors: string[] };
      expect(body.errors.length).toBeGreaterThan(0);
    });
  });
});

describe("POST /api/forwards/reorder", () => {
  it("reorders rules and returns the new order", async () => {
    const r2: ForwardRule = {
      ...baseRule,
      id: "r2",
      name: "Rule 2",
      listenPort: 49903,
      targetPort: 49904
    };

    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards/reorder`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ids: ["r2", "r1"] })
      });
      expect(response.status).toBe(200);
      const rules = (await response.json()) as Array<{ id: string }>;
      expect(rules[0].id).toBe("r2");
      expect(rules[1].id).toBe("r1");
    }, { rules: [baseRule, r2] });
  });

  it("returns 400 for non-array ids", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards/reorder`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ids: "not-an-array" })
      });
      expect(response.status).toBe(400);
    });
  });
});

describe("GET /api/forwards", () => {
  it("returns empty array when no rules exist", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards`);
      expect(response.status).toBe(200);
      expect(await response.json()).toEqual([]);
    });
  });

  it("returns existing rules with advisories", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards`);
      expect(response.status).toBe(200);
      const rules = (await response.json()) as Array<{ id: string; advisories: unknown[] }>;
      expect(rules).toHaveLength(1);
      expect(rules[0].id).toBe("r1");
      expect(Array.isArray(rules[0].advisories)).toBe(true);
    }, { rules: [baseRule] });
  });
});

describe("POST /api/forwards", () => {
  it("creates a valid rule and returns 201", async () => {
    await withServer(async (port) => {
      const payload = {
        name: "New Rule",
        protocol: "tcp",
        listenHost: "127.0.0.1",
        listenPort: 49910,
        targetHost: "127.0.0.1",
        targetPort: 49911,
        enabled: false
      };
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });
      expect(response.status).toBe(201);
      const body = (await response.json()) as { id: string; name: string };
      expect(body.name).toBe("New Rule");
      expect(typeof body.id).toBe("string");
    });
  });

  it("rejects an invalid rule with 400 and errors array", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "", protocol: "tcp" })
      });
      expect(response.status).toBe(400);
      const body = (await response.json()) as { errors: string[] };
      expect(Array.isArray(body.errors)).toBe(true);
      expect(body.errors.length).toBeGreaterThan(0);
    });
  });

  it("rejects a duplicate listen binding with 409", async () => {
    await withServer(async (port) => {
      const payload = {
        name: "Duplicate",
        protocol: "tcp",
        listenHost: "127.0.0.1",
        listenPort: 49901, // same as baseRule
        targetHost: "127.0.0.1",
        targetPort: 49999,
        enabled: false
      };
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });
      expect(response.status).toBe(409);
      const body = (await response.json()) as { errors: string[] };
      expect(body.errors.length).toBeGreaterThan(0);
    }, { rules: [baseRule] });
  });
});

describe("PATCH /api/forwards/:id", () => {
  it("updates an existing rule and returns the updated rule", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards/r1`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "Renamed Rule" })
      });
      expect(response.status).toBe(200);
      const body = (await response.json()) as { id: string; name: string };
      expect(body.id).toBe("r1");
      expect(body.name).toBe("Renamed Rule");
    }, { rules: [baseRule] });
  });

  it("returns 404 for a non-existent rule", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards/no-such-rule`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "New Name" })
      });
      expect(response.status).toBe(404);
      const body = (await response.json()) as { errors: string[] };
      expect(body.errors.length).toBeGreaterThan(0);
    });
  });

  it("rejects update that would create a duplicate listen binding with 409", async () => {
    const r2: ForwardRule = {
      ...baseRule,
      id: "r2",
      name: "Rule 2",
      listenPort: 49912,
      targetPort: 49913
    };
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards/r2`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ listenPort: 49901 }) // conflicts with r1
      });
      expect(response.status).toBe(409);
    }, { rules: [baseRule, r2] });
  });
});

describe("DELETE /api/forwards/:id", () => {
  it("deletes an existing rule and returns 204", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards/r1`, {
        method: "DELETE"
      });
      expect(response.status).toBe(204);

      const listResponse = await fetch(`http://127.0.0.1:${port}/api/forwards`);
      expect(await listResponse.json()).toEqual([]);
    }, { rules: [baseRule] });
  });

  it("returns 404 when deleting a non-existent rule", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards/no-such-rule`, {
        method: "DELETE"
      });
      expect(response.status).toBe(404);
      const body = (await response.json()) as { errors: string[] };
      expect(body.errors.length).toBeGreaterThan(0);
    });
  });
});

describe("GET /api/status", () => {
  it("returns status array with one entry per rule", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/status`);
      expect(response.status).toBe(200);
      const statuses = (await response.json()) as Array<{ ruleId: string; running: boolean }>;
      expect(statuses).toHaveLength(1);
      expect(statuses[0].ruleId).toBe("r1");
      expect(statuses[0].running).toBe(false);
    }, { rules: [baseRule] });
  });

  it("returns empty array when no rules exist", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/status`);
      expect(response.status).toBe(200);
      expect(await response.json()).toEqual([]);
    });
  });
});

describe("POST /api/forwards/:id/start and /stop", () => {
  it("starts and stops a rule using ephemeral listen port", async () => {
    const listenPort = await getFreeTcpPort();
    const rule: ForwardRule = {
      ...baseRule,
      listenPort,
      targetPort: 49990 // target doesn't need to be listening for the server to bind
    };

    await withServer(async (port) => {
      const startResponse = await fetch(`http://127.0.0.1:${port}/api/forwards/r1/start`, {
        method: "POST"
      });
      expect(startResponse.status).toBe(200);
      const startStatus = (await startResponse.json()) as { ruleId: string; running: boolean };
      expect(startStatus.ruleId).toBe("r1");
      expect(startStatus.running).toBe(true);

      const stopResponse = await fetch(`http://127.0.0.1:${port}/api/forwards/r1/stop`, {
        method: "POST"
      });
      expect(stopResponse.status).toBe(200);
      const stopStatus = (await stopResponse.json()) as { ruleId: string; running: boolean };
      expect(stopStatus.ruleId).toBe("r1");
      expect(stopStatus.running).toBe(false);
    }, { rules: [rule] });
  });

  it("returns 404 when starting a non-existent rule", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards/no-such-rule/start`, {
        method: "POST"
      });
      expect(response.status).toBe(404);
    });
  });

  it("returns 404 when stopping a non-existent rule", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards/no-such-rule/stop`, {
        method: "POST"
      });
      expect(response.status).toBe(404);
    });
  });
});

describe("POST /api/config/import — replace mode", () => {
  it("replaces all existing rules with imported ones", async () => {
    const importedRule: ForwardRule = {
      id: "r-imported",
      name: "Replacement Rule",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 49920,
      targetHost: "127.0.0.1",
      targetPort: 49921,
      enabled: false
    };

    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/config/import`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          mode: "replace",
          config: {
            version: "1",
            exportedAt: new Date().toISOString(),
            rules: [importedRule]
          }
        })
      });
      expect(response.status).toBe(200);
      const body = (await response.json()) as { result: { imported: number }; rules: Array<{ id: string }> };
      expect(body.result.imported).toBe(1);
      expect(body.rules).toHaveLength(1);
      expect(body.rules[0].id).toBe("r-imported");
    }, { rules: [baseRule] });
  });

  it("rejects replace import with invalid config body", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/config/import`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ mode: "replace", config: { version: "2" } })
      });
      expect(response.status).toBe(400);
      const body = (await response.json()) as { errors: string[] };
      expect(body.errors.length).toBeGreaterThan(0);
    });
  });
});

describe("static file serving", () => {
  it("starts and serves API when static dir does not exist", async () => {
    await withServer(
      async (port) => {
        const response = await fetch(`http://127.0.0.1:${port}/api/forwards`);
        expect(response.status).toBe(200);
        expect(await response.json()).toEqual([]);
      },
      { staticClientDir: "/nonexistent-portier-test-static-dir" }
    );
  });

  it("returns JSON 404 for unknown /api routes", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/nonexistent`);
      expect(response.status).toBe(404);
      const body = (await response.json()) as { errors: string[] };
      expect(body.errors).toEqual(["API route was not found."]);
    });
  });

  it("returns index.html for non-API routes when static dir exists", async () => {
    const tempDir = mkdtempSync(join(tmpdir(), "portier-test-"));
    try {
      writeFileSync(join(tempDir, "index.html"), "<html><body>Portier</body></html>");
      await withServer(
        async (port) => {
          const response = await fetch(`http://127.0.0.1:${port}/some/spa/route`);
          expect(response.status).toBe(200);
          const body = await response.text();
          expect(body).toContain("Portier");
        },
        { staticClientDir: tempDir }
      );
    } finally {
      rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("serves static assets from the static dir", async () => {
    const tempDir = mkdtempSync(join(tmpdir(), "portier-test-"));
    try {
      writeFileSync(join(tempDir, "index.html"), "<html><body>Portier</body></html>");
      mkdirSync(join(tempDir, "assets"));
      writeFileSync(join(tempDir, "assets", "app.js"), "console.log('portier');");
      await withServer(
        async (port) => {
          const response = await fetch(`http://127.0.0.1:${port}/assets/app.js`);
          expect(response.status).toBe(200);
          const body = await response.text();
          expect(body).toContain("portier");
        },
        { staticClientDir: tempDir }
      );
    } finally {
      rmSync(tempDir, { recursive: true, force: true });
    }
  });
});

describe("GET /api/runtime", () => {
  const testRuntimeInfo: RuntimeInfoOptions = {
    version: "1.1.0-test",
    managementHost: "127.0.0.1",
    managementPort: 47831,
    configPath: "/test/data/forwards.json",
    staticDir: "/test/web",
    serviceMode: false,
    startedAt: new Date("2026-01-01T00:00:00.000Z")
  };

  it("returns 200 with runtime info", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/runtime`);
      expect(response.status).toBe(200);
    }, { runtimeInfo: testRuntimeInfo });
  });

  it("runtime field is 'node'", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/runtime`);
      const body = (await response.json()) as { runtime: string };
      expect(body.runtime).toBe("node");
    }, { runtimeInfo: testRuntimeInfo });
  });

  it("version reflects the provided info", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/runtime`);
      const body = (await response.json()) as { version: string };
      expect(typeof body.version).toBe("string");
      expect(body.version).toBe("1.1.0-test");
    }, { runtimeInfo: testRuntimeInfo });
  });

  it("configPath and staticDir are present strings", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/runtime`);
      const body = (await response.json()) as { configPath: string; staticDir: string };
      expect(typeof body.configPath).toBe("string");
      expect(typeof body.staticDir).toBe("string");
      expect(body.configPath).toBe("/test/data/forwards.json");
      expect(body.staticDir).toBe("/test/web");
    }, { runtimeInfo: testRuntimeInfo });
  });

  it("uptimeSeconds is a non-negative number", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/runtime`);
      const body = (await response.json()) as { uptimeSeconds: number };
      expect(typeof body.uptimeSeconds).toBe("number");
      expect(body.uptimeSeconds).toBeGreaterThanOrEqual(0);
    }, { runtimeInfo: testRuntimeInfo });
  });

  it("startedAt is a parseable ISO timestamp", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/runtime`);
      const body = (await response.json()) as { startedAt: string };
      expect(typeof body.startedAt).toBe("string");
      expect(() => new Date(body.startedAt)).not.toThrow();
      expect(new Date(body.startedAt).toISOString()).toBe("2026-01-01T00:00:00.000Z");
    }, { runtimeInfo: testRuntimeInfo });
  });

  it("returns runtime info even without explicit runtimeInfo option", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/runtime`);
      expect(response.status).toBe(200);
      const body = (await response.json()) as { runtime: string; name: string; pid: number };
      expect(body.runtime).toBe("node");
      expect(body.name).toBe("Portier");
      expect(typeof body.pid).toBe("number");
    });
  });
});

// ── Diagnose helpers ────────────────────────────────────────────────────────

function startEchoServer(): Promise<{ port: number; close: () => Promise<void> }> {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address() as net.AddressInfo;
      resolve({
        port,
        close: () => new Promise<void>((res, rej) => server.close((err) => (err ? rej(err) : res())))
      });
    });
  });
}

describe("POST /api/forwards/:id/diagnose", () => {
  it("returns 404 for an unknown rule", async () => {
    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards/no-such-id/diagnose`, {
        method: "POST"
      });
      expect(response.status).toBe(404);
      const body = (await response.json()) as { errors: string[] };
      expect(Array.isArray(body.errors)).toBe(true);
      expect(body.errors.length).toBeGreaterThan(0);
    });
  });

  it("diagnoses a TCP rule with a reachable target and returns pass for target-connect", async () => {
    const echo = await startEchoServer();
    const listenPort = await getFreeTcpPort();
    const rule: ForwardRule = {
      id: "diag-tcp",
      name: "Diag TCP",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort: echo.port,
      enabled: false
    };

    try {
      await withServer(async (port) => {
        const response = await fetch(`http://127.0.0.1:${port}/api/forwards/diag-tcp/diagnose`, {
          method: "POST"
        });
        expect(response.status).toBe(200);
        const body = (await response.json()) as {
          ruleId: string;
          ruleName: string;
          protocol: string;
          summary: { status: string; message: string };
          checks: Array<{ id: string; status: string }>;
          diagnosedAt: string;
        };
        expect(body.ruleId).toBe("diag-tcp");
        expect(body.protocol).toBe("tcp");
        expect(typeof body.diagnosedAt).toBe("string");
        expect(() => new Date(body.diagnosedAt)).not.toThrow();

        const connectCheck = body.checks.find((c) => c.id === "target-connect");
        expect(connectCheck).toBeDefined();
        expect(connectCheck?.status).toBe("pass");
      }, { rules: [rule] });
    } finally {
      await echo.close();
    }
  });

  it("diagnoses a TCP rule with an unreachable target and returns fail for target-connect", async () => {
    const unreachablePort = await getFreeTcpPort();
    // Port is free — nothing listening there
    const rule: ForwardRule = {
      id: "diag-tcp-fail",
      name: "Diag TCP Fail",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: await getFreeTcpPort(),
      targetHost: "127.0.0.1",
      targetPort: unreachablePort,
      enabled: false
    };

    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards/diag-tcp-fail/diagnose`, {
        method: "POST"
      });
      expect(response.status).toBe(200);
      const body = (await response.json()) as { checks: Array<{ id: string; status: string }> };
      const connectCheck = body.checks.find((c) => c.id === "target-connect");
      expect(connectCheck).toBeDefined();
      expect(connectCheck?.status).toBe("fail");
    }, { rules: [rule] });
  });

  it("diagnoses a UDP rule and skips target-connect", async () => {
    const rule: ForwardRule = {
      id: "diag-udp",
      name: "Diag UDP",
      protocol: "udp",
      listenHost: "127.0.0.1",
      listenPort: await getFreeUdpPort(),
      targetHost: "127.0.0.1",
      targetPort: await getFreeUdpPort(),
      enabled: false,
      udpMode: "one-way"
    };

    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards/diag-udp/diagnose`, {
        method: "POST"
      });
      expect(response.status).toBe(200);
      const body = (await response.json()) as { checks: Array<{ id: string; status: string }> };
      const connectCheck = body.checks.find((c) => c.id === "target-connect");
      expect(connectCheck).toBeDefined();
      expect(connectCheck?.status).toBe("skip");

      const udpModeCheck = body.checks.find((c) => c.id === "udp-mode");
      expect(udpModeCheck).toBeDefined();
      expect(udpModeCheck?.status).toBe("pass");
    }, { rules: [rule] });
  });

  it("warns on LAN exposure for 0.0.0.0 listen host", async () => {
    const rule: ForwardRule = {
      id: "diag-lan",
      name: "Diag LAN",
      protocol: "tcp",
      listenHost: "0.0.0.0",
      listenPort: await getFreeTcpPort(),
      targetHost: "127.0.0.1",
      targetPort: await getFreeTcpPort(),
      enabled: false
    };

    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards/diag-lan/diagnose`, {
        method: "POST"
      });
      expect(response.status).toBe(200);
      const body = (await response.json()) as { checks: Array<{ id: string; status: string }> };
      const lanCheck = body.checks.find((c) => c.id === "lan-exposure");
      expect(lanCheck).toBeDefined();
      expect(lanCheck?.status).toBe("warn");

      const hostCheck = body.checks.find((c) => c.id === "listen-host");
      expect(hostCheck).toBeDefined();
      expect(hostCheck?.status).toBe("warn");
    }, { rules: [rule] });
  });

  it("does not fail listen-bind when the rule is running (Portier owns the socket)", async () => {
    const listenPort = await getFreeTcpPort();
    const echo = await startEchoServer();
    const rule: ForwardRule = {
      id: "diag-running",
      name: "Diag Running",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort: echo.port,
      enabled: true
    };

    try {
      await withServer(async (port) => {
        // Start the rule so it owns the listen socket
        await fetch(`http://127.0.0.1:${port}/api/forwards/diag-running/start`, { method: "POST" });

        const response = await fetch(`http://127.0.0.1:${port}/api/forwards/diag-running/diagnose`, {
          method: "POST"
        });
        expect(response.status).toBe(200);
        const body = (await response.json()) as { checks: Array<{ id: string; status: string }> };
        const bindCheck = body.checks.find((c) => c.id === "listen-bind");
        expect(bindCheck).toBeDefined();
        // Must not be fail — running rule owns the socket legitimately
        expect(bindCheck?.status).not.toBe("fail");
      }, { rules: [rule] });
    } finally {
      await echo.close();
    }
  });

  it("response shape is stable (required fields present)", async () => {
    const rule: ForwardRule = {
      id: "diag-shape",
      name: "Diag Shape",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: await getFreeTcpPort(),
      targetHost: "127.0.0.1",
      targetPort: await getFreeTcpPort(),
      enabled: false
    };

    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards/diag-shape/diagnose`, {
        method: "POST"
      });
      expect(response.status).toBe(200);
      const body = (await response.json()) as Record<string, unknown>;
      expect(typeof body.ruleId).toBe("string");
      expect(typeof body.ruleName).toBe("string");
      expect(typeof body.protocol).toBe("string");
      expect(typeof body.diagnosedAt).toBe("string");
      expect(body.summary).toBeDefined();
      expect(typeof (body.summary as Record<string, unknown>).status).toBe("string");
      expect(typeof (body.summary as Record<string, unknown>).message).toBe("string");
      expect(Array.isArray(body.checks)).toBe(true);
      const checks = body.checks as Array<Record<string, unknown>>;
      expect(checks.length).toBeGreaterThan(0);
      for (const check of checks) {
        expect(typeof check.id).toBe("string");
        expect(typeof check.label).toBe("string");
        expect(typeof check.status).toBe("string");
        expect(typeof check.message).toBe("string");
      }
    }, { rules: [rule] });
  });

  it("warns for bidirectional-last-client UDP mode", async () => {
    const rule: ForwardRule = {
      id: "diag-udp-bidi",
      name: "Diag UDP Bidi",
      protocol: "udp",
      listenHost: "127.0.0.1",
      listenPort: await getFreeUdpPort(),
      targetHost: "127.0.0.1",
      targetPort: await getFreeUdpPort(),
      enabled: false,
      udpMode: "bidirectional-last-client"
    };

    await withServer(async (port) => {
      const response = await fetch(`http://127.0.0.1:${port}/api/forwards/diag-udp-bidi/diagnose`, {
        method: "POST"
      });
      expect(response.status).toBe(200);
      const body = (await response.json()) as { checks: Array<{ id: string; status: string }> };
      const udpModeCheck = body.checks.find((c) => c.id === "udp-mode");
      expect(udpModeCheck).toBeDefined();
      expect(udpModeCheck?.status).toBe("warn");
    }, { rules: [rule] });
  });
});
