#!/usr/bin/env node
/* global console, process, setTimeout */
/**
 * Native runtime smoke of the HOST-arch portable artifact.
 *
 * Unlike validate-runtime.js (which smokes the staged build/portier/), this extracts the
 * actual shipped portable archive for the current platform/arch and runs THAT:
 *   - reports the runner architecture (uname -m + process.arch) as evidence,
 *   - asserts the service binary's machine type matches the host arch (so an emulated/wrong
 *     binary cannot pass as native),
 *   - runs `portier version` (must report the package version),
 *   - starts the service on a free port, asserts GET /api/health and that GET /api/runtime
 *     reports the package version, then stops it cleanly.
 *
 * This is the NATIVE arm64 smoke when run on an arm64 runner (it is honest about the arch it
 * actually ran on — it never claims arm64 unless `uname -m` is arm64/aarch64). It runs on any
 * host against that host's own portable (amd64 or arm64). Config/data live in a temp dir; no
 * private docs are touched.
 *
 * Usage:
 *   node scripts/validate-portable-smoke.js [--archive <path>]
 */

import { existsSync, readFileSync, mkdirSync, writeFileSync, rmSync, statSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { spawnSync, spawn } from "node:child_process";
import { createServer } from "node:net";
import { get } from "node:http";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import AdmZip from "adm-zip";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const platform = { win32: "windows", darwin: "macos", linux: "linux" }[process.platform] || process.platform;
const arch = process.arch === "arm64" ? "arm64" : "amd64";
const isWindows = process.platform === "win32";
const binExt = isWindows ? ".exe" : "";
const ext = isWindows ? "zip" : "tar.gz";

const rawArgs = process.argv.slice(2);
const flagValue = (f) => { const i = rawArgs.indexOf(f); return i >= 0 && i + 1 < rawArgs.length ? rawArgs[i + 1] : null; };

const version = JSON.parse(readFileSync(join(repoRoot, "package.json"), "utf8")).version;
const archiveName = `portier-${version}-${platform}-${arch}.${ext}`;
const archivePath = flagValue("--archive")
  ? resolve(flagValue("--archive"))
  : join(repoRoot, "build", "releases", platform, archiveName);

let passed = 0;
let failed = 0;
const pass = (m) => { console.log(`  ✓ ${m}`); passed++; };
const fail = (m) => { console.error(`  ✗ ${m}`); failed++; };

// machineArch reads an executable header → "amd64"/"arm64" (ELF, Mach-O, PE).
function machineArch(buf) {
  if (!buf || buf.length < 8) return "empty";
  if (buf[0] === 0x7f && buf[1] === 0x45 && buf[2] === 0x4c && buf[3] === 0x46) {
    const m = buf.readUInt16LE(18);
    return m === 0x3e ? "amd64" : m === 0xb7 ? "arm64" : `elf-0x${m.toString(16)}`;
  }
  if (buf[0] === 0xcf && buf[1] === 0xfa && buf[2] === 0xed && buf[3] === 0xfe) {
    const cpu = buf.readUInt32LE(4);
    return cpu === 0x01000007 ? "amd64" : cpu === 0x0100000c ? "arm64" : `macho-0x${cpu.toString(16)}`;
  }
  if (buf[0] === 0x4d && buf[1] === 0x5a && buf.length >= 0x40) {
    const peOff = buf.readUInt32LE(0x3c);
    if (buf.length >= peOff + 6 && buf.readUInt32LE(peOff) === 0x00004550) {
      const machine = buf.readUInt16LE(peOff + 4);
      return machine === 0x8664 ? "amd64" : machine === 0xaa64 ? "arm64" : `pe-0x${machine.toString(16)}`;
    }
  }
  return "unknown";
}

function getFreePort() {
  return new Promise((res, rej) => {
    const srv = createServer();
    srv.listen(0, "127.0.0.1", () => { const { port } = srv.address(); srv.close(() => res(port)); });
    srv.on("error", rej);
  });
}
function httpGet(url) {
  return new Promise((res, rej) => {
    const req = get(url, (r) => { let b = ""; r.on("data", (d) => (b += d)); r.on("end", () => res({ status: r.statusCode, body: b })); });
    req.on("error", rej);
    req.setTimeout(2000, () => req.destroy(new Error("timeout")));
  });
}
async function waitForHealth(url, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try { if ((await httpGet(url)).status === 200) return true; } catch { /* not ready */ }
    await new Promise((r) => setTimeout(r, 400));
  }
  return false;
}

