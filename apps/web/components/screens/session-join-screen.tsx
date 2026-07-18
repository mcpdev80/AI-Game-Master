"use client";

import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { ArrowRight, Users } from "lucide-react";
import { joinSession, type SessionJoinPreview } from "../../lib/api";
import { useI18n } from "../../lib/i18n";

export function SessionJoinScreen({ token, preview }: { token: string; preview: SessionJoinPreview | null }) {
  const router = useRouter();
  const { tr } = useI18n();
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();
  const [joiningNew, setJoiningNew] = useState(!preview?.has_progress);

  function handleJoin() {
    if (!displayName.trim()) {
      setError(tr("Please enter a display name.", "Bitte einen Anzeigenamen eingeben."));
      return;
    }
    setError(null);
    startTransition(async () => {
      try {
        const response = await joinSession(token, { display_name: displayName.trim() });
        router.push(`/player-portal/${response.portal_token}`);
        router.refresh();
      } catch (joinError) {
        setError(joinError instanceof Error ? joinError.message : tr("Join failed", "Beitritt fehlgeschlagen"));
      }
    });
  }

  function handleResumeExisting(playerSlotId: string) {
    setError(null);
    startTransition(async () => {
      try {
        const response = await joinSession(token, { player_slot_id: playerSlotId });
        router.push(`/player-portal/${response.portal_token}`);
        router.refresh();
      } catch (joinError) {
        setError(joinError instanceof Error ? joinError.message : tr("Resume failed", "Wiederaufnahme fehlgeschlagen"));
      }
    });
  }

  return (
    <main className="join-screen">
      <section className="join-card">
        <p className="eyebrow">{tr("Session Join", "Session-Beitritt")}</p>
        <h1>{tr("Join the session", "Der Runde beitreten")}</h1>
        <p>
          {preview?.has_progress
            ? tr("This session is being resumed. You can take over an existing character or join as a new player.", "Diese Session wird wieder aufgenommen. Du kannst einen bestehenden Charakter der Runde übernehmen oder neu hinzukommen.")
            : tr("Use this general join link, then choose an available character or create a new one.", "Du trittst der Session über den allgemeinen Beitrittslink bei und wählst danach einen freien Charakter oder erstellst einen neuen.")}
        </p>
        <div className="hint-box">
          <Users size={16} />
          <span>
            {preview?.has_progress
              ? tr("When resuming, you can reclaim an existing character or join as a new player.", "Bei einer Wiederaufnahme kannst du einen vorhandenen Charakter der Runde wieder aufnehmen oder als neuer Spieler dazukommen.")
              : tr("After joining, choose your character in the portal and mark yourself ready.", "Nach dem Beitritt kannst du im Portal deinen Charakter wählen und dich als bereit melden.")}
          </span>
        </div>
        {preview?.has_progress && preview.existing_players.length > 0 ? (
          <div className="form-grid">
            {preview.existing_players.map((entry) => (
              <div className="scope-card" key={entry.player_slot.id}>
                <strong>{entry.character?.name || entry.player_slot.display_name}</strong>
                <p>
                  {entry.character
                    ? `${entry.character.race || "—"} · ${entry.character.class_and_level || "—"}`
                    : tr("Occupied player slot", "Bereits belegter Spielerplatz")}
                </p>
                <p>{tr("Player name", "Spielername")}: {entry.player_slot.display_name}</p>
                <button className="studio-button studio-button--ghost" disabled={isPending} onClick={() => handleResumeExisting(entry.player_slot.id)} type="button">
                  {tr("Take over this character", "Diesen Charakter übernehmen")}
                </button>
              </div>
            ))}
            <button className="studio-button studio-button--ghost" disabled={isPending} onClick={() => setJoiningNew((current) => !current)} type="button">
              {joiningNew ? tr("Hide new-player form", "Formular für neue Spieler ausblenden") : tr("I am joining as a new player", "Ich komme neu hinzu")}
            </button>
          </div>
        ) : null}
        {joiningNew ? (
          <div className="form-grid">
            <input onChange={(event) => setDisplayName(event.target.value)} placeholder={tr("Your display name", "Dein Anzeigename")} value={displayName} />
          </div>
        ) : null}
        {error ? <p className="error-copy">{error}</p> : null}
        {joiningNew ? (
          <div className="button-row">
            <button className="studio-button" disabled={isPending} onClick={handleJoin} type="button">
              {isPending ? tr("Joining...", "Beitritt läuft...") : tr("Join as new player", "Neu beitreten")}
              <ArrowRight size={16} />
            </button>
          </div>
        ) : null}
      </section>
    </main>
  );
}
