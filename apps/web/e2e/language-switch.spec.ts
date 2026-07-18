import { expect, test } from "@playwright/test";

test("switches the complete character workspace between English and German and persists it", async ({ page }) => {
  await page.goto("/characters");
  await expect(page.getByRole("heading", { name: "Character roster with AI-guided creation" })).toBeVisible();

  await page.getByRole("button", { name: "Deutsch" }).first().click();
  await expect(page.getByRole("heading", { name: "Charakterübersicht mit KI-geführter Erstellung" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Neuen Charakter erstellen" })).toBeVisible();

  await page.reload();
  await expect(page.getByRole("heading", { name: "Charakterübersicht mit KI-geführter Erstellung" })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("lang", "de");

  await page.getByRole("button", { name: "English" }).first().click();
  await expect(page.getByRole("heading", { name: "Character roster with AI-guided creation" })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
});
