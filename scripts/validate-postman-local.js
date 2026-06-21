#!/usr/bin/env node
/* global console, process, setTimeout */
/**
 * Local-only Newman runtime smoke for the generated Postman collection.
 *
 * Runs `postman/collection.json` (with `postman/environment.json`) against a live,
 * locally-started Portier runtime. This complements the static `validate:postman`
 * drift check: it proves the generated consumer artifact actually executes against a
 * running runtime and that documented statuses/envelopes hold.
 *
 * It is local-only by design (not wired into push/PR CI). It needs no external
 * internet: Newman is a dev dependency and the runtime is started on loopback with a
 * throwaway config.
 *
 * Runtime target — the NestJS/TypeScript server (`server/build/index.js`):
 *   The Postman collection is generated from the canonical OpenAPI document, which is
 *   itself produced from the NestJS server. The NestJS server is therefore the
 *   contract-faithful target for this artifact. The two runtimes are at parity on the
 *   whole frozen `/api` surface; they differ on exactly one thing — the liveness probe
 *   path: NestJS (and OpenAPI) document `GET /health`, while the Go production service
 *   exposes `GET /api/health` and intentionally does NOT serve `/health` (see
 *   service/sources/api/route_inventory_test.go). Running the OpenAPI-derived collection
 *   against the Go service would 404 on that one documented-divergent probe; running it
 *   against NestJS exercises the collection exactly as documented. The Go runtime's
 *   liveness divergence is already covered by `validate:runtime:smoke` and the Go
 *   route-inventory parity test.
 *
 * Usage:
 *   node scripts/validate-postman-local.js [--port <managementPort>] [--listen-port <p>]
 *                                     [--folder "<name>"] [--keep-going] [--verbose]
 *
 *   --port         management port to bind (default: a free port; env PORTIER_PORT also honored)
 *   --listen-port  listen port the happy-path demo rule binds when started
 *                  (default: a free port; env PORTIER_LISTEN_PORT also honored)
 *   --folder       run only one top-level folder (repeatable); default runs all three
 *   --keep-going   run every folder even if one fails (default: stop on first failure)
 *   --verbose      pass Newman's verbose reporter
 *
 * Each top-level folder runs against its OWN freshly-started runtime with an empty,
 * temporary rules.json. This isolates the folders: the Atomic Endpoint Tests folder
 * intentionally creates a demo rule on the same binding the Happy Path Rule Flow uses,
 * so running them against a single shared runtime would make the happy-path create
 * collide (409). Fresh-runtime-per-folder keeps every folder independent and self-
 * cleaning, and nothing the smoke creates ever survives the run (the temp config dir
 * is deleted on teardown). No user/production config is touched.
 */

import { existsSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { spawn, spawnSync } from "node:child_process";
import { createServer } from "node:net";
import { get } from "node:http";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..");
const isWindows = process.platform === "win32";
const npmCommand = isWindows ? "npm.cmd" : "npm";

const collectionPath = join(repoRoot, "postman", "collection.json");
const environmentPath = join(repoRoot, "postman", "environment.json");
const newmanCli = join(repoRoot, "node_modules", "newman", "bin", "newman.js");
const serverEntry = join(repoRoot, "server", "build", "index.js");

// The three top-level folders the collection ships, in execution order.
const ALL_FOLDERS = ["Atomic Endpoint Tests", "Happy Path Rule Flow", "Negative/Error Tests"];

function parseArgs(argv) {
  const out = { folders: [], keepGoing: false, verbose: false, port: null, listenPort: null };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--keep-going") out.keepGoing = true;
    else if (a === "--verbose") out.verbose = true;
    else if (a === "--folder") out.folders.push(argv[++i]);
    else if (a === "--port") out.port = Number(argv[++i]);
    else if (a === "--listen-port") out.listenPort = Number(argv[++i]);
    else {
      console.error(`[postman-local] Unknown argument: ${a}`);
      process.exit(2);
    }
  }
  return out;
}

function getFreePort() {
  return new Promise((res, reject) => {
    const srv = createServer();
    srv.listen(0, "127.0.0.1", () => {
      const { port } = srv.address();
      srv.close(() => res(port));
    });
    srv.on("error", reject);
  });
}

function httpGet(url) {
  return new Promise((res, reject) => {
    const req = get(url, (r) => {
      let body = "";
      r.on("data", (d) => { body += d; });
      r.on("end", () => res({ status: r.statusCode, body }));
    });
    req.on("error", reject);
    req.setTimeout(2000, () => req.destroy(new Error("timeout")));
  });
}

async function waitForHealth(url, timeoutMs = 20000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const r = await httpGet(url);
      if (r.status === 200) return true;
    } catch {
      // not ready yet
    }
    await new Promise((r) => setTimeout(r, 300));
  }
  return false;
}

// Ensure the NestJS server bundle exists; build it (shared + server) if missing.
// Never builds release/package artifacts.
function ensureServerBuilt() {
  if (existsSync(serverEntry)) return;
  console.log("[postman-local] server/build/index.js missing — building (build:shared + build:server)...");
  for (const script of ["build:shared", "build:server"]) {
    const r = spawnSync(npmCommand, ["run", script], { cwd: repoRoot, stdio: "inherit", shell: isWindows });
    if ((r.status ?? 1) !== 0) {
      console.error(`[postman-local] Failed: npm run ${script}`);
      process.exit(1);
    }
  }
  if (!existsSync(serverEntry)) {
    console.error("[postman-local] server/build/index.js still missing after build.");
    process.exit(1);
  }
}

