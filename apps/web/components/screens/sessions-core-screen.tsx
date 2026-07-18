"use client";

import Link from "next/link";
import { useMemo, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { Trash2 } from "lucide-react";
import { PageIntro, StatCard, StatusPill } from "../studio-primitives";
import { useNotifications } from "../notifications-provider";
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
      setError("Bitte Session-Name, Regelwerk und Adventure auswaehlen.");
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
        });
        notify({ title: "Session erstellt", message: `${created.name} ist bereit fuer den Join-Flow.`, tone: "success" });
        setIsCreateOpen(false);
        router.refresh();
      } catch (createError) {
        setError(createError instanceof Error ? createError.message : "Session konnte nicht erstellt werden.");
      }
    });
  }

  function handleDelete(session: Session) {
    if (!window.confirm(`Session "${session.name}" wirklich loeschen?`)) {
      return;
    }
    startTransition(async () => {
      try {
        await deleteSession(session.id);
        notify({ title: "Session geloescht", message: `${session.name} wurde entfernt.`, tone: "success" });
        router.refresh();
      } catch (deleteError) {
        notify({ title: "Session", message: deleteError instanceof Error ? deleteError.message : "Loeschen fehlgeschlagen.", tone: "error" });
      }
    });
  }

  return (
    <div className="page-stack">
      <PageIntro
        eyebrow="Sessions"
        title="AI DM Sessions"
        description="Sessions verbinden Adventure, Regelwerk, Spieler-Join, den gemeinsamen Visual Board Screen sowie Voice-, Ambient- und Würfelfluss."
        actions={
          <div className="page-actions">
            <button className="studio-button" onClick={openCreateModal} type="button">
              Add Session
            </button>
          </div>
        }
      />

      <div className="stat-grid">
        <StatCard label="Sessions" value={sessions.length} />
        <StatCard label="Live" value={sessions.filter((session) => session.status === "live").length} />
        <StatCard label="Adventures" value={adventures.length} />
        <StatCard label="Rulebooks" value={documents.filter((document) => document.type === "rules").length} />
        <StatCard label="Characters" value={characters.length} />
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
              <p>{session.ruleset_work} {session.ruleset_version} · {adventure?.name || "Kein Adventure"}</p>
              <p className="muted-copy">Geplant fuer {session.target_player_count} Spieler</p>
              <div className="button-row">
                <Link className="studio-button studio-button--ghost studio-button--inline" href={`/sessions/${session.id}`}>
                  Open Session
                </Link>
                <Link className="studio-button studio-button--ghost studio-button--inline" href={`/session-join/${session.join_token}`}>
                  Join-Link
                </Link>
                <button className="studio-button studio-button--danger studio-button--inline" onClick={() => handleDelete(session)} type="button">
                  <Trash2 size={16} />
                  Delete
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
                <h2 className="studio-panel__title">Add Session</h2>
                <p className="studio-panel__description">Name, Regelwerk, Adventure und geplante Spielerzahl definieren den Startpunkt der Runde.</p>
              </div>
              <button className="studio-button studio-button--ghost studio-button--inline" onClick={() => setIsCreateOpen(false)} type="button">
                Close
              </button>
            </div>

            <div className="form-grid">
              <input onChange={(event) => setName(event.target.value)} placeholder="Session-Name" value={name} />
              <select onChange={(event) => setRulesetKey(event.target.value)} value={rulesetKey}>
                <option value="">Regelwerk waehlen</option>
                {availableRulesets.map((group) => (
                  <option key={`${group.work}:${group.version}`} value={`${group.work}:${group.version}`}>
                    {group.label}
                  </option>
                ))}
              </select>
              <p className="muted-copy">Gefundene Regelwerke: {availableRulesets.length}</p>
              <select onChange={(event) => setAdventureId(event.target.value)} value={adventureId}>
                <option value="">Adventure waehlen</option>
                {rulesetAdventures.map((adventure) => (
                  <option key={adventure.id} value={adventure.id}>
                    {adventure.name}
                  </option>
                ))}
              </select>
              <input min={1} onChange={(event) => setTargetPlayers(event.target.value)} placeholder="Geplante Spielerzahl" type="number" value={targetPlayers} />
            </div>

            {error ? <p className="error-copy">{error}</p> : null}

            <div className="modal-card__footer">
              <span className="modal-card__spacer">{isPending ? "Creating..." : ""}</span>
              <div className="button-row modal-card__actions">
                <button className="studio-button studio-button--ghost" disabled={isPending} onClick={() => setIsCreateOpen(false)} type="button">
                  Cancel
                </button>
                <button className="studio-button" disabled={isPending} onClick={handleCreate} type="button">
                  Create Session
                </button>
              </div>
            </div>
          </section>
        </div>
      ) : null}
    </div>
  );
}
