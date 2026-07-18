"use client";

import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { ArrowRight, Users } from "lucide-react";
import { joinSession, type SessionJoinPreview } from "../../lib/api";

export function SessionJoinScreen({ token, preview }: { token: string; preview: SessionJoinPreview | null }) {
  const router = useRouter();
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();
  const [joiningNew, setJoiningNew] = useState(!preview?.has_progress);

  function handleJoin() {
    if (!displayName.trim()) {
      setError("Bitte einen Anzeigenamen eingeben.");
      return;
    }
    setError(null);
    startTransition(async () => {
      try {
        const response = await joinSession(token, { display_name: displayName.trim() });
        router.push(`/player-portal/${response.portal_token}`);
        router.refresh();
      } catch (joinError) {
        setError(joinError instanceof Error ? joinError.message : "Join fehlgeschlagen");
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
        setError(joinError instanceof Error ? joinError.message : "Wiederaufnahme fehlgeschlagen");
      }
    });
  }

  return (
    <main className="join-screen">
      <section className="join-card">
        <p className="eyebrow">Session Join</p>
        <h1>Der Runde beitreten</h1>
        <p>
          {preview?.has_progress
            ? "Diese Session wird wieder aufgenommen. Du kannst einen bestehenden Charakter der Runde übernehmen oder neu hinzukommen."
            : "Du trittst der Session ueber den allgemeinen Join-Link bei und waehlst danach einen freien Charakter oder erstellst einen neuen."}
        </p>
        <div className="hint-box">
          <Users size={16} />
          <span>
            {preview?.has_progress
              ? "Bei einer Wiederaufnahme kannst du einen vorhandenen Charakter der Runde wieder aufnehmen oder als neuer Spieler dazukommen."
              : "Nach dem Join kannst du im Portal deinen Charakter waehlen und dich als ready melden."}
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
                    : "Bereits belegter Spieler-Slot"}
                </p>
                <p>Spielername: {entry.player_slot.display_name}</p>
                <button className="studio-button studio-button--ghost" disabled={isPending} onClick={() => handleResumeExisting(entry.player_slot.id)} type="button">
                  Diesen Charakter übernehmen
                </button>
              </div>
            ))}
            <button className="studio-button studio-button--ghost" disabled={isPending} onClick={() => setJoiningNew((current) => !current)} type="button">
              {joiningNew ? "Neuen Join ausblenden" : "Ich komme neu hinzu"}
            </button>
          </div>
        ) : null}
        {joiningNew ? (
          <div className="form-grid">
            <input onChange={(event) => setDisplayName(event.target.value)} placeholder="Dein Anzeigename" value={displayName} />
          </div>
        ) : null}
        {error ? <p className="error-copy">{error}</p> : null}
        {joiningNew ? (
          <div className="button-row">
            <button className="studio-button" disabled={isPending} onClick={handleJoin} type="button">
              {isPending ? "Joining..." : "Neu beitreten"}
              <ArrowRight size={16} />
            </button>
          </div>
        ) : null}
      </section>
    </main>
  );
}
