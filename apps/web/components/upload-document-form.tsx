"use client";

import { useState, useTransition } from "react";
import { apiUpload, type Document } from "../lib/api";

type Props = {
  onCreated: (document: Document) => void;
};

export function UploadDocumentForm({ onCreated }: Props) {
  const [type, setType] = useState("adventure");
  const [name, setName] = useState("");
  const [language, setLanguage] = useState("de");
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState("");
  const [isPending, startTransition] = useTransition();

  return (
    <form
      className="stack-form panel"
      onSubmit={(event) => {
        event.preventDefault();
        if (!file) {
          setError("Bitte eine Datei auswaehlen.");
          return;
        }

        setError("");
        startTransition(async () => {
          try {
            const formData = new FormData();
            formData.append("file", file);
            formData.append("type", type);
            formData.append("name", name || file.name);
            formData.append("language", language);

            const created = await apiUpload<Document>("/api/documents/upload", formData);
            setFile(null);
            setName("");
            onCreated(created);
          } catch (err) {
            setError(err instanceof Error ? err.message : "Upload failed");
          }
        });
      }}
    >
      <p className="label">PDF oder Asset hochladen</p>
      <input type="file" onChange={(event) => setFile(event.target.files?.[0] ?? null)} required />
      <select value={type} onChange={(event) => setType(event.target.value)}>
        <option value="rules">Rules</option>
        <option value="adventure">Adventure</option>
        <option value="character_sheet">Character Sheet</option>
        <option value="asset">Asset</option>
      </select>
      <input value={name} onChange={(event) => setName(event.target.value)} placeholder="Anzeigename" />
      <input value={language} onChange={(event) => setLanguage(event.target.value)} placeholder="Sprache" />
      <button type="submit" disabled={isPending}>
        {isPending ? "Laedt hoch..." : "Datei hochladen"}
      </button>
      {error ? <p className="error-text">{error}</p> : null}
    </form>
  );
}
