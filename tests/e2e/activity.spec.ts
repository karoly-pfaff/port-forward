import { test, expect } from "@playwright/test";
import { clearAllRules, createRule } from "./helpers/api.js";
import { getFreePort } from "./helpers/port.js";

// Reset all rules before each test for isolation.
test.beforeEach(async ({ baseURL }) => {
  await clearAllRules(baseURL!);
});

// ── Activity view ──────────────────────────────────────────────────────────

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

  await page.goto("/");
  await page.getByRole("navigation", { name: "Main navigation" })
    .getByRole("button", { name: "Activity Log" })
    .click();

  // Activity view loads — use the unique subtitle to avoid strict-mode violations.
  await expect(page.getByText("Recent forwarding and rule events")).toBeVisible();

  // After creating a rule, there should be at least one event.
  await expect(page.locator("table, .activity-log, [class*='activity']").first()).toBeVisible({ timeout: 5_000 });

  // Verify at least the rule creation event exists via API.
  const resp = await fetch(`${baseURL}/api/activity?ruleId=${rule.id}`);
  const body = (await resp.json()) as { events: Array<{ type: string }> };
  expect(body.events.length).toBeGreaterThan(0);
});
