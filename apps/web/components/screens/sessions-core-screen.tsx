"use client";

import Link from "next/link";
import { useMemo, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { Plus, Trash2 } from "lucide-react";
import { PageIntro, StatCard, StatusPill } from "../studio-primitives";
import { useNotifications } from "../notifications-provider";
import { useI18n } from "../../lib/i18n";
import {
  apiPost,
  deleteSession,
  splitMetadataList,
  type Adventure,
  type Campaign,
  type Character,
  type Document,
  type Session,
} from "../../lib/api";

type Props = {
  sessions: Session[];
  campaigns: Campaign[];
  characters: Character[];
  adventures: Adventure[];
  documents: Document[];
};

type CompanionDraft = {
  id: string;
  template_character_id: string;
  name: string;
  tactics_note: string;
};

function statusTone(status: string): "default" | "ready" | "warning" | "live" | "info" {
  switch (status) {
    case "live":
      return "live";
    case "paused":
      return "warning";
    case "ready_to_start":
      return "ready";
    default:
      return "default";
  }
}

function rulesetGroups(documents: Document[]) {
  const groups = new Map<string, { work: string; version: string; label: string }>();
  for (const document of documents) {
    if (document.type !== "rules") continue;
    let work = String(document.metadata.ruleset_work ?? "").trim();
    let version = String(document.metadata.ruleset_version ?? "").trim();
    if (!work || !version) {
      const derivedKeys = splitMetadataList(document.metadata.ruleset_keys);
      const firstKey = derivedKeys[0] ?? "";
      if (firstKey.includes(":")) {
        const [derivedWork, derivedVersion] = firstKey.split(":");
        work = work || derivedWork.trim();
        version = version || derivedVersion.trim();
      }
    }
    if (!work || !version) continue;
    const key = `${work}:${version}`;
    if (!groups.has(key)) {
      groups.set(key, { work, version, label: `${work} ${version}` });
    }
  }
  return Array.from(groups.values()).sort((a, b) => a.label.localeCompare(b.label, "de"));
}

function ensureFallbackRulesets(groups: { work: string; version: string; label: string }[], adventures: Adventure[]) {
  if (groups.length > 0) {
    return groups;
  }
  const has5EAdventure = adventures.some((adventure) => splitMetadataList(adventure.metadata.compatible_rulesets).includes("5E:2014"));
  if (has5EAdventure) {
    return [{ work: "5E", version: "2014", label: "5E-compatible (2014)" }];
  }
  return groups;
}

function sessionAdventure(session: Session, adventures: Adventure[]) {
  return adventures.find((adventure) => adventure.id === session.adventure_id) ?? null;
}

export function SessionsCoreScreen({ sessions, campaigns, characters, adventures, documents }: Props) {
  const router = useRouter();
  const { locale, tr } = useI18n();
  const { notify } = useNotifications();
  const [isPending, startTransition] = useTransition();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [rulesetKey, setRulesetKey] = useState("");
  const [adventureId, setAdventureId] = useState("");
  const [targetPlayers, setTargetPlayers] = useState("4");
  const [useCompanions, setUseCompanions] = useState(false);
  const [companionDrafts, setCompanionDrafts] = useState<CompanionDraft[]>([]);
  const [error, setError] = useState<string | null>(null);
  const statusLabel = (status: string) => ({
    live: tr("live", "live"),
    paused: tr("paused", "pausiert"),
    ready_to_start: tr("ready to start", "startbereit"),
    ended: tr("ended", "beendet"),
    draft: tr("draft", "Entwurf"),
  }[status] ?? status);

  const availableRulesets = useMemo(() => ensureFallbackRulesets(rulesetGroups(documents), adventures), [documents, adventures]);
  const sortedCharacters = useMemo(
    () => [...characters].sort((a, b) => a.name.localeCompare(b.name, locale === "de" ? "de" : "en")),
    [characters, locale],
  );
  const rulesetAdventures = useMemo(() => {
    if (!rulesetKey) return adventures;
    return adventures.filter((adventure) => {
      const compatible = splitMetadataList(adventure.metadata.compatible_rulesets);
      return compatible.length === 0 || compatible.includes(rulesetKey);
    });
  }, [adventures, rulesetKey]);

  function resetModal() {
    setName("");
    setRulesetKey("");
    setAdventureId("");
    setTargetPlayers("4");
    setUseCompanions(false);
    setCompanionDrafts([]);
    setError(null);
  }

  function addCompanionDraft() {
    setCompanionDrafts((current) => {
      if (current.length >= 3) {
        return current;
      }
      return [
        ...current,
        {
          id: `${Date.now()}-${current.length}`,
          template_character_id: "",
          name: "",
          tactics_note: "",
        },
      ];
    });
  }

  function removeCompanionDraft(id: string) {
    setCompanionDrafts((current) => current.filter((item) => item.id !== id));
  }

  function updateCompanionDraft(id: string, patch: Partial<CompanionDraft>) {
    setCompanionDrafts((current) =>
      current.map((item) => {
        if (item.id !== id) {
          return item;
        }
        const next = { ...item, ...patch };
        if (patch.template_character_id !== undefined) {
          const template = sortedCharacters.find((character) => character.id === patch.template_character_id);
          if (template && !next.name.trim()) {
            next.name = template.name;
          }
        }
        return next;
      }),
    );
  }

  function openCreateModal() {
    resetModal();
    const firstRuleset = availableRulesets[0] ? `${availableRulesets[0].work}:${availableRulesets[0].version}` : "";
    const firstAdventure =
      firstRuleset === ""
        ? adventures[0]?.id ?? ""
        : adventures.find((adventure) => {
            const compatible = splitMetadataList(adventure.metadata.compatible_rulesets);
            return compatible.length === 0 || compatible.includes(firstRuleset);
          })?.id ?? adventures[0]?.id ?? "";
    setRulesetKey(firstRuleset);
    setAdventureId(firstAdventure);
    setIsCreateOpen(true);
  }

  function handleCreate() {
    const [rulesetWork, rulesetVersion] = rulesetKey.split(":");
    const adventure = adventures.find((item) => item.id === adventureId) ?? null;
    const campaignId = adventure?.campaign_id ?? campaigns[0]?.id ?? "";
    if (!name.trim() || !rulesetWork || !rulesetVersion || !adventureId || !campaignId) {
      setError(tr("Choose a session name, ruleset, and adventure.", "Bitte Sitzungsname, Regelwerk und Abenteuer auswählen."));
      return;
    }
    if (useCompanions) {
      if (companionDrafts.length < 1 || companionDrafts.length > 3) {
        setError(tr("Add between 1 and 3 companion NPCs.", "Bitte 1 bis 3 Begleiter-NPCs hinzufügen."));
        return;
      }
      for (const draft of companionDrafts) {
        if (!draft.template_character_id || !draft.name.trim()) {
          setError(tr("Each companion needs a template and a new name.", "Jeder Begleiter braucht eine Vorlage und einen neuen Namen."));
          return;
        }
      }
    }
    setError(null);
    startTransition(async () => {
      try {
        const created = await apiPost<Session>("/api/sessions", {
          name: name.trim(),
          campaign_id: campaignId,
          adventure_id: adventureId,
          ruleset_work: rulesetWork,
          ruleset_version: rulesetVersion,
          target_player_count: Number(targetPlayers) || 4,
          language: locale,
          companion_templates: useCompanions
            ? companionDrafts.map((draft) => ({
                template_character_id: draft.template_character_id,
                name: draft.name.trim(),
                tactics_note: draft.tactics_note.trim(),
              }))
            : [],
        });
        notify({ title: tr("Session created", "Sitzung erstellt"), message: tr(`${created.name} is ready for players to join.`, `${created.name} ist für den Spielerbeitritt bereit.`), tone: "success" });
        setIsCreateOpen(false);
        router.refresh();
      } catch (createError) {
        setError(createError instanceof Error ? createError.message : tr("Could not create session.", "Sitzung konnte nicht erstellt werden."));
      }
    });
  }

  function handleDelete(session: Session) {
    if (!window.confirm(tr(`Delete session "${session.name}"?`, `Session „${session.name}“ wirklich löschen?`))) {
      return;
    }
    startTransition(async () => {
      try {
        await deleteSession(session.id);
        notify({ title: tr("Session deleted", "Sitzung gelöscht"), message: tr(`${session.name} was removed.`, `${session.name} wurde entfernt.`), tone: "success" });
        router.refresh();
      } catch (deleteError) {
        notify({ title: "Session", message: deleteError instanceof Error ? deleteError.message : tr("Delete failed.", "Löschen fehlgeschlagen."), tone: "error" });
      }
    });
  }

  return (
    <div className="page-stack">
      <PageIntro
        eyebrow={tr("Sessions", "Sitzungen")}
        title={tr("AI DM Sessions", "KI-Spielleiter-Sitzungen")}
        description={tr("Sessions connect the adventure, ruleset, player join flow, shared visual board, voice, ambience, and dice.", "Sitzungen verbinden Abenteuer, Regelwerk, Spielerbeitritt, gemeinsame Bildanzeige, Sprache, Atmosphäre und Würfel.")}
        actions={
          <div className="page-actions">
            <button className="studio-button" onClick={openCreateModal} type="button">
              {tr("Add Session", "Sitzung hinzufügen")}
            </button>
          </div>
        }
      />

      <div className="stat-grid">
        <StatCard label={tr("Sessions", "Sitzungen")} value={sessions.length} />
        <StatCard label="Live" value={sessions.filter((session) => session.status === "live").length} />
        <StatCard label={tr("Adventures", "Abenteuer")} value={adventures.length} />
        <StatCard label={tr("Rulebooks", "Regelbücher")} value={documents.filter((document) => document.type === "rules").length} />
        <StatCard label={tr("Characters", "Charaktere")} value={characters.length} />
      </div>

      <div className="session-card-grid">
        {sessions.map((session) => {
          const adventure = sessionAdventure(session, adventures);
          return (
            <article className="session-card" key={session.id}>
              <div className="asset-chip__head">
                <StatusPill tone={statusTone(session.status)}>{statusLabel(session.status)}</StatusPill>
              </div>
              <strong>{session.name}</strong>
              <p>{session.ruleset_work} {session.ruleset_version} · {adventure?.name || tr("No adventure", "Kein Abenteuer")}</p>
              <p className="muted-copy">{tr(`Planned for ${session.target_player_count} players`, `Geplant für ${session.target_player_count} Spieler`)}</p>
              <div className="button-row">
                <Link className="studio-button studio-button--ghost studio-button--inline" href={`/sessions/${session.id}`}>
                  {tr("Open Session", "Sitzung öffnen")}
                </Link>
                <Link className="studio-button studio-button--ghost studio-button--inline" href={`/session-join/${session.join_token}`}>
                  {tr("Join link", "Beitrittslink")}
                </Link>
                <button className="studio-button studio-button--danger studio-button--inline" onClick={() => handleDelete(session)} type="button">
                  <Trash2 size={16} />
                  {tr("Delete", "Löschen")}
                </button>
              </div>
            </article>
          );
        })}
      </div>

      {isCreateOpen ? (
        <div className="modal-overlay" onClick={() => !isPending && setIsCreateOpen(false)} role="presentation">
          <section aria-modal="true" className="modal-card" onClick={(event) => event.stopPropagation()} role="dialog">
            <div className="modal-card__header">
              <div>
                <h2 className="studio-panel__title">{tr("Add Session", "Sitzung hinzufügen")}</h2>
                <p className="studio-panel__description">{tr("Name, ruleset, adventure, and planned player count define the starting point.", "Name, Regelwerk, Abenteuer und geplante Spielerzahl definieren den Startpunkt der Runde.")}</p>
              </div>
              <button className="studio-button studio-button--ghost studio-button--inline" onClick={() => setIsCreateOpen(false)} type="button">
                {tr("Close", "Schließen")}
              </button>
            </div>

            <div className="form-grid">
              <input onChange={(event) => setName(event.target.value)} placeholder={tr("Session name", "Sitzungsname")} value={name} />
              <select onChange={(event) => setRulesetKey(event.target.value)} value={rulesetKey}>
                <option value="">{tr("Choose ruleset", "Regelwerk wählen")}</option>
                {availableRulesets.map((group) => (
                  <option key={`${group.work}:${group.version}`} value={`${group.work}:${group.version}`}>
                    {group.label}
                  </option>
                ))}
              </select>
              <p className="muted-copy">{tr("Rulesets found", "Gefundene Regelwerke")}: {availableRulesets.length}</p>
              <select onChange={(event) => setAdventureId(event.target.value)} value={adventureId}>
                <option value="">{tr("Choose adventure", "Abenteuer wählen")}</option>
                {rulesetAdventures.map((adventure) => (
                  <option key={adventure.id} value={adventure.id}>
                    {adventure.name}
                  </option>
                ))}
              </select>
              <input min={1} onChange={(event) => setTargetPlayers(event.target.value)} placeholder={tr("Planned player count", "Geplante Spielerzahl")} type="number" value={targetPlayers} />
            </div>

            <div className="studio-panel" style={{ marginTop: 16 }}>
              <div className="button-row" style={{ justifyContent: "space-between", alignItems: "center" }}>
                <div>
                  <strong>{tr("DM companion NPCs", "Begleiter-NPCs des Spielleiters")}</strong>
                  <p className="muted-copy">{tr("Optional: clone up to 3 existing characters into this session as DM-controlled companions.", "Optional: Bis zu 3 bestehende Charaktere als vom Spielleiter gesteuerte Begleiter in diese Sitzung klonen.")}</p>
                </div>
                <label className="button-row" style={{ gap: 8 }}>
                  <input
                    checked={useCompanions}
                    onChange={(event) => {
                      const checked = event.target.checked;
                      setUseCompanions(checked);
                      if (checked && companionDrafts.length === 0) {
                        addCompanionDraft();
                      }
                      if (!checked) {
                        setCompanionDrafts([]);
                      }
                    }}
                    type="checkbox"
                  />
                  <span>{tr("Enable", "Aktivieren")}</span>
                </label>
              </div>

              {useCompanions ? (
                <>
                  <div className="button-row" style={{ justifyContent: "flex-end", marginTop: 12 }}>
                    <button className="studio-button studio-button--ghost studio-button--inline" disabled={companionDrafts.length >= 3} onClick={addCompanionDraft} type="button">
                      <Plus size={16} />
                      {tr("Add companion", "Begleiter hinzufügen")}
                    </button>
                  </div>

                  <div className="list-stack" style={{ marginTop: 12 }}>
                    {companionDrafts.map((draft, index) => (
                      <div className="session-card" key={draft.id}>
                        <div className="button-row" style={{ justifyContent: "space-between", alignItems: "center" }}>
                          <strong>{tr(`Companion ${index + 1}`, `Begleiter ${index + 1}`)}</strong>
                          <button className="studio-button studio-button--danger studio-button--inline" onClick={() => removeCompanionDraft(draft.id)} type="button">
                            <Trash2 size={16} />
                            {tr("Remove", "Entfernen")}
                          </button>
                        </div>
                        <div className="form-grid" style={{ marginTop: 12 }}>
                          <select onChange={(event) => updateCompanionDraft(draft.id, { template_character_id: event.target.value })} value={draft.template_character_id}>
                            <option value="">{tr("Choose template character", "Vorlagencharakter wählen")}</option>
                            {sortedCharacters.map((character) => (
                              <option key={character.id} value={character.id}>
                                {character.name} · {character.class_and_level || tr("No class", "Keine Klasse")}
                              </option>
                            ))}
                          </select>
                          <input onChange={(event) => updateCompanionDraft(draft.id, { name: event.target.value })} placeholder={tr("New companion name", "Neuer Begleitername")} value={draft.name} />
                          <input onChange={(event) => updateCompanionDraft(draft.id, { tactics_note: event.target.value })} placeholder={tr("Optional tactic or role", "Optionale Taktik oder Rolle")} value={draft.tactics_note} />
                        </div>
                      </div>
                    ))}
                  </div>
                </>
              ) : null}
            </div>

            {error ? <p className="error-copy">{error}</p> : null}

            <div className="modal-card__footer">
              <span className="modal-card__spacer">{isPending ? tr("Creating...", "Wird erstellt...") : ""}</span>
              <div className="button-row modal-card__actions">
                <button className="studio-button studio-button--ghost" disabled={isPending} onClick={() => setIsCreateOpen(false)} type="button">
                  {tr("Cancel", "Abbrechen")}
                </button>
                <button className="studio-button" disabled={isPending} onClick={handleCreate} type="button">
                  {tr("Create Session", "Sitzung erstellen")}
                </button>
              </div>
            </div>
          </section>
        </div>
      ) : null}
    </div>
  );
}
