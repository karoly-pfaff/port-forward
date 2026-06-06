import { mkdirSync, writeFileSync } from "fs";
import { resolve } from "path";

// Creates a writable empty config file for the E2E server before it starts.
// Playwright runs globalSetup before launching webServer.
export default async function globalSetup(): Promise<void> {
  const configPath = resolve("test-results/portier-e2e-config.json");
  mkdirSync(resolve("test-results"), { recursive: true });
  writeFileSync(configPath, JSON.stringify({ version: "1", rules: [] }));
}
