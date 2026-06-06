#!/usr/bin/env node
/* global console, process */
import { existsSync } from "node:fs";
import { spawnSync } from "node:child_process";
import path from "node:path";

const ROOT = process.cwd();
const prettierBin = process.platform === "win32"
  ? path.join(ROOT, "node_modules", ".bin", "prettier.cmd")
  : path.join(ROOT, "node_modules", ".bin", "prettier");

if (!existsSync(prettierBin)) {
  console.log("Prettier is not installed locally; skipping format.");
  console.log("Install/configure Prettier before relying on npm run format for source formatting.");
  process.exit(0);
}

const targets = [
  "server/**/*.{ts,tsx,json,md}",
  "client/**/*.{ts,tsx,json,md}",
  "shared/**/*.{ts,tsx,json,md}",
  "docs/**/*.md",
  "*.md",
  "*.json"
];

const result = spawnSync(prettierBin, ["--write", ...targets], {
  cwd: ROOT,
  stdio: "inherit"
});

process.exit(result.status ?? 1);
