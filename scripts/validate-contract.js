#!/usr/bin/env node
/* global console, process, setTimeout, URL, Buffer */
/**
 * API contract parity tests.
 *
 * Runs all Portier API endpoints against the TypeScript server.
 * If the Go service binary is available, runs the same scenarios against it
 * and verifies status codes, response shapes, field names, and error shapes match.
 *
 * Usage:
 *   node scripts/test-contract.js [--skip-go]
 *
 *   --skip-go   Skip Go parity even if the binary is present.
 */

import { existsSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { spawn } from "node:child_process";
import { createServer } from "node:net";
import { get, request as httpRequest } from "node:http";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..");
const isWindows = process.platform === "win32";

let passed = 0;
let failed = 0;
let skipped = 0;

function pass(msg) {
  console.log(`  ✓ ${msg}`);
  passed++;
}

function fail(msg) {
  console.error(`  ✗ ${msg}`);
  failed++;
}

function skip(msg) {
  console.log(`  - ${msg} [skip]`);
  skipped++;
}

function getFreePort() {
  return new Promise((resolve, reject) => {
    const srv = createServer();
    srv.listen(0, "127.0.0.1", () => {
      const { port } = srv.address();
      srv.close(() => resolve(port));
    });
    srv.on("error", reject);
  });
}

function httpGet(url) {
  return new Promise((resolve, reject) => {
    const req = get(url, (res) => {
      let body = "";
      res.on("data", (d) => { body += d; });
      res.on("end", () => resolve({ status: res.statusCode, body, json: () => JSON.parse(body) }));
    });
    req.on("error", reject);
    req.setTimeout(3000, () => req.destroy(new Error("timeout")));
  });
}

function httpMethod(method, url, body) {
  return new Promise((resolve, reject) => {
    const data = body !== undefined ? JSON.stringify(body) : undefined;
    const urlObj = new URL(url);
    const req = httpRequest({
      hostname: urlObj.hostname,
      port: urlObj.port,
      path: urlObj.pathname + urlObj.search,
      method,
      headers: data ? {
        "Content-Type": "application/json",
        "Content-Length": Buffer.byteLength(data),
      } : {},
    }, (res) => {
      let responseBody = "";
      res.on("data", (d) => { responseBody += d; });
      res.on("end", () => resolve({
        status: res.statusCode,
        body: responseBody,
        json: () => JSON.parse(responseBody),
      }));
    });
    req.on("error", reject);
    req.setTimeout(3000, () => req.destroy(new Error("timeout")));
    if (data) req.write(data);
    req.end();
  });
}

async function waitForReady(baseUrl, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const res = await httpGet(`${baseUrl}/api/forwards`);
      if (res.status === 200) return true;
    } catch {
      // not ready yet
    }
    await new Promise((r) => setTimeout(r, 400));
  }
  return false;
}

async function startServer(binary, args, label) {
  const port = await getFreePort();
  const tempDir = join(tmpdir(), `portier-contract-${label}-${Date.now()}-${process.pid}`);
  mkdirSync(tempDir, { recursive: true });
  const tempConfig = join(tempDir, "rules.json");
  writeFileSync(tempConfig, "[]");

  const proc = spawn(binary, [
    ...args,
    "--config", tempConfig,
    "--host", "127.0.0.1",
    "--port", String(port),
  ], { stdio: ["ignore", "pipe", "pipe"], detached: false });

  const baseUrl = `http://127.0.0.1:${port}`;
  const ready = await waitForReady(baseUrl, 15000);
  return { proc, port, tempDir, ready, baseUrl };
}

function cleanup(ctx) {
  try { ctx.proc.kill(); } catch { /* best effort */ }
  try { rmSync(ctx.tempDir, { recursive: true, force: true }); } catch { /* best effort */ }
}

// Rule listen ports — fixed and in an unused range.
// These rules are not started, so no actual port binding occurs.
const TCP_LISTEN = 49901;
const UDP_LISTEN = 49902;
const UDP_DEFAULT_LISTEN = 49903;

