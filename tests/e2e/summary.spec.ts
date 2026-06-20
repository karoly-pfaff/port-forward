import { test, expect } from "@playwright/test";
import { clearAllRules } from "./helpers/api.js";

// Reset all rules before each test for isolation.
test.beforeEach(async ({ baseURL }) => {
  await clearAllRules(baseURL!);
});

// The rule summary cards (Total Rules / Running / Stopped / Error) must render on
// every primary view, not just the dashboard. Each card is asserted via a string
// that is unique to the summary (the Total Rules label and the three card
// descriptions), so the check is unambiguous on views that also show "Running"/
// "Stopped"/"Error" as filter options.
const SUMMARY_VIEWS = ["Dashboard", "Forward Rules", "Activity Log", "Live Connections"];

for (const view of SUMMARY_VIEWS) {
  test(`summary: ${view} view shows the rule summary cards`, async ({ page }) => {
    await page.goto("/");
    await page.getByRole("navigation", { name: "Main navigation" })
      .getByRole("button", { name: view })
      .click();

    await expect(page.getByText("Total Rules")).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText("Currently forwarding")).toBeVisible();
    await expect(page.getByText("Not forwarding")).toBeVisible();
    await expect(page.getByText("Needs attention")).toBeVisible();
  });
}
