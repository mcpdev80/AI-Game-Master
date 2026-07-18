"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { ArrowRight, Shield } from "lucide-react";
import { joinPlayerPortal } from "../../lib/api";

export function PlayerJoinScreen({ token }: { token: string }) {
  const router = useRouter();
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
        setError(joinError instanceof Error ? joinError.message : "Join failed");
      }
    });
  }

  return (
    <main className="join-screen">
      <section className="join-card">
        <p className="eyebrow">Player Join</p>
        <h1>Join the local session portal</h1>
        <p>
          This link is intended for a single player slot inside the local network. After joining, the player lands in a portal that only shows released and player-safe information.
        </p>
        <div className="hint-box">
          <Shield size={16} />
          <span>Token-based access keeps the player route separate from the operator console.</span>
        </div>
        <div className="meta-list meta-list--stack">
          <div>
            <dt>Join token</dt>
            <dd>{token}</dd>
          </div>
          <div>
            <dt>Target</dt>
            <dd>/player-portal/{token}</dd>
          </div>
          <div>
            <dt>Access</dt>
            <dd>Only player-safe content released by the AI session operator appears after join.</dd>
          </div>
        </div>
        {error ? <p className="error-copy">{error}</p> : null}
        <div className="button-row">
          <button className="studio-button" disabled={isPending} onClick={handleJoin} type="button">
            {isPending ? "Joining..." : "Join Session"}
            <ArrowRight size={16} />
          </button>
          <Link className="studio-button studio-button--ghost" href={`/player-portal/${token}`}>
            Open Portal Directly
          </Link>
        </div>
      </section>
    </main>
  );
}
