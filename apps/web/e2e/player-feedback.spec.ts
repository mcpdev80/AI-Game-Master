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
      const session = await jsonRequest<{ state: { last_narration: string } }>(request, "get", `/api/sessions/${sessionId}`);
      return session.state.last_narration;
    })
    .toContain("Rain whispers");
}

async function setupPlayableSession(page: import("@playwright/test").Page, request: APIRequestContext) {
  const demo = await jsonRequest<{
    campaign: { id: string; name: string };
    adventure: { id: string };
  }>(request, "post", "/api/demo/fungal-caverns", { language: "en" });

  await page.goto("/characters");
  await page.getByRole("button", { name: "Create New Character" }).click();
  await page.getByLabel("Campaign").selectOption(demo.campaign.id);
  await page.getByLabel("Player name").fill("Feedback Player");

  const startResponsePromise = page.waitForResponse((response) => response.url().includes("/api/characters/builder/start") && response.request().method() === "POST");
  await page.getByRole("button", { name: "Start Builder" }).click();
  const startResponse = await startResponsePromise;
  expect(startResponse.ok()).toBeTruthy();
  const started = (await startResponse.json()) as { character: { id: string } };
  const characterId = started.character.id;

  await jsonRequest(request, "post", `/api/characters/${characterId}/builder/apply`, {
    patch: {
      name: "Feedback Ranger",
      player_name: "Feedback Player",
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
    name: "Browser Feedback Fungal Caverns",
    ruleset_work: "5E",
    ruleset_version: "2014",
    target_player_count: 1,
    current_scene: "Sheltered entrance",
    current_location: "Cavern entrance",
    language: "en",
  });
  const link = await jsonRequest<{ player_slot: { id: string } }>(request, "post", `/api/sessions/${session.id}/player-links`, {
    display_name: "Feedback Player",
    character_id: characterId,
  });
  await jsonRequest(request, "put", `/api/player-slots/${link.player_slot.id}/status`, { status: "ready" });
  await jsonRequest(request, "post", `/api/sessions/${session.id}/start`, {});
  await waitForOpening(request, session.id);

  await page.goto("/player-screen");
  await page.getByRole("button", { name: "Activate Board" }).click();
  const composer = page.locator(".player-overlay__composer");
  await expect(composer).toBeVisible();
  return composer;
}

test("player screen shows sending state and visible fallback notice", async ({ page, request, context }) => {
  await page.addInitScript(() => {
    Object.defineProperty(globalThis.crypto, "randomUUID", { configurable: true, value: undefined });
  });
  await context.grantPermissions(["camera", "microphone"], { origin: "http://localhost:13005" });
  const composer = await setupPlayableSession(page, request);

  await page.route("**/api/gm/respond", async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 600));
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        session_id: "feedback-session",
        narration: "Your action hangs in the air, but the world has not responded clearly.",
        language: "en",
        rules_used: ["fallback_resolution"],
        state_updates: [],
        scene_events: [],
        dm_notes: ["fallback used"],
        context_chunks: [],
        prompt_source: "fallback",
        raw_model: "fallback",
        created_at: "2026-07-20T19:00:00Z",
      }),
    });
  });

  await composer.locator("textarea").fill("I inspect the boulder and wait for the DM.");
  await composer.getByRole("button", { name: "Send" }).click();
  await expect(composer.getByText("AI DM is responding...")).toBeVisible();
  await expect(composer.getByRole("button", { name: "Sending..." })).toBeVisible();
  await expect(composer.getByText("Fallback narration was used. Retry the turn for a full GPT-5.6 response.")).toBeVisible();
  await expect(composer.getByRole("button", { name: "Retry turn" })).toBeVisible();
});

test("player screen shows generic error and allows retrying the turn", async ({ page, request, context }) => {
  await page.addInitScript(() => {
    Object.defineProperty(globalThis.crypto, "randomUUID", { configurable: true, value: undefined });
  });
  await context.grantPermissions(["camera", "microphone"], { origin: "http://localhost:13005" });
  const composer = await setupPlayableSession(page, request);

  let attempts = 0;
  await page.route("**/api/gm/respond", async (route) => {
    attempts += 1;
    if (attempts === 1) {
      await route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({ error: "internal error" }),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        session_id: "feedback-session",
        narration: "The world answers on the second try.",
        language: "en",
        rules_used: [],
        state_updates: [],
        scene_events: [],
        dm_notes: [],
        context_chunks: [],
        prompt_source: "llm",
        raw_model: "gpt-5.6",
        created_at: "2026-07-20T19:00:01Z",
      }),
    });
  });

  await composer.locator("textarea").fill("I inspect the boulder and wait for the DM.");
  await composer.getByRole("button", { name: "Send" }).click();
  await expect(composer.getByText("The AI DM is temporarily unavailable. Retry the turn.")).toBeVisible();
  const retryButton = composer.getByRole("button", { name: "Retry turn" });
  await expect(retryButton).toBeVisible();
  await retryButton.click();
  await expect(composer.getByText("The AI DM is temporarily unavailable. Retry the turn.")).toBeHidden();
});
