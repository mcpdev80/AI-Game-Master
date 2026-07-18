"use client";

import { useState, useTransition } from "react";
import { apiUpload, type ZipImportReport } from "../lib/api";

type Props = {
  onImported: (report: ZipImportReport) => void;
};

export function ImportAdventureZipForm({ onImported }: Props) {
  const [file, setFile] = useState<File | null>(null);
  const [adventureName, setAdventureName] = useState("");
  const [language, setLanguage] = useState("de");
  const [error, setError] = useState("");
  const [isPending, startTransition] = useTransition();

  return (
    <form
      className="stack-form panel"
      onSubmit={(event) => {
        event.preventDefault();
        if (!file) {
          setError("Bitte eine ZIP-Datei auswaehlen.");
          return;
        }

        setError("");
        startTransition(async () => {
          try {
            const formData = new FormData();
            formData.append("file", file);
            formData.append("adventure_name", adventureName || file.name.replace(/\.zip$/i, ""));
            formData.append("language", language);

            const report = await apiUpload<ZipImportReport>("/api/adventures/import-zip", formData);
            onImported(report);
            setFile(null);
            setAdventureName("");
          } catch (err) {
            setError(err instanceof Error ? err.message : "ZIP import failed");
          }
        });
      }}
    >
      <p className="label">Abenteuerpaket importieren</p>
      <input type="file" accept=".zip" onChange={(event) => setFile(event.target.files?.[0] ?? null)} required />
      <input value={adventureName} onChange={(event) => setAdventureName(event.target.value)} placeholder="Abenteuername" />
      <input value={language} onChange={(event) => setLanguage(event.target.value)} placeholder="Sprache" />
      <button type="submit" disabled={isPending}>
        {isPending ? "Importiert..." : "ZIP importieren"}
      </button>
      {error ? <p className="error-text">{error}</p> : null}
    </form>
  );
}
