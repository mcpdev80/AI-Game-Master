"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { ArrowRight, Shield } from "lucide-react";
import { joinPlayerPortal } from "../../lib/api";
import { useI18n } from "../../lib/i18n";

export function PlayerJoinScreen({ token }: { token: string }) {
  const router = useRouter();
  const { tr } = useI18n();
  const [error, setError] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();

  function handleJoin() {
    setError(null);
    startTransition(async () => {
      try {
        await joinPlayerPortal(token);
        router.push(`/player-portal/${token}`);
        router.refresh();
      } catch (joinError) {
        setError(joinError instanceof Error ? joinError.message : tr("Join failed", "Beitritt fehlgeschlagen"));
      }
    });
  }

  return (
    <main className="join-screen">
      <section className="join-card">
        <p className="eyebrow">{tr("Player Join", "Spielerbeitritt")}</p>
        <h1>{tr("Join the local session portal", "Dem lokalen Sitzungsportal beitreten")}</h1>
        <p>
          {tr("This link is intended for a single player slot inside the local network. After joining, the player lands in a portal that only shows released and player-safe information.", "Dieser Link ist für einen einzelnen Spielerplatz im lokalen Netzwerk bestimmt. Nach dem Beitritt zeigt das Portal ausschließlich freigegebene und spielersichere Informationen.")}
        </p>
        <div className="hint-box">
          <Shield size={16} />
          <span>{tr("Token-based access keeps the player route separate from the operator console.", "Der tokenbasierte Zugriff trennt den Spielerbereich von der Spielleiter-Konsole.")}</span>
        </div>
        <div className="meta-list meta-list--stack">
          <div>
            <dt>{tr("Join token", "Beitritts-Token")}</dt>
            <dd>{token}</dd>
          </div>
          <div>
            <dt>{tr("Target", "Ziel")}</dt>
            <dd>/player-portal/{token}</dd>
          </div>
          <div>
            <dt>{tr("Access", "Zugriff")}</dt>
            <dd>{tr("Only player-safe content released by the AI session operator appears after join.", "Nach dem Beitritt erscheinen nur spielersichere Inhalte, die von der Sitzungsleitung freigegeben wurden.")}</dd>
          </div>
        </div>
        {error ? <p className="error-copy">{error}</p> : null}
        <div className="button-row">
          <button className="studio-button" disabled={isPending} onClick={handleJoin} type="button">
            {isPending ? tr("Joining...", "Beitritt läuft …") : tr("Join Session", "Sitzung beitreten")}
            <ArrowRight size={16} />
          </button>
          <Link className="studio-button studio-button--ghost" href={`/player-portal/${token}`}>
            {tr("Open Portal Directly", "Portal direkt öffnen")}
          </Link>
        </div>
      </section>
    </main>
  );
}
