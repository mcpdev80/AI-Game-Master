"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState, useTransition } from "react";
import { Backpack, Coins, FileText, ImageIcon, Info, LayoutDashboard, LockKeyhole, Menu, ScrollText, Send, Shield, Sparkles, UserRoundCheck, WandSparkles, X } from "lucide-react";
import { PageIntro, Panel, StatusPill } from "../studio-primitives";
import { useNotifications } from "../notifications-provider";
import {
  apiBaseUrl,
  fetchPlayerPortalPrivateChat,
  sendPlayerPortalPrivateChat,
  updatePlayerPortalCharacter,
  updatePlayerPortalGroupInventory,
  updatePlayerSlotCharacter,
  updatePlayerSlotStatus,
  type Character,
  type PrivateChatMessage,
  type PlayerPortalSession,
} from "../../lib/api";
import { useI18n } from "../../lib/i18n";

function metaString(character: Character | null | undefined, key: string, fallback = ""): string {
  if (!character?.metadata) return fallback;
  const value = character.metadata[key];
  if (typeof value === "string") return value;
  if (typeof value === "number") return String(value);
  return fallback;
}

function metaList(character: Character | null | undefined, key: string): string[] {
  if (!character?.metadata) return [];
  const value = character.metadata[key];
  if (Array.isArray(value)) {
    return value.map((item) => String(item).trim()).filter(Boolean);
  }
  if (typeof value === "string") {
    return value.split(/\n|,|;/).map((item) => item.trim()).filter(Boolean);
  }
  return [];
}

function rowsToText(items: string[]): string {
  return items.join("\n");
}

function textToRows(value: string): string[] {
  return value.split(/\n|;/).map((item) => item.trim()).filter(Boolean);
}

type ParsedTableRow = {
  columns: string[];
  description: string;
};

type ActionCardRow = {
  title: string;
  badge: string;
  ability: string;
  range: string;
  bonus: string;
  damage: string;
  damageType: string;
  description: string;
};

function parseStructuredRows(value: string, expectedColumns: number) {
  const rows: ParsedTableRow[] = [];
  let pendingDescription = "";
  for (const rawLine of value.replaceAll("\r", "\n").split("\n")) {
    const line = rawLine.trim();
    if (!line) continue;
    const lower = line.toLowerCase();
    if (lower.startsWith("description:") || lower.startsWith("beschreibung:")) {
      const detail = line.slice(line.indexOf(":") + 1).trim();
      if (rows.length > 0) {
        rows[rows.length - 1].description = rows[rows.length - 1].description
          ? `${rows[rows.length - 1].description} ${detail}`
          : detail;
      } else {
        pendingDescription = pendingDescription ? `${pendingDescription} ${detail}` : detail;
      }
      continue;
    }
    const columns = line.split("|").map((chunk) => chunk.trim());
    if (columns.length >= expectedColumns) {
      rows.push({ columns: columns.slice(0, expectedColumns), description: pendingDescription });
      pendingDescription = "";
      continue;
    }
    if (rows.length > 0) {
      rows[rows.length - 1].description = rows[rows.length - 1].description
        ? `${rows[rows.length - 1].description} ${line}`
        : line;
    } else {
      pendingDescription = pendingDescription ? `${pendingDescription} ${line}` : line;
    }
  }
  return rows;
}

function parseSpellNoteRows(value: string) {
  return value
    .replaceAll("\r", "\n")
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const separator = line.indexOf(":");
      if (separator <= 0) {
        return { name: line, description: "" };
      }
      return {
        name: line.slice(0, separator).trim(),
        description: line.slice(separator + 1).trim(),
      };
    });
}

type PortalSection = "overview" | "session" | "gear" | "spells" | "details" | "private" | "handouts";

