"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { FileText, ImageIcon, Shield, Sparkles, UserRoundCheck } from "lucide-react";
import { PageIntro, Panel, StatusPill } from "../studio-primitives";
import { useNotifications } from "../notifications-provider";
import { apiBaseUrl, updatePlayerSlotCharacter, updatePlayerSlotStatus, type PlayerPortalSession } from "../../lib/api";
import { useI18n } from "../../lib/i18n";

export function PlayerPortalScreen({ portal }: { portal: PlayerPortalSession }) {
  const router = useRouter();
  const { locale, tr } = useI18n();
  const { notify } = useNotifications();
  const [isPending, startTransition] = useTransition();
  const [selectedCharacterId, setSelectedCharacterId] = useState(portal.character?.id ?? "");
  const statusLabel = (status: string) => (({ ready: tr("ready", "bereit"), joined: tr("joined", "beigetreten"), invited: tr("invited", "eingeladen") } as Record<string, string>)[status] ?? status);
  const abilityLabel = (ability: string) => locale === "de" ? (({ strength: "STÄ", dexterity: "GES", constitution: "KON", intelligence: "INT", wisdom: "WEI", charisma: "CHA" } as Record<string, string>)[ability] ?? ability.toUpperCase()) : ability.toUpperCase();
  const contentTypeLabel = (type: string) => (({ document: tr("document", "Dokument"), image: tr("image", "Bild"), map: tr("map", "Karte"), battlemap: tr("battle map", "Kampfkarte"), handout: tr("handout", "Handout"), asset: tr("asset", "Medium") } as Record<string, string>)[type] ?? type);

  const releasedNarration =
    portal.session.state.scene_summary || portal.session.state.last_narration || tr("The AI has not released a player-safe scene yet.", "Die KI hat noch keine spielersichere Szene freigegeben.");
  const visibleHandouts = portal.visible_state.visible_handouts;
  const visibleMedia = portal.visible_state.visible_media;

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

  return (
    <main className="portal-page">
      <div className="page-stack">
        <PageIntro
          eyebrow={tr("Player Portal", "Spielerportal")}
          title={portal.character?.name || portal.player_slot.display_name}
          description={tr("Choose your character, view released content, and mark yourself ready for the session.", "Wähle deinen Charakter, sieh freigegebene Inhalte und melde dich für den Sitzungsstart als bereit.")}
          actions={
            <div className="button-row">
              <StatusPill tone="ready">{tr("Player-Safe Zone", "Spielersicherer Bereich")}</StatusPill>
              <StatusPill tone={portal.player_slot.status === "ready" ? "ready" : portal.player_slot.status === "joined" ? "live" : "info"}>
                {statusLabel(portal.player_slot.status)}
              </StatusPill>
            </div>
          }
        />

        <div className="dashboard-grid">
          <Panel title={tr("Join Status", "Beitrittsstatus")} description={tr("Choose a character and mark yourself ready for the session.", "Charakter wählen und für den Sitzungsstart bereit melden.")}>
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
              {!portal.character ? <p className="muted-copy">{tr("If your character is missing, create it directly from this portal in the Character Builder.", "Falls dein Charakter fehlt, kannst du ihn direkt aus diesem Portal im Character Builder anlegen.")}</p> : null}
            </div>
          </Panel>

          <Panel title={tr("Session Feed", "Sitzungsverlauf")} description={tr("The scene currently released for this player.", "Die aktuell für diesen Spieler freigegebene Szene.")}>
            <div className="portal-feed">
              <article className="story-box story-box--hero">
                <Sparkles size={18} />
                <div>
                  <strong>{tr("Current scene", "Aktuelle Szene")}</strong>
                  <p>{releasedNarration}</p>
                </div>
              </article>
              <div className="portal-meta-grid">
                <article className="scope-card">
                  <strong>{tr("Session", "Sitzung")}</strong>
                  <p>{portal.session.name}</p>
                </article>
                <article className="scope-card">
                  <strong>{tr("Ruleset", "Regelwerk")}</strong>
                  <p>{portal.session.ruleset_work} {portal.session.ruleset_version}</p>
                </article>
                <article className="scope-card">
                  <strong>{tr("Board mode", "Board-Modus")}</strong>
                  <p>{portal.session.state.visual_mode || "pause_or_recap"}</p>
                </article>
              </div>
            </div>
          </Panel>

          <Panel title={tr("Character", "Charakter")} description={tr("The character currently assigned to this slot.", "Der aktuell diesem Spielerplatz zugewiesene Charakter.")}>
            {portal.character ? (
              <div className="character-summary">
                <div className="ability-grid">
                  {Object.entries(portal.character.abilities || {}).map(([ability, value]) => (
                    <article className="ability-card" key={ability}>
                      <span>{abilityLabel(ability)}</span>
                      <strong>{value}</strong>
                    </article>
                  ))}
                </div>
                <div className="meta-list">
                  <div>
                    <dt>{tr("Ancestry", "Volk")}</dt>
                    <dd>{portal.character.race || tr("Unknown", "Unbekannt")}</dd>
                  </div>
                  <div>
                    <dt>{tr("Class", "Klasse")}</dt>
                    <dd>{portal.character.class_and_level || tr("Unknown", "Unbekannt")}</dd>
                  </div>
                  <div>
                    <dt>{tr("Background", "Hintergrund")}</dt>
                    <dd>{portal.character.background || tr("Unknown", "Unbekannt")}</dd>
                  </div>
                  <div>
                    <dt>{tr("Player slot", "Spielerplatz")}</dt>
                    <dd>{portal.player_slot.display_name}</dd>
                  </div>
                </div>
              </div>
            ) : (
              <p className="empty-copy">{tr("No character is assigned to this slot yet.", "Diesem Spielerplatz ist noch kein Charakter zugewiesen.")}</p>
            )}
          </Panel>

          <Panel title={tr("Handouts & Media", "Handouts & Medien")} description={tr("Only explicitly released content appears here.", "Nur ausdrücklich freigegebene Inhalte erscheinen hier.")}>
            <div className="portal-assets">
              {visibleHandouts.length === 0 && visibleMedia.length === 0 ? (
                <p className="empty-copy">{tr("No released handouts or media yet.", "Noch keine Handouts oder Medien freigegeben.")}</p>
              ) : null}
              {visibleHandouts.map((item) => (
                <article className="portal-asset-card" key={String(item.id)}>
                  <div className="portal-asset-card__preview portal-asset-card__preview--icon">
                    <FileText size={22} />
                  </div>
                  <div className="portal-asset-card__body">
                    <div className="button-row">
                      <StatusPill tone="info">Handout</StatusPill>
                      <StatusPill tone="ready">{tr("Released", "Freigegeben")}</StatusPill>
                    </div>
                    <strong>{String(item.name)}</strong>
                    <p>{contentTypeLabel(String(item.type || "document"))}</p>
                  </div>
                  <a
                    className="studio-button studio-button--ghost studio-button--inline"
                    href={`${apiBaseUrl}/api/documents/${String(item.id)}/file`}
                    rel="noreferrer"
                    target="_blank"
                  >
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

          <Panel title={tr("Safety", "Sicherheit")} description={tr("The portal remains strictly player-safe.", "Das Portal bleibt strikt spielersicher.")}>
            <div className="story-box">
              <Shield size={16} />
              <div>
                <strong>{tr("Player-safe route", "Spielersicherer Zugang")}</strong>
                <p>{tr("Only released handouts, media, and character data appear here. GM notes remain hidden.", "Nur freigegebene Handouts, Medien und Charakterdaten erscheinen hier. Spielleiter-Notizen bleiben verborgen.")}</p>
              </div>
            </div>
          </Panel>
        </div>
      </div>
    </main>
  );
}
