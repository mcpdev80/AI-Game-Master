"use client";

import { useState, useTransition } from "react";
import { apiPost, type Campaign, type Session } from "../lib/api";

type Props = {
  campaigns: Campaign[];
  onCreated: (session: Session) => void;
};

export function CreateSessionForm({ campaigns, onCreated }: Props) {
  const [campaignId, setCampaignId] = useState(campaigns[0]?.id ?? "");
  const [currentScene, setCurrentScene] = useState("");
  const [currentLocation, setCurrentLocation] = useState("");
  const [language, setLanguage] = useState("de");
  const [error, setError] = useState("");
  const [isPending, startTransition] = useTransition();

  return (
    <form
      className="stack-form panel"
      onSubmit={(event) => {
        event.preventDefault();
        setError("");
        startTransition(async () => {
          try {
            const created = await apiPost<Session>("/api/sessions", {
              campaign_id: campaignId,
              current_scene: currentScene,
              current_location: currentLocation,
              language,
            });
            setCurrentScene("");
            setCurrentLocation("");
            onCreated(created);
          } catch (err) {
            setError(err instanceof Error ? err.message : "Session creation failed");
          }
        });
      }}
    >
      <p className="label">Neue Session</p>
      <select value={campaignId} onChange={(e) => setCampaignId(e.target.value)} required>
        {campaigns.map((campaign) => (
          <option key={campaign.id} value={campaign.id}>
            {campaign.name}
          </option>
        ))}
      </select>
      <input
        value={currentScene}
        onChange={(e) => setCurrentScene(e.target.value)}
        placeholder="Aktuelle Szene"
      />
      <input
        value={currentLocation}
        onChange={(e) => setCurrentLocation(e.target.value)}
        placeholder="Ort"
      />
      <input value={language} onChange={(e) => setLanguage(e.target.value)} placeholder="Sprache" />
      <button type="submit" disabled={isPending || campaigns.length === 0}>
        {isPending ? "Speichert..." : "Session anlegen"}
      </button>
      {campaigns.length === 0 ? <p className="error-text">Zuerst eine Kampagne anlegen.</p> : null}
      {error ? <p className="error-text">{error}</p> : null}
    </form>
  );
}