// Run a single Newman folder against an already-running runtime on managementPort.
// Returns the Newman process exit code (0 = all assertions passed).
function runNewmanFolder(folder, managementPort, listenPort, verbose) {
  const args = [
    newmanCli,
    "run", collectionPath,
    "-e", environmentPath,
    "--folder", folder,
    // Override the environment so the collection targets THIS runtime. baseUrl is
    // `http://{{host}}:{{port}}`, so overriding host/port cascades into every URL.
    "--env-var", "host=127.0.0.1",
    "--env-var", `port=${managementPort}`,
    "--env-var", `listenPort=${listenPort}`,
    "--reporters", "cli",
  ];
  if (verbose) args.push("--verbose");
  const r = spawnSync(process.execPath, args, { cwd: repoRoot, stdio: "inherit" });
  return r.status ?? 1;
}

// Start a fresh runtime (empty temp rules.json, given management port), run one folder
// against it, then stop the runtime and delete the temp dir. Fully isolated.
async function smokeFolder(folder, managementPort, listenPort, verbose) {
  const tempDir = join(tmpdir(), `portier-postman-${process.pid}-${managementPort}`);
  mkdirSync(tempDir, { recursive: true });
  const tempConfig = join(tempDir, "rules.json");
  writeFileSync(tempConfig, "[]");

  // The NestJS server documents liveness at /health (the path the collection uses).
  const healthUrl = `http://127.0.0.1:${managementPort}/health`;
  console.log(`\n[postman-local] ── Folder: ${folder}`);
  console.log(`[postman-local]    runtime : node ${serverEntry}`);
  console.log(`[postman-local]    mgmt    : 127.0.0.1:${managementPort}`);
  console.log(`[postman-local]    listen  : ${listenPort} (happy-path bind)`);
  console.log(`[postman-local]    config  : ${tempConfig} (temp, discarded)`);

  const proc = spawn(
    process.execPath,
    [
      serverEntry,
      "--service",
      "--config", tempConfig,
      "--host", "127.0.0.1",
      "--port", String(managementPort),
      // The smoke only exercises the API; point static-dir at the temp dir so the
      // runtime does not resolve a "web" dir relative to the cwd.
      "--static-dir", tempDir,
    ],
    { stdio: ["ignore", "ignore", "inherit"], detached: false }
  );

  let exitedEarly = null;
  proc.on("exit", (code) => { exitedEarly = code; });

  try {
    const ready = await waitForHealth(healthUrl);
    if (!ready) {
      console.error(`[postman-local] Runtime did not become healthy within 20s (exit=${exitedEarly}).`);
      return 1;
    }
    return runNewmanFolder(folder, managementPort, listenPort, verbose);
  } finally {
    if (exitedEarly === null) {
      proc.kill();
    }
    try {
      rmSync(tempDir, { recursive: true, force: true });
    } catch {
      // best effort
    }
  }
}

async function main() {
  const opts = parseArgs(process.argv.slice(2));

  if (!existsSync(collectionPath) || !existsSync(environmentPath)) {
    console.error("[postman-local] Missing postman/collection.json or postman/environment.json.");
    console.error("[postman-local] Run: npm run generate:postman");
    process.exit(1);
  }
  if (!existsSync(newmanCli)) {
    console.error("[postman-local] Newman is not installed. Run: npm install");
    process.exit(1);
  }

  const folders = opts.folders.length ? opts.folders : ALL_FOLDERS;
  for (const f of folders) {
    if (!ALL_FOLDERS.includes(f)) {
      console.error(`[postman-local] Unknown folder: ${JSON.stringify(f)}`);
      console.error(`[postman-local] Known folders: ${ALL_FOLDERS.map((x) => JSON.stringify(x)).join(", ")}`);
      process.exit(2);
    }
  }

  ensureServerBuilt();
  const envPort = Number(process.env.PORTIER_PORT) || null;
  const envListen = Number(process.env.PORTIER_LISTEN_PORT) || null;
  const managementPort = opts.port || envPort || (await getFreePort());
  const listenPort = opts.listenPort || envListen || (await getFreePort());

  console.log("[postman-local] Local Newman runtime smoke (local-only; no network, no user config touched).");

  let failed = 0;
  for (const folder of folders) {
    const code = await smokeFolder(folder, managementPort, listenPort, opts.verbose);
    if (code !== 0) {
      failed++;
      console.error(`[postman-local] FAILED: "${folder}" (newman exit ${code}).`);
      if (!opts.keepGoing) break;
    } else {
      console.log(`[postman-local] PASSED: "${folder}".`);
    }
  }

  if (failed > 0) {
    console.error(`\n[postman-local] ${failed} folder(s) failed.`);
    process.exit(1);
  }
  console.log(`\n[postman-local] All ${folders.length} folder(s) passed against a live local runtime.`);
}

main().catch((err) => {
  console.error("[postman-local] Unexpected error:", err);
  process.exit(1);
});
