import http from "node:http";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { describe, expect, it } from "vitest";
import type { ForwardRule } from "@portier/shared";
import { createApp } from "./api.js";
import { ForwardManager, type RuleStore } from "./forward-manager.js";
import { ActivityStore } from "./activity/activity-store.js";
import { getFreeTcpPort } from "./test-helpers.js";

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
    staticClientDir: options?.staticClientDir
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
