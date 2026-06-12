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
  await ruleRow.getByRole("button", { name: "Edit", exact: true }).click();

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

// ── Rule group metadata (v1.8) ───────────────────────────────────────────────

test("rule group: set on create, shown in list, cleared on edit", async ({ page, baseURL }) => {
  const listenPort = await getFreePort();
  const targetPort = await getFreePort();
  const ruleName = "E2E Group Test";

  // Create a rule with a group via the UI.
  await page.goto("/");
  await page.getByRole("button", { name: "+ Add Rule" }).click();

  const addDrawer = page.getByRole("complementary", { name: "Add Forward Rule" });
  await expect(addDrawer).toBeVisible();
  await addDrawer.getByLabel("Name").fill(ruleName);
  await addDrawer.getByLabel("Group").fill("web-team");
  await addDrawer.getByRole("textbox", { name: "Listen Host" }).fill("127.0.0.1");
  await addDrawer.getByLabel("Listen Port").fill(String(listenPort));
  await addDrawer.getByLabel("Target Host").fill("127.0.0.1");
  await addDrawer.getByLabel("Target Port").fill(String(targetPort));
  await addDrawer.getByRole("button", { name: "Add Rule", exact: true }).click();
  await expect(addDrawer).not.toBeVisible({ timeout: 5_000 });

  // Group label is shown in the list, and persisted via the API.
  const ruleRow = page.locator("tr", { hasText: ruleName });
  await expect(ruleRow.getByText("web-team")).toBeVisible({ timeout: 5_000 });

  const created = (await (await fetch(`${baseURL}/api/forwards`)).json()) as Array<{ name: string; group?: string }>;
  expect(created.find((r) => r.name === ruleName)?.group).toBe("web-team");

  // Edit the rule and clear the group.
  await ruleRow.getByRole("button", { name: "Edit" }).click();
  const editDrawer = page.getByRole("complementary", { name: "Edit Forward Rule" });
  await expect(editDrawer).toBeVisible();
  await expect(editDrawer.getByLabel("Group")).toHaveValue("web-team");
  await editDrawer.getByLabel("Group").fill("");
  await editDrawer.getByRole("button", { name: "Save Changes" }).click();
  await expect(editDrawer).not.toBeVisible({ timeout: 5_000 });

  // Group label is gone from the list, and the API reports no group.
  await expect(page.locator("tr", { hasText: ruleName }).getByText("web-team")).not.toBeVisible();

  const cleared = (await (await fetch(`${baseURL}/api/forwards`)).json()) as Array<{ name: string; group?: string }>;
  expect(cleared.find((r) => r.name === ruleName)?.group).toBeUndefined();
});

test("rule group filter: filters the list by group and ungrouped", async ({ page, baseURL }) => {
  const base = { protocol: "tcp" as const, listenHost: "127.0.0.1", targetHost: "127.0.0.1", targetPort: 9999 };
  await createRule(baseURL!, { ...base, name: "Web One", listenPort: await getFreePort(), group: "web" });
  await createRule(baseURL!, { ...base, name: "Api One", listenPort: await getFreePort(), group: "api" });
  await createRule(baseURL!, { ...base, name: "Loose One", listenPort: await getFreePort() });

  await page.goto("/");
  const tbody = page.locator("tbody");
  await expect(tbody.getByText("Web One")).toBeVisible();

  const groupFilter = page.getByLabel("Filter by group");
  await expect(groupFilter).toBeVisible();

  // Filter to the "web" group.
  await groupFilter.selectOption("web");
  await expect(tbody.getByText("Web One")).toBeVisible();
  await expect(tbody.getByText("Api One")).not.toBeVisible();
  await expect(tbody.getByText("Loose One")).not.toBeVisible();

  // Filter to Ungrouped.
  await groupFilter.selectOption({ label: "Ungrouped" });
  await expect(tbody.getByText("Loose One")).toBeVisible();
  await expect(tbody.getByText("Web One")).not.toBeVisible();

  // Back to All Groups restores everything.
  await groupFilter.selectOption({ label: "All Groups" });
  await expect(tbody.getByText("Web One")).toBeVisible();
  await expect(tbody.getByText("Api One")).toBeVisible();
  await expect(tbody.getByText("Loose One")).toBeVisible();
});

