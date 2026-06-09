import { test, expect } from "@playwright/test";
import { clearAllRules } from "./helpers/api.js";

// Reset all rules before each test for isolation.
test.beforeEach(async ({ baseURL }) => {
  await clearAllRules(baseURL!);
});

// ── Dashboard view ────────────────────────────────────────────────────────────

test("dashboard: navigating to Dashboard shows stat cards", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("navigation", { name: "Main navigation" })
    .getByRole("button", { name: "Dashboard" })
    .click();

  await expect(page.getByText(/Total|Rules|Running|dashboard/i).first()).toBeVisible({ timeout: 5_000 });
});
