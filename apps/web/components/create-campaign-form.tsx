"use client";

import { useState, useTransition } from "react";
import { apiPost, type Campaign } from "../lib/api";

type Props = {
  onCreated: (campaign: Campaign) => void;
};

export function CreateCampaignForm({ onCreated }: Props) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
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
            const created = await apiPost<Campaign>("/api/campaigns", {
              name,
              description,
            });
            setName("");
            setDescription("");
            onCreated(created);
          } catch (err) {
            setError(err instanceof Error ? err.message : "Campaign creation failed");
          }
        });
      }}
    >
      <p className="label">Neue Kampagne</p>
      <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Name" required />
      <textarea
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        placeholder="Beschreibung"
        rows={4}
      />
      <button type="submit" disabled={isPending}>
        {isPending ? "Speichert..." : "Kampagne anlegen"}
      </button>
      {error ? <p className="error-text">{error}</p> : null}
    </form>
  );
}
