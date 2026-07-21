import { expect, test, type APIRequestContext } from "@playwright/test";

async function jsonRequest<T>(request: APIRequestContext, method: "get" | "post" | "put", path: string, data?: unknown): Promise<T> {
  const response = await request[method](path, data === undefined ? undefined : { data });
  if (!response.ok()) {
    throw new Error(`${method.toUpperCase()} ${path}: ${response.status()} ${await response.text()}`);
  }
  return (await response.json()) as T;
}

test("english builder spell step stays fully localized in chat and sheet", async ({ page, request }) => {
  const characterName = `Elira UI ${Date.now()}`;
  await page.goto("/characters");
  await page.getByRole("button", { name: "Create New Character" }).click();
  await page.getByLabel("Character name").fill(characterName);

  const startResponsePromise = page.waitForResponse((response) => response.url().includes("/api/characters/builder/start") && response.request().method() === "POST");
  await page.getByRole("button", { name: "Start Builder" }).click();
  const startResponse = await startResponsePromise;
  expect(startResponse.ok()).toBeTruthy();
  const started = (await startResponse.json()) as { character: { id: string } };
  const characterId = started.character.id;

  await jsonRequest(request, "post", `/api/characters/${characterId}/builder/apply`, {
    patch: {
      class_and_level: "Wizard, Level 1",
      race: "High Elf",
      background: "Acolyte",
      alignment: "Neutral Good",
      armor_class: 12,
      speed: "30 ft.",
      hit_point_max: 8,
      proficiency_bonus: "+2",
      abilities: { strength: 8, dexterity: 14, constitution: 12, intelligence: 16, wisdom: 10, charisma: 13 },
      features: ["Spellcasting", "Arcane Recovery"],
      metadata: {
        builder_stage: "spellcasting_if_available",
        language: "en",
        creation_method: "standard_array",
        concept: "Curious elven wizard",
        personality_traits: "Observant",
        ideals: "Knowledge",
        bonds: "Ancient library",
        flaws: "Reckless curiosity",
        skill_proficiencies: ["Arcana", "Investigation"],
        saving_throw_proficiencies: ["Intelligence", "Wisdom"],
        starting_equipment: ["Quarterstaff", "Component pouch", "Scholar pack", "Spellbook"],
        hit_dice: "1d6",
        senses: "darkvision 60 ft., passive Perception 10",
      },
    },
  });

  await page.reload();
  const card = page.locator("article").filter({ has: page.getByRole("heading", { name: characterName }) }).first();
  await expect(card).toBeVisible();
  await card.getByRole("button", { name: "Edit" }).click();
  await expect(page.getByRole("heading", { name: "AI-guided Character Draft" })).toBeVisible();

  await page.getByPlaceholder("Describe your concept, role, origin, or answer the AI's latest question.").fill("which spells should I take?");
  await page.getByRole("button", { name: "Send message" }).click();

  await expect(page.getByText("For Wizard at level 1", { exact: false })).toBeVisible();
  await expect(page.getByText("Recommended default selection:", { exact: false })).toBeVisible();
  await expect(page.getByText("Spell Save DC 13", { exact: false })).toBeVisible();
  await expect(page.getByText("If you want, I can apply this exact selection now.", { exact: false })).toBeVisible();

  await page.getByPlaceholder("Describe your concept, role, origin, or answer the AI's latest question.").fill("do that");
  await page.getByRole("button", { name: "Send message" }).click();

  await expect(page.getByText("I’ll apply the recommended spells:", { exact: false })).toBeVisible();
  await expect(page.getByText('Check briefly whether this looks right. If yes, reply with "ok" or "continue".', { exact: false })).toBeVisible();

  await page.getByRole("button", { name: "Magic", exact: true }).click();
  await expect(page.getByText("Spell Attacks", { exact: true })).toBeVisible();
  await expect(page.getByText("Cantrip", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Fire Bolt", { exact: true })).toBeVisible();
  await expect(page.getByText("Spell Attack", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Level 1", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Saving Throw", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Description", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Ranged fire spell attack.", { exact: true })).toBeVisible();

  await expect(page.getByText("Zaubertrick", { exact: false })).toHaveCount(0);
  await expect(page.getByText("Zauberangriff", { exact: false })).toHaveCount(0);
  await expect(page.getByText("Rettungswurf", { exact: false })).toHaveCount(0);
  await expect(page.getByText("Prüfe kurz", { exact: false })).toHaveCount(0);
});
