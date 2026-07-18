"use client";

import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { Brain, Camera, Check, Copy, FileText, ImageIcon, Map as MapIcon, Monitor, Pause, Send, Users, X } from "lucide-react";
import { Panel, StatusPill } from "../studio-primitives";
import {
  apiPost,
  pauseSession,
  startSession,
  stopSession,
  updateSession,
  updateSessionRuntimeState,
  type Adventure,
  type Asset,
  type Character,
  type Document,
  type GMResponse,
  type PlayerLinkSlot,
  type Session,
  type SessionEvent,
} from "../../lib/api";

function splitMetadataList(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.flatMap((entry) => splitMetadataList(entry));
  }
  if (typeof value !== "string") {
    return [];
  }
  return value
    .split(/[,\n]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function buildRulesetOptions(documents: Document[]) {
  const groups = new Map<string, { key: string; work: string; version: string; label: string; documentCount: number }>();
  for (const document of documents) {
    if (document.type !== "rules") continue;
    let work = String(document.metadata.ruleset_work ?? "").trim();
    let version = String(document.metadata.ruleset_version ?? "").trim();
    if (!work || !version) {
      const firstKey = splitMetadataList(document.metadata.ruleset_keys)[0] ?? "";
      if (firstKey.includes(":")) {
        const [derivedWork, derivedVersion] = firstKey.split(":");
        work = work || derivedWork.trim();
        version = version || derivedVersion.trim();
      }
    }
    if (!work || !version) continue;
    const key = `${work}:${version}`;
    const existing = groups.get(key);
    if (existing) {
      existing.documentCount += 1;
      continue;
    }
    groups.set(key, {
      key,
      work,
      version,
      label: `${work} ${version}`,
      documentCount: 1,
    });
  }
  return Array.from(groups.values()).sort((a, b) => a.label.localeCompare(b.label, "de"));
}

type ActiveSessionScreenProps = {
  session: Session;
  events: SessionEvent[];
  playerLinks: PlayerLinkSlot[];
  adventures: Adventure[];
  documents: Document[];
  assets: Asset[];
  characters: Character[];
};

type SessionSheetTab = "overview" | "abilities" | "combat" | "magic" | "personality" | "gear";

function summarizeEvent(event: SessionEvent): { title: string; body: string } {
  if (event.type === "gm_response") {
    const playerInput = typeof event.payload.player_input === "string" ? event.payload.player_input : "No player input";
    const response = event.payload.response as Record<string, unknown> | undefined;
    const narration = typeof response?.narration === "string" ? response.narration : "No narration captured";
    return {
      title: playerInput,
      body: narration,
    };
  }

  return {
    title: event.type,
    body: JSON.stringify(event.payload),
  };
}

export function ActiveSessionScreen({ session, events, playerLinks, adventures, documents, assets, characters }: ActiveSessionScreenProps) {
  const recentEvents = events.slice(0, 8);
  const router = useRouter();
  const [prompt, setPrompt] = useState("");
  const [localResponse, setLocalResponse] = useState<GMResponse | null>(null);
  const [selectedHandoutId, setSelectedHandoutId] = useState("");
  const [selectedAssetId, setSelectedAssetId] = useState("");
  const [sessionName, setSessionName] = useState(session.name);
  const [sessionAdventureId, setSessionAdventureId] = useState(session.adventure_id ?? "");
  const [sessionRulesetKey, setSessionRulesetKey] = useState(`${session.ruleset_work}:${session.ruleset_version}`);
  const [sessionTargetPlayers, setSessionTargetPlayers] = useState(String(session.target_player_count));
  const [selectedRulebookIDs, setSelectedRulebookIDs] = useState<string[]>(session.state.selected_rulebook_ids ?? []);
  const [gmStyle, setGMStyle] = useState(session.state.prompt_config?.gm_style ?? "immersive");
  const [introStyle, setIntroStyle] = useState(session.state.prompt_config?.intro_style ?? "cinematic");
  const [adventureFocus, setAdventureFocus] = useState(session.state.prompt_config?.adventure_focus ?? "strict_adventure_first");
  const [rulesStrictness, setRulesStrictness] = useState(session.state.prompt_config?.rules_strictness ?? "table_balanced");
  const [playerAgencyStyle, setPlayerAgencyStyle] = useState(session.state.prompt_config?.player_agency_style ?? "proactive_questions");
  const [promptOverride, setPromptOverride] = useState(session.state.prompt_config?.prompt_override ?? "");
  const [groupGold, setGroupGold] = useState(String(session.state.group_inventory?.gold ?? 0));
  const [groupItems, setGroupItems] = useState((session.state.group_inventory?.items ?? []).join("\n"));
  const [groupNotes, setGroupNotes] = useState(session.state.group_inventory?.notes ?? "");
  const [pushMessage, setPushMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [selectedCharacter, setSelectedCharacter] = useState<Character | null>(null);
  const [selectedCharacterTab, setSelectedCharacterTab] = useState<SessionSheetTab>("overview");
  const [joinUrlCopied, setJoinUrlCopied] = useState(false);
  const [isPending, startTransition] = useTransition();
  const releasedHandouts = documents.filter((document) => document.type === "handout" || document.type === "character_sheet" || document.type === "adventure");
  const releasedAssets = assets.filter((asset) => ["image", "portrait", "battlemap", "map", "handout"].includes(asset.type));
  const availableRulesets = buildRulesetOptions(documents);
  const matchingRulebooks = documents.filter((document) => {
    if (document.type !== "rules") return false;
    const work = String(document.metadata.ruleset_work ?? "").trim();
    const version = String(document.metadata.ruleset_version ?? "").trim();
    const keyFromFields = work && version ? `${work}:${version}` : "";
    const derivedKey = splitMetadataList(document.metadata.ruleset_keys)[0] ?? "";
    return keyFromFields === sessionRulesetKey || derivedKey === sessionRulesetKey;
  });
  const selectedAdventure = adventures.find((adventure) => adventure.id === sessionAdventureId) ?? null;
  const joinUrl = `/session-join/${session.join_token}`;
  const joinUrlAbsolute =
    typeof window !== "undefined" ? `${window.location.origin}${joinUrl}` : joinUrl;

  async function sendPrompt(nextPrompt: string) {
    const response = await apiPost<GMResponse>("/api/gm/respond", {
      session_id: session.id,
      player_input: nextPrompt,
      language: session.language,
    });
    setLocalResponse(response);
  }

  function handleSend() {
    if (!prompt.trim()) {
      return;
    }
    setError(null);
    setPushMessage(null);
    startTransition(async () => {
      try {
        await sendPrompt(prompt.trim());
        setPrompt("");
      } catch (sendError) {
        setError(sendError instanceof Error ? sendError.message : "Could not send prompt to AI");
      }
    });
  }

  function handleRulesQuery() {
    if (!prompt.trim()) {
      setError("Enter a rule or monster question first.");
      return;
    }
    setError(null);
    setPushMessage(null);
    startTransition(async () => {
      try {
        await sendPrompt(`Rules query: ${prompt.trim()}`);
      } catch (sendError) {
        setError(sendError instanceof Error ? sendError.message : "Could not query rules");
      }
    });
  }

  function handleCopyJoinUrl() {
    setError(null);
    setPushMessage(null);
    startTransition(async () => {
      try {
        await navigator.clipboard.writeText(joinUrlAbsolute);
        setJoinUrlCopied(true);
        window.setTimeout(() => setJoinUrlCopied(false), 1600);
      } catch (copyError) {
        setError(copyError instanceof Error ? copyError.message : "Join-URL konnte nicht kopiert werden");
      }
    });
  }

  const displayedNarration =
    localResponse?.narration || session.state.last_narration || "The AI DM is ready to describe the next scene once the session is started.";
  const displayedNotes = localResponse?.dm_notes ?? session.state.last_dm_notes ?? [];
  const displayedCue = localResponse?.scene_events?.[0]?.name || session.state.active_media_cue;

  function splitLines(value: unknown): string[] {
    if (Array.isArray(value)) {
      return value.flatMap((entry) => splitLines(entry));
    }
    if (typeof value !== "string") {
      return [];
    }
    return value
      .split(/\r?\n|,/)
      .map((item) => item.trim())
      .filter(Boolean);
  }

  function openCharacterSheet(character: Character) {
    setSelectedCharacter(character);
    setSelectedCharacterTab("overview");
  }

  function characterMetaValue(character: Character, key: string) {
    return character.metadata?.[key];
  }

  function renderCharacterSheet(character: Character) {
    const tabLabels: Array<[SessionSheetTab, string]> = [
      ["overview", "Überblick"],
      ["abilities", "Attribute & Fertigkeiten"],
      ["combat", "Kampf"],
      ["magic", "Magie"],
      ["personality", "Persönlichkeit"],
      ["gear", "Ausrüstung & Magie"],
    ];
    const currentInventory = splitLines(characterMetaValue(character, "current_inventory"));
    const startingEquipment = splitLines(characterMetaValue(character, "starting_equipment"));
    const spells = splitLines(characterMetaValue(character, "spells"));
    const tools = splitLines(characterMetaValue(character, "tools_and_proficiencies"));
    const skillProficiencies = splitLines(characterMetaValue(character, "skill_proficiencies"));
    const savingThrowProficiencies = splitLines(characterMetaValue(character, "saving_throw_proficiencies"));
    const currentMoney = String(characterMetaValue(character, "current_money") ?? "—");
    const experiencePoints = String(characterMetaValue(character, "experience_points") ?? "—");
    const levelUpAvailable = String(characterMetaValue(character, "level_up_available") ?? "—");
    const combatOverview = String(characterMetaValue(character, "combat_overview") ?? "");
    const concept = String(characterMetaValue(character, "concept") ?? "");
    const backstory = String(characterMetaValue(character, "backstory") ?? "");
    const personalityTraits = String(characterMetaValue(character, "personality_traits") ?? "");
    const ideals = String(characterMetaValue(character, "ideals") ?? "");
    const bonds = String(characterMetaValue(character, "bonds") ?? "");
    const flaws = String(characterMetaValue(character, "flaws") ?? "");
    const senses = String(characterMetaValue(character, "senses") ?? "");
    const allies = String(characterMetaValue(character, "allies") ?? "");
    const weaponNotes = splitLines(characterMetaValue(character, "weapon_notes"));
    const spellNotes = String(characterMetaValue(character, "spell_notes") ?? "");
    const combatAttacks = String(characterMetaValue(character, "combat_attacks") ?? "");
    const spellAttacks = String(characterMetaValue(character, "spell_attacks") ?? "");
    const age = String(characterMetaValue(character, "age") ?? "—");
    const size = String(characterMetaValue(character, "size") ?? "—");
    const weight = String(characterMetaValue(character, "weight") ?? "—");
    const eyes = String(characterMetaValue(character, "eyes") ?? "—");
    const skin = String(characterMetaValue(character, "skin") ?? "—");
    const hair = String(characterMetaValue(character, "hair") ?? "—");
    const currentHitPoints = String(characterMetaValue(character, "current_hit_points") ?? character.hit_point_max ?? "—");
    const temporaryHitPoints = String(characterMetaValue(character, "temporary_hit_points") ?? "0");

    return (
      <section className="sheet-canvas">
        <div className="sheet-tabs">
          {tabLabels.map(([tab, label]) => (
            <button
              className={`sheet-tab${selectedCharacterTab === tab ? " is-active" : ""}`}
              key={tab}
              onClick={() => setSelectedCharacterTab(tab)}
              type="button"
            >
              {label}
            </button>
          ))}
        </div>
        <header className="sheet-canvas__header">
          <div className="sheet-canvas__name">
            <span>CHARAKTERNAME</span>
            <strong>{character.name || "—"}</strong>
          </div>
          <div className="sheet-canvas__identity">
            <article><span>Klasse & Stufe</span><strong>{character.class_and_level || "—"}</strong></article>
            <article><span>Volk</span><strong>{character.race || "—"}</strong></article>
            <article><span>Hintergrund</span><strong>{character.background || "—"}</strong></article>
            <article><span>Gesinnung</span><strong>{character.alignment || "—"}</strong></article>
            <article><span>Spieler</span><strong>{character.player_name || "—"}</strong></article>
          </div>
        </header>

        {selectedCharacterTab === "overview" ? (
          <div className="sheet-tab-panel">
            <div className="sheet-tab-grid sheet-tab-grid--overview">
              <section className="sheet-box sheet-box--story">
                <div className="sheet-box__title-row">
                  <strong>Konzept & Geschichte</strong>
                  <span>aktueller Sheet-Stand</span>
                </div>
                <p>{concept || "Noch kein Konzept eingetragen."}</p>
                <p>{backstory || "Noch keine Hintergrundgeschichte hinterlegt."}</p>
              </section>
              <section className="sheet-box">
                <strong>Grunddaten</strong>
                <dl className="sheet-detail-list">
                  <div><dt>Spieler</dt><dd>{character.player_name || "—"}</dd></div>
                  <div><dt>Alter</dt><dd>{age}</dd></div>
                  <div><dt>Größe</dt><dd>{size}</dd></div>
                  <div><dt>Gewicht</dt><dd>{weight}</dd></div>
                  <div><dt>Augen</dt><dd>{eyes}</dd></div>
                  <div><dt>Haut</dt><dd>{skin}</dd></div>
                  <div><dt>Haare</dt><dd>{hair}</dd></div>
                </dl>
              </section>
            </div>
          </div>
        ) : null}

        {selectedCharacterTab === "abilities" ? (
          <div className="sheet-tab-panel">
            <div className="sheet-tab-grid sheet-tab-grid--main-abilities">
              {Object.entries(character.abilities || {}).map(([ability, score]) => (
                <article className="sheet-ability" key={ability}>
                  <span>{ability.toUpperCase()}</span>
                  <strong>{score || "—"}</strong>
                </article>
              ))}
            </div>
            <div className="sheet-tab-grid sheet-tab-grid--ability-status">
              <section className="sheet-box">
                <strong>Fertigkeiten</strong>
                <dl className="sheet-detail-list">
                  <div><dt>Geübt</dt><dd>{skillProficiencies.join(", ") || "—"}</dd></div>
                  <div><dt>Rettungswürfe</dt><dd>{savingThrowProficiencies.join(", ") || "—"}</dd></div>
                  <div><dt>Sprachen</dt><dd>{character.languages.join(", ") || "—"}</dd></div>
                </dl>
              </section>
              <section className="sheet-box">
                <strong>Abgeleitete Werte</strong>
                <dl className="sheet-detail-list">
                  <div><dt>Rüstungsklasse</dt><dd>{character.armor_class ?? "—"}</dd></div>
                  <div><dt>Bewegung</dt><dd>{character.speed || "—"}</dd></div>
                  <div><dt>TP max</dt><dd>{character.hit_point_max ?? "—"}</dd></div>
                  <div><dt>Aktuelle TP</dt><dd>{currentHitPoints}</dd></div>
                  <div><dt>Temp. TP</dt><dd>{temporaryHitPoints}</dd></div>
                  <div><dt>Übungsbonus</dt><dd>{character.proficiency_bonus || "—"}</dd></div>
                </dl>
              </section>
            </div>
          </div>
        ) : null}

        {selectedCharacterTab === "combat" ? (
          <div className="sheet-tab-panel">
            <section className="sheet-box sheet-box--combat">
              <article><span>Rüstungsklasse</span><strong>{character.armor_class ?? "—"}</strong></article>
              <article><span>Bewegung</span><strong>{character.speed || "—"}</strong></article>
              <article><span>TP max</span><strong>{character.hit_point_max ?? "—"}</strong></article>
              <article><span>Aktuelle TP</span><strong>{currentHitPoints}</strong></article>
              <article><span>Temp. TP</span><strong>{temporaryHitPoints}</strong></article>
              <article><span>Übungsbonus</span><strong>{character.proficiency_bonus || "—"}</strong></article>
            </section>
            <section className="sheet-box">
              <div className="sheet-box__title-row">
                <strong>Kampfübersicht</strong>
                <span>Kampfnotizen und Angriffe</span>
              </div>
              <p>{combatOverview || "Keine Kampfübersicht hinterlegt."}</p>
            </section>
            <section className="sheet-box">
              <div className="sheet-box__title-row">
                <strong>Angriffe</strong>
                <span>roher Tabellen-/Sheet-Inhalt</span>
              </div>
              <p style={{ whiteSpace: "pre-wrap" }}>{combatAttacks || "Keine Angriffe hinterlegt."}</p>
            </section>
          </div>
        ) : null}

        {selectedCharacterTab === "magic" ? (
          <div className="sheet-tab-panel">
            <section className="sheet-box">
              <div className="sheet-box__title-row">
                <strong>Zauber</strong>
                <span>Magie und Notizen</span>
              </div>
              <dl className="sheet-detail-list">
                <div><dt>Zauber</dt><dd>{spells.join(", ") || "—"}</dd></div>
                <div><dt>Zauber-Notizen</dt><dd>{spellNotes || "—"}</dd></div>
                <div><dt>Zauberangriffe</dt><dd style={{ whiteSpace: "pre-wrap" }}>{spellAttacks || "—"}</dd></div>
              </dl>
            </section>
          </div>
        ) : null}

        {selectedCharacterTab === "personality" ? (
          <div className="sheet-tab-panel">
            <div className="sheet-tab-grid sheet-tab-grid--overview">
              <section className="sheet-box">
                <strong>Persönlichkeit</strong>
                <dl className="sheet-detail-list">
                  <div><dt>Merkmale</dt><dd>{personalityTraits || "—"}</dd></div>
                  <div><dt>Ideale</dt><dd>{ideals || "—"}</dd></div>
                  <div><dt>Bindungen</dt><dd>{bonds || "—"}</dd></div>
                  <div><dt>Makel</dt><dd>{flaws || "—"}</dd></div>
                </dl>
              </section>
              <section className="sheet-box">
                <strong>Auftreten & Kontakte</strong>
                <dl className="sheet-detail-list">
                  <div><dt>Sprachen</dt><dd>{character.languages.join(", ") || "—"}</dd></div>
                  <div><dt>Sinne</dt><dd>{senses || "—"}</dd></div>
                  <div><dt>Verbündete</dt><dd>{allies || "—"}</dd></div>
                  <div><dt>Features</dt><dd>{character.features.join(", ") || "—"}</dd></div>
                </dl>
              </section>
            </div>
          </div>
        ) : null}

        {selectedCharacterTab === "gear" ? (
          <div className="sheet-tab-panel">
            <section className="sheet-box">
              <strong>Ausrüstung</strong>
              <dl className="sheet-detail-list">
                <div><dt>Startausrüstung</dt><dd>{startingEquipment.join(", ") || "—"}</dd></div>
                <div><dt>Aktuelles Geld</dt><dd>{currentMoney}</dd></div>
                <div><dt>Aktuelles Inventar</dt><dd>{currentInventory.join(", ") || "—"}</dd></div>
                <div><dt>Werkzeuge</dt><dd>{tools.join(", ") || "—"}</dd></div>
                <div><dt>Waffen-Notizen</dt><dd>{weaponNotes.join(", ") || "—"}</dd></div>
                <div><dt>EP</dt><dd>{experiencePoints}</dd></div>
                <div><dt>Level-Up bereit</dt><dd>{levelUpAvailable}</dd></div>
              </dl>
            </section>
          </div>
        ) : null}
      </section>
    );
  }

  function handleLifecycle(action: "start" | "pause" | "stop") {
    setError(null);
    startTransition(async () => {
      try {
        if (action === "start") {
          await startSession(session.id);
        } else if (action === "pause") {
          await pauseSession(session.id);
        } else {
          await stopSession(session.id);
        }
        router.refresh();
      } catch (lifecycleError) {
        setError(lifecycleError instanceof Error ? lifecycleError.message : "Could not update session status");
      }
    });
  }

  function handleSaveSettings() {
    const [rulesetWork, rulesetVersion] = sessionRulesetKey.split(":");
    setError(null);
    startTransition(async () => {
      try {
        await updateSession(session.id, {
          campaign_id: session.campaign_id,
          name: sessionName.trim(),
          adventure_id: sessionAdventureId || null,
          ruleset_work: rulesetWork?.trim() || session.ruleset_work,
          ruleset_version: rulesetVersion?.trim() || session.ruleset_version,
          target_player_count: Number(sessionTargetPlayers) || 4,
          current_scene: session.current_scene,
          current_location: session.current_location,
          language: session.language,
          default_voice_profile_id: session.default_voice_profile_id,
          selected_rulebook_ids: selectedRulebookIDs,
          prompt_config: {
            gm_style: gmStyle,
            intro_style: introStyle,
            adventure_focus: adventureFocus,
            rules_strictness: rulesStrictness,
            player_agency_style: playerAgencyStyle,
            prompt_override: promptOverride,
          },
          group_inventory: {
            gold: Number(groupGold) || 0,
            items: groupItems.split("\n").map((item) => item.trim()).filter(Boolean),
            notes: groupNotes,
          },
        });
        router.refresh();
      } catch (saveError) {
        setError(saveError instanceof Error ? saveError.message : "Session konnte nicht gespeichert werden");
      }
    });
  }

  return (
    <div className="active-session">
      <header className="active-session__topbar">
        <div className="active-session__identity">
          <StatusPill tone={session.status === "live" ? "live" : session.status === "paused" ? "warning" : "ready"}>
            {session.status.toUpperCase()}
          </StatusPill>
          <div className="active-session__ai">
            <Brain size={16} />
            <span>AI DM narrating the session</span>
          </div>
          <h1>{session.name || session.current_scene || "Active Session"}</h1>
        </div>
        <div className="button-row">
          <button
            className="studio-button studio-button--ghost"
            disabled={isPending}
            onClick={() => handleLifecycle(session.status === "live" ? "pause" : "start")}
            type="button"
          >
            <Pause size={16} />
            {session.status === "live" ? "Pause Session" : "Start Session"}
          </button>
          <button className="studio-button studio-button--danger" disabled={isPending} onClick={() => handleLifecycle("stop")} type="button">
            End Session
          </button>
        </div>
      </header>

      <div className="active-session__grid">
        <aside className="active-session__rail">
          <Panel title="Session Settings" description="Regelwerk, Abenteuer und Session-Rahmen bearbeiten.">
            <div className="form-grid">
              <input onChange={(event) => setSessionName(event.target.value)} value={sessionName} />
              <select onChange={(event) => setSessionRulesetKey(event.target.value)} value={sessionRulesetKey}>
                <option value="">Regelwerk wählen</option>
                {availableRulesets.map((ruleset) => (
                  <option key={ruleset.key} value={ruleset.key}>
                    {ruleset.label}
                  </option>
                ))}
              </select>
              <select onChange={(event) => setSessionAdventureId(event.target.value)} value={sessionAdventureId}>
                <option value="">Adventure wählen</option>
                {adventures
                  .filter((adventure) => !adventure.campaign_id || adventure.campaign_id === session.campaign_id)
                  .map((adventure) => (
                    <option key={adventure.id} value={adventure.id}>
                      {adventure.name}
                    </option>
                  ))}
              </select>
              <input min={1} onChange={(event) => setSessionTargetPlayers(event.target.value)} type="number" value={sessionTargetPlayers} />
            </div>
            <p className="muted-copy">
              Passende Rulebooks in der Library: {matchingRulebooks.length}. Das primäre Regelwerk der Session wird hier direkt umgestellt.
            </p>
            <div className="button-row">
              <button className="studio-button" disabled={isPending} onClick={handleSaveSettings} type="button">
                Save Session
              </button>
            </div>
          </Panel>

          <Panel title="Rulebooks in Scope" description="Nur diese Regelbücher sollen für diese Session gelten.">
            <div className="list-stack">
              {matchingRulebooks.map((document) => {
                const checked = selectedRulebookIDs.includes(document.id);
                return (
                  <label className="list-row" key={document.id}>
                    <input
                      checked={checked}
                      onChange={(event) =>
                        setSelectedRulebookIDs((current) =>
                          event.target.checked ? [...current, document.id] : current.filter((item) => item !== document.id)
                        )
                      }
                      type="checkbox"
                    />
                    <div className="list-row__body">
                      <strong>{document.name}</strong>
                      <p>{document.type}</p>
                    </div>
                  </label>
                );
              })}
              {matchingRulebooks.length === 0 ? <p className="muted-copy">Keine passenden Regelbücher für das gewählte Regelwerk gefunden.</p> : null}
            </div>
          </Panel>

          <Panel title="AI Prompt Setup" description="Der AI DM bekommt pro Session einen anpassbaren Stil- und Fokusrahmen.">
            <div className="form-grid">
              <input onChange={(event) => setGMStyle(event.target.value)} placeholder="GM Style" value={gmStyle} />
              <input onChange={(event) => setIntroStyle(event.target.value)} placeholder="Intro Style" value={introStyle} />
              <input onChange={(event) => setAdventureFocus(event.target.value)} placeholder="Adventure Focus" value={adventureFocus} />
              <input onChange={(event) => setRulesStrictness(event.target.value)} placeholder="Rules Strictness" value={rulesStrictness} />
              <input onChange={(event) => setPlayerAgencyStyle(event.target.value)} placeholder="Player Agency Style" value={playerAgencyStyle} />
              <textarea className="studio-textarea" onChange={(event) => setPromptOverride(event.target.value)} placeholder="Zusätzlicher Session-Prompt-Override" rows={5} value={promptOverride} />
            </div>
          </Panel>

          <Panel title="Scene Context" description="What the AI currently believes about the scene.">
            <dl className="meta-list">
              <div>
                <dt>Location</dt>
                <dd>{session.current_location || "Unknown"}</dd>
              </div>
              <div>
                <dt>Scene</dt>
                <dd>{session.current_scene || "Not set"}</dd>
              </div>
              <div>
                <dt>Active cue</dt>
                <dd>{displayedCue || "No active media cue"}</dd>
              </div>
              <div>
                <dt>Board mode</dt>
                <dd>{session.state.visual_mode || "pause_or_recap"}</dd>
              </div>
              <div>
                <dt>Ambient</dt>
                <dd>{session.state.ambient_cue_id || "None"}</dd>
              </div>
            </dl>
          </Panel>

          <Panel title="Camera Feed" description="Dice and table recognition feed.">
            <div className="camera-box">
              <Camera size={32} />
            </div>
            <p className="muted-copy">Dice recognition is part of the live operator workflow and character generation path.</p>
          </Panel>
        </aside>

        <main className="active-session__stage">
          <section className="narrative-stage">
            <article className="narrative-card narrative-card--ai">
              <div className="narrative-card__meta">
                <StatusPill tone="info">AI Output</StatusPill>
              </div>
              <p>{displayedNarration}</p>
            </article>

            {localResponse?.context_chunks?.length ? (
              <article className="narrative-card">
                <div className="narrative-card__meta">
                  <StatusPill tone="warning">Rules Context</StatusPill>
                </div>
                <div className="list-stack">
                  {localResponse.context_chunks.slice(0, 3).map((chunk, index) => (
                    <div className="context-snippet" key={`${chunk.document_id}-${index}`}>
                      <strong>{chunk.document_name}</strong>
                      <p>{chunk.chunk_text}</p>
                    </div>
                  ))}
                </div>
              </article>
            ) : null}

            {recentEvents.map((event) => {
              const summary = summarizeEvent(event);
              return (
                <article className="narrative-card" key={event.id}>
                  <div className="narrative-card__meta">
                    <StatusPill tone="default">{event.type}</StatusPill>
                  </div>
                  <strong className="event-title">{summary.title}</strong>
                  <p>{summary.body}</p>
                </article>
              );
            })}
          </section>

          <section className="admin-composer">
            <div className="admin-composer__head">
              <div>
                <strong>Admin Input</strong>
                <p>This is operator guidance for the AI DM, not direct player-facing text.</p>
              </div>
              <StatusPill tone="warning">Operator only</StatusPill>
            </div>
            <div className="button-row">
              <button
                className="studio-button studio-button--ghost"
                disabled={!selectedHandoutId}
                onClick={() => {
                  setError(null);
                  setPushMessage(null);
                  startTransition(async () => {
                    try {
                      const selectedDocument = releasedHandouts.find((item) => item.id === selectedHandoutId);
                      await updateSessionRuntimeState(session.id, {
                        visual_mode: "rules_reference",
                        visual_payload: {
                          document_id: selectedHandoutId,
                          document_name: selectedDocument?.name || "Dokument",
                          popup: true,
                        },
                      });
                      setPushMessage("Dokument-Popup wurde auf dem Player-Screen angezeigt.");
                      router.refresh();
                    } catch (pushError) {
                      setError(pushError instanceof Error ? pushError.message : "Dokument konnte nicht angezeigt werden");
                    }
                  });
                }}
                type="button"
              >
                <FileText size={16} />
                Regelwerk/Handout als Popup
              </button>
              <button
                className="studio-button studio-button--ghost"
                disabled={!selectedAssetId}
                onClick={() => {
                  setError(null);
                  setPushMessage(null);
                  startTransition(async () => {
                    try {
                      await updateSessionRuntimeState(session.id, {
                        visual_mode: "combat",
                        visual_payload: {
                          image_asset_id: selectedAssetId,
                          popup: true,
                        },
                      });
                      setPushMessage("Bild-Popup wurde auf dem Player-Screen angezeigt.");
                      router.refresh();
                    } catch (pushError) {
                      setError(pushError instanceof Error ? pushError.message : "Bild konnte nicht angezeigt werden");
                    }
                  });
                }}
                type="button"
              >
                <ImageIcon size={16} />
                Bild als Popup
              </button>
              <button
                className="studio-button studio-button--ghost"
                onClick={() => {
                  setError(null);
                  setPushMessage(null);
                  startTransition(async () => {
                    try {
                      await updateSessionRuntimeState(session.id, {
                        visual_mode: "scene",
                        visual_payload: { dismiss_popup: true, auto_close: false },
                      });
                      setPushMessage("Popup wurde vom Player-Screen entfernt.");
                      router.refresh();
                    } catch (pushError) {
                      setError(pushError instanceof Error ? pushError.message : "Popup konnte nicht geschlossen werden");
                    }
                  });
                }}
                type="button"
              >
                <Monitor size={16} />
                Popup schließen
              </button>
            </div>
            <div className="form-grid" style={{ gridTemplateColumns: "1fr 1fr" }}>
              <select onChange={(event) => setSelectedHandoutId(event.target.value)} value={selectedHandoutId}>
                <option value="">Regelwerk/Handout wählen</option>
                {releasedHandouts.map((document) => (
                  <option key={document.id} value={document.id}>
                    {document.name}
                  </option>
                ))}
              </select>
              <select onChange={(event) => setSelectedAssetId(event.target.value)} value={selectedAssetId}>
                <option value="">Bild/Asset wählen</option>
                {releasedAssets.map((asset) => (
                  <option key={asset.id} value={asset.id}>
                    {asset.name}
                  </option>
                ))}
              </select>
            </div>
            <textarea
              className="studio-textarea"
              onChange={(event) => setPrompt(event.target.value)}
              placeholder="Guide the AI, pass in player actions, or ask a rule/monster question..."
              value={prompt}
            />
            {error ? <p className="error-copy">{error}</p> : null}
            {pushMessage ? <p className="success-copy">{pushMessage}</p> : null}
            <div className="button-row">
              <button className="studio-button studio-button--ghost" disabled={isPending || !prompt.trim()} onClick={handleRulesQuery} type="button">
                <MapIcon size={16} />
                Query Rules
              </button>
              <button className="studio-button" disabled={isPending || !prompt.trim()} onClick={handleSend} type="button">
                <Send size={16} />
                {isPending ? "Sending..." : "Send to AI"}
              </button>
            </div>
          </section>
        </main>

        <aside className="active-session__rail">
          <Panel title="Session Header" description="Adventure, Join-Link und Spielerstatus für diese Runde.">
            <div className="list-stack">
              <article className="list-row">
                <Users size={16} />
                <div className="list-row__body">
                  <strong>Join-Link</strong>
                  <p>Per Klick vollständige URL kopieren und direkt an Spieler weitergeben.</p>
                </div>
                <button
                  aria-label="Join-Link kopieren"
                  className="studio-button studio-button--ghost studio-button--inline"
                  onClick={handleCopyJoinUrl}
                  type="button"
                >
                  {joinUrlCopied ? <Check size={16} /> : <Copy size={16} />}
                </button>
              </article>
              <article className="list-row">
                <MapIcon size={16} />
                <div className="list-row__body">
                  <strong>{selectedAdventure?.name || "Kein Abenteuer ausgewählt"}</strong>
                  <p>{selectedAdventure?.description || "Für diese Session ist noch kein Abenteuer hinterlegt."}</p>
                </div>
              </article>
            </div>
            <div className="list-stack">
              {playerLinks.map((slot) => {
                const character = characters.find((item) => item.id === slot.player_slot.character_id) ?? null;
                const clickable = Boolean(character);
                return (
                  <button
                    className="list-row"
                    key={slot.player_slot.id}
                    onClick={() => {
                      if (character) openCharacterSheet(character);
                    }}
                    style={{ textAlign: "left", cursor: clickable ? "pointer" : "default", width: "100%" }}
                    type="button"
                  >
                    <Users size={16} />
                    <div className="list-row__body">
                      <strong>{slot.player_slot.display_name}</strong>
                      <p>{character ? `${character.name} · ${character.class_and_level}` : "Noch kein Charakter gewählt"} · {slot.player_slot.status}</p>
                    </div>
                  </button>
                );
              })}
            </div>
          </Panel>

          <Panel title="Group Inventory" description="Geteilte Gruppenbeute, Gold und Notizen für diese Session.">
            <div className="form-grid">
              <input min={0} onChange={(event) => setGroupGold(event.target.value)} placeholder="Gold" type="number" value={groupGold} />
              <textarea className="studio-textarea" onChange={(event) => setGroupItems(event.target.value)} placeholder="Items, ein Eintrag pro Zeile" rows={6} value={groupItems} />
              <textarea className="studio-textarea" onChange={(event) => setGroupNotes(event.target.value)} placeholder="Notizen zum Gruppeninventar" rows={4} value={groupNotes} />
            </div>
          </Panel>

          <Panel title="Output Routing" description="What the AI can show to players and where.">
            <div className="output-list">
              <div className="output-list__item">
                <Monitor size={16} />
                <div>
                  <strong>AI DM Visual Board</strong>
                  <p>{session.state.visual_mode || "pause_or_recap"} · current shared output</p>
                </div>
              </div>
              <div className="output-list__item">
                <Users size={16} />
                <div>
                  <strong>Player Portals</strong>
                  <p>Character stats, session feed, and released handouts</p>
                </div>
              </div>
              <div className="output-list__item">
                <Brain size={16} />
                <div>
                  <strong>Voice & Ambient</strong>
                  <p>{session.state.active_voice_profile_id || "narrator-default"} · {session.state.ambient_cue_id || "silence"}</p>
                </div>
              </div>
            </div>
          </Panel>

          <Panel title="DM Notes" description="Internal notes stored in session state or returned by the last prompt.">
            <div className="list-stack">
              {displayedNotes.length === 0 ? (
                <p className="empty-copy">No DM notes yet.</p>
              ) : (
                displayedNotes.map((note) => <p className="note-chip" key={note}>{note}</p>)
              )}
            </div>
          </Panel>
        </aside>
      </div>

      {selectedCharacter ? (
        <div className="modal-overlay" onClick={() => setSelectedCharacter(null)} role="presentation">
          <section className="modal-card modal-card--builder" onClick={(event) => event.stopPropagation()} role="dialog">
            <div className="modal-card__header">
              <div>
                <p className="eyebrow">Character Sheet</p>
                <h2 className="studio-panel__title">{selectedCharacter.name}</h2>
              </div>
              <button className="studio-button studio-button--ghost studio-button--inline" onClick={() => setSelectedCharacter(null)} type="button">
                <X size={16} />
                Schließen
              </button>
            </div>
            {renderCharacterSheet(selectedCharacter)}
          </section>
        </div>
      ) : null}
    </div>
  );
}
