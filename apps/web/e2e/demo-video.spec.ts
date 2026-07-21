import { expect, test, type APIRequestContext, type Page } from "@playwright/test";

test.use({
  video: {
    mode: "on",
    size: { width: 1920, height: 1080 },
  },
  viewport: { width: 1920, height: 1080 },
});

test.setTimeout(420_000);

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

async function hold(page: Page, ms: number) {
  await page.waitForTimeout(ms);
}

test("record polished golden-path demo video", async ({ page, request, context }) => {
  await page.addInitScript(() => {
    Object.defineProperty(globalThis.crypto, "randomUUID", { configurable: true, value: undefined });
  });
  await context.grantPermissions(["camera", "microphone"], { origin: "http://localhost:13005" });

  await page.goto("/control-center");
  await expect(page.getByRole("button", { name: "Start Fungal Caverns Demo" })).toBeVisible();
  await hold(page, 12000);

  await page.getByRole("button", { name: "Start Fungal Caverns Demo" }).click();
  await expect(page).toHaveURL(/\/sessions\/[0-9a-f-]+$/);
  await hold(page, 10000);

  const demo = await jsonRequest<{
    campaign: { id: string; name: string };
    adventure: { id: string };
    map_asset: { id: string; name: string };
  }>(request, "post", "/api/demo/fungal-caverns", { language: "en" });

  await page.goto("/characters");
  await expect(page.getByRole("button", { name: "Create New Character" })).toBeVisible();
  await hold(page, 9000);

  await page.getByRole("button", { name: "Create New Character" }).click();
  await page.getByLabel("Campaign").selectOption(demo.campaign.id);
  await page.getByLabel("Player name").fill("Video Demo Player");
  await hold(page, 6000);

  const startResponsePromise = page.waitForResponse((response) => response.url().includes("/api/characters/builder/start") && response.request().method() === "POST");
  await page.getByRole("button", { name: "Start Builder" }).click();
  const startResponse = await startResponsePromise;
  expect(startResponse.ok()).toBeTruthy();
  const started = (await startResponse.json()) as { character: { id: string } };
  const characterId = started.character.id;

  await expect(page.getByRole("heading", { name: "AI-guided Character Draft" })).toBeVisible();
  await hold(page, 14000);

  await page.getByPlaceholder("Describe your concept, role, origin, or answer the AI's latest question.").fill(
    "I want to play a careful cave scout who protects the group and maps hidden routes."
  );
  await hold(page, 5000);
  await page.getByRole("button", { name: "Send message" }).click();
  await expect(page.getByText("A careful cave scout is a strong concept.")).toBeVisible();
  await hold(page, 18000);

  await jsonRequest(request, "post", `/api/characters/${characterId}/builder/apply`, {
    patch: {
      name: "Eira Video",
      player_name: "Video Demo Player",
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
  await page.getByRole("dialog").getByRole("button", { name: "Close" }).first().click();
  await expect(page.getByRole("heading", { name: "AI-guided Character Draft" })).toBeHidden();
  await hold(page, 12000);

  const session = await jsonRequest<{ id: string }>(request, "post", "/api/sessions", {
    campaign_id: demo.campaign.id,
    adventure_id: demo.adventure.id,
    name: "Video Demo Fungal Caverns",
    ruleset_work: "5E",
    ruleset_version: "2014",
    target_player_count: 1,
    current_scene: "Sheltered entrance",
    current_location: "Cavern entrance",
    language: "en",
  });
  const link = await jsonRequest<{ player_slot: { id: string }; join_url: string }>(request, "post", `/api/sessions/${session.id}/player-links`, {
    display_name: "Video Demo Player",
    character_id: characterId,
  });
  const joinToken = link.join_url.split("/").pop()!;
  await jsonRequest(request, "put", `/api/player-slots/${link.player_slot.id}/status`, { status: "ready" });
  await jsonRequest(request, "post", `/api/sessions/${session.id}/start`, {});
  await waitForOpening(request, session.id);

  await page.goto(`/sessions/${session.id}`);
  await expect(page.getByRole("heading", { name: "Video Demo Fungal Caverns" })).toBeVisible();
  await hold(page, 22000);

  await page.getByPlaceholder("Guide the AI, pass in player actions, or ask a rule/monster question...").fill(
    "I inspect the boulder and search for a safe passage."
  );
  await hold(page, 7000);
  const rollResponsePromise = page.waitForResponse((response) => response.url().includes("/api/gm/respond") && response.request().method() === "POST");
  await page.getByRole("button", { name: "Send to AI" }).click();
  const rollResponse = await rollResponsePromise;
  expect(rollResponse.ok()).toBeTruthy();
  await hold(page, 22000);

  await expect
    .poll(async () => {
      const value = await jsonRequest<{ state: { visual_mode: string } }>(request, "get", `/api/sessions/${session.id}`);
      return value.state.visual_mode;
    })
    .toBe("dice_capture");

  await page.goto("/player-screen");
  await page.getByRole("button", { name: "Activate Board" }).click();
  await expect(page.getByText("Find the safe passage", { exact: true }).first()).toBeVisible();
  await hold(page, 22000);

  await page.locator(".roll-die-input input").fill("17");
  await hold(page, 7000);
  const resolutionPromise = page.waitForResponse((response) => response.url().includes("/api/gm/respond") && response.request().method() === "POST");
  await page.getByRole("button", { name: "Confirm Roll" }).click();
  expect((await resolutionPromise).ok()).toBeTruthy();
  await expect(page.getByRole("img", { name: demo.map_asset.name })).toBeVisible();
  await hold(page, 24000);

  await page.goto(`/player-portal/${joinToken}`);
  await expect(page.getByText("Eira Video", { exact: true })).toBeVisible();
  await hold(page, 15000);

  await page.goto("/control-center");
  await expect(page.getByRole("button", { name: "Start Fungal Caverns Demo" })).toBeVisible();
  await hold(page, 12000);

  const finalSession = await jsonRequest<{ state: { visual_mode: string; group_inventory: { gold: number } } }>(request, "get", `/api/sessions/${session.id}`);
  expect(finalSession.state.visual_mode).toBe("scene");
  expect(finalSession.state.group_inventory.gold).toBe(3);
});
