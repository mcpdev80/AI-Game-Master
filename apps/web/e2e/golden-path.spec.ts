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

test("complete browser golden path from demo and character builder to dice resolution", async ({ page, request, context }) => {
  await page.addInitScript(() => {
    Object.defineProperty(globalThis.crypto, "randomUUID", { configurable: true, value: undefined });
  });
  await context.grantPermissions(["camera", "microphone"], { origin: "http://localhost:13005" });

  await page.goto("/control-center");
  await page.getByRole("button", { name: "Start Fungal Caverns Demo" }).click();
  await expect(page).toHaveURL(/\/sessions\/[0-9a-f-]+$/);

  const demo = await jsonRequest<{
    campaign: { id: string; name: string };
    adventure: { id: string };
    map_asset: { id: string; name: string };
  }>(request, "post", "/api/demo/fungal-caverns", { language: "en" });

  await page.goto("/characters");
  await page.getByRole("button", { name: "Create New Character" }).click();
  await page.getByLabel("Campaign").selectOption(demo.campaign.id);
  await page.getByLabel("Player name").fill("Browser Golden Player");

  const startResponsePromise = page.waitForResponse((response) => response.url().includes("/api/characters/builder/start") && response.request().method() === "POST");
  await page.getByRole("button", { name: "Start Builder" }).click();
  const startResponse = await startResponsePromise;
  expect(startResponse.ok()).toBeTruthy();
  expect(startResponse.request().postDataJSON()).toMatchObject({ language: "en" });
  const started = (await startResponse.json()) as { character: { id: string } };
  const characterId = started.character.id;
  await expect(page.getByRole("heading", { name: "AI-guided Character Draft" })).toBeVisible();

  await page.getByPlaceholder("Describe your concept, role, origin, or answer the AI's latest question.").fill(
    "I want to play a careful cave scout who protects the group and maps hidden routes."
  );
  await page.getByRole("button", { name: "Send message" }).click();
  await expect(page.getByText("A careful cave scout is a strong concept.")).toBeVisible();

  await jsonRequest(request, "post", `/api/characters/${characterId}/builder/apply`, {
    patch: {
      name: "Eira Browser",
      player_name: "Browser Golden Player",
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
  await expect(page.getByRole("heading", { name: "AI-guided Character Draft" })).toBeHidden();
  await expect(page.getByRole("heading", { name: "Eira Browser" }).first()).toBeVisible();

  const session = await jsonRequest<{ id: string; join_token: string }>(request, "post", "/api/sessions", {
    campaign_id: demo.campaign.id,
    adventure_id: demo.adventure.id,
    name: "Browser Golden Fungal Caverns",
    ruleset_work: "5E",
    ruleset_version: "2014",
    target_player_count: 1,
    current_scene: "Sheltered entrance",
    current_location: "Cavern entrance",
    language: "en",
  });
  const link = await jsonRequest<{ player_slot: { id: string }; join_url: string }>(request, "post", `/api/sessions/${session.id}/player-links`, {
    display_name: "Browser Golden Player",
    character_id: characterId,
  });
  const joinToken = link.join_url.split("/").pop()!;
  const portal = await jsonRequest<{ token: string }>(request, "post", `/api/player-portal/join/${joinToken}`, {});
  await jsonRequest(request, "put", `/api/player-slots/${link.player_slot.id}/status`, { status: "ready" });
  await jsonRequest(request, "post", `/api/sessions/${session.id}/start`, {});
  await waitForOpening(request, session.id);

  await page.goto(`/sessions/${session.id}`);
  await page.getByPlaceholder("Guide the AI, pass in player actions, or ask a rule/monster question...").fill(
    "I inspect the boulder and search for a safe passage."
  );
  const rollResponsePromise = page.waitForResponse((response) => response.url().includes("/api/gm/respond") && response.request().method() === "POST");
  await page.getByRole("button", { name: "Send to AI" }).click();
  const rollResponse = await rollResponsePromise;
  expect(rollResponse.ok()).toBeTruthy();
  expect(rollResponse.request().postDataJSON()).toMatchObject({ language: "en" });
  await expect.poll(async () => {
    const value = await jsonRequest<{ state: { visual_mode: string } }>(request, "get", `/api/sessions/${session.id}`);
    return value.state.visual_mode;
  }).toBe("dice_capture");

  await page.goto("/player-screen");
  await page.getByRole("button", { name: "Activate Board" }).click();
  await expect(page.getByText("Find the safe passage", { exact: true }).first()).toBeVisible();
  await page.locator(".roll-die-input input").fill("17");
  const resolutionPromise = page.waitForResponse((response) => response.url().includes("/api/gm/respond") && response.request().method() === "POST");
  await page.getByRole("button", { name: "Confirm Roll" }).click();
  expect((await resolutionPromise).ok()).toBeTruthy();
  await expect(page.getByRole("img", { name: demo.map_asset.name })).toBeVisible();

  const finalSession = await jsonRequest<{ state: { visual_mode: string; visual_payload: { image_asset_id: string }; group_inventory: { gold: number } } }>(
    request,
    "get",
    `/api/sessions/${session.id}`
  );
  expect(finalSession.state.visual_mode).toBe("scene");
  expect(finalSession.state.visual_payload.image_asset_id).toBe(demo.map_asset.id);
  expect(finalSession.state.group_inventory.gold).toBe(3);

  await page.goto(`/player-portal/${portal.token}`);
  await expect(page.getByText("Eira Browser", { exact: true })).toBeVisible();
  await expect(page.getByText("ready", { exact: true }).first()).toBeVisible();
});
