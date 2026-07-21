import { expect, test, type APIRequestContext } from "@playwright/test";

async function jsonRequest<T>(request: APIRequestContext, method: "get" | "post" | "put", path: string, data?: unknown): Promise<T> {
  const response = await request[method](path, data === undefined ? undefined : { data });
  if (!response.ok()) {
    throw new Error(`${method.toUpperCase()} ${path}: ${response.status()} ${await response.text()}`);
  }
  return (await response.json()) as T;
}

async function waitForOpening(request: APIRequestContext, sessionId: string) {
  await expect
    .poll(async () => {
      const session = await jsonRequest<{ current_scene: string; state: { last_narration: string } }>(request, "get", `/api/sessions/${sessionId}`);
      const narration = session.state.last_narration ?? "";
      return narration && narration !== session.current_scene ? narration : "";
    })
    .not.toEqual("");
}

async function dismissPlayerPopupIfVisible(page: import("@playwright/test").Page) {
  for (let attempt = 0; attempt < 3; attempt += 1) {
    const closeButton = page.getByRole("button", { name: "Close" }).first();
    if (!(await closeButton.isVisible().catch(() => false))) {
      return;
    }
    await closeButton.click();
    await expect(closeButton).toBeHidden();
  }
}

test("player screen shows a visible error when gm/respond is rate-limited", async ({ page, request, context }) => {
  await page.addInitScript(() => {
    Object.defineProperty(globalThis.crypto, "randomUUID", { configurable: true, value: undefined });
  });
  await context.grantPermissions(["camera", "microphone"], { origin: "http://localhost:13005" });

  const demo = await jsonRequest<{
    campaign: { id: string; name: string };
    adventure: { id: string };
  }>(request, "post", "/api/demo/fungal-caverns", { language: "en" });

  await page.goto("/characters");
  await page.getByRole("button", { name: "Create New Character" }).click();
  await page.getByLabel("Campaign").selectOption(demo.campaign.id);
  await page.getByLabel("Player name").fill("Rate Limit Player");

  const startResponsePromise = page.waitForResponse((response) => response.url().includes("/api/characters/builder/start") && response.request().method() === "POST");
  await page.getByRole("button", { name: "Start Builder" }).click();
  const startResponse = await startResponsePromise;
  expect(startResponse.ok()).toBeTruthy();
  const started = (await startResponse.json()) as { character: { id: string } };
  const characterId = started.character.id;

  await jsonRequest(request, "post", `/api/characters/${characterId}/builder/apply`, {
    patch: {
      name: "Rate Limit Ranger",
      player_name: "Rate Limit Player",
      class_and_level: "Ranger 1",
      background: "Cave Cartographer",
      race: "Human",
      alignment: "Good",
      armor_class: 14,
      speed: "30 ft",
      hit_point_max: 11,
      proficiency_bonus: "+2",
      abilities: { strength: 10, dexterity: 16, constitution: 13, intelligence: 12, wisdom: 15, charisma: 8 },
      languages: ["Common"],
      features: ["Keen observer", "Cave navigation"],
      metadata: { builder_stage: "review", skill_proficiencies: ["Perception", "Survival"] },
    },
  });
  await page.getByRole("button", { name: "Mark as Ready" }).click();

  const session = await jsonRequest<{ id: string }>(request, "post", "/api/sessions", {
    campaign_id: demo.campaign.id,
    adventure_id: demo.adventure.id,
    name: "Browser Rate Limit Fungal Caverns",
    ruleset_work: "5E",
    ruleset_version: "2014",
    target_player_count: 1,
    current_scene: "Sheltered entrance",
    current_location: "Cavern entrance",
    language: "en",
  });
  const link = await jsonRequest<{ player_slot: { id: string } }>(request, "post", `/api/sessions/${session.id}/player-links`, {
    display_name: "Rate Limit Player",
    character_id: characterId,
  });
  await jsonRequest(request, "put", `/api/player-slots/${link.player_slot.id}/status`, { status: "ready" });
  await jsonRequest(request, "post", `/api/sessions/${session.id}/start`, {});
  await waitForOpening(request, session.id);

  await page.route("**/api/gm/respond", async (route) => {
    await route.fulfill({
      status: 429,
      contentType: "application/json",
      body: JSON.stringify({ error: "rate limit exceeded", details: "try again shortly" }),
    });
  });

  await page.goto("/player-screen");
  await page.getByRole("button", { name: "Activate Board" }).click();
  await dismissPlayerPopupIfVisible(page);
  const composer = page.locator(".player-overlay__composer");
  await expect(composer).toBeVisible();
  await composer.locator("textarea").fill("I inspect the boulder and wait for the DM.");
  await dismissPlayerPopupIfVisible(page);
  await composer.getByRole("button", { name: "Send" }).evaluate((node) => (node as HTMLButtonElement).click());
  await expect(page.locator(".error-copy").filter({ hasText: "429 Too Many Requests" })).toBeVisible();
});