async function main() {
  console.log("[portable-smoke] Native runtime smoke of the host-arch portable artifact.");
  const uname = isWindows ? "(n/a on windows)" : (spawnSync("uname", ["-m"], { encoding: "utf8" }).stdout || "").trim();
  console.log(`[portable-smoke]   platform : ${platform}`);
  console.log(`[portable-smoke]   process.arch : ${process.arch}  →  artifact arch : ${arch}`);
  console.log(`[portable-smoke]   uname -m : ${uname}`);
  console.log(`[portable-smoke]   artifact : ${archiveName}`);

  // Honesty guard: never claim an arm64 native smoke unless the host really is arm64.
  if (arch === "arm64") {
    if (/^(arm64|aarch64)$/.test(uname)) pass(`running on native arm64 hardware (uname -m = ${uname})`);
    else fail(`expected arm64 hardware but uname -m = "${uname}" — refusing to call this a native arm64 smoke`);
  } else {
    pass(`running on ${arch} (uname -m = ${uname})`);
  }

  if (!existsSync(archivePath)) {
    fail(`portable artifact not found: ${archivePath} — build with: npm run build:release:${platform}`);
    summarize();
    return;
  }
  pass(`${archiveName} present (${(statSync(archivePath).size / 1024 / 1024).toFixed(1)} MB)`);

  const stage = join(tmpdir(), `portier-portable-smoke-${process.pid}-${Date.now()}`);
  mkdirSync(stage, { recursive: true });
  try {
    // Extract: tar.gz via the system tar (Unix), zip via adm-zip.
    if (ext === "tar.gz") {
      const r = spawnSync("tar", ["-xzf", archivePath, "-C", stage], { encoding: "utf8" });
      if ((r.status ?? 1) !== 0) { fail(`tar extraction failed: ${(r.stderr || "").trim()}`); return; }
    } else {
      new AdmZip(archivePath).extractAllTo(stage, true);
    }
    pass("artifact extracted");

    // Confirm the staged binaries exist.
    const cli = join(stage, `portier${binExt}`);
    const service = join(stage, `service${binExt}`);
    for (const [label, p] of [["portier", cli], ["service", service]]) {
      if (existsSync(p)) pass(`extracted ${label}${binExt}`);
      else { fail(`missing extracted ${label}${binExt}`); }
    }
    if (!existsSync(cli) || !existsSync(service)) return;

    // Machine type matches the host arch (native, not emulated/mislabeled).
    const svcArch = machineArch(readFileSync(service).subarray(0, 8192));
    if (svcArch === arch) pass(`service binary machine type is ${svcArch} (matches host)`);
    else fail(`service binary machine type is ${svcArch}, expected ${arch}`);

    // CLI version.
    const v = spawnSync(cli, ["version"], { encoding: "utf8" });
    const out = (v.stdout || "").trim();
    if ((v.status ?? 1) === 0 && out.includes(version)) pass(`portier version reports ${version} ("${out}")`);
    else { fail(`portier version: status=${v.status} out="${out}" expected to include ${version}`); }

    // Runtime smoke: start the extracted service, hit /api/health + /api/runtime.
    const port = await getFreePort();
    const config = join(stage, "rules.json");
    writeFileSync(config, "[]");
    const staticDir = join(stage, "web");
    const base = `http://127.0.0.1:${port}`;
    console.log(`[portable-smoke]   starting service on ${base} ...`);
    const proc = spawn(service, ["--service", "--config", config, "--host", "127.0.0.1", "--port", String(port), "--static-dir", staticDir], { stdio: ["ignore", "pipe", "pipe"], detached: false });
    try {
      if (!(await waitForHealth(`${base}/api/health`))) {
        fail("service did not respond to /api/health within 15s");
      } else {
        pass("GET /api/health responded OK");
        const rt = await httpGet(`${base}/api/runtime`);
        let doc = null;
        try { doc = JSON.parse(rt.body); } catch { doc = null; }
        if (rt.status === 200 && doc && doc.version === version) pass(`GET /api/runtime reports version ${version}`);
        else fail(`GET /api/runtime: status=${rt.status} version=${JSON.stringify(doc && doc.version)} expected ${version}`);
      }
    } finally {
      proc.kill();
    }
  } finally {
    try { rmSync(stage, { recursive: true, force: true }); } catch { /* best effort */ }
  }

  summarize();
}

function summarize() {
  console.log(`\n[portable-smoke] ${passed} passed, ${failed} failed.`);
  if (failed > 0) {
    console.error("[portable-smoke] Portable runtime smoke FAILED.\n");
    process.exit(1);
  }
  console.log(`[portable-smoke] Native ${platform}/${arch} portable runtime smoke passed.\n`);
}

main().catch((err) => {
  console.error("[portable-smoke] Unexpected error:", err);
  process.exit(1);
});
