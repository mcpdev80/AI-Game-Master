"use client";

import { useState } from "react";
import type { Campaign, Session } from "../lib/api";
import { CreateCampaignForm } from "./create-campaign-form";
import { CreateSessionForm } from "./create-session-form";

type Props = {
  initialCampaigns: Campaign[];
  initialSessions: Session[];
};

export function CampaignsWorkspace({ initialCampaigns, initialSessions }: Props) {
  const [campaigns, setCampaigns] = useState(initialCampaigns);
  const [sessions, setSessions] = useState(initialSessions);

  return (
    <section className="dashboard-grid">
      <CreateCampaignForm onCreated={(campaign) => setCampaigns((current) => [campaign, ...current])} />
      <CreateSessionForm
        campaigns={campaigns}
        onCreated={(session) => setSessions((current) => [session, ...current])}
      />
      <section className="panel">
        <p className="label">Kampagnen</p>
        <div className="list-grid">
          {campaigns.map((campaign) => (
            <article className="panel inset-panel" key={campaign.id}>
              <p className="label">{campaign.name}</p>
              <p>{campaign.description || "Ohne Beschreibung"}</p>
            </article>
          ))}
          {campaigns.length === 0 ? <p>Noch keine Kampagnen vorhanden.</p> : null}
        </div>
      </section>
      <section className="panel">
        <p className="label">Sessions</p>
        <div className="list-grid">
          {sessions.map((session) => (
            <article className="panel inset-panel" key={session.id}>
              <p className="label">{session.current_scene || "Unbenannte Szene"}</p>
              <p>
                {session.current_location || "Ohne Ort"} | {session.language} | {session.status}
              </p>
            </article>
          ))}
          {sessions.length === 0 ? <p>Noch keine Sessions vorhanden.</p> : null}
        </div>
      </section>
    </section>
  );
}
