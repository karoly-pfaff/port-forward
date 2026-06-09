import { test, expect } from "@playwright/test";
import { clearAllRules, createRule } from "./helpers/api.js";
import { getFreePort } from "./helpers/port.js";

// Reset all rules before each test for isolation.
test.beforeEach(async ({ baseURL }) => {
  await clearAllRules(baseURL!);
});

// Navigate to Connections view and wait for the title.
async function goToConnections(page: Parameters<Parameters<typeof test>[1]>[0]): Promise<void> {
  await page.getByRole("navigation", { name: "Main navigation" })
    .getByRole("button", { name: "Connections" })
    .click();
  await expect(page.getByText("Live Connections")).toBeVisible({ timeout: 5_000 });
}

// ── A. Page loads with title and tabs ─────────────────────────────────────────

test("connections: page loads with Live Connections title and all tabs", async ({ page }) => {
  await page.goto("/");
  await goToConnections(page);

  await expect(page.getByRole("tablist", { name: "Connection views" })).toBeVisible();
  await expect(page.getByRole("tab", { name: /^TCP/ })).toBeVisible();
  await expect(page.getByRole("tab", { name: /^UDP/ })).toBeVisible();
  await expect(page.getByRole("tab", { name: /^Rule Summary/ })).toBeVisible();
});

// ── B. TCP tab empty state (default active tab) ────────────────────────────────

test("connections: TCP tab is active by default and shows empty state", async ({ page }) => {
  await page.goto("/");
  await goToConnections(page);

  await expect(page.getByRole("tab", { name: /^TCP/ })).toHaveAttribute("aria-selected", "true");
  await expect(page.getByText("No active TCP connections.")).toBeVisible({ timeout: 5_000 });
});

// ── C. Tab switching ──────────────────────────────────────────────────────────

test("connections: tabs switch between TCP, UDP, and Rule Summary", async ({ page }) => {
  await page.goto("/");
  await goToConnections(page);

  // Switch to UDP tab
  await page.getByRole("tab", { name: /^UDP/ }).click();
  await expect(page.getByText("No active or recent UDP sessions.")).toBeVisible({ timeout: 3_000 });

  // Switch to Summary tab
  await page.getByRole("tab", { name: /^Rule Summary/ }).click();
  await expect(page.getByText("No rule summaries available.")).toBeVisible({ timeout: 3_000 });

  // Switch back to TCP tab
  await page.getByRole("tab", { name: /^TCP/ }).click();
  await expect(page.getByText("No active TCP connections.")).toBeVisible({ timeout: 3_000 });
});

// ── D. Summary stats bar ──────────────────────────────────────────────────────

test("connections: summary stats bar renders with four counters", async ({ page }) => {
  await page.goto("/");
  await goToConnections(page);

  const summary = page.getByLabel("Live connections summary");
  await expect(summary).toBeVisible({ timeout: 5_000 });
  await expect(summary.getByText("TCP Connections")).toBeVisible();
  await expect(summary.getByText("UDP Sessions")).toBeVisible();
  await expect(summary.getByText("Active Rules")).toBeVisible();
  await expect(summary.getByText("Total Traffic")).toBeVisible();
});

// ── E. Protocol filter set and cleared ───────────────────────────────────────

test("connections: protocol filter can be set and cleared", async ({ page }) => {
  await page.goto("/");
  await goToConnections(page);

  // Filters render
  await expect(page.getByLabel("Filter by protocol")).toBeVisible({ timeout: 5_000 });
  await expect(page.getByLabel("Filter by status")).toBeVisible();

  // Setting a filter shows the Clear all filters button
  await page.getByLabel("Filter by protocol").selectOption("tcp");
  await expect(page.getByRole("button", { name: "Clear all filters" })).toBeVisible({ timeout: 2_000 });

  // Clearing removes the button
  await page.getByRole("button", { name: "Clear all filters" }).click();
  await expect(page.getByRole("button", { name: "Clear all filters" })).not.toBeVisible({ timeout: 2_000 });
});

// ── F. Auto-refresh toggle ────────────────────────────────────────────────────

test("connections: auto-refresh is enabled by default and can be toggled off and on", async ({ page }) => {
  await page.goto("/");
  await goToConnections(page);

  // The native checkbox is visually hidden by the custom toggle styling.
  // Click the label to toggle the state and read the checked property directly.
  const toggleLabel = page.locator("label.auto-refresh-toggle");
  await expect(toggleLabel).toBeVisible({ timeout: 5_000 });
  const toggle = page.locator('input[type="checkbox"][aria-label="Auto-refresh"]');
  await expect(toggle).toBeChecked();

  await toggleLabel.click();
  await expect(toggle).not.toBeChecked();

  await toggleLabel.click();
  await expect(toggle).toBeChecked();
});

// ── G. Footer item count per tab ──────────────────────────────────────────────

test("connections: footer shows correct plural count for each active tab", async ({ page }) => {
  await page.goto("/");
  await goToConnections(page);

  // TCP tab: "0 connections"
  await expect(page.getByText("0 connections")).toBeVisible({ timeout: 5_000 });

  // UDP tab: "0 sessions"
  await page.getByRole("tab", { name: /^UDP/ }).click();
  await expect(page.getByText("0 sessions")).toBeVisible({ timeout: 3_000 });

  // Summary tab: "0 rules"
  await page.getByRole("tab", { name: /^Rule Summary/ }).click();
  await expect(page.getByText("0 rules")).toBeVisible({ timeout: 3_000 });
});

// ── H. Rule filter populated when a rule has been running ─────────────────────
//
// Starts a TCP rule so that /api/connections can return a non-empty ruleSummaries
// array, which causes the rule-filter dropdown to appear in the view.

test("connections: started rule appears in rule filter dropdown after traffic", async ({ page, baseURL }) => {
  const listenPort = await getFreePort();
  await createRule(baseURL!, {
    name: "Connections Filter Rule",
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort,
    targetHost: "127.0.0.1",
    targetPort: 9999,
    enabled: false,
  });

  await page.goto("/");

  // Start the rule via the Forward Rules UI so connections can be tracked.
  const ruleRow = page.locator("tr", { hasText: "Connections Filter Rule" });
  await expect(ruleRow).toBeVisible();
  await ruleRow.getByRole("button", { name: "Start" }).click();
  await expect(ruleRow.getByText("Running")).toBeVisible({ timeout: 10_000 });

  await goToConnections(page);

  // Navigate to the Summary tab — the started rule should appear in the summary
  // (even with no active connections) and the rule-filter dropdown should be present.
  await page.getByRole("tab", { name: /^Rule Summary/ }).click();

  // The rule filter dropdown is rendered only when ruleSummaries is non-empty.
  // Retry-assert with a longer timeout to allow the auto-refresh to fetch.
  await expect(page.getByLabel("Filter by rule")).toBeVisible({ timeout: 8_000 });

  // The rule name appears as an option in the dropdown.
  // <option> elements are always hidden in a closed <select>; use toHaveCount instead.
  const ruleSelect = page.getByLabel("Filter by rule");
  await expect(ruleSelect.locator("option").filter({ hasText: "Connections Filter Rule" })).toHaveCount(1);
});
