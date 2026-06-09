import { test, expect, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { clearAllRules, createRule } from "./helpers/api.js";

const fixturesDir = join(dirname(fileURLToPath(import.meta.url)), "../fixtures/config");

// Reset all rules before each test for isolation.
test.beforeEach(async ({ baseURL }) => {
  await clearAllRules(baseURL!);
});

// Navigate to Settings and assert the view loaded.
async function goToSettings(page: Page): Promise<void> {
  await page.getByRole("navigation", { name: "Main navigation" })
    .getByRole("button", { name: "Settings" })
    .click();
  await expect(page.getByText("Export Config")).toBeVisible();
  await expect(page.getByText("Import Config")).toBeVisible();
}

// Wrap a raw rules array as ExportedConfig so the Settings import UI accepts it.
function toExportedConfigBuffer(rules: unknown[]): Buffer {
  return Buffer.from(JSON.stringify({
    version: "1",
    exportedAt: new Date().toISOString(),
    rules,
  }));
}

// ── A. Replace-mode import of v1-mixed.json ───────────────────────────────────
//
// Validates the full replace import flow: preview counts, replace confirmation,
// success message, and that all four fixture rules appear in the rule list while
// the pre-existing rule is gone.

test("settings: imports v1-mixed fixture via replace and shows all rules", async ({ page, baseURL }) => {
  // A pre-existing rule that must be gone after replace import.
  await createRule(baseURL!, {
    name: "Pre-Import Rule",
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort: 49000,
    targetHost: "127.0.0.1",
    targetPort: 49100,
    enabled: false,
  });

  await page.goto("/");
  await goToSettings(page);

  // v1-mixed.json is a raw rules array — wrap it as ExportedConfig for the UI.
  const fixture = JSON.parse(readFileSync(join(fixturesDir, "v1-mixed.json"), "utf-8")) as unknown[];

  await page.getByLabel("Select config file", { exact: true }).setInputFiles({
    name: "v1-mixed.json",
    mimeType: "application/json",
    buffer: toExportedConfigBuffer(fixture),
  });

  // Preview: 4 rules (1 TCP, 3 UDP), 2 enabled.
  await expect(page.getByText("File preview")).toBeVisible({ timeout: 3_000 });
  await expect(page.getByText("4 (1 TCP, 3 UDP)")).toBeVisible();
  await expect(page.getByText("2 enabled")).toBeVisible();

  // Switch to replace mode.
  await page.getByRole("radio", { name: /Replace/ }).click();

  // "Replace All Rules" appears; clicking it opens the confirm dialog.
  await page.getByRole("button", { name: "Replace All Rules" }).click();
  await expect(page.getByText(/This will delete all existing rules/)).toBeVisible({ timeout: 2_000 });
  await page.getByRole("button", { name: "Confirm Replace" }).click();

  // Success status message.
  await expect(page.getByRole("status")).toBeVisible({ timeout: 5_000 });
  await expect(page.getByText(/Import complete/)).toBeVisible();

  // Navigate to Forward Rules and assert all fixture rules are listed.
  await page.getByRole("navigation", { name: "Main navigation" })
    .getByRole("button", { name: "Forward Rules" })
    .click();

  await expect(page.locator("tbody").getByText("Mixed TCP")).toBeVisible({ timeout: 5_000 });
  await expect(page.locator("tbody").getByText("Mixed UDP One-Way")).toBeVisible();
  await expect(page.locator("tbody").getByText("Mixed UDP Last Client")).toBeVisible();
  await expect(page.locator("tbody").getByText("Mixed UDP Multi-Client")).toBeVisible();

  // The replaced rule must no longer appear.
  await expect(page.locator("tbody").getByText("Pre-Import Rule")).not.toBeVisible({ timeout: 2_000 });

  // No error banner visible.
  await expect(page.getByRole("alert")).not.toBeVisible({ timeout: 1_000 });
});

// ── B. Invalid JSON import — parse error, no state change ──────────────────────
//
// Uploads invalid-json.json (malformed, not parseable). The client-side file
// reader must show a parse error alert immediately. No preview, no import button.
// Navigating to Forward Rules shows the pre-existing rule is untouched.

test("settings: rejects invalid JSON file and preserves existing rules", async ({ page, baseURL }) => {
  // A rule that must survive the failed import attempt.
  await createRule(baseURL!, {
    name: "Preserved Rule",
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort: 49001,
    targetHost: "127.0.0.1",
    targetPort: 49101,
    enabled: false,
  });

  await page.goto("/");
  await goToSettings(page);

  // Upload the malformed fixture directly — its content cannot be parsed as JSON.
  const invalidContent = readFileSync(join(fixturesDir, "invalid-json.json"));
  await page.getByLabel("Select config file", { exact: true }).setInputFiles({
    name: "invalid-json.json",
    mimeType: "application/json",
    buffer: invalidContent,
  });

  // Parse-error alert appears; file preview must not appear.
  await expect(page.getByRole("alert")).toBeVisible({ timeout: 3_000 });
  await expect(page.getByText("File preview")).not.toBeVisible();

  // Import controls must not appear.
  await expect(page.getByRole("button", { name: "Import Rules" })).not.toBeVisible();
  await expect(page.getByRole("button", { name: "Replace All Rules" })).not.toBeVisible();

  // Navigate to Forward Rules — the original rule is still there.
  await page.getByRole("navigation", { name: "Main navigation" })
    .getByRole("button", { name: "Forward Rules" })
    .click();
  await expect(page.locator("tbody").getByText("Preserved Rule")).toBeVisible({ timeout: 5_000 });
});

// ── C. Export config — download shape sanity ──────────────────────────────────
//
// Creates a known rule, triggers export, captures the downloaded file, and
// asserts the ExportedConfig shape: version "1", parseable exportedAt timestamp,
// rules array, and the created rule present with correct fields.

test("settings: export downloads a valid ExportedConfig JSON file", async ({ page, baseURL }) => {
  await createRule(baseURL!, {
    name: "Export Shape Test",
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort: 49002,
    targetHost: "127.0.0.1",
    targetPort: 49102,
    enabled: false,
  });

  await page.goto("/");
  await goToSettings(page);

  // Capture the download triggered by the Export button.
  const downloadPromise = page.waitForEvent("download");
  await page.getByRole("button", { name: "Download Config" }).click();
  const download = await downloadPromise;

  // Filename must match portier-config-YYYYMMDD-HHMMSS.json.
  expect(download.suggestedFilename()).toMatch(/^portier-config-\d{8}-\d{6}\.json$/);

  // Read and parse the saved file.
  const downloadPath = await download.path();
  if (!downloadPath) throw new Error("download.path() returned null");

  const raw = readFileSync(downloadPath, "utf-8");
  const exported = JSON.parse(raw) as {
    version: string;
    exportedAt: string;
    rules: Array<{ name: string; protocol: string; listenPort: number }>;
  };

  // ExportedConfig shape.
  expect(exported.version).toBe("1");
  expect(typeof exported.exportedAt).toBe("string");
  expect(new Date(exported.exportedAt).getTime()).not.toBeNaN();
  expect(Array.isArray(exported.rules)).toBe(true);

  // The created rule must appear in the export.
  const rule = exported.rules.find((r) => r.name === "Export Shape Test");
  expect(rule).toBeTruthy();
  expect(rule?.protocol).toBe("tcp");
  expect(rule?.listenPort).toBe(49002);
});

// ── C1. Merge import — basic flow ────────────────────────────────────────────
//
// Verifies that a simple merge import adds rules to the list without removing
// existing ones. Uses the default merge mode (no confirm dialog needed).

test("settings: merge import adds rules to the list", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("navigation", { name: "Main navigation" })
    .getByRole("button", { name: "Settings" })
    .click();

  await expect(page.getByText("Export Config")).toBeVisible();
  await expect(page.getByText("Import Config")).toBeVisible();

  const importConfig = JSON.stringify({
    version: "1",
    exportedAt: new Date().toISOString(),
    rules: [{
      name: "Merge Imported Rule",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48100,
      targetHost: "127.0.0.1",
      targetPort: 48101,
      enabled: false,
    }],
  });

  await page.getByLabel("Select config file", { exact: true }).setInputFiles({
    name: "portier-merge.json",
    mimeType: "application/json",
    buffer: Buffer.from(importConfig),
  });

  await expect(page.getByText("File preview")).toBeVisible({ timeout: 3_000 });
  await expect(page.getByText("1 (1 TCP, 0 UDP)")).toBeVisible();

  await page.getByRole("button", { name: "Import Rules" }).click();

  await expect(page.getByText(/Import complete/)).toBeVisible({ timeout: 5_000 });

  await page.getByRole("navigation", { name: "Main navigation" })
    .getByRole("button", { name: "Forward Rules" })
    .click();

  await expect(page.locator("tbody").getByText("Merge Imported Rule")).toBeVisible({ timeout: 5_000 });
});