async function runScenarios(baseUrl, runtime) {
  console.log(`\n  --- Scenarios: ${runtime} at ${baseUrl} ---\n`);

  const api = {
    get: (path) => httpGet(`${baseUrl}${path}`),
    post: (path, body) => httpMethod("POST", `${baseUrl}${path}`, body),
    patch: (path, body) => httpMethod("PATCH", `${baseUrl}${path}`, body),
    delete: (path) => httpMethod("DELETE", `${baseUrl}${path}`),
  };

  // GET /api/runtime — RuntimeInfo shape
  {
    const res = await api.get("/api/runtime");
    if (res.status === 200) {
      const data = res.json();
      const required = ["name", "version", "runtime", "platform", "arch", "uptimeSeconds", "startedAt", "managementHost", "managementPort", "configPath", "staticDir", "serviceMode", "pid"];
      const missing = required.filter((f) => !(f in data));
      if (missing.length === 0) {
        pass("GET /api/runtime → 200 with all required fields");
      } else {
        fail(`GET /api/runtime → 200 but missing fields: ${missing.join(", ")}`);
      }
      if (typeof data.uptimeSeconds === "number" && data.uptimeSeconds >= 0) {
        pass("GET /api/runtime → uptimeSeconds is a non-negative number");
      } else {
        fail(`GET /api/runtime → uptimeSeconds invalid: ${data.uptimeSeconds}`);
      }
      if (typeof data.startedAt === "string" && !isNaN(new Date(data.startedAt).getTime())) {
        pass("GET /api/runtime → startedAt is a parseable timestamp");
      } else {
        fail(`GET /api/runtime → startedAt not parseable: ${data.startedAt}`);
      }
      if (data.runtime === runtime.toLowerCase() || (runtime === "TypeScript" && data.runtime === "node") || (runtime === "Go" && data.runtime === "go")) {
        pass(`GET /api/runtime → runtime="${data.runtime}" matches ${runtime}`);
      } else {
        fail(`GET /api/runtime → runtime="${data.runtime}" does not match expected for ${runtime}`);
      }
    } else {
      fail(`GET /api/runtime → expected 200, got ${res.status}`);
    }
  }

  // GET /api/forwards — initially empty
  {
    const res = await api.get("/api/forwards");
    if (res.status !== 200) { fail(`GET /api/forwards → expected 200, got ${res.status}`); return; }
    const data = res.json();
    if (Array.isArray(data)) {
      pass("GET /api/forwards → 200 []");
    } else {
      fail("GET /api/forwards → 200 but not an array");
    }
  }

  // POST /api/forwards — TCP rule
  let tcpId = null;
  {
    const res = await api.post("/api/forwards", {
      name: "Contract TCP",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: TCP_LISTEN,
      targetHost: "127.0.0.1",
      targetPort: 49801,
      enabled: true,
    });
    if (res.status === 201) {
      const data = res.json();
      if (data.id && data.name === "Contract TCP" && data.protocol === "tcp") {
        pass("POST /api/forwards (TCP) → 201 with id, name, protocol");
        tcpId = data.id;
      } else {
        fail(`POST /api/forwards (TCP) → 201 but shape mismatch: ${JSON.stringify(data)}`);
      }
      if (Array.isArray(data.advisories)) {
        pass("POST /api/forwards → response includes advisories[]");
      } else {
        fail("POST /api/forwards → response missing advisories[]");
      }
      if (data.udpMode === undefined) {
        pass("POST /api/forwards (TCP) → no udpMode on TCP rule");
      } else {
        fail(`POST /api/forwards (TCP) → unexpected udpMode: ${data.udpMode}`);
      }
    } else {
      fail(`POST /api/forwards (TCP) → expected 201, got ${res.status}: ${res.body}`);
    }
  }

  // POST /api/forwards — UDP rule with explicit udpMode
  let udpId = null;
  {
    const res = await api.post("/api/forwards", {
      name: "Contract UDP",
      protocol: "udp",
      listenHost: "127.0.0.1",
      listenPort: UDP_LISTEN,
      targetHost: "127.0.0.1",
      targetPort: 49802,
      enabled: true,
      udpMode: "bidirectional-last-client",
    });
    if (res.status === 201) {
      const data = res.json();
      if (data.udpMode === "bidirectional-last-client") {
        pass("POST /api/forwards (UDP) → 201 with udpMode \"bidirectional-last-client\"");
        udpId = data.id;
      } else {
        fail(`POST /api/forwards (UDP) → 201 but udpMode: ${data.udpMode}`);
      }
    } else {
      fail(`POST /api/forwards (UDP) → expected 201, got ${res.status}`);
    }
  }

  // POST /api/forwards — UDP without udpMode defaults to "one-way"
  let udpDefaultId = null;
  {
    const res = await api.post("/api/forwards", {
      name: "Contract UDP Default",
      protocol: "udp",
      listenHost: "127.0.0.1",
      listenPort: UDP_DEFAULT_LISTEN,
      targetHost: "127.0.0.1",
      targetPort: 49803,
      enabled: true,
    });
    if (res.status === 201) {
      const data = res.json();
      if (data.udpMode === "one-way") {
        pass("POST /api/forwards (UDP no mode) → default udpMode \"one-way\"");
        udpDefaultId = data.id;
      } else {
        fail(`POST /api/forwards (UDP no mode) → expected "one-way", got: ${data.udpMode}`);
      }
    } else {
      fail(`POST /api/forwards (UDP default) → expected 201, got ${res.status}`);
    }
  }

  // POST /api/forwards — duplicate binding → 409
  if (tcpId) {
    const res = await api.post("/api/forwards", {
      name: "Conflict",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: TCP_LISTEN,
      targetHost: "127.0.0.1",
      targetPort: 49900,
      enabled: true,
    });
    if (res.status === 409) {
      pass("POST /api/forwards (duplicate binding) → 409 conflict");
    } else {
      fail(`POST /api/forwards (duplicate binding) → expected 409, got ${res.status}`);
    }
  }

  // POST /api/forwards — validation error → 400 with errors[]
  {
    const res = await api.post("/api/forwards", {
      name: "",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 99999,
      targetHost: "127.0.0.1",
      targetPort: 80,
      enabled: true,
    });
    if (res.status === 400) {
      const data = res.json();
      if (Array.isArray(data.errors) && data.errors.length > 0) {
        pass("POST /api/forwards (invalid) → 400 with errors[]");
      } else {
        fail(`POST /api/forwards (invalid) → 400 but no errors[]: ${res.body}`);
      }
    } else {
      fail(`POST /api/forwards (invalid) → expected 400, got ${res.status}`);
    }
  }

  // GET /api/forwards — 3 rules now, check full response shape
  {
    const res = await api.get("/api/forwards");
    if (res.status === 200) {
      const data = res.json();
      const expectedCount = [tcpId, udpId, udpDefaultId].filter(Boolean).length;
      if (Array.isArray(data) && data.length === expectedCount) {
        pass(`GET /api/forwards → 200, ${expectedCount} rules`);
        if (data.length > 0) {
          const rule = data[0];
          const required = ["id", "name", "protocol", "listenHost", "listenPort", "targetHost", "targetPort", "enabled", "advisories"];
          const missing = required.filter((f) => !(f in rule));
          if (missing.length === 0) {
            pass("GET /api/forwards → rule has all required fields");
          } else {
            fail(`GET /api/forwards → rule missing fields: ${missing.join(", ")}`);
          }
        }
      } else {
        fail(`GET /api/forwards → expected ${expectedCount} rules, got ${Array.isArray(data) ? data.length : "non-array"}`);
      }
    } else {
      fail(`GET /api/forwards → expected 200, got ${res.status}`);
    }
  }

  // PATCH /api/forwards/:id — update name
  if (tcpId) {
    const res = await api.patch(`/api/forwards/${tcpId}`, { name: "Renamed Contract TCP" });
    if (res.status === 200) {
      const data = res.json();
      if (data.name === "Renamed Contract TCP" && data.id === tcpId) {
        pass("PATCH /api/forwards/:id → 200 with updated name");
      } else {
        fail(`PATCH /api/forwards/:id → 200 but data mismatch: ${JSON.stringify(data)}`);
      }
    } else {
      fail(`PATCH /api/forwards/:id → expected 200, got ${res.status}: ${res.body}`);
    }
  }

  // PATCH /api/forwards/unknown → 404
  {
    const res = await api.patch("/api/forwards/no-such-id", { name: "X" });
    if (res.status === 404) {
      const data = res.json();
      if (Array.isArray(data.errors)) {
        pass("PATCH /api/forwards/unknown → 404 with errors[]");
      } else {
        fail("PATCH /api/forwards/unknown → 404 but no errors[]");
      }
    } else {
      fail(`PATCH /api/forwards/unknown → expected 404, got ${res.status}`);
    }
  }

  // GET /api/status — array with required fields
  {
    const res = await api.get("/api/status");
    if (res.status === 200) {
      const data = res.json();
      if (Array.isArray(data)) {
        pass("GET /api/status → 200 array");
        if (data.length > 0) {
          const status = data[0];
          const required = ["ruleId", "running", "bytesIn", "bytesOut"];
          const missing = required.filter((f) => !(f in status));
          if (missing.length === 0) {
            pass("GET /api/status → ForwardStatus has required fields");
          } else {
            fail(`GET /api/status → ForwardStatus missing: ${missing.join(", ")}`);
          }
        }
      } else {
        fail(`GET /api/status → expected array, got ${typeof data}`);
      }
    } else {
      fail(`GET /api/status → expected 200, got ${res.status}`);
    }
  }

  // POST /api/forwards/:id/start — 200 with {ruleId, running}
  if (tcpId) {
    const res = await api.post(`/api/forwards/${tcpId}/start`, {});
    if (res.status === 200) {
      const data = res.json();
      if ("running" in data && "ruleId" in data) {
        pass("POST /api/forwards/:id/start → 200 with ruleId, running");
      } else {
        fail(`POST /api/forwards/:id/start → 200 but missing fields: ${JSON.stringify(data)}`);
      }
    } else {
      fail(`POST /api/forwards/:id/start → expected 200, got ${res.status}: ${res.body}`);
    }
  }

  // POST /api/forwards/:id/stop — 200 with {ruleId, running}
  if (tcpId) {
    const res = await api.post(`/api/forwards/${tcpId}/stop`, {});
    if (res.status === 200) {
      const data = res.json();
      if ("running" in data && "ruleId" in data) {
        pass("POST /api/forwards/:id/stop → 200 with ruleId, running");
      } else {
        fail(`POST /api/forwards/:id/stop → 200 but missing fields: ${JSON.stringify(data)}`);
      }
    } else {
      fail(`POST /api/forwards/:id/stop → expected 200, got ${res.status}: ${res.body}`);
    }
  }

  // POST /api/forwards/unknown/start → 404
  {
    const res = await api.post("/api/forwards/no-such-id/start", {});
    if (res.status === 404) {
      pass("POST /api/forwards/unknown/start → 404");
    } else {
      fail(`POST /api/forwards/unknown/start → expected 404, got ${res.status}`);
    }
  }

  // GET /api/activity — {events: [...]}
  {
    const res = await api.get("/api/activity");
    if (res.status === 200) {
      const data = res.json();
      if (data && typeof data === "object" && Array.isArray(data.events)) {
        pass("GET /api/activity → 200 {events: [...]}");
        if (data.events.length > 0) {
          const ev = data.events[0];
          const required = ["id", "timestamp", "type", "severity"];
          const missing = required.filter((f) => !(f in ev));
          if (missing.length === 0) {
            pass("GET /api/activity → ActivityEvent has required fields");
          } else {
            fail(`GET /api/activity → ActivityEvent missing: ${missing.join(", ")}`);
          }
        }
      } else {
        fail(`GET /api/activity → expected {events:[]}, got: ${JSON.stringify(data)}`);
      }
    } else {
      fail(`GET /api/activity → expected 200, got ${res.status}`);
    }
  }

  // DELETE /api/activity — clears the log, returns 204
  {
    const res = await api.delete("/api/activity");
    if (res.status === 204) {
      pass("DELETE /api/activity → 204 No Content");
    } else {
      fail(`DELETE /api/activity → expected 204, got ${res.status}`);
    }
  }

  // GET /api/activity after clear — events array is empty
  {
    const res = await api.get("/api/activity");
    if (res.status === 200) {
      const data = res.json();
      if (data && typeof data === "object" && Array.isArray(data.events) && data.events.length === 0) {
        pass("GET /api/activity after DELETE → events array is empty");
      } else {
        fail(`GET /api/activity after DELETE → expected empty events, got: ${JSON.stringify(data)}`);
      }
    } else {
      fail(`GET /api/activity after DELETE → expected 200, got ${res.status}`);
    }
  }

  // GET /api/config/export — {version:"1", exportedAt, rules:[...]}
  let exportedConfig = null;
  {
    const res = await api.get("/api/config/export");
    if (res.status === 200) {
      const data = res.json();
      if (data.version === "1" && typeof data.exportedAt === "string" && Array.isArray(data.rules)) {
        pass("GET /api/config/export → 200 with version, exportedAt, rules[]");
        exportedConfig = data;
      } else {
        fail(`GET /api/config/export → shape mismatch: ${JSON.stringify(data)}`);
      }
    } else {
      fail(`GET /api/config/export → expected 200, got ${res.status}`);
    }
  }

  // POST /api/config/import (replace) — {result: {imported, skipped, errors}, rules:[...]}
  if (exportedConfig) {
    const res = await api.post("/api/config/import", { mode: "replace", config: exportedConfig });
    if (res.status === 200) {
      const data = res.json();
      if (
        data.result &&
        typeof data.result.imported === "number" &&
        typeof data.result.skipped === "number" &&
        Array.isArray(data.result.errors) &&
        Array.isArray(data.rules)
      ) {
        pass("POST /api/config/import (replace) → 200 with result.imported, result.skipped, result.errors, rules[]");
      } else {
        fail(`POST /api/config/import (replace) → shape mismatch: ${JSON.stringify(data)}`);
      }
    } else {
      fail(`POST /api/config/import (replace) → expected 200, got ${res.status}: ${res.body}`);
    }
  }

  // POST /api/config/import — invalid mode → 400
  {
    const res = await api.post("/api/config/import", {
      mode: "overwrite",
      config: { version: "1", exportedAt: new Date().toISOString(), rules: [] },
    });
    if (res.status === 400) {
      const data = res.json();
      if (Array.isArray(data.errors)) {
        pass("POST /api/config/import (invalid mode) → 400 with errors[]");
      } else {
        fail("POST /api/config/import (invalid mode) → 400 but no errors[]");
      }
    } else {
      fail(`POST /api/config/import (invalid mode) → expected 400, got ${res.status}`);
    }
  }

  // POST /api/config/import — invalid config structure → 400
  {
    const res = await api.post("/api/config/import", {
      mode: "replace",
      config: { version: "2", rules: [] },
    });
    if (res.status === 400) {
      pass("POST /api/config/import (wrong version) → 400");
    } else {
      fail(`POST /api/config/import (wrong version) → expected 400, got ${res.status}`);
    }
  }

  // GET /api/ports/advisory — privileged + common port
  {
    const res = await api.get("/api/ports/advisory?port=80&purpose=forward");
    if (res.status === 200) {
      const data = res.json();
      if (Array.isArray(data)) {
        const codes = data.map((a) => a.code);
        if (codes.includes("PRIVILEGED_PORT") && codes.includes("COMMON_PORT")) {
          pass("GET /api/ports/advisory (port=80) → PRIVILEGED_PORT + COMMON_PORT");
        } else {
          fail(`GET /api/ports/advisory (port=80) → expected PRIVILEGED_PORT+COMMON_PORT, got: ${codes.join(",")}`);
        }
        if (data[0] && "code" in data[0] && "severity" in data[0] && "message" in data[0]) {
          pass("GET /api/ports/advisory → PortAdvisory has code, severity, message");
        } else {
          fail("GET /api/ports/advisory → PortAdvisory missing required fields");
        }
      } else {
        fail(`GET /api/ports/advisory → expected array, got ${typeof data}`);
      }
    } else {
      fail(`GET /api/ports/advisory → expected 200, got ${res.status}`);
    }
  }

  // GET /api/ports/advisory — clean port in recommended range → no advisories
  {
    const res = await api.get("/api/ports/advisory?port=48500&purpose=forward");
    if (res.status === 200) {
      const data = res.json();
      if (Array.isArray(data) && data.length === 0) {
        pass("GET /api/ports/advisory (port=48500) → [] (no advisories)");
      } else {
        fail(`GET /api/ports/advisory (port=48500) → expected [], got: ${JSON.stringify(data)}`);
      }
    } else {
      fail(`GET /api/ports/advisory (48500) → expected 200, got ${res.status}`);
    }
  }

  // GET /api/ports/advisory — LAN exposure
  {
    const res = await api.get("/api/ports/advisory?port=48500&purpose=forward&listenHost=0.0.0.0");
    if (res.status === 200) {
      const data = res.json();
      if (Array.isArray(data) && data.some((a) => a.code === "LAN_EXPOSURE")) {
        pass("GET /api/ports/advisory (0.0.0.0) → LAN_EXPOSURE advisory");
      } else {
        fail(`GET /api/ports/advisory (0.0.0.0) → expected LAN_EXPOSURE, got: ${JSON.stringify(data.map((a) => a.code))}`);
      }
    } else {
      fail(`GET /api/ports/advisory (0.0.0.0) → expected 200, got ${res.status}`);
    }
  }

  // GET /api/ports/advisory — invalid port → 400
  {
    const res = await api.get("/api/ports/advisory?port=notaport&purpose=forward");
    if (res.status === 400) {
      const data = res.json();
      if (Array.isArray(data.errors)) {
        pass("GET /api/ports/advisory (invalid port) → 400 with errors[]");
      } else {
        fail("GET /api/ports/advisory (invalid port) → 400 but no errors[]");
      }
    } else {
      fail(`GET /api/ports/advisory (invalid port) → expected 400, got ${res.status}`);
    }
  }

  // GET /api/ports/advisory — invalid purpose → 400
  {
    const res = await api.get("/api/ports/advisory?port=48500&purpose=unknown");
    if (res.status === 400) {
      pass("GET /api/ports/advisory (invalid purpose) → 400");
    } else {
      fail(`GET /api/ports/advisory (invalid purpose) → expected 400, got ${res.status}`);
    }
  }

  // DELETE /api/forwards/:id → 204
  if (tcpId) {
    const res = await api.delete(`/api/forwards/${tcpId}`);
    if (res.status === 204) {
      pass("DELETE /api/forwards/:id → 204");
    } else {
      fail(`DELETE /api/forwards/:id → expected 204, got ${res.status}`);
    }
    tcpId = null;
  }

  // DELETE /api/forwards/unknown → 404
  {
    const res = await api.delete("/api/forwards/no-such-id");
    if (res.status === 404) {
      const data = res.json();
      if (Array.isArray(data.errors)) {
        pass("DELETE /api/forwards/unknown → 404 with errors[]");
      } else {
        fail("DELETE /api/forwards/unknown → 404 but no errors[]");
      }
    } else {
      fail(`DELETE /api/forwards/unknown → expected 404, got ${res.status}`);
    }
  }

  // Cleanup remaining rules
  for (const id of [udpId, udpDefaultId].filter(Boolean)) {
    try { await api.delete(`/api/forwards/${id}`); } catch { /* best effort cleanup */ }
  }

  // Unknown API route → 404 with errors[]
  {
    const res = await api.get("/api/no-such-route");
    if (res.status === 404) {
      const data = res.json();
      if (Array.isArray(data.errors)) {
        pass("GET /api/unknown → 404 with errors[]");
      } else {
        fail("GET /api/unknown → 404 but no errors[]");
      }
    } else {
      fail(`GET /api/unknown → expected 404, got ${res.status}`);
    }
  }

  // ── POST /api/forwards/:id/diagnose ─────────────────────────────────────

  // Unknown rule → 404 with errors[]
  {
    const res = await api.post("/api/forwards/no-such-id/diagnose", {});
    if (res.status === 404) {
      const data = res.json();
      if (Array.isArray(data.errors)) {
        pass("POST /api/forwards/unknown/diagnose → 404 with errors[]");
      } else {
        fail("POST /api/forwards/unknown/diagnose → 404 but no errors[]");
      }
    } else {
      fail(`POST /api/forwards/unknown/diagnose → expected 404, got ${res.status}`);
    }
  }

  // Diagnose with reachable TCP target — create a temporary echo server
  {
    const echoPort = await getFreePort();
    const echoServer = await new Promise((resolve, reject) => {
      const srv = createServer();
      srv.on("error", reject);
      srv.listen(echoPort, "127.0.0.1", () => resolve(srv));
    }).catch(() => null);

    const diagListenPort = await getFreePort();
    // Create a TCP rule pointing to the echo server
    const createRes = await api.post("/api/forwards", {
      name: "Diag Contract TCP",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: diagListenPort,
      targetHost: "127.0.0.1",
      targetPort: echoPort,
      enabled: false,
    });

    let diagTcpId = null;
    if (createRes.status === 201) {
      diagTcpId = createRes.json().id;
    }

    if (diagTcpId && echoServer) {
      const res = await api.post(`/api/forwards/${diagTcpId}/diagnose`, {});
      if (res.status === 200) {
        const data = res.json();
        const requiredFields = ["ruleId", "ruleName", "protocol", "summary", "checks", "diagnosedAt"];
        const missing = requiredFields.filter((f) => !(f in data));
        if (missing.length === 0) {
          pass("POST /api/forwards/:id/diagnose → 200 with required fields");
        } else {
          fail(`POST /api/forwards/:id/diagnose → missing fields: ${missing.join(", ")}`);
        }
        if (data.ruleId === diagTcpId && data.protocol === "tcp") {
          pass("POST /api/forwards/:id/diagnose → ruleId and protocol correct");
        } else {
          fail(`POST /api/forwards/:id/diagnose → ruleId/protocol mismatch: ${JSON.stringify(data)}`);
        }
        if (data.summary && ["pass", "warn", "fail"].includes(data.summary.status)) {
          pass("POST /api/forwards/:id/diagnose → summary.status is valid");
        } else {
          fail(`POST /api/forwards/:id/diagnose → summary.status invalid: ${data.summary?.status}`);
        }
        if (Array.isArray(data.checks) && data.checks.length > 0) {
          const checkIds = data.checks.map((c) => c.id);
          const expectedIds = ["listen-host", "lan-exposure", "privileged-port", "common-port", "listen-bind", "target-host", "target-connect"];
          const missingIds = expectedIds.filter((id) => !checkIds.includes(id));
          if (missingIds.length === 0) {
            pass("POST /api/forwards/:id/diagnose → all expected check IDs present");
          } else {
            fail(`POST /api/forwards/:id/diagnose → missing check IDs: ${missingIds.join(", ")}`);
          }
          const connectCheck = data.checks.find((c) => c.id === "target-connect");
          if (connectCheck && connectCheck.status === "pass") {
            pass("POST /api/forwards/:id/diagnose → target-connect passes for reachable target");
          } else {
            fail(`POST /api/forwards/:id/diagnose → target-connect status: ${connectCheck?.status} (want pass)`);
          }
          const allHaveShape = data.checks.every(
            (c) => typeof c.id === "string" && typeof c.label === "string" &&
              typeof c.status === "string" && typeof c.message === "string"
          );
          if (allHaveShape) {
            pass("POST /api/forwards/:id/diagnose → each check has id, label, status, message");
          } else {
            fail("POST /api/forwards/:id/diagnose → some checks are missing required fields");
          }
        } else {
          fail("POST /api/forwards/:id/diagnose → checks must be a non-empty array");
        }
      } else {
        fail(`POST /api/forwards/:id/diagnose → expected 200, got ${res.status}: ${res.body}`);
      }
    } else {
      skip("POST /api/forwards/:id/diagnose (TCP reachable) — echo server or rule creation failed");
    }

    // Clean up
    if (echoServer) { try { echoServer.close(); } catch { /* best effort */ } }
    if (diagTcpId) { try { await api.delete(`/api/forwards/${diagTcpId}`); } catch { /* best effort */ } }
  }

  // Diagnose UDP rule — target-connect must be skip, udp-mode check must be present
  {
    const udpDiagPort = await getFreePort();
    const createRes = await api.post("/api/forwards", {
      name: "Diag Contract UDP",
      protocol: "udp",
      listenHost: "127.0.0.1",
      listenPort: udpDiagPort,
      targetHost: "127.0.0.1",
      targetPort: 19998,
      enabled: false,
      udpMode: "bidirectional-last-client",
    });

    let diagUdpId = null;
    if (createRes.status === 201) {
      diagUdpId = createRes.json().id;
    }

    if (diagUdpId) {
      const res = await api.post(`/api/forwards/${diagUdpId}/diagnose`, {});
      if (res.status === 200) {
        const data = res.json();
        const connectCheck = data.checks?.find((c) => c.id === "target-connect");
        if (connectCheck && connectCheck.status === "skip") {
          pass("POST /api/forwards/:id/diagnose (UDP) → target-connect is skip");
        } else {
          fail(`POST /api/forwards/:id/diagnose (UDP) → target-connect status: ${connectCheck?.status} (want skip)`);
        }
        const udpModeCheck = data.checks?.find((c) => c.id === "udp-mode");
        if (udpModeCheck && udpModeCheck.status === "warn") {
          pass("POST /api/forwards/:id/diagnose (UDP) → udp-mode warns for bidirectional-last-client");
        } else {
          fail(`POST /api/forwards/:id/diagnose (UDP) → udp-mode status: ${udpModeCheck?.status} (want warn)`);
        }
      } else {
        fail(`POST /api/forwards/:id/diagnose (UDP) → expected 200, got ${res.status}`);
      }
      try { await api.delete(`/api/forwards/${diagUdpId}`); } catch { /* best effort */ }
    } else {
      skip("POST /api/forwards/:id/diagnose (UDP) — rule creation failed");
    }
  }

  // Diagnose with 0.0.0.0 listen host → lan-exposure must warn
  {
    const lanDiagPort = await getFreePort();
    const createRes = await api.post("/api/forwards", {
      name: "Diag Contract LAN",
      protocol: "tcp",
      listenHost: "0.0.0.0",
      listenPort: lanDiagPort,
      targetHost: "127.0.0.1",
      targetPort: 19999,
      enabled: false,
    });

    let diagLanId = null;
    if (createRes.status === 201) {
      diagLanId = createRes.json().id;
    }

    if (diagLanId) {
      const res = await api.post(`/api/forwards/${diagLanId}/diagnose`, {});
      if (res.status === 200) {
        const data = res.json();
        const lanCheck = data.checks?.find((c) => c.id === "lan-exposure");
        if (lanCheck && lanCheck.status === "warn") {
          pass("POST /api/forwards/:id/diagnose (0.0.0.0) → lan-exposure warns");
        } else {
          fail(`POST /api/forwards/:id/diagnose (0.0.0.0) → lan-exposure status: ${lanCheck?.status} (want warn)`);
        }
      } else {
        fail(`POST /api/forwards/:id/diagnose (0.0.0.0) → expected 200, got ${res.status}`);
      }
      try { await api.delete(`/api/forwards/${diagLanId}`); } catch { /* best effort */ }
    } else {
      skip("POST /api/forwards/:id/diagnose (LAN) — rule creation failed");
    }
  }
}

