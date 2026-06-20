import { test, expect } from "@playwright/test";
import { clearAllRules } from "./helpers/api.js";

// Reset all rules before each test for isolation.
test.beforeEach(async ({ baseURL }) => {
  await clearAllRules(baseURL!);
});

// ── API Docs view ─────────────────────────────────────────────────────────────

test("api docs: view opens and endpoint list is visible", async ({ page }) => {
  await page.goto("/");
  // API Docs is reached from the header (not the left menu).
  await page.getByRole("banner")
    .getByRole("button", { name: "API Docs" })
    .click();

  await expect(page.getByText(/API Docs|api/i).first()).toBeVisible();
  await expect(page.getByText("/api/forwards").first()).toBeVisible({ timeout: 5_000 });
});

test("api docs: GET /api/connections endpoint is listed", async ({ page }) => {
  await page.goto("/");
  // API Docs is reached from the header (not the left menu).
  await page.getByRole("banner")
    .getByRole("button", { name: "API Docs" })
    .click();

  await expect(page.getByText("/api/connections").first()).toBeVisible({ timeout: 5_000 });
});
