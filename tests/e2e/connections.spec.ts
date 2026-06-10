import { connect } from "node:net";
import { test, expect, type Page } from "@playwright/test";
import { clearAllRules, createRule, startRule } from "./helpers/api.js";
import { getFreePort } from "./helpers/port.js";
import { startTcpEchoServer, closeTcpServer } from "./helpers/network.js";

// Reset all rules before each test for isolation.
test.beforeEach(async ({ baseURL }) => {
  await clearAllRules(baseURL!);
});

// Navigate to Connections view and wait for the title.
async function goToConnections(page: Page): Promise<void> {
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

  // State is read via the accessible aria-label (not a CSS-class node); the
  // visible "Auto-refresh" label is the user-facing control to click. The native
  // checkbox is visually hidden by the custom toggle styling, so it is toggled by
  // clicking its label rather than the input directly.
  const toggle = page.locator('input[aria-label="Auto-refresh"]');
  const toggleControl = page.getByText("Auto-refresh", { exact: true });
  await expect(toggleControl).toBeVisible({ timeout: 5_000 });
  await expect(toggle).toBeChecked();

  await toggleControl.click();
  await expect(toggle).not.toBeChecked();

  await toggleControl.click();
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

// ── I. Populated TCP table with a live connection ─────────────────────────────
//
// Proves the primary value of the view: a real forwarded TCP connection renders
// as a populated row (rule name, endpoints, Active status), and the empty state
// disappears. Uses the real E2E runtime — a TCP rule forwarding to an in-process
// echo server, with a client socket held open across the page fetch.

test("connections: a live TCP connection appears as a populated table row", async ({ page, baseURL }) => {
  const targetPort = await getFreePort();
  const echo = await startTcpEchoServer(targetPort);
  const listenPort = await getFreePort();

  const { id } = await createRule(baseURL!, {
    name: "Live TCP Inspect",
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort,
    targetHost: "127.0.0.1",
    targetPort,
    enabled: false,
  });
  await startRule(baseURL!, id);

  // Open and hold a TCP connection through the forwarded listen port so the
  // service tracks it while the browser polls /api/connections.
  const held = connect(listenPort, "127.0.0.1");
  await new Promise<void>((resolve, reject) => {
    held.once("connect", () => resolve());
    held.once("error", reject);
  });

  try {
    await page.goto("/");
    await goToConnections(page);

    // TCP tab is active by default; auto-refresh (2 s) fetches the live data.
    const table = page.getByRole("table", { name: "TCP connections" });
    await expect(table).toBeVisible({ timeout: 12_000 });

    const row = table.locator("tbody tr").filter({ hasText: "Live TCP Inspect" });
    await expect(row).toBeVisible({ timeout: 12_000 });
    await expect(row.getByText("Active")).toBeVisible();
    await expect(row.getByText(/127\.0\.0\.1/).first()).toBeVisible();

    // Empty state is gone, and the summary/footer reflect the connection.
    await expect(page.getByText("No active TCP connections.")).toHaveCount(0);
    await expect(page.getByRole("tab", { name: /^TCP \(/ })).toContainText(/TCP \([1-9]/);
  } finally {
    held.destroy();
    await closeTcpServer(echo);
  }
});

// ── J. API failure shows a user-visible error banner ──────────────────────────
//
// Intercepts the connections API at the browser layer and forces it to fail,
// proving the view surfaces a role="alert" error banner (not a crash or a stuck
// spinner). Uses Playwright route interception — the rest of the app loads from
// the real server.

test("connections: shows an error banner when the connections API fails", async ({ page }) => {
  await page.route("**/api/connections*", (route) => route.abort());

  await page.goto("/");
  await goToConnections(page);

  const alert = page.getByRole("alert");
  await expect(alert).toBeVisible({ timeout: 10_000 });
  await expect(alert).not.toBeEmpty();

  // The app does not crash: navigation and the view tabs remain usable.
  await expect(page.getByRole("tab", { name: /^UDP/ })).toBeVisible();
});
