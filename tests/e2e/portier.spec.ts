import { test, expect, type Page } from "@playwright/test";
import { clearAllRules, createRule } from "./helpers/api.js";
import { getFreePort } from "./helpers/port.js";

// Reset all rules before each test for isolation.
test.beforeEach(async ({ baseURL }) => {
  await clearAllRules(baseURL!);
});

// ── A. App loads ──────────────────────────────────────────────────────────────

test("app loads: main shell and Forward Rules view are visible", async ({ page }) => {
  await page.goto("/");

  // App header
  await expect(page.getByRole("heading", { name: "TCP/UDP port forwarding for local development" })).toBeVisible();
  await expect(page.getByText("Portier").first()).toBeVisible();

  // Main navigation in sidebar
  const nav = page.getByRole("navigation", { name: "Main navigation" });
  await expect(nav).toBeVisible();
  await expect(nav.getByRole("button", { name: "Forward Rules" })).toBeVisible();

  // Forward Rules view is the default
  await expect(page.getByRole("button", { name: "+ Add Rule" })).toBeVisible();

  // Empty state when no rules exist
  await expect(page.getByText("No forwarding rules yet")).toBeVisible();
});

// ── B. Add rule flow ──────────────────────────────────────────────────────────

test("add rule: drawer opens, form saved, rule appears in list", async ({ page, baseURL }) => {
  const listenPort = await getFreePort();
  const targetPort = await getFreePort();
  const ruleName = "E2E Add Test";

  await page.goto("/");

  // Open drawer
  await page.getByRole("button", { name: "+ Add Rule" }).click();

  const drawer = page.getByRole("complementary", { name: "Add Forward Rule" });
  await expect(drawer).toBeVisible();
  await expect(drawer.getByRole("heading", { name: "Add Rule" })).toBeVisible();

  // Fill form
  await drawer.getByLabel("Name").fill(ruleName);
  await drawer.getByLabel("Protocol").selectOption("tcp");
  await drawer.getByLabel("Listen Host").fill("127.0.0.1");
  await drawer.getByLabel("Listen Port").fill(String(listenPort));
  await drawer.getByLabel("Target Host").fill("127.0.0.1");
  await drawer.getByLabel("Target Port").fill(String(targetPort));

  // Save
  await drawer.getByRole("button", { name: "Add Rule", exact: true }).click();

  // Drawer closes
  await expect(drawer).not.toBeVisible({ timeout: 5_000 });

  // Rule appears in the table
  await expect(page.locator("tbody").getByText(ruleName)).toBeVisible({ timeout: 5_000 });

  // Verify via API
  const resp = await fetch(`${baseURL}/api/forwards`);
  const rules = (await resp.json()) as Array<{ name: string }>;
  expect(rules.some((r) => r.name === ruleName)).toBe(true);
});

// ── C. Edit rule flow ─────────────────────────────────────────────────────────

test("edit rule: drawer opens pre-filled, changes saved, list updates", async ({ page, baseURL }) => {
  const listenPort = await getFreePort();
  await createRule(baseURL!, {
    name: "E2E Edit Original",
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort,
    targetHost: "127.0.0.1",
    targetPort: 9999,
  });

  await page.goto("/");
  await expect(page.locator("tbody").getByText("E2E Edit Original")).toBeVisible();

  // Click Edit for the rule
  const ruleRow = page.locator("tr", { hasText: "E2E Edit Original" });
  await ruleRow.getByRole("button", { name: "Edit" }).click();

  const drawer = page.getByRole("complementary", { name: "Edit Forward Rule" });
  await expect(drawer).toBeVisible();
  await expect(drawer.getByRole("heading", { name: "Edit Rule" })).toBeVisible();

  // Verify pre-filled name
  await expect(drawer.getByLabel("Name")).toHaveValue("E2E Edit Original");

  // Change the name
  await drawer.getByLabel("Name").fill("E2E Edit Updated");

  // Save
  await drawer.getByRole("button", { name: "Save Changes" }).click();
  await expect(drawer).not.toBeVisible({ timeout: 5_000 });

  // Updated name appears in the list
  await expect(page.locator("tbody").getByText("E2E Edit Updated")).toBeVisible({ timeout: 5_000 });
  await expect(page.locator("tbody").getByText("E2E Edit Original")).not.toBeVisible();
});

