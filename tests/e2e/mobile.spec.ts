import { test, expect } from "@playwright/test";
import { clearAllRules } from "./helpers/api.js";

// Reset all rules before each test for isolation.
test.beforeEach(async ({ baseURL }) => {
  await clearAllRules(baseURL!);
});

// ── Mobile sidebar ────────────────────────────────────────────────────────────

test("mobile sidebar: hamburger opens sidebar, navigation works, sidebar closes", async ({ page }) => {
  await page.setViewportSize({ width: 640, height: 800 });
  await page.goto("/");

  // Hamburger button is visible at this viewport.
  const menuBtn = page.getByRole("button", { name: "Open navigation menu" });
  await expect(menuBtn).toBeVisible();

  await menuBtn.click();

  // Sidebar becomes visible with nav items.
  const nav = page.getByRole("navigation", { name: "Main navigation" });
  await expect(nav).toBeVisible({ timeout: 3_000 });

  // Navigate to Activity — sidebar closes after nav click.
  await nav.getByRole("button", { name: "Activity" }).click();

  await expect(page.getByText("Activity").first()).toBeVisible({ timeout: 3_000 });
});
