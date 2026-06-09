import { expect, type Page } from "@playwright/test";

export interface AddRuleOptions {
  name: string;
  protocol: "tcp" | "udp";
  udpMode?: "one-way" | "bidirectional-last-client" | "bidirectional-multi-client";
  listenHost: string;
  listenPort: number;
  targetHost: string;
  targetPort: number;
}

// Opens the Add Rule drawer, fills all fields, and saves.
// Waits for the drawer to close and the rule name to appear in the table.
export async function addRuleViaUI(page: Page, rule: AddRuleOptions): Promise<void> {
  await page.getByRole("button", { name: "+ Add Rule" }).click();
  const drawer = page.getByRole("complementary", { name: "Add Forward Rule" });
  await expect(drawer).toBeVisible();

  await drawer.getByLabel("Name").fill(rule.name);
  await drawer.getByLabel("Protocol").selectOption(rule.protocol);

  if (rule.protocol === "udp" && rule.udpMode) {
    await drawer.getByLabel("UDP Mode").selectOption(rule.udpMode);
  }

  await drawer.getByRole("textbox", { name: "Listen Host" }).fill(rule.listenHost);
  await drawer.getByLabel("Listen Port").fill(String(rule.listenPort));
  await drawer.getByLabel("Target Host").fill(rule.targetHost);
  await drawer.getByLabel("Target Port").fill(String(rule.targetPort));

  await drawer.getByRole("button", { name: "Add Rule", exact: true }).click();
  await expect(drawer).not.toBeVisible({ timeout: 5_000 });
  await expect(page.locator("tbody").getByText(rule.name)).toBeVisible({ timeout: 5_000 });
}

// Clicks Start for the named rule row and waits for Running status.
export async function startRuleViaUI(page: Page, ruleName: string): Promise<void> {
  const ruleRow = page.locator("tr", { hasText: ruleName });
  await expect(ruleRow).toBeVisible();
  await ruleRow.getByRole("button", { name: "Start" }).click();
  await expect(ruleRow.getByText("Running")).toBeVisible({ timeout: 10_000 });
}

// Clicks Stop for the named rule row and waits for Stopped status.
export async function stopRuleViaUI(page: Page, ruleName: string): Promise<void> {
  const ruleRow = page.locator("tr", { hasText: ruleName });
  await ruleRow.getByRole("button", { name: "Stop" }).click();
  await expect(ruleRow.getByText("Stopped")).toBeVisible({ timeout: 10_000 });
}
