"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { FileText, ImageIcon, Shield, Sparkles, UserRoundCheck } from "lucide-react";
import { PageIntro, Panel, StatusPill } from "../studio-primitives";
import { useNotifications } from "../notifications-provider";
import { apiBaseUrl, updatePlayerSlotCharacter, updatePlayerSlotStatus, type PlayerPortalSession } from "../../lib/api";

export function PlayerPortalScreen({ portal }: { portal: PlayerPortalSession }) {
  const router = useRouter();
  const { notify } = useNotifications();
  const [isPending, startTransition] = useTransition();
  const [selectedCharacterId, setSelectedCharacterId] = useState(portal.character?.id ?? "");

  const releasedNarration =
    portal.session.state.scene_summary || portal.session.state.last_narration || "Die KI hat noch keine spielersichere Szene freigegeben.";
  const visibleHandouts = portal.visible_state.visible_handouts;
  const visibleMedia = portal.visible_state.visible_media;

  function handleAssignCharacter() {
    if (!selectedCharacterId) {
      notify({ title: "Player Portal", message: "Bitte zuerst einen Charakter waehlen.", tone: "warning" });
      return;
    }
    startTransition(async () => {
      try {
        await updatePlayerSlotCharacter(portal.player_slot.id, { character_id: selectedCharacterId });
        notify({ title: "Character", message: "Charakter wurde dem Spieler-Slot zugewiesen.", tone: "success" });
        router.refresh();
      } catch (error) {
        notify({ title: "Player Portal", message: error instanceof Error ? error.message : "Zuweisung fehlgeschlagen.", tone: "error" });
      }
    });
  }

  function handleSetReady() {
    startTransition(async () => {
      try {
        await updatePlayerSlotStatus(portal.player_slot.id, { status: "ready" });
        notify({ title: "Ready", message: "Du bist jetzt als ready markiert.", tone: "success" });
        router.refresh();
      } catch (error) {
        notify({ title: "Player Portal", message: error instanceof Error ? error.message : "Ready-Status fehlgeschlagen.", tone: "error" });
      }
    });
  }

  return (
    <main className="portal-page">
      <div className="page-stack">
        <PageIntro
          eyebrow="Player Portal"
          title={portal.character?.name || portal.player_slot.display_name}
          description="Spieler waehlen hier ihren Charakter, sehen freigegebene Inhalte und melden sich fuer den Session-Start als ready."
          actions={
            <div className="button-row">
              <StatusPill tone="ready">Player-Safe Zone</StatusPill>
              <StatusPill tone={portal.player_slot.status === "ready" ? "ready" : portal.player_slot.status === "joined" ? "live" : "info"}>
                {portal.player_slot.status}
              </StatusPill>
            </div>
          }
        />

        <div className="dashboard-grid">
          <Panel title="Join Status" description="Charakter waehlen und fuer den Session-Start bereit melden.">
            <div className="form-grid">
              <select onChange={(event) => setSelectedCharacterId(event.target.value)} value={selectedCharacterId}>
                <option value="">Charakter waehlen</option>
                {portal.available_characters.map((character) => (
                  <option key={character.id} value={character.id}>
                    {character.name} · {character.race || "—"} · {character.class_and_level || "—"}
                  </option>
                ))}
              </select>
              <div className="button-row">
                <button className="studio-button studio-button--ghost" disabled={isPending || !selectedCharacterId} onClick={handleAssignCharacter} type="button">
                  Charakter uebernehmen
                </button>
                <Link className="studio-button studio-button--ghost" href={`/characters?portal_token=${encodeURIComponent(portal.token)}`}>
                  Neuen Charakter erstellen
                </Link>
                <button className="studio-button" disabled={isPending || !portal.character} onClick={handleSetReady} type="button">
                  <UserRoundCheck size={16} />
                  Ready
                </button>
              </div>
              {!portal.character ? <p className="muted-copy">Falls dein Charakter fehlt, kannst du ihn direkt aus diesem Portal im Character Builder anlegen.</p> : null}
            </div>
          </Panel>

          <Panel title="Session Feed" description="Aktuell freigegebene Szene fuer diesen Spieler.">
            <div className="portal-feed">
              <article className="story-box story-box--hero">
                <Sparkles size={18} />
                <div>
                  <strong>Current scene</strong>
                  <p>{releasedNarration}</p>
                </div>
              </article>
              <div className="portal-meta-grid">
                <article className="scope-card">
                  <strong>Session</strong>
                  <p>{portal.session.name}</p>
                </article>
                <article className="scope-card">
                  <strong>Regelwerk</strong>
                  <p>{portal.session.ruleset_work} {portal.session.ruleset_version}</p>
                </article>
                <article className="scope-card">
                  <strong>Board Modus</strong>
                  <p>{portal.session.state.visual_mode || "pause_or_recap"}</p>
                </article>
              </div>
            </div>
          </Panel>

          <Panel title="Character" description="Der aktuell zugewiesene Charakter fuer diesen Slot.">
            {portal.character ? (
              <div className="character-summary">
                <div className="ability-grid">
                  {Object.entries(portal.character.abilities || {}).map(([ability, value]) => (
                    <article className="ability-card" key={ability}>
                      <span>{ability.toUpperCase()}</span>
                      <strong>{value}</strong>
                    </article>
                  ))}
                </div>
                <div className="meta-list">
                  <div>
                    <dt>Race</dt>
                    <dd>{portal.character.race || "Unknown"}</dd>
                  </div>
                  <div>
                    <dt>Class</dt>
                    <dd>{portal.character.class_and_level || "Unknown"}</dd>
                  </div>
                  <div>
                    <dt>Background</dt>
                    <dd>{portal.character.background || "Unknown"}</dd>
                  </div>
                  <div>
                    <dt>Player Slot</dt>
                    <dd>{portal.player_slot.display_name}</dd>
                  </div>
                </div>
              </div>
            ) : (
              <p className="empty-copy">Diesem Slot ist noch kein Charakter zugewiesen.</p>
            )}
          </Panel>

          <Panel title="Handouts & Media" description="Nur explizit freigegebene Inhalte erscheinen hier.">
            <div className="portal-assets">
              {visibleHandouts.length === 0 && visibleMedia.length === 0 ? (
                <p className="empty-copy">No released handouts or media yet.</p>
              ) : null}
              {visibleHandouts.map((item) => (
                <article className="portal-asset-card" key={String(item.id)}>
                  <div className="portal-asset-card__preview portal-asset-card__preview--icon">
                    <FileText size={22} />
                  </div>
                  <div className="portal-asset-card__body">
                    <div className="button-row">
                      <StatusPill tone="info">Handout</StatusPill>
                      <StatusPill tone="ready">Released</StatusPill>
                    </div>
                    <strong>{String(item.name)}</strong>
                    <p>{String(item.type || "document")}</p>
                  </div>
                  <a
                    className="studio-button studio-button--ghost studio-button--inline"
                    href={`${apiBaseUrl}/api/documents/${String(item.id)}/file`}
                    rel="noreferrer"
                    target="_blank"
                  >
                    Open
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
                        <StatusPill tone="default">Media</StatusPill>
                        <StatusPill tone="ready">Player Safe</StatusPill>
                      </div>
                      <strong>{String(item.name)}</strong>
                      <p>{String(item.type || "asset")}</p>
                    </div>
                    <a className="studio-button studio-button--ghost studio-button--inline" href={assetUrl} rel="noreferrer" target="_blank">
                      Open
                    </a>
                  </article>
                );
              })}
            </div>
          </Panel>

          <Panel title="Safety" description="Das Portal bleibt strikt spielersicher.">
            <div className="story-box">
              <Shield size={16} />
              <div>
                <strong>Player-safe route</strong>
                <p>Nur freigegebene Handouts, Medien und Charakterdaten erscheinen hier. DM-Notizen bleiben verborgen.</p>
              </div>
            </div>
          </Panel>
        </div>
      </div>
    </main>
  );
}
