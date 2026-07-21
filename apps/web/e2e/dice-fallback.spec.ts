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

test("manual roll fallback works when camera permission is denied", async ({ page, request }) => {
  await page.addInitScript(() => {
    Object.defineProperty(globalThis.crypto, "randomUUID", { configurable: true, value: undefined });
    const mediaDevices = navigator.mediaDevices;
    if (!mediaDevices?.getUserMedia) {
      return;
    }
    const originalGetUserMedia = mediaDevices.getUserMedia.bind(mediaDevices);
    mediaDevices.getUserMedia = async (constraints?: MediaStreamConstraints) => {
      if (typeof constraints === "object" && constraints?.video) {
        throw new DOMException("Camera permission denied by test", "NotAllowedError");
      }
      return originalGetUserMedia(constraints);
    };
  });

  const demo = await jsonRequest<{
    campaign: { id: string; name: string };
    adventure: { id: string };
    map_asset: { id: string; name: string };
  }>(request, "post", "/api/demo/fungal-caverns", { language: "en" });

  await page.goto("/characters");
  await page.getByRole("button", { name: "Create New Character" }).click();
  await page.getByLabel("Campaign").selectOption(demo.campaign.id);
  await page.getByLabel("Player name").fill("Fallback Player");

  const startResponsePromise = page.waitForResponse((response) => response.url().includes("/api/characters/builder/start") && response.request().method() === "POST");
  await page.getByRole("button", { name: "Start Builder" }).click();
  const startResponse = await startResponsePromise;
  expect(startResponse.ok()).toBeTruthy();
  const started = (await startResponse.json()) as { character: { id: string } };
  const characterId = started.character.id;

  await jsonRequest(request, "post", `/api/characters/${characterId}/builder/apply`, {
    patch: {
      name: "Fallback Ranger",
      player_name: "Fallback Player",
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
    name: "Browser Fallback Fungal Caverns",
    ruleset_work: "5E",
    ruleset_version: "2014",
    target_player_count: 1,
    current_scene: "Sheltered entrance",
    current_location: "Cavern entrance",
    language: "en",
  });
  const link = await jsonRequest<{ player_slot: { id: string }; join_url: string }>(request, "post", `/api/sessions/${session.id}/player-links`, {
    display_name: "Fallback Player",
    character_id: characterId,
  });
  await jsonRequest(request, "put", `/api/player-slots/${link.player_slot.id}/status`, { status: "ready" });
  await jsonRequest(request, "post", `/api/sessions/${session.id}/start`, {});
  await waitForOpening(request, session.id);

  await page.goto(`/sessions/${session.id}`);
  await page.getByPlaceholder("Guide the AI, pass in player actions, or ask a rule/monster question...").fill(
    "I inspect the boulder and search for a safe passage."
  );
  const rollResponsePromise = page.waitForResponse((response) => response.url().includes("/api/gm/respond") && response.request().method() === "POST");
  await page.getByRole("button", { name: "Send to AI" }).click();
  expect((await rollResponsePromise).ok()).toBeTruthy();
  await expect.poll(async () => {
    const value = await jsonRequest<{ state: { visual_mode: string } }>(request, "get", `/api/sessions/${session.id}`);
    return value.state.visual_mode;
  }).toBe("dice_capture");

  await page.goto("/player-screen");
  await page.getByRole("button", { name: "Activate Board" }).click();
  const rollDialog = page.locator(".player-popup");
  try {
    await expect(rollDialog).toBeVisible({ timeout: 3000 });
  } catch {
    await page.getByRole("button", { name: "Open Roll" }).click();
    await expect(rollDialog).toBeVisible();
  }
  const cameraInactiveText = rollDialog.getByText("Camera not active", { exact: true });
  await expect(cameraInactiveText).toBeVisible();
  await rollDialog.locator(".roll-die-input input").fill("17");
  const resolutionPromise = page.waitForResponse((response) => response.url().includes("/api/gm/respond") && response.request().method() === "POST");
  await rollDialog.getByRole("button", { name: "Confirm Roll" }).click();
  expect((await resolutionPromise).ok()).toBeTruthy();
  await expect(page.getByRole("img", { name: demo.map_asset.name })).toBeVisible();

  await expect.poll(async () => {
    const finalSession = await jsonRequest<{ state: { visual_mode: string; group_inventory: { gold: number } } }>(request, "get", `/api/sessions/${session.id}`);
    return {
      visualMode: finalSession.state.visual_mode,
      gold: finalSession.state.group_inventory.gold,
    };
  }).toEqual({ visualMode: "scene", gold: 3 });
});
