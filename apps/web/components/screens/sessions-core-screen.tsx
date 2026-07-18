"use client";

import Link from "next/link";
import { useMemo, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { Trash2 } from "lucide-react";
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
  const [error, setError] = useState<string | null>(null);

  const availableRulesets = useMemo(() => ensureFallbackRulesets(rulesetGroups(documents), adventures), [documents, adventures]);
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
    setError(null);
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
      setError(tr("Choose a session name, ruleset, and adventure.", "Bitte Session-Name, Regelwerk und Abenteuer auswählen."));
      return;
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
        });
        notify({ title: tr("Session created", "Session erstellt"), message: tr(`${created.name} is ready for players to join.`, `${created.name} ist für den Spielerbeitritt bereit.`), tone: "success" });
        setIsCreateOpen(false);
        router.refresh();
      } catch (createError) {
        setError(createError instanceof Error ? createError.message : tr("Could not create session.", "Session konnte nicht erstellt werden."));
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
        notify({ title: tr("Session deleted", "Session gelöscht"), message: tr(`${session.name} was removed.`, `${session.name} wurde entfernt.`), tone: "success" });
        router.refresh();
      } catch (deleteError) {
        notify({ title: "Session", message: deleteError instanceof Error ? deleteError.message : tr("Delete failed.", "Löschen fehlgeschlagen."), tone: "error" });
      }
    });
  }

  return (
    <div className="page-stack">
      <PageIntro
        eyebrow={tr("Sessions", "Sessions")}
        title={tr("AI GM Sessions", "KI-Spielleiter-Sessions")}
        description={tr("Sessions connect the adventure, ruleset, player join flow, shared visual board, voice, ambience, and dice.", "Sessions verbinden Abenteuer, Regelwerk, Spielerbeitritt, gemeinsames Visual Board, Sprache, Atmosphäre und Würfel.")}
        actions={
          <div className="page-actions">
            <button className="studio-button" onClick={openCreateModal} type="button">
              {tr("Add Session", "Session hinzufügen")}
            </button>
          </div>
        }
      />

      <div className="stat-grid">
        <StatCard label="Sessions" value={sessions.length} />
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
                <StatusPill tone={statusTone(session.status)}>{session.status}</StatusPill>
              </div>
              <strong>{session.name}</strong>
              <p>{session.ruleset_work} {session.ruleset_version} · {adventure?.name || tr("No adventure", "Kein Abenteuer")}</p>
              <p className="muted-copy">{tr(`Planned for ${session.target_player_count} players`, `Geplant für ${session.target_player_count} Spieler`)}</p>
              <div className="button-row">
                <Link className="studio-button studio-button--ghost studio-button--inline" href={`/sessions/${session.id}`}>
                  {tr("Open Session", "Session öffnen")}
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
                <h2 className="studio-panel__title">{tr("Add Session", "Session hinzufügen")}</h2>
                <p className="studio-panel__description">{tr("Name, ruleset, adventure, and planned player count define the starting point.", "Name, Regelwerk, Abenteuer und geplante Spielerzahl definieren den Startpunkt der Runde.")}</p>
              </div>
              <button className="studio-button studio-button--ghost studio-button--inline" onClick={() => setIsCreateOpen(false)} type="button">
                {tr("Close", "Schließen")}
              </button>
            </div>

            <div className="form-grid">
              <input onChange={(event) => setName(event.target.value)} placeholder={tr("Session name", "Session-Name")} value={name} />
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

            {error ? <p className="error-copy">{error}</p> : null}

            <div className="modal-card__footer">
              <span className="modal-card__spacer">{isPending ? tr("Creating...", "Wird erstellt...") : ""}</span>
              <div className="button-row modal-card__actions">
                <button className="studio-button studio-button--ghost" disabled={isPending} onClick={() => setIsCreateOpen(false)} type="button">
                  {tr("Cancel", "Abbrechen")}
                </button>
                <button className="studio-button" disabled={isPending} onClick={handleCreate} type="button">
                  {tr("Create Session", "Session erstellen")}
                </button>
              </div>
            </div>
          </section>
        </div>
      ) : null}
    </div>
  );
}
