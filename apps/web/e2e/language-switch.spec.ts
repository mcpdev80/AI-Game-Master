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

  await page.goto("/control-center");
  await expect(page.getByRole("heading", { name: "Sitzungsbereitschaft, bevor die KI übernimmt" })).toBeVisible();
  await expect(page.getByText("Kamera & Würfel", { exact: true })).toBeVisible();

  await page.goto("/library");
  await expect(page.getByRole("heading", { name: "Kampagnenwissen, Regelbücher, Abenteuer und Medien" })).toBeVisible();

  await page.goto("/player-screen");
  await expect(page.getByRole("button", { name: "Board aktivieren" })).toBeVisible();

  await page.goto("/characters");
  await page.getByRole("button", { name: "English" }).first().click();
  await expect(page.getByRole("heading", { name: "Character roster with AI-guided creation" })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");

  await page.goto("/control-center");
  await expect(page.getByRole("heading", { name: "Session readiness before the AI takes over" })).toBeVisible();
  await expect(page.getByText("Camera & Dice", { exact: true })).toBeVisible();

  await page.goto("/library");
  await expect(page.getByRole("heading", { name: "Campaign knowledge, rulebooks, adventures, and assets" })).toBeVisible();

  await page.goto("/player-screen");
  await expect(page.getByRole("button", { name: "Activate Board" })).toBeVisible();
});