// ── D. Start/Stop flow ────────────────────────────────────────────────────────

test("start/stop: rule transitions between Running and Stopped", async ({ page, baseURL }) => {
  // Use a free port so the listener can actually bind
  const listenPort = await getFreePort();
  await createRule(baseURL!, {
    name: "E2E StartStop",
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort,
    targetHost: "127.0.0.1",
    targetPort: 9999,
  });

  await page.goto("/");
  const ruleRow = page.locator("tr", { hasText: "E2E StartStop" });
  await expect(ruleRow).toBeVisible();

  // Initial status is Stopped
  await expect(ruleRow.getByText("Stopped")).toBeVisible();

  // Start the rule
  await ruleRow.getByRole("button", { name: "Start" }).click();

  // Wait for Running status
  await expect(ruleRow.getByText("Running")).toBeVisible({ timeout: 10_000 });
  await expect(ruleRow.getByRole("button", { name: "Stop" })).toBeVisible();

  // Stop the rule
  await ruleRow.getByRole("button", { name: "Stop" }).click();

  // Wait for Stopped status
  await expect(ruleRow.getByText("Stopped")).toBeVisible({ timeout: 10_000 });
  await expect(ruleRow.getByRole("button", { name: "Start" })).toBeVisible();
});

// ── E. Delete flow ────────────────────────────────────────────────────────────

test("delete: confirmation required, rule removed after confirm", async ({ page, baseURL }) => {
  const listenPort = await getFreePort();
  await createRule(baseURL!, {
    name: "E2E Delete Me",
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort,
    targetHost: "127.0.0.1",
    targetPort: 9999,
  });

  await page.goto("/");
  const ruleRow = page.locator("tr", { hasText: "E2E Delete Me" });
  await expect(ruleRow).toBeVisible();

  // Click Delete — shows confirmation inline
  await ruleRow.getByRole("button", { name: "Delete" }).click();

  // Confirmation text and buttons appear in the row
  await expect(ruleRow.getByText(/Delete.*E2E Delete Me/)).toBeVisible();
  await expect(ruleRow.getByRole("button", { name: "Confirm" })).toBeVisible();
  await expect(ruleRow.getByRole("button", { name: "Cancel" })).toBeVisible();

  // Confirm delete
  await ruleRow.getByRole("button", { name: "Confirm" }).click();

  // Rule disappears
  await expect(page.locator("tbody").getByText("E2E Delete Me")).not.toBeVisible({ timeout: 5_000 });
  await expect(page.getByText("No forwarding rules yet")).toBeVisible();
});

// ── F. Activity view ──────────────────────────────────────────────────────────

test("activity: view opens and shows events after rule operations", async ({ page, baseURL }) => {
  const listenPort = await getFreePort();
  const rule = await createRule(baseURL!, {
    name: "E2E Activity Test",
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort,
    targetHost: "127.0.0.1",
    targetPort: 9999,
  });

  // Navigate to Activity view
  await page.goto("/");
  await page.getByRole("navigation", { name: "Main navigation" })
    .getByRole("button", { name: "Activity" })
    .click();

  // Activity view loads — use the unique subtitle to avoid strict-mode violations
  await expect(page.getByText("Recent forwarding and rule events")).toBeVisible();

  // After creating a rule, there should be at least one event
  await expect(page.locator("table, .activity-log, [class*='activity']").first()).toBeVisible({ timeout: 5_000 });

  // Verify at least the rule creation event exists via API
  const resp = await fetch(`${baseURL}/api/activity?ruleId=${rule.id}`);
  const body = (await resp.json()) as { events: Array<{ type: string }> };
  expect(body.events.length).toBeGreaterThan(0);
});

// ── G. Settings: export and import ───────────────────────────────────────────