test("rule group actions: start and stop every rule in a selected group", async ({ page, baseURL }) => {
  const base = { protocol: "tcp" as const, listenHost: "127.0.0.1", targetHost: "127.0.0.1", targetPort: 9999 };
  await createRule(baseURL!, { ...base, name: "Ops One", listenPort: await getFreePort(), group: "ops" });
  await createRule(baseURL!, { ...base, name: "Ops Two", listenPort: await getFreePort(), group: "ops" });

  await page.goto("/");
  const opsOne = page.locator("tr", { hasText: "Ops One" });
  const opsTwo = page.locator("tr", { hasText: "Ops Two" });
  await expect(opsOne.getByText("Stopped")).toBeVisible();

  // No group action buttons until a concrete group is filtered.
  await expect(page.getByRole("button", { name: /^Start group/ })).toHaveCount(0);

  await page.getByLabel("Filter by group").selectOption("ops");
  const startBtn = page.getByRole("button", { name: 'Start group "ops"' });
  await expect(startBtn).toBeVisible();

  await startBtn.click();
  // Result summary appears and both rules transition to Running.
  await expect(page.getByRole("status")).toContainText('Started group "ops"');
  await expect(opsOne.getByText("Running")).toBeVisible({ timeout: 10_000 });
  await expect(opsTwo.getByText("Running")).toBeVisible({ timeout: 10_000 });

  await page.getByRole("button", { name: 'Stop group "ops"' }).click();
  await expect(page.getByRole("status")).toContainText('Stopped group "ops"');
  await expect(opsOne.getByText("Stopped")).toBeVisible({ timeout: 10_000 });
  await expect(opsTwo.getByText("Stopped")).toBeVisible({ timeout: 10_000 });
});

test("rule duplicate: copies a rule into a new editable rule, source unchanged", async ({ page, baseURL }) => {
  const base = { protocol: "tcp" as const, listenHost: "127.0.0.1", targetHost: "127.0.0.1", targetPort: 9999 };
  const sourceName = "Dup Source";
  const sourcePort = await getFreePort();
  const dupPort = await getFreePort();
  await createRule(baseURL!, { ...base, name: sourceName, listenPort: sourcePort, group: "dupes" });

  await page.goto("/");
  const sourceRow = page.locator("tr", { hasText: sourceName });
  await expect(sourceRow).toBeVisible();

  // Duplicate opens the create form pre-filled from the source rule.
  await sourceRow.getByRole("button", { name: `Duplicate rule ${sourceName}` }).click();
  const dupDrawer = page.getByRole("complementary", { name: "Duplicate Forward Rule" });
  await expect(dupDrawer).toBeVisible();
  await expect(dupDrawer.getByRole("heading", { name: "Duplicate Rule" })).toBeVisible();
  await expect(dupDrawer.getByLabel("Name")).toHaveValue(`${sourceName} copy`);
  await expect(dupDrawer.getByLabel("Group")).toHaveValue("dupes");

  // Change name + listen port (avoid duplicate-binding) and save the new rule.
  const dupName = "Dup Copy";
  await dupDrawer.getByLabel("Name").fill(dupName);
  await dupDrawer.getByLabel("Listen Port").fill(String(dupPort));
  await dupDrawer.getByRole("button", { name: "Add Rule", exact: true }).click();
  await expect(dupDrawer).not.toBeVisible({ timeout: 5_000 });

  // Both rules now exist; the duplicate preserved the group chip.
  const dupRow = page.locator("tr", { hasText: dupName });
  await expect(dupRow).toBeVisible({ timeout: 5_000 });
  await expect(sourceRow).toBeVisible();
  await expect(dupRow.locator(".rule-group-label")).toHaveText("dupes");

  // The API has two distinct rules; the source is unchanged.
  const rules = (await (await fetch(`${baseURL}/api/forwards`)).json()) as Array<{
    name: string;
    group?: string;
    listenPort: number;
    enabled: boolean;
  }>;
  const source = rules.find((r) => r.name === sourceName);
  const dup = rules.find((r) => r.name === dupName);
  expect(source?.listenPort).toBe(sourcePort);
  expect(source?.group).toBe("dupes");
  expect(dup?.group).toBe("dupes");
  expect(dup?.listenPort).toBe(dupPort);
  expect(dup?.enabled).toBe(false); // duplicate never auto-starts
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

  await ruleRow.getByRole("button", { name: "Start", exact: true }).click();
  await expect(ruleRow.getByText("Running")).toBeVisible({ timeout: 10_000 });
  await expect(ruleRow.getByRole("button", { name: "Stop", exact: true })).toBeVisible();

  await ruleRow.getByRole("button", { name: "Stop", exact: true }).click();
  await expect(ruleRow.getByText("Stopped")).toBeVisible({ timeout: 10_000 });
  await expect(ruleRow.getByRole("button", { name: "Start", exact: true })).toBeVisible();
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

  await ruleRow.getByRole("button", { name: "Delete", exact: true }).click();
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

  await ruleRow.getByRole("button", { name: "Diagnose", exact: true }).click();

  // The panel is open (its close affordance has an accessible name).
  await expect(page.getByRole("button", { name: "Close diagnostics" })).toBeVisible({ timeout: 5_000 });
  // Diagnostics finish and render real, user-visible check results — the
  // "Listen address" check is always present — rather than asserting a styling
  // container by CSS class.
  await expect(page.getByText("Running diagnostics…")).not.toBeVisible({ timeout: 10_000 });
  await expect(page.getByText("Listen address", { exact: true })).toBeVisible({ timeout: 5_000 });
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