async function main() {
  const cliArgs = process.argv.slice(2);
  const skipGo = cliArgs.includes("--skip-go");

  console.log("[validate:contract] API contract parity tests\n");

  const tsServerPath = join(repoRoot, "server", "build", "index.js");
  const packageBinary = join(repoRoot, "build", "portier", isWindows ? "service.exe" : "service");
  const devBinary = join(repoRoot, "service", "build", isWindows ? "portier-service.exe" : "portier-service");

  // ── TypeScript server ────────────────────────────────────────────────────────

  console.log("[validate:contract] TypeScript server");

  if (!existsSync(tsServerPath)) {
    skip(`TypeScript server build not found: ${tsServerPath}`);
    skip("Run: npm run build:server");
    console.log("\n[validate:contract] SKIPPED (TypeScript server not built).\n");
    process.exit(0);
  }

  const ts = await startServer("node", [tsServerPath, "--service"], "ts");
  if (!ts.ready) {
    fail("TypeScript server did not respond to /api/forwards within 15s");
    cleanup(ts);
    console.error("\n[validate:contract] FAILED (TypeScript server did not start).\n");
    process.exit(1);
  }
  pass("TypeScript server started");

  try {
    await runScenarios(ts.baseUrl, "TypeScript");
  } finally {
    cleanup(ts);
  }

  // ── Go service ───────────────────────────────────────────────────────────────

  if (skipGo) {
    console.log("\n[validate:contract] Go parity: --skip-go set, skipping.\n");
  } else {
    const goPath = existsSync(packageBinary) ? packageBinary
      : existsSync(devBinary) ? devBinary
      : null;

    if (!goPath) {
      console.log(`\n[validate:contract] Go parity: binary not found.`);
      console.log(`  Checked: ${packageBinary}`);
      console.log(`  Checked: ${devBinary}`);
      console.log("  Run: npm run build:runtime  (or npm run build:service)");
      console.log("  Go parity skipped (not a failure).\n");
      skipped++;
    } else {
      console.log(`\n[validate:contract] Go service: ${goPath}`);
      const go = await startServer(goPath, ["--service"], "go");
      if (!go.ready) {
        fail("Go service did not respond to /api/forwards within 15s");
        cleanup(go);
      } else {
        pass("Go service started");
        try {
          await runScenarios(go.baseUrl, "Go");
        } finally {
          cleanup(go);
        }
      }
    }
  }

  // ── Summary ──────────────────────────────────────────────────────────────────

  console.log(`\n[validate:contract] ${passed} passed, ${skipped} skipped, ${failed} failed.\n`);
  if (failed > 0) {
    console.error("[validate:contract] FAILED.\n");
    process.exit(1);
  }
  console.log("[validate:contract] All contract tests passed.\n");
}

main().catch((err) => {
  console.error("[validate:contract] Unexpected error:", err);
  process.exit(1);
});