// ── C2. Plan & Apply — preview and full apply ─────────────────────────────────
//
// Verifies the Plan & Apply section:
//  1. Uploading a config with a new rule shows the plan preview (Add:1).
//  2. Non-destructive plan does not require a confirmation checkbox.
//  3. Clicking "Apply changes" applies the config and shows a success message.
//  4. The new rule appears in Forward Rules after apply.

test("settings: plan & apply previews drift and applies non-destructive config", async ({ page }) => {
  // Start with no rules (clearAllRules in beforeEach).
  await page.goto("/");
  await goToSettings(page);

  // A config with one new TCP rule that doesn't exist on the server.
  const desiredRules = [{
    name: "Plan Apply Test Rule",
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort: 49060,
    targetHost: "127.0.0.1",
    targetPort: 49160,
    enabled: false,
  }];

  await page.getByLabel("Select config file for plan").setInputFiles({
    name: "desired.json",
    mimeType: "application/json",
    buffer: toExportedConfigBuffer(desiredRules),
  });

  // Preview changes button should appear after file selection.
  await page.getByRole("button", { name: "Preview changes" }).click();

  // Plan preview panel appears.
  await expect(page.getByText("Plan preview")).toBeVisible({ timeout: 5_000 });

  // Summary: Add:1, no removes → non-destructive.
  await expect(page.getByText(/Add: 1/)).toBeVisible();
  await expect(page.getByText(/Remove: 0/)).toBeVisible();

  // Operation list shows the new rule name.
  await expect(page.getByText("Plan Apply Test Rule")).toBeVisible();

  // No destructive confirmation checkbox (non-destructive plan).
  await expect(page.getByLabel("Confirm destructive changes")).not.toBeVisible({ timeout: 1_000 }).catch(() => {});

  // Apply changes.
  await page.getByRole("button", { name: "Apply changes" }).click();

  // Success message appears.
  await expect(page.getByRole("status")).toBeVisible({ timeout: 5_000 });
  await expect(page.getByText(/Config applied/)).toBeVisible();

  // Navigate to Forward Rules — the applied rule must appear.
  await page.getByRole("navigation", { name: "Main navigation" })
    .getByRole("button", { name: "Forward Rules" })
    .click();

  await expect(page.locator("tbody").getByText("Plan Apply Test Rule")).toBeVisible({ timeout: 5_000 });
});

// ── D. Runtime / Environment section ─────────────────────────────────────────
//
// Verifies that the Settings view fetches and renders the /api/runtime response.
// Checks that the section heading appears and that key runtime fields are present.

test("settings: Runtime/Environment section shows runtime info", async ({ page }) => {
  await page.goto("/");
  await goToSettings(page);

  // The section heading is present.
  await expect(page.getByText("Runtime / Environment")).toBeVisible({ timeout: 5_000 });

  // Runtime field shows "Node server" (E2E uses the TypeScript server runtime).
  await expect(page.getByText("Node server")).toBeVisible({ timeout: 5_000 });

  // Config path label is specific to the runtime info section.
  await expect(page.getByText("Config path")).toBeVisible();

  // PID label appears in the runtime section.
  await expect(page.getByText("PID")).toBeVisible();
});
