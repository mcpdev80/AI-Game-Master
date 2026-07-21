import { expect, test, type APIRequestContext } from "@playwright/test";

async function jsonRequest<T>(request: APIRequestContext, method: "get" | "post" | "put", path: string, data?: unknown): Promise<T> {
  const response = await request[method](path, data === undefined ? undefined : { data });
  if (!response.ok()) {
    throw new Error(`${method.toUpperCase()} ${path}: ${response.status()} ${await response.text()}`);
  }
  return (await response.json()) as T;
}

test("deutscher builder-zauberschritt bleibt in chat und charakterbogen vollstaendig deutsch", async ({ page, request }) => {
  const characterName = `Elira DE ${Date.now()}`;

  await page.goto("/characters");
  await page.getByRole("button", { name: "Deutsch" }).click();
  await page.getByRole("button", { name: "Neuen Charakter erstellen" }).click();
  await page.getByLabel("Charaktername").fill(characterName);

  const startResponsePromise = page.waitForResponse((response) => response.url().includes("/api/characters/builder/start") && response.request().method() === "POST");
  await page.getByRole("button", { name: "Builder starten" }).click();
  const startResponse = await startResponsePromise;
  expect(startResponse.ok()).toBeTruthy();
  const started = (await startResponse.json()) as { character: { id: string } };
  const characterId = started.character.id;

  await jsonRequest(request, "post", `/api/characters/${characterId}/builder/apply`, {
    patch: {
      class_and_level: "Magier, Stufe 1",
      race: "Hochelf",
      background: "Akolyth",
      alignment: "Neutral gut",
      armor_class: 12,
      speed: "9 m",
      hit_point_max: 8,
      proficiency_bonus: "+2",
      abilities: { strength: 8, dexterity: 14, constitution: 12, intelligence: 16, wisdom: 10, charisma: 13 },
      features: ["Zauberwirken", "Arkane Erholung"],
      metadata: {
        builder_stage: "spellcasting_if_available",
        language: "de",
        creation_method: "standardwerte",
        concept: "Neugierige elfische Magierin",
        personality_traits: "Aufmerksam",
        ideals: "Wissen",
        bonds: "Alte Bibliothek",
        flaws: "Riskante Neugier",
        skill_proficiencies: ["Arkane Kunde", "Nachforschungen"],
        saving_throw_proficiencies: ["Intelligenz", "Weisheit"],
        starting_equipment: ["Quarterstaff", "Komponentenbeutel", "Gelehrtenpack", "Zauberbuch"],
        hit_dice: "1W6",
        senses: "Dunkelsicht 18 m, passive Wahrnehmung 10",
      },
    },
  });

  await page.reload();
  const card = page.locator("article").filter({ has: page.getByRole("heading", { name: characterName }) }).first();
  await expect(card).toBeVisible();
  await card.getByRole("button", { name: "Bearbeiten" }).click();
  await expect(page.getByRole("heading", { name: "KI-geführter Charakterentwurf" })).toBeVisible();
  await expect(page.getByText("Kernmerkmale", { exact: true })).toBeVisible();
  await expect(page.getByText("Zauberwirken, Arkane Erholung", { exact: false })).toBeVisible();
  await expect(page.getByText("Dunkelsicht 18 m, passive Wahrnehmung 10", { exact: false })).toBeVisible();

  await page.getByPlaceholder("Beschreibe Konzept, Rolle oder Herkunft oder beantworte die letzte Frage der KI.").fill("welche zauber sollte ich nehmen?");
  await page.getByRole("button", { name: "Nachricht senden" }).click();

  await expect(page.getByText("Für Magier auf Stufe 1", { exact: false })).toBeVisible();
  await expect(page.getByText("Empfohlene Standardauswahl:", { exact: false })).toBeVisible();
  await expect(page.getByText("Zauber-SG 13", { exact: false })).toBeVisible();
  await expect(page.getByText("Wenn du möchtest, übernehme ich genau diese Auswahl direkt.", { exact: false })).toBeVisible();

  await page.getByPlaceholder("Beschreibe Konzept, Rolle oder Herkunft oder beantworte die letzte Frage der KI.").fill("mach das");
  await page.getByRole("button", { name: "Nachricht senden" }).click();

  await expect(page.getByText("Ich übernehme die empfohlenen Zauber:", { exact: false })).toBeVisible();
  await expect(page.getByText("Prüfe kurz, ob das so passt.", { exact: false })).toBeVisible();

  await page.getByRole("button", { name: "Magie", exact: true }).click();
  await expect(page.getByText("Zauberangriffe", { exact: true })).toBeVisible();
  await expect(page.getByText("Zaubertrick", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Feuerpfeil", { exact: true })).toBeVisible();
  await expect(page.getByText("Zauberangriff", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Grad 1", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Rettungswurf", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Beschreibung", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Fernzauberangriff mit Feuer.", { exact: true })).toBeVisible();
  await expect(page.getByText("Weitere Zauber", { exact: true })).toBeVisible();
  await expect(page.getByText("Schild", { exact: true })).toBeVisible();
  await expect(page.getByText("Reaktionszauber mit +5 RK bis zum Beginn deines nächsten Zuges.", { exact: true })).toBeVisible();

  await expect(page.getByText("Cantrip", { exact: false })).toHaveCount(0);
  await expect(page.getByText("Spell Attack", { exact: false })).toHaveCount(0);
  await expect(page.getByText("Saving Throw", { exact: false })).toHaveCount(0);
  await expect(page.getByText("Check briefly whether this looks right.", { exact: false })).toHaveCount(0);
});