export function PlayerPortalScreen({ portal }: { portal: PlayerPortalSession }) {
  const router = useRouter();
  const { locale, tr } = useI18n();
  const { notify } = useNotifications();
  const [isPending, startTransition] = useTransition();
  const [selectedCharacterId, setSelectedCharacterId] = useState(portal.character?.id ?? "");
  const [currentHitPoints, setCurrentHitPoints] = useState(metaString(portal.character, "current_hit_points", portal.character?.hit_point_max != null ? String(portal.character.hit_point_max) : ""));
  const [temporaryHitPoints, setTemporaryHitPoints] = useState(metaString(portal.character, "temporary_hit_points", "0"));
  const [currentMoney, setCurrentMoney] = useState(metaString(portal.character, "current_money"));
  const [experiencePoints, setExperiencePoints] = useState(metaString(portal.character, "experience_points"));
  const [inspiration, setInspiration] = useState(metaString(portal.character, "inspiration"));
  const [sessionNotes, setSessionNotes] = useState(rowsToText(metaList(portal.character, "session_notes")));
  const [currentInventory, setCurrentInventory] = useState(rowsToText(metaList(portal.character, "current_inventory")));
  const [groupGold, setGroupGold] = useState(String(portal.session.state.group_inventory?.gold ?? 0));
  const [groupItems, setGroupItems] = useState(rowsToText(portal.session.state.group_inventory?.items ?? []));
  const [groupNotes, setGroupNotes] = useState(portal.session.state.group_inventory?.notes ?? "");
  const [privateMessages, setPrivateMessages] = useState<PrivateChatMessage[]>([]);
  const [privateDraft, setPrivateDraft] = useState("");
  const [privateChatLoading, setPrivateChatLoading] = useState(true);
  const [activeSection, setActiveSection] = useState<PortalSection>("session");
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const previousAssistantMessageCountRef = useRef(0);

  const statusLabel = (status: string) => (({ ready: tr("ready", "bereit"), joined: tr("joined", "beigetreten"), invited: tr("invited", "eingeladen"), locked: tr("locked", "gesperrt") } as Record<string, string>)[status] ?? status);
  const abilityLabel = (ability: string) => locale === "de" ? (({ strength: "STÄ", dexterity: "GES", constitution: "KON", intelligence: "INT", wisdom: "WEI", charisma: "CHA" } as Record<string, string>)[ability] ?? ability.toUpperCase()) : ability.toUpperCase();
  const contentTypeLabel = (type: string) => (({ document: tr("document", "Dokument"), image: tr("image", "Bild"), map: tr("map", "Karte"), battlemap: tr("battle map", "Kampfkarte"), handout: tr("handout", "Handout"), asset: tr("asset", "Medium") } as Record<string, string>)[type] ?? type);
  const releasedNarration = portal.session.state.scene_summary || portal.session.state.last_narration || tr("The AI has not released a player-safe scene yet.", "Die KI hat noch keine spielersichere Szene freigegeben.");
  const visibleHandouts = portal.visible_state.visible_handouts;
  const visibleMedia = portal.visible_state.visible_media;
  const visibleCompanions = (portal.session.companions ?? []).filter((companion) => companion.visibility !== "hidden");
  const spellList = useMemo(() => metaList(portal.character, "spells"), [portal.character]);
  const languages = portal.character?.languages ?? [];
  const features = portal.character?.features ?? [];
  const combatAttacks = metaString(portal.character, "combat_attacks");
  const spellAttacks = metaString(portal.character, "spell_attacks");
  const spellNotes = metaString(portal.character, "spell_notes");
  const featureNotes = metaString(portal.character, "feature_notes");
  const senses = metaString(portal.character, "senses");
  const weaponNotes = metaList(portal.character, "weapon_notes");
  const maxHitPoints = portal.character?.hit_point_max != null ? String(portal.character.hit_point_max) : "—";
  const combatRows = useMemo(() => parseStructuredRows(combatAttacks, 7), [combatAttacks]);
  const spellAttackRows = useMemo(() => parseStructuredRows(spellAttacks, 7), [spellAttacks]);
  const spellNoteRows = useMemo(() => parseSpellNoteRows(spellNotes), [spellNotes]);
  const combatActionRows = useMemo<ActionCardRow[]>(
    () =>
      combatRows.map((row) => ({
        title: row.columns[0] || "—",
        badge: row.columns[4] || row.columns[1] || "—",
        ability: row.columns[2] || "—",
        range: row.columns[3] || "—",
        bonus: row.columns[4] || "—",
        damage: row.columns[5] || "—",
        damageType: row.columns[6] || "—",
        description: row.description,
      })),
    [combatRows],
  );
  const spellActionRows = useMemo<ActionCardRow[]>(
    () =>
      spellAttackRows.map((row) => ({
        title: row.columns[1] || "—",
        badge: row.columns[0] || "—",
        ability: row.columns[2] || "—",
        range: row.columns[3] || "—",
        bonus: row.columns[4] || "—",
        damage: row.columns[5] || "—",
        damageType: row.columns[6] || "—",
        description: row.description,
      })),
    [spellAttackRows],
  );
  const overviewActionRows = useMemo<ActionCardRow[]>(
    () => [...combatActionRows, ...spellActionRows],
    [combatActionRows, spellActionRows],
  );
  const privateAssistantMessages = useMemo(() => privateMessages.filter((message) => message.role !== "user"), [privateMessages]);
  const privateBubbleCount = privateAssistantMessages.length;
  const sectionEntries: Array<{ key: PortalSection; label: string; icon: typeof LayoutDashboard; bubble?: number }> = [
    { key: "overview", label: tr("Overview", "Übersicht"), icon: LayoutDashboard },
    { key: "session", label: tr("Session", "Session"), icon: ScrollText },
    { key: "gear", label: tr("Equipment", "Ausrüstung"), icon: Backpack },
    { key: "spells", label: tr("Spells", "Zauber"), icon: WandSparkles },
    { key: "details", label: tr("Details", "Details"), icon: Info },
    { key: "private", label: tr("Private DM", "Privater DM"), icon: LockKeyhole, bubble: privateBubbleCount },
    { key: "handouts", label: tr("Handouts", "Handouts"), icon: FileText },
  ];

  useEffect(() => {
    let active = true;
    setPrivateChatLoading(true);
    fetchPlayerPortalPrivateChat(portal.token)
      .then((items) => {
        if (active) {
          setPrivateMessages(items);
        }
      })
      .catch(() => {
        if (active) {
          setPrivateMessages([]);
        }
      })
      .finally(() => {
        if (active) {
          setPrivateChatLoading(false);
        }
      });
    return () => {
      active = false;
    };
  }, [portal.token]);

  useEffect(() => {
    const nextCount = privateAssistantMessages.length;
    const previousCount = previousAssistantMessageCountRef.current;
    previousAssistantMessageCountRef.current = nextCount;
    if (nextCount <= previousCount || typeof window === "undefined") {
      return;
    }
    try {
      const AudioContextCtor = window.AudioContext || (window as typeof window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
      if (!AudioContextCtor) {
        return;
      }
      const context = new AudioContextCtor();
      const oscillator = context.createOscillator();
      const gain = context.createGain();
      oscillator.type = "sine";
      oscillator.frequency.value = 880;
      gain.gain.value = 0.03;
      oscillator.connect(gain);
      gain.connect(context.destination);
      oscillator.start();
      oscillator.stop(context.currentTime + 0.12);
      window.setTimeout(() => void context.close().catch(() => undefined), 180);
    } catch {
      // ignore audio notification failures
    }
  }, [privateAssistantMessages.length]);

  function openSection(section: PortalSection) {
    setActiveSection(section);
    setMobileMenuOpen(false);
  }

  function handleAssignCharacter() {
    if (!selectedCharacterId) {
      notify({ title: tr("Player Portal", "Spielerportal"), message: tr("Choose a character first.", "Bitte zuerst einen Charakter wählen."), tone: "warning" });
      return;
    }
    startTransition(async () => {
      try {
        await updatePlayerSlotCharacter(portal.player_slot.id, { character_id: selectedCharacterId });
        notify({ title: tr("Character", "Charakter"), message: tr("Character assigned to the player slot.", "Charakter wurde dem Spielerplatz zugewiesen."), tone: "success" });
        router.refresh();
      } catch (error) {
        notify({ title: tr("Player Portal", "Spielerportal"), message: error instanceof Error ? error.message : tr("Assignment failed.", "Zuweisung fehlgeschlagen."), tone: "error" });
      }
    });
  }

  function handleSetReady() {
    startTransition(async () => {
      try {
        await updatePlayerSlotStatus(portal.player_slot.id, { status: "ready" });
        notify({ title: tr("Ready", "Bereit"), message: tr("You are now marked as ready.", "Du bist jetzt als bereit markiert."), tone: "success" });
        router.refresh();
      } catch (error) {
        notify({ title: tr("Player Portal", "Spielerportal"), message: error instanceof Error ? error.message : tr("Ready status failed.", "Bereitschaftsstatus fehlgeschlagen."), tone: "error" });
      }
    });
  }

  function handleSaveCharacter() {
    if (!portal.character) return;
    startTransition(async () => {
      try {
        await updatePlayerPortalCharacter(portal.token, {
          current_hit_points: Number(currentHitPoints || 0),
          temporary_hit_points: Number(temporaryHitPoints || 0),
          current_money: currentMoney,
          experience_points: experiencePoints,
          inspiration,
          session_notes: sessionNotes,
          current_inventory: textToRows(currentInventory),
        });
        notify({ title: tr("Character", "Charakter"), message: tr("Your character sheet details were updated.", "Deine Charakterbogen-Daten wurden aktualisiert."), tone: "success" });
        router.refresh();
      } catch (error) {
        notify({ title: tr("Character", "Charakter"), message: error instanceof Error ? error.message : tr("Character update failed.", "Charakteraktualisierung fehlgeschlagen."), tone: "error" });
      }
    });
  }

  function handleSaveGroupInventory() {
    startTransition(async () => {
      try {
        await updatePlayerPortalGroupInventory(portal.token, {
          gold: Number(groupGold || 0),
          items: textToRows(groupItems),
          notes: groupNotes,
        });
        notify({ title: tr("Group Inventory", "Gruppeninventar"), message: tr("Shared inventory was updated.", "Das Gruppeninventar wurde aktualisiert."), tone: "success" });
        router.refresh();
      } catch (error) {
        notify({ title: tr("Group Inventory", "Gruppeninventar"), message: error instanceof Error ? error.message : tr("Group inventory update failed.", "Gruppeninventar-Aktualisierung fehlgeschlagen."), tone: "error" });
      }
    });
  }

  function handleSendPrivateChat() {
    const message = privateDraft.trim();
    if (!message) {
      return;
    }
    startTransition(async () => {
      try {
        const response = await sendPlayerPortalPrivateChat(portal.token, {
          message,
          language: locale,
        });
        setPrivateMessages(response.messages);
        setPrivateDraft("");
        notify({
          title: tr("Private DM Chat", "Privater DM-Chat"),
          message: tr("Your private sidebar exchange was recorded.", "Euer privater Nebenkanal wurde protokolliert."),
          tone: "success",
        });
        router.refresh();
      } catch (error) {
        notify({
          title: tr("Private DM Chat", "Privater DM-Chat"),
          message: error instanceof Error ? error.message : tr("Private message failed.", "Private Nachricht fehlgeschlagen."),
          tone: "error",
        });
      }
    });
  }

  const overviewMetrics = [
    { label: tr("Armor Class", "RK"), value: portal.character?.armor_class != null ? String(portal.character.armor_class) : "—" },
    { label: tr("Max HP", "TP max"), value: maxHitPoints },
    { label: tr("Current HP", "TP"), value: currentHitPoints || "—" },
    { label: tr("Spell Save DC", "Zauber-SG"), value: metaString(portal.character, "spell_save_dc", "—") },
  ];
  const renderActionCards = (rows: ActionCardRow[], keyPrefix: string) => (
    <div className="portal-attack-list">
      {rows.map((row, index) => {
        return (
          <article className="portal-attack-card" key={`${keyPrefix}-${index}`}>
            <div className="portal-attack-card__top">
              <strong>{row.title || "—"}</strong>
              <span>{row.badge || "—"}</span>
            </div>
            <div className="portal-attack-card__stats">
              <div><span>{tr("Ability", "Attribut")}</span><strong>{row.ability || "—"}</strong></div>
              <div><span>{tr("Range", "Reichweite")}</span><strong>{row.range || "—"}</strong></div>
              <div><span>{tr("Bonus", "Bonus")}</span><strong>{row.bonus || "—"}</strong></div>
              <div><span>{tr("Damage", "Schaden")}</span><strong>{row.damage || "—"}</strong></div>
              <div><span>{tr("Type", "Typ")}</span><strong>{row.damageType || "—"}</strong></div>
            </div>
            {row.description ? (
              <div className="portal-attack-card__description">
                <span>{tr("Description", "Beschreibung")}</span>
                <p>{row.description}</p>
              </div>
            ) : null}
          </article>
        );
      })}
    </div>
  );

  return (
    <main className="portal-page">
      <div className="page-stack">
        <PageIntro
          eyebrow={tr("Player Portal", "Spielerportal")}
          title={portal.character?.name || portal.player_slot.display_name}
          description={tr("Manage your character, track party resources, and follow the current session from your own device.", "Verwalte deinen Charakter, verfolge Gruppenressourcen und behalte die aktuelle Sitzung auf deinem eigenen Gerät im Blick.")}
          actions={
            <div className="button-row">
              <StatusPill tone="ready">{tr("Player-Safe Zone", "Spielersicherer Bereich")}</StatusPill>
              <StatusPill tone={portal.player_slot.status === "ready" ? "ready" : portal.player_slot.status === "joined" ? "live" : "info"}>
                {statusLabel(portal.player_slot.status)}
              </StatusPill>
            </div>
          }
        />

        <div className="portal-shell">
          <aside className={`portal-nav${mobileMenuOpen ? " is-open" : ""}`}>
            <div className="portal-nav__header">
              <strong>{tr("Sections", "Bereiche")}</strong>
              <button className="studio-button studio-button--ghost studio-button--inline portal-nav__close" onClick={() => setMobileMenuOpen(false)} type="button">
                <X size={16} />
              </button>
            </div>
            <nav className="portal-nav__list" aria-label={tr("Player portal sections", "Spielerportal-Bereiche")}>
              {sectionEntries.map((item) => {
                const Icon = item.icon;
                return (
                  <button className={`portal-nav__item${activeSection === item.key ? " is-active" : ""}`} key={item.key} onClick={() => openSection(item.key)} type="button">
                    <span className="portal-nav__item-icon"><Icon size={16} /></span>
                    <span>{item.label}</span>
                    {item.bubble ? <span className="portal-nav__bubble">{item.bubble}</span> : null}
                  </button>
                );
              })}
            </nav>
          </aside>

          {mobileMenuOpen ? <button aria-label={tr("Close menu overlay", "Menü schließen")} className="portal-nav__backdrop" onClick={() => setMobileMenuOpen(false)} type="button" /> : null}

          <div className="portal-content">
            <div className="portal-topbar">
              <button className="studio-button studio-button--ghost portal-menu-button" onClick={() => setMobileMenuOpen((current) => !current)} type="button">
                <Menu size={16} />
                {tr("Menu", "Menü")}
              </button>
              <div className="portal-topbar__title">
                <strong>{sectionEntries.find((entry) => entry.key === activeSection)?.label}</strong>
                <span>{portal.session.name}</span>
              </div>
            </div>

            {activeSection === "overview" ? (
              <div className="dashboard-grid">
                <Panel className="portal-panel portal-panel--primary" title={tr("Character Overview", "Charakterübersicht")} description={tr("The most important values at a glance.", "Die wichtigsten Werte auf einen Blick.")}>
                  {portal.character ? (
                    <div className="page-stack">
                      <div className="portal-metric-grid">
                        {overviewMetrics.map((card) => (
                          <article className="ability-card portal-metric-card" key={card.label}>
                            <span>{card.label}</span>
                            <strong>{card.value}</strong>
                          </article>
                        ))}
                      </div>
                      <div className="ability-grid">
                        {Object.entries(portal.character.abilities || {}).map(([ability, value]) => (
                          <article className="ability-card" key={ability}>
                            <span>{abilityLabel(ability)}</span>
                            <strong>{value}</strong>
                          </article>
                        ))}
                      </div>
                      <div className="meta-list">
                        <div><dt>{tr("Features", "Merkmale")}</dt><dd>{features.join(", ") || "—"}</dd></div>
                        <div><dt>{tr("Senses", "Sinne")}</dt><dd>{senses || "—"}</dd></div>
                        <div><dt>{tr("Notes", "Notizen")}</dt><dd style={{ whiteSpace: "pre-wrap" }}>{sessionNotes || "—"}</dd></div>
                        <div><dt>{tr("Inventory", "Inventar")}</dt><dd style={{ whiteSpace: "pre-wrap" }}>{currentInventory || "—"}</dd></div>
                        <div><dt>{tr("Group Inventory", "Gruppeninventar")}</dt><dd>{`${groupGold}${textToRows(groupItems).length ? ` · ${textToRows(groupItems).join(", ")}` : ""}` || "—"}</dd></div>
                        <div><dt>{tr("Companions", "Companions")}</dt><dd>{visibleCompanions.map((companion) => companion.display_name || companion.character?.name || "—").join(", ") || "—"}</dd></div>
                      </div>
                      {overviewActionRows.length > 0 ? (
                        <div className="page-stack">
                          <strong>{tr("Actions", "Aktionen")}</strong>
                          {renderActionCards(overviewActionRows, "overview-actions")}
                        </div>
                      ) : null}
                    </div>
                  ) : (
                    <p className="empty-copy">{tr("No character is assigned to this slot yet.", "Diesem Spielerplatz ist noch kein Charakter zugewiesen.")}</p>
                  )}
                </Panel>

                <Panel className="portal-panel portal-panel--secondary" title={tr("Character Tracking", "Charakterverwaltung")} description={tr("Edit HP, money, notes, inventory, and group inventory.", "TP, Geld, Notizen, Inventar und Gruppeninventar bearbeiten.")}>
                  {portal.character ? (
                    <div className="form-grid">
                      <label>
                        <span>{tr("Current HP", "Aktuelle TP")}</span>
                        <input className="studio-input" inputMode="numeric" onChange={(event) => setCurrentHitPoints(event.target.value)} value={currentHitPoints} />
                      </label>
                      <label>
                        <span>{tr("Temporary HP", "Temporäre TP")}</span>
                        <input className="studio-input" inputMode="numeric" onChange={(event) => setTemporaryHitPoints(event.target.value)} value={temporaryHitPoints} />
                      </label>
                      <label>
                        <span>{tr("Current Money", "Aktuelles Geld")}</span>
                        <input className="studio-input" onChange={(event) => setCurrentMoney(event.target.value)} placeholder={tr("e.g. 12 gp, 4 sp", "z. B. 12 GM, 4 SM")} value={currentMoney} />
                      </label>
                      <label>
                        <span>{tr("XP", "EP")}</span>
                        <input className="studio-input" onChange={(event) => setExperiencePoints(event.target.value)} value={experiencePoints} />
                      </label>
                      <label>
                        <span>{tr("Inspiration", "Inspiration")}</span>
                        <input className="studio-input" onChange={(event) => setInspiration(event.target.value)} placeholder={tr("yes / no or a short note", "ja / nein oder kurze Notiz")} value={inspiration} />
                      </label>
                      <label>
                        <span>{tr("Personal Session Notes", "Persönliche Sitzungsnotizen")}</span>
                        <textarea className="studio-textarea" onChange={(event) => setSessionNotes(event.target.value)} rows={4} value={sessionNotes} />
                      </label>
                      <label>
                        <span>{tr("Current Inventory", "Aktuelles Inventar")}</span>
                        <textarea className="studio-textarea" onChange={(event) => setCurrentInventory(event.target.value)} rows={6} value={currentInventory} />
                      </label>
                      <label>
                        <span>{tr("Group Gold", "Gruppengold")}</span>
                        <input className="studio-input" inputMode="numeric" onChange={(event) => setGroupGold(event.target.value)} value={groupGold} />
                      </label>
                      <label>
                        <span>{tr("Shared Items", "Geteilte Gegenstände")}</span>
                        <textarea className="studio-textarea" onChange={(event) => setGroupItems(event.target.value)} rows={4} value={groupItems} />
                      </label>
                      <label>
                        <span>{tr("Shared Notes", "Geteilte Notizen")}</span>
                        <textarea className="studio-textarea" onChange={(event) => setGroupNotes(event.target.value)} rows={4} value={groupNotes} />
                      </label>
                      <div className="button-row">
                        <button className="studio-button" disabled={isPending} onClick={handleSaveCharacter} type="button">
                          {tr("Save character changes", "Charakteränderungen speichern")}
                        </button>
                        <button className="studio-button studio-button--ghost" disabled={isPending} onClick={handleSaveGroupInventory} type="button">
                          <Coins size={16} />
                          {tr("Save group inventory", "Gruppeninventar speichern")}
                        </button>
                      </div>
                    </div>
                  ) : (
                    <p className="empty-copy">{tr("Assign a character first to edit tracking values.", "Weise zuerst einen Charakter zu, um Werte zu bearbeiten.")}</p>
                  )}
                </Panel>
              </div>
            ) : null}

            {activeSection === "session" ? (
              <div className="dashboard-grid">
                <Panel className="portal-panel portal-panel--secondary" title={tr("Join Status", "Beitrittsstatus")} description={tr("Choose a character and mark yourself ready for the session.", "Charakter wählen und für den Sitzungsstart bereit melden.")}>
                  <div className="form-grid">
                    <select onChange={(event) => setSelectedCharacterId(event.target.value)} value={selectedCharacterId}>
                      <option value="">{tr("Choose character", "Charakter wählen")}</option>
                      {portal.available_characters.map((character) => (
                        <option key={character.id} value={character.id}>
                          {character.name} · {character.race || "—"} · {character.class_and_level || "—"}
                        </option>
                      ))}
                    </select>
                    <div className="button-row">
                      <button className="studio-button studio-button--ghost" disabled={isPending || !selectedCharacterId} onClick={handleAssignCharacter} type="button">
                        {tr("Assign character", "Charakter übernehmen")}
                      </button>
                      <Link className="studio-button studio-button--ghost" href={`/characters?portal_token=${encodeURIComponent(portal.token)}`}>
                        {tr("Create new character", "Neuen Charakter erstellen")}
                      </Link>
                      <button className="studio-button" disabled={isPending || !portal.character} onClick={handleSetReady} type="button">
                        <UserRoundCheck size={16} />
                        {tr("Ready", "Bereit")}
                      </button>
                    </div>
                  </div>
                </Panel>
                <Panel className="portal-panel portal-panel--primary" title={tr("Session Overview", "Sitzungsübersicht")} description={tr("The current player-safe narration and session state.", "Die aktuelle spielersichere Erzählung und der Sitzungsstatus.")}>
                  <div className="portal-feed">
                    <article className="story-box story-box--hero">
                      <Sparkles size={18} />
                      <div>
                        <strong>{tr("Current scene", "Aktuelle Szene")}</strong>
                        <p>{releasedNarration}</p>
                      </div>
                    </article>
                    <div className="portal-meta-grid">
                      <article className="scope-card"><strong>{tr("Session", "Sitzung")}</strong><p>{portal.session.name}</p></article>
                      <article className="scope-card"><strong>{tr("Ruleset", "Regelwerk")}</strong><p>{portal.session.ruleset_work} {portal.session.ruleset_version}</p></article>
                      <article className="scope-card"><strong>{tr("Status", "Status")}</strong><p>{portal.session.status}</p></article>
                      <article className="scope-card"><strong>{tr("Board mode", "Board-Modus")}</strong><p>{portal.session.state.visual_mode || "pause_or_recap"}</p></article>
                    </div>
                    {visibleCompanions.length > 0 ? (
                      <div className="meta-list">
                        <div>
                          <dt>{tr("Companions", "Companions")}</dt>
                          <dd>{visibleCompanions.map((companion) => {
                            const summary = companion.character ? `${companion.character.race || "—"} · ${companion.character.class_and_level || "—"}` : "";
                            return `${companion.display_name || companion.character?.name || "—"}${summary ? ` (${summary})` : ""}`;
                          }).join(", ")}</dd>
                        </div>
                      </div>
                    ) : null}
                  </div>
                </Panel>
              </div>
            ) : null}

            {activeSection === "gear" ? (
              <div className="dashboard-grid">
                <Panel className="portal-panel portal-panel--full" title={tr("Equipment", "Ausrüstung")} description={tr("All tracked gear and combat equipment.", "Alle erfassten Gegenstände und Kampfausrüstung.")}>
                  {portal.character ? (
                    <div className="page-stack">
                      <div className="meta-list">
                        <div><dt>{tr("Inventory", "Inventar")}</dt><dd style={{ whiteSpace: "pre-wrap" }}>{currentInventory || "—"}</dd></div>
                        <div><dt>{tr("Weapon Notes", "Waffennotizen")}</dt><dd>{weaponNotes.join(", ") || "—"}</dd></div>
                        <div><dt>{tr("Feature Notes", "Merkmalsnotizen")}</dt><dd style={{ whiteSpace: "pre-wrap" }}>{featureNotes || "—"}</dd></div>
                      </div>
                      {combatActionRows.length > 0 ? renderActionCards(combatActionRows, "gear-combat") : <p className="empty-copy">{tr("No combat attacks entered yet.", "Noch keine Kampfangriffe eingetragen.")}</p>}
                    </div>
                  ) : (
                    <p className="empty-copy">{tr("No character details available yet.", "Noch keine Charakterdetails verfügbar.")}</p>
                  )}
                </Panel>
              </div>
            ) : null}

            {activeSection === "spells" ? (
              <div className="dashboard-grid">
                <Panel className="portal-panel portal-panel--full" title={tr("Spells", "Zauber")} description={tr("Spell attacks, spell list, and short notes.", "Zauberangriffe, Zauberliste und Kurznotizen.")}>
                  {portal.character ? (
                    <div className="page-stack">
                      {spellActionRows.length > 0 ? renderActionCards(spellActionRows, "spell-attacks") : <p className="empty-copy">{tr("No spell attacks entered yet.", "Noch keine Zauberangriffe eingetragen.")}</p>}
                      <div className="portal-spell-notes">
                        <strong>{tr("Spell Notes", "Zaubernotizen")}</strong>
                        {spellNoteRows.length > 0 ? (
                          <div className="portal-spell-notes__list">
                            {spellNoteRows.map((row, index) => (
                              <article className="portal-spell-note" key={`spell-note-${index}`}>
                                <strong>{row.name}</strong>
                                <p>{row.description || "—"}</p>
                              </article>
                            ))}
                          </div>
                        ) : (
                          <p className="empty-copy">{tr("No spell notes entered yet.", "Noch keine Zaubernotizen eingetragen.")}</p>
                        )}
                      </div>
                      <article className="story-box"><div><strong>{tr("Spells", "Zauber")}</strong><p>{spellList.join(", ") || "—"}</p></div></article>
                    </div>
                  ) : (
                    <p className="empty-copy">{tr("No spell details available yet.", "Noch keine Zauberdetails verfügbar.")}</p>
                  )}
                </Panel>
              </div>
            ) : null}

            {activeSection === "details" ? (
              <div className="dashboard-grid">
                <Panel className="portal-panel portal-panel--full" title={tr("Character Details", "Charakterdetails")} description={tr("Background and lower-priority sheet details.", "Hintergrund und weniger kritische Bogendetails.")}>
                  {portal.character ? (
                    <div className="meta-list">
                      <div><dt>{tr("Ancestry", "Volk")}</dt><dd>{portal.character.race || "—"}</dd></div>
                      <div><dt>{tr("Class", "Klasse")}</dt><dd>{portal.character.class_and_level || "—"}</dd></div>
                      <div><dt>{tr("Background", "Hintergrund")}</dt><dd>{portal.character.background || "—"}</dd></div>
                      <div><dt>{tr("Alignment", "Gesinnung")}</dt><dd>{portal.character.alignment || "—"}</dd></div>
                      <div><dt>{tr("Languages", "Sprachen")}</dt><dd>{languages.join(", ") || "—"}</dd></div>
                      <div><dt>{tr("Senses", "Sinne")}</dt><dd>{senses || "—"}</dd></div>
                      <div><dt>{tr("Features", "Merkmale")}</dt><dd>{features.join(", ") || "—"}</dd></div>
                    </div>
                  ) : (
                    <p className="empty-copy">{tr("No character is assigned to this slot yet.", "Diesem Spielerplatz ist noch kein Charakter zugewiesen.")}</p>
                  )}
                </Panel>
              </div>
            ) : null}

            {activeSection === "private" ? (
              <div className="dashboard-grid">
                <Panel className="portal-panel portal-panel--full" title={tr("Private DM Chat", "Privater DM-Chat")} description={tr("A confidential sidebar between you and the AI DM.", "Ein vertraulicher Nebenkanal zwischen dir und dem KI-Spielleiter.")}>
                  <div className="page-stack">
                    <div className="story-box">
                      <LockKeyhole size={16} />
                      <div>
                        <strong>{tr("Confidential scope", "Vertraulicher Kanal")}</strong>
                        <p>{tr("Use this for secret intentions, side deals, private observations, or one-on-one DM questions.", "Nutze das für geheime Absichten, Nebenabreden, private Beobachtungen oder direkte Eins-zu-eins-Fragen an den DM.")}</p>
                      </div>
                    </div>
                    <div className="portal-private-chat">
                      {privateChatLoading ? (
                        <p className="muted-copy">{tr("Loading private chat…", "Privater Chat wird geladen…")}</p>
                      ) : privateMessages.length === 0 ? (
                        <p className="empty-copy">{tr("No private messages yet.", "Noch keine privaten Nachrichten.")}</p>
                      ) : (
                        privateMessages.map((message) => (
                          <article className={`story-box ${message.role === "user" ? "" : "story-box--hero"}`} key={message.id}>
                            <div>
                              <strong>{message.role === "user" ? tr("You privately", "Du privat") : "AI DM"}</strong>
                              <p>{message.content}</p>
                            </div>
                          </article>
                        ))
                      )}
                    </div>
                    <label>
                      <span>{tr("Private message to the AI DM", "Private Nachricht an den KI-DM")}</span>
                      <textarea className="studio-textarea" onChange={(event) => setPrivateDraft(event.target.value)} rows={4} value={privateDraft} />
                    </label>
                    <div className="button-row">
                      <button className="studio-button" disabled={isPending || !privateDraft.trim()} onClick={handleSendPrivateChat} type="button">
                        <Send size={16} />
                        {tr("Send privately", "Privat senden")}
                      </button>
                    </div>
                  </div>
                </Panel>
              </div>
            ) : null}

            {activeSection === "handouts" ? (
              <div className="dashboard-grid">
                <Panel className="portal-panel portal-panel--primary" title={tr("Handouts & Media", "Handouts & Medien")} description={tr("Only explicitly released content appears here.", "Nur ausdrücklich freigegebene Inhalte erscheinen hier.")}>
                  <div className="portal-assets">
                    {visibleHandouts.length === 0 && visibleMedia.length === 0 ? <p className="empty-copy">{tr("No released handouts or media yet.", "Noch keine Handouts oder Medien freigegeben.")}</p> : null}
                    {visibleHandouts.map((item) => (
                      <article className="portal-asset-card" key={String(item.id)}>
                        <div className="portal-asset-card__preview portal-asset-card__preview--icon"><FileText size={22} /></div>
                        <div className="portal-asset-card__body">
                          <div className="button-row">
                            <StatusPill tone="info">Handout</StatusPill>
                            <StatusPill tone="ready">{tr("Released", "Freigegeben")}</StatusPill>
                          </div>
                          <strong>{String(item.name)}</strong>
                          <p>{contentTypeLabel(String(item.type || "document"))}</p>
                        </div>
                        <a className="studio-button studio-button--ghost studio-button--inline" href={`${apiBaseUrl}/api/documents/${String(item.id)}/file`} rel="noreferrer" target="_blank">
                          {tr("Open", "Öffnen")}
                        </a>
                      </article>
                    ))}
                    {visibleMedia.map((item) => {
                      const mimeType = String(item.mime_type || "");
                      const isImage = mimeType.startsWith("image/");
                      const assetUrl = `${apiBaseUrl}/api/assets/${String(item.id)}/file`;
                      return (
                        <article className="portal-asset-card" key={String(item.id)}>
                          <div className={`portal-asset-card__preview${isImage ? "" : " portal-asset-card__preview--icon"}`}>
                            {isImage ? <img alt={String(item.name)} className="portal-asset-image" src={assetUrl} /> : <ImageIcon size={22} />}
                          </div>
                          <div className="portal-asset-card__body">
                            <div className="button-row">
                              <StatusPill tone="default">{tr("Media", "Medien")}</StatusPill>
                              <StatusPill tone="ready">{tr("Player Safe", "Spielersicher")}</StatusPill>
                            </div>
                            <strong>{String(item.name)}</strong>
                            <p>{contentTypeLabel(String(item.type || "asset"))}</p>
                          </div>
                          <a className="studio-button studio-button--ghost studio-button--inline" href={assetUrl} rel="noreferrer" target="_blank">
                            {tr("Open", "Öffnen")}
                          </a>
                        </article>
                      );
                    })}
                  </div>
                </Panel>

                <Panel className="portal-panel portal-panel--secondary" title={tr("Safety", "Sicherheit")} description={tr("The portal remains strictly player-safe.", "Das Portal bleibt strikt spielersicher.")}>
                  <div className="story-box">
                    <Shield size={16} />
                    <div>
                      <strong>{tr("Player-safe route", "Spielersicherer Zugang")}</strong>
                      <p>{tr("Only released handouts, media, and player-visible session data appear here. GM notes remain hidden.", "Nur freigegebene Handouts, Medien und spielersichtbare Sitzungsdaten erscheinen hier. Spielleiter-Notizen bleiben verborgen.")}</p>
                    </div>
                  </div>
                </Panel>
              </div>
            ) : null}
          </div>
        </div>
      </div>
    </main>
  );
}
