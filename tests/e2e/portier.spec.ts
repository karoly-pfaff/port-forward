import { test, expect, type Page } from "@playwright/test";
import { clearAllRules, createRule } from "./helpers/api.js";
import { getFreePort } from "./helpers/port.js";

// Reset all rules before each test for isolation.
test.beforeEach(async ({ baseURL }) => {
  await clearAllRules(baseURL!);
});

// ── App loads ──────────────────────────────────────────────────────────────

test("app loads: main shell and Forward Rules view are visible", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { name: "TCP/UDP port forwarding for local development" })).toBeVisible();
  await expect(page.getByText("Portier").first()).toBeVisible();

  const nav = page.getByRole("navigation", { name: "Main navigation" });
  await expect(nav).toBeVisible();
  await expect(nav.getByRole("button", { name: "Forward Rules" })).toBeVisible();

  await expect(page.getByRole("button", { name: "+ Add Rule" })).toBeVisible();
  await expect(page.getByText("No forwarding rules yet")).toBeVisible();
});

// ── Add rule flow ──────────────────────────────────────────────────────────

test("add rule: drawer opens, form saved, rule appears in list", async ({ page, baseURL }) => {
  const listenPort = await getFreePort();
  const targetPort = await getFreePort();
  const ruleName = "E2E Add Test";

  await page.goto("/");
  await page.getByRole("button", { name: "+ Add Rule" }).click();

  const drawer = page.getByRole("complementary", { name: "Add Forward Rule" });
  await expect(drawer).toBeVisible();
  await expect(drawer.getByRole("heading", { name: "Add Rule" })).toBeVisible();

  await drawer.getByLabel("Name").fill(ruleName);
  await drawer.getByLabel("Protocol").selectOption("tcp");
  await drawer.getByRole("textbox", { name: "Listen Host" }).fill("127.0.0.1");
  await drawer.getByLabel("Listen Port").fill(String(listenPort));
  await drawer.getByLabel("Target Host").fill("127.0.0.1");
  await drawer.getByLabel("Target Port").fill(String(targetPort));
  await drawer.getByRole("button", { name: "Add Rule", exact: true }).click();

  await expect(drawer).not.toBeVisible({ timeout: 5_000 });
  await expect(page.locator("tbody").getByText(ruleName)).toBeVisible({ timeout: 5_000 });

  const resp = await fetch(`${baseURL}/api/forwards`);
  const rules = (await resp.json()) as Array<{ name: string }>;
  expect(rules.some((r) => r.name === ruleName)).toBe(true);
});

// ── Edit rule flow ─────────────────────────────────────────────────────────

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

  const ruleRow = page.locator("tr", { hasText: "E2E Edit Original" });
  await ruleRow.getByRole("button", { name: "Edit" }).click();

  const drawer = page.getByRole("complementary", { name: "Edit Forward Rule" });
  await expect(drawer).toBeVisible();
  await expect(drawer.getByRole("heading", { name: "Edit Rule" })).toBeVisible();
  await expect(drawer.getByLabel("Name")).toHaveValue("E2E Edit Original");

  await drawer.getByLabel("Name").fill("E2E Edit Updated");
  await drawer.getByRole("button", { name: "Save Changes" }).click();
  await expect(drawer).not.toBeVisible({ timeout: 5_000 });

  await expect(page.locator("tbody").getByText("E2E Edit Updated")).toBeVisible({ timeout: 5_000 });
  await expect(page.locator("tbody").getByText("E2E Edit Original")).not.toBeVisible();
});

// ── Start/Stop flow ────────────────────────────────────────────────────────

test("start/stop: rule transitions between Running and Stopped", async ({ page, baseURL }) => {
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
  await expect(ruleRow.getByText("Stopped")).toBeVisible();

  await ruleRow.getByRole("button", { name: "Start" }).click();
  await expect(ruleRow.getByText("Running")).toBeVisible({ timeout: 10_000 });
  await expect(ruleRow.getByRole("button", { name: "Stop" })).toBeVisible();

  await ruleRow.getByRole("button", { name: "Stop" }).click();
  await expect(ruleRow.getByText("Stopped")).toBeVisible({ timeout: 10_000 });
  await expect(ruleRow.getByRole("button", { name: "Start" })).toBeVisible();
});

// ── Delete flow ────────────────────────────────────────────────────────────

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

  await ruleRow.getByRole("button", { name: "Delete" }).click();
  await expect(ruleRow.getByText(/Delete.*E2E Delete Me/)).toBeVisible();
  await expect(ruleRow.getByRole("button", { name: "Confirm" })).toBeVisible();
  await expect(ruleRow.getByRole("button", { name: "Cancel" })).toBeVisible();

  await ruleRow.getByRole("button", { name: "Confirm" }).click();
  await expect(page.locator("tbody").getByText("E2E Delete Me")).not.toBeVisible({ timeout: 5_000 });
  await expect(page.getByText("No forwarding rules yet")).toBeVisible();
});

// ── Rule form validation ───────────────────────────────────────────────────

test("rule form: submitting without a name shows Name is required error", async ({ page }) => {
  const listenPort = await getFreePort();
  const targetPort = await getFreePort();

  await page.goto("/");
  await page.getByRole("button", { name: "+ Add Rule" }).click();

  const drawer = page.getByRole("complementary", { name: "Add Forward Rule" });
  await expect(drawer).toBeVisible();

  await drawer.getByRole("textbox", { name: "Listen Host" }).fill("127.0.0.1");
  await drawer.getByLabel("Listen Port").fill(String(listenPort));
  await drawer.getByLabel("Target Host").fill("127.0.0.1");
  await drawer.getByLabel("Target Port").fill(String(targetPort));
  await drawer.getByRole("button", { name: "Add Rule", exact: true }).click();

  await expect(drawer.getByText("Name is required")).toBeVisible({ timeout: 3_000 });
  await expect(drawer).toBeVisible();
});

// ── Diagnose rule ──────────────────────────────────────────────────────────

test("diagnose: clicking Diagnose on a rule opens the diagnostics panel with results", async ({ page, baseURL }) => {
  const listenPort = await getFreePort();
  await createRule(baseURL!, {
    name: "E2E Diagnose Test",
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort,
    targetHost: "127.0.0.1",
    targetPort: 9999,
  });

  await page.goto("/");
  const ruleRow = page.locator("tr", { hasText: "E2E Diagnose Test" });
  await expect(ruleRow).toBeVisible();

  await ruleRow.getByRole("button", { name: "Diagnose" }).click();

  await expect(page.locator(".diag-panel-title")).toBeVisible({ timeout: 5_000 });
  await expect(page.getByText("Running diagnostics…")).not.toBeVisible({ timeout: 10_000 });
  await expect(page.locator(".diag-panel-body")).toBeVisible({ timeout: 5_000 });
  await expect(page.getByRole("button", { name: "Close diagnostics" })).toBeVisible();
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
  await drawer.getByRole("textbox", { name: "Listen Host" }).fill("127.0.0.1");
  await drawer.getByLabel("Listen Port").fill(String(rule.listenPort));
  await drawer.getByLabel("Target Host").fill("127.0.0.1");
  await drawer.getByLabel("Target Port").fill(String(rule.targetPort));
  await drawer.getByRole("button", { name: "Add Rule", exact: true }).click();
  await expect(drawer).not.toBeVisible({ timeout: 5_000 });
  await expect(page.locator("tbody").getByText(rule.name)).toBeVisible({ timeout: 5_000 });
}

export { addRuleViaUI };
