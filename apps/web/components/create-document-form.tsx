"use client";

import { useState, useTransition } from "react";
import { apiPost, type Document } from "../lib/api";

type Props = {
  onCreated: (document: Document) => void;
};

export function CreateDocumentForm({ onCreated }: Props) {
  const [type, setType] = useState("adventure");
  const [name, setName] = useState("");
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
            const created = await apiPost<Document>("/api/documents", {
              type,
              name,
              metadata: {
                source_type: type,
                language,
              },
            });
            setName("");
            onCreated(created);
          } catch (err) {
            setError(err instanceof Error ? err.message : "Document creation failed");
          }
        });
      }}
    >
      <p className="label">Dokument-Metadaten</p>
      <select value={type} onChange={(e) => setType(e.target.value)}>
        <option value="rules">Rules</option>
        <option value="adventure">Adventure</option>
        <option value="character_sheet">Character Sheet</option>
        <option value="asset">Asset</option>
      </select>
      <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Dateiname" required />
      <input value={language} onChange={(e) => setLanguage(e.target.value)} placeholder="Sprache" />
      <button type="submit" disabled={isPending}>
        {isPending ? "Speichert..." : "Dokument anlegen"}
      </button>
      {error ? <p className="error-text">{error}</p> : null}
    </form>
  );
}
