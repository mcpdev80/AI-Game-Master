"use client";

import { useState, useTransition } from "react";
import { apiUpload, type ZipImportReport } from "../lib/api";

type Props = {
  onCreated: (report: ZipImportReport) => void;
};

export function CreateAdventureModal({ onCreated }: Props) {
  const [isOpen, setIsOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [language, setLanguage] = useState("de");
  const [pdfFile, setPdfFile] = useState<File | null>(null);
  const [zipFile, setZipFile] = useState<File | null>(null);
  const [error, setError] = useState("");
  const [isPending, startTransition] = useTransition();

  function resetForm() {
    setName("");
    setDescription("");
    setLanguage("de");
    setPdfFile(null);
    setZipFile(null);
    setError("");
  }

  return (
    <>
      <section className="panel hero-panel">
        <p className="label">Abenteuer Import</p>
        <h2 className="section-title">Neues Abenteuer mit PDF und Zusatzinhalten</h2>
        <p className="meta-copy">
          Lege ein Abenteuer in einem Schritt an. Das PDF wird Pflicht, das Ressourcen-ZIP ist optional und wird
          automatisch demselben Abenteuer zugeordnet.
        </p>
        <button type="button" onClick={() => setIsOpen(true)}>
          Neues Abenteuer
        </button>
      </section>

      {isOpen ? (
        <div className="modal-backdrop" role="presentation" onClick={() => !isPending && setIsOpen(false)}>
          <div className="modal-panel" role="dialog" aria-modal="true" aria-labelledby="create-adventure-title" onClick={(event) => event.stopPropagation()}>
            <form
              className="stack-form"
              onSubmit={(event) => {
                event.preventDefault();
                if (!pdfFile) {
                  setError("Bitte das Abenteuer-PDF auswaehlen.");
                  return;
                }

                setError("");
                startTransition(async () => {
                  try {
                    const formData = new FormData();
                    formData.append("name", name);
                    formData.append("description", description);
                    formData.append("language", language);
                    formData.append("pdf", pdfFile);
                    if (zipFile) {
                      formData.append("resources_zip", zipFile);
                    }

                    const report = await apiUpload<ZipImportReport>("/api/adventures/create-package", formData);
                    onCreated(report);
                    resetForm();
                    setIsOpen(false);
                  } catch (err) {
                    setError(err instanceof Error ? err.message : "Adventure import failed");
                  }
                });
              }}
            >
              <div className="modal-header">
                <div>
                  <p className="label">Neues Abenteuer</p>
                  <h2 className="section-title" id="create-adventure-title">
                    PDF und Zusatzressourcen gemeinsam importieren
                  </h2>
                </div>
                <button type="button" className="ghost-button" onClick={() => !isPending && setIsOpen(false)}>
                  Schliessen
                </button>
              </div>

              <input value={name} onChange={(event) => setName(event.target.value)} placeholder="Abenteuername" required />
              <textarea value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Beschreibung optional" rows={3} />
              <input value={language} onChange={(event) => setLanguage(event.target.value)} placeholder="Sprache" />

              <label className="file-field">
                <span>Abenteuer-PDF</span>
                <input type="file" accept=".pdf,application/pdf" onChange={(event) => setPdfFile(event.target.files?.[0] ?? null)} required />
              </label>

              <label className="file-field">
                <span>Zusatzressourcen ZIP optional</span>
                <input type="file" accept=".zip,application/zip" onChange={(event) => setZipFile(event.target.files?.[0] ?? null)} />
              </label>

              <button type="submit" disabled={isPending}>
                {isPending ? "Importiert..." : "Abenteuer anlegen"}
              </button>
              {error ? <p className="error-text">{error}</p> : null}
            </form>
          </div>
        </div>
      ) : null}
    </>
  );
}