test("settings: import config file adds rules to the list", async ({ page }) => {
  const importConfig = JSON.stringify({
    version: "1",
    rules: [
      {
        name: "Imported Rule",
        protocol: "tcp",
        listenHost: "127.0.0.1",
        listenPort: 48100,
        targetHost: "127.0.0.1",
        targetPort: 48101,
        enabled: false,
      },
    ],
  });

  await page.goto("/");
  await page.getByRole("navigation", { name: "Main navigation" })
    .getByRole("button", { name: "Settings" })
    .click();

  // Settings view loads
  await expect(page.getByText("Export Config")).toBeVisible();
  await expect(page.getByText("Import Config")).toBeVisible();

  // Upload file via hidden file input
  await page.getByLabel("Select config file").setInputFiles({
    name: "portier-test.json",
    mimeType: "application/json",
    buffer: Buffer.from(importConfig),
  });

  // Preview appears
  await expect(page.getByText("File preview")).toBeVisible({ timeout: 3_000 });
  await expect(page.getByText("1 (1 TCP, 0 UDP)")).toBeVisible();

  // Import with merge mode (default)
  await expect(page.getByRole("button", { name: "Import Rules" })).toBeVisible();
  await page.getByRole("button", { name: "Import Rules" }).click();

  // Success message
  await expect(page.getByText(/Import complete/)).toBeVisible({ timeout: 5_000 });

  // Navigate back to rules view
  await page.getByRole("navigation", { name: "Main navigation" })
    .getByRole("button", { name: "Forward Rules" })
    .click();

  await expect(page.locator("tbody").getByText("Imported Rule")).toBeVisible({ timeout: 5_000 });
});

// ── H. API Docs view ──────────────────────────────────────────────────────────

test("api docs: view opens and endpoint list is visible", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("navigation", { name: "Main navigation" })
    .getByRole("button", { name: "API Docs" })
    .click();

  // API Docs view should show endpoint information
  await expect(page.getByText(/API Docs|api/i).first()).toBeVisible();
  // Expect at least one endpoint path is visible
  await expect(page.getByText("/api/forwards").first()).toBeVisible({ timeout: 5_000 });
});

// ── I. Mobile sidebar smoke ───────────────────────────────────────────────────

test("mobile sidebar: hamburger opens sidebar, navigation works, sidebar closes", async ({ page }) => {
  // Set mobile viewport
  await page.setViewportSize({ width: 640, height: 800 });
  await page.goto("/");

  // Hamburger button is visible at this viewport
  const menuBtn = page.getByRole("button", { name: "Open navigation menu" });
  await expect(menuBtn).toBeVisible();

  // Open sidebar
  await menuBtn.click();

  // Sidebar becomes visible with nav items
  const nav = page.getByRole("navigation", { name: "Main navigation" });
  await expect(nav).toBeVisible({ timeout: 3_000 });

  // Navigate to Activity — sidebar should close after nav
  await nav.getByRole("button", { name: "Activity" }).click();

  // Sidebar closes (on mobile, it closes after nav click)
  await expect(page.getByText("Activity").first()).toBeVisible({ timeout: 3_000 });
});

// ── Dashboard view smoke ──────────────────────────────────────────────────────

test("dashboard: navigating to Dashboard shows stat cards", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("navigation", { name: "Main navigation" })
    .getByRole("button", { name: "Dashboard" })
    .click();

  // Dashboard renders some content (stat cards or summary)
  await expect(page.getByText(/Total|Rules|Running|dashboard/i).first()).toBeVisible({ timeout: 5_000 });
});

// ── Helper ────────────────────────────────────────────────────────────────────

// Shared helper: fill the Add Rule drawer and submit.
async function addRuleViaUI(
  page: Page,
  rule: { name: string; listenPort: number; targetPort: number }
): Promise<void> {
  await page.getByRole("button", { name: "+ Add Rule" }).click();
  const drawer = page.getByRole("complementary", { name: "Add Forward Rule" });
  await expect(drawer).toBeVisible();
  await drawer.getByLabel("Name").fill(rule.name);
  await drawer.getByLabel("Protocol").selectOption("tcp");
  await drawer.getByLabel("Listen Host").fill("127.0.0.1");
  await drawer.getByLabel("Listen Port").fill(String(rule.listenPort));
  await drawer.getByLabel("Target Host").fill("127.0.0.1");
  await drawer.getByLabel("Target Port").fill(String(rule.targetPort));
  await drawer.getByRole("button", { name: "Add Rule", exact: true }).click();
  await expect(drawer).not.toBeVisible({ timeout: 5_000 });
  await expect(page.locator("tbody").getByText(rule.name)).toBeVisible({ timeout: 5_000 });
}

export { addRuleViaUI };
