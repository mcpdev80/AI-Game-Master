"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { CheckCircle2, Link2, Lock, RefreshCw, Users } from "lucide-react";
import { PageIntro, Panel, StatusPill } from "../studio-primitives";
import { apiPut } from "../../lib/api";
import type { Campaign, PlayerLinkSlot, Session } from "../../lib/api";

type SessionsScreenProps = {
  sessions: Session[];
  campaigns: Campaign[];
  playerLinks: PlayerLinkSlot[];
};

function statusTone(status: string): "default" | "ready" | "warning" | "live" | "info" {
  switch (status) {
    case "live":
      return "live";
    case "ready":
      return "ready";
    case "paused":
      return "warning";
    default:
      return "default";
  }
}

export function SessionsScreen({ sessions, campaigns, playerLinks }: SessionsScreenProps) {
  const featured = sessions[0] ?? null;
  const router = useRouter();

  async function handleRegenerate(playerSlotId: string) {
    await apiPut(`/api/player-slots/${playerSlotId}/regenerate-link`, {});
    router.refresh();
  }

  async function handleToggleLock(slot: PlayerLinkSlot) {
    const isRevoked = Boolean(slot.link?.revoked_at);
    if (isRevoked) {
      await apiPut(`/api/player-slots/${slot.player_slot.id}/regenerate-link`, {});
    } else {
      await apiPut(`/api/player-slots/${slot.player_slot.id}/link-state`, { revoked: true });
    }
    router.refresh();
  }

  async function handleCopy(slot: PlayerLinkSlot) {
    if (!slot.join_url) {
      return;
    }
    await navigator.clipboard.writeText(slot.join_url);
  }

  return (
    <div className="page-stack">
      <PageIntro
        eyebrow="Sessions"
        title="Create the session, then let the AI DM run it"
        description="Sessions bind together the campaign, adventure, rulesets, player links, devices, and output targets. The human operator prepares the room, then the AI leads the table."
        actions={<button className="studio-button">New Session</button>}
      />

      <section className="dashboard-grid">
        <Panel
          title="Session Index"
          description="Draft, ready, live, paused, or finished sessions with direct entry into the active operator view."
          className="hero-panel"
        >
          <div className="list-stack">
            {sessions.map((session) => {
              const campaign = campaigns.find((item) => item.id === session.campaign_id);
              return (
                <article className="list-row list-row--session" key={session.id}>
                  <div className="list-row__body">
                    <strong>{campaign?.name || "Campaign session"}</strong>
                    <p>
                      {session.current_scene || "No scene yet"} · {session.current_location || "No location yet"}
                    </p>
                  </div>
                  <StatusPill tone={statusTone(session.status)}>{session.status}</StatusPill>
                  <Link className="studio-button studio-button--ghost studio-button--inline" href={`/sessions/${session.id}`}>
                    Open Session
                  </Link>
                </article>
              );
            })}
          </div>
        </Panel>
      </section>

      <section className="dashboard-grid">
        <Panel title="Session Wizard Structure" description="The screen hierarchy the final UI will implement.">
          <ol className="wizard-list">
            <li>Choose campaign and attached adventure package</li>
            <li>Select the rulebooks the AI DM should consult</li>
            <li>Assign characters and prepare player slots</li>
            <li>Verify camera, audio, player screen, and local portal routing</li>
            <li>Review readiness, then start the AI-led session</li>
          </ol>
        </Panel>

        <Panel title="Player Invite Flow" description="Each slot gets its own local link that can be shared in the LAN, for example via WhatsApp.">
          <div className="list-stack">
            {playerLinks.length === 0 ? (
              <p className="empty-copy">No player links yet. Once a session has slots, invite tokens appear here.</p>
            ) : (
              playerLinks.map((slot) => (
                <article className="invite-row" key={slot.player_slot.id}>
                  <div className="invite-row__main">
                    <div className="list-row__body">
                      <strong>{slot.player_slot.display_name}</strong>
                      <p>{slot.player_slot.status === "joined" ? "Joined from player portal" : "Waiting for player join"}</p>
                    </div>
                    <div className="meta-chip-row">
                      <StatusPill tone={slot.player_slot.status === "joined" ? "ready" : slot.player_slot.status === "locked" ? "warning" : "info"}>
                        {slot.player_slot.status}
                      </StatusPill>
                      {slot.join_url ? <code className="inline-code">{slot.join_url}</code> : <span className="muted-copy">Link revoked</span>}
                    </div>
                  </div>
                  <div className="icon-button-row">
                    <button
                      className="icon-button"
                      aria-label="Regenerate link"
                      onClick={() => handleRegenerate(slot.player_slot.id)}
                      title="Regenerate link"
                      type="button"
                    >
                      <RefreshCw size={16} />
                    </button>
                    <button
                      className="icon-button"
                      aria-label="Toggle lock"
                      onClick={() => handleToggleLock(slot)}
                      title={slot.link?.revoked_at ? "Generate fresh invite link" : "Lock or revoke"}
                      type="button"
                    >
                      <Lock size={16} />
                    </button>
                    <button
                      className="icon-button"
                      aria-label="Copy invite"
                      disabled={!slot.join_url}
                      onClick={() => handleCopy(slot)}
                      title="Copy invite"
                      type="button"
                    >
                      <Link2 size={16} />
                    </button>
                  </div>
                </article>
              ))
            )}
          </div>
          <div className="hint-box">
            <strong>Tip:</strong> Share the links via WhatsApp, email, or another messaging app. Players can join before or during the session.
          </div>
        </Panel>

        <Panel title="Current Readiness Snapshot" description="How the operator should think about readiness before handing control to the AI.">
          <div className="readiness-list">
            <div className="readiness-row">
              <CheckCircle2 size={16} />
              <span>Adventure and rules context imported</span>
            </div>
            <div className="readiness-row">
              <Users size={16} />
              <span>{playerLinks.filter((slot) => slot.player_slot.status === "joined").length} players already joined through local links</span>
            </div>
            <div className="readiness-row">
              <Link2 size={16} />
              <span>{featured ? "Active session selected for AI handoff" : "No active session selected yet"}</span>
            </div>
          </div>
        </Panel>
      </section>
    </div>
  );
}
