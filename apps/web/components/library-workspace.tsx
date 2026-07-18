"use client";

import { useMemo, useState } from "react";
import type { Adventure, Asset, Document, ZipImportReport } from "../lib/api";
import { CreateAdventureModal } from "./create-adventure-modal";
import { CreateDocumentForm } from "./create-document-form";
import { ImportAdventureZipForm } from "./import-adventure-zip-form";
import { UploadDocumentForm } from "./upload-document-form";

type Props = {
  initialAdventures: Adventure[];
  initialDocuments: Document[];
  initialAssets: Asset[];
};

export function LibraryWorkspace({ initialAdventures, initialDocuments, initialAssets }: Props) {
  const [adventures, setAdventures] = useState(initialAdventures);
  const [documents, setDocuments] = useState(initialDocuments);
  const [assets, setAssets] = useState(initialAssets);

  const grouped = useMemo(() => {
    return adventures.map((adventure) => ({
      adventure,
      documents: documents.filter((document) => document.adventure_id === adventure.id),
      assets: assets.filter((asset) => asset.adventure_id === adventure.id),
    }));
  }, [adventures, documents, assets]);

  return (
    <section className="dashboard-grid">
      <CreateAdventureModal
        onCreated={(report: ZipImportReport) => {
          setAdventures((current) => [report.adventure, ...current]);
          setDocuments((current) => [...report.documents, ...current]);
          setAssets((current) => [...report.assets, ...current]);
        }}
      />

      <section className="panel">
        <p className="label">Weitere Importe</p>
        <p className="meta-copy">
          Einzelne Dokumente oder ZIPs kannst du weiterhin separat hochladen. Der Hauptweg fuer neue Abenteuer ist aber
          der kombinierte Dialog.
        </p>
        <div className="dashboard-grid compact-grid">
          <UploadDocumentForm onCreated={(document) => setDocuments((current) => [document, ...current])} />
          <ImportAdventureZipForm
            onImported={(report: ZipImportReport) => {
              setAdventures((current) => [report.adventure, ...current]);
              setDocuments((current) => [...report.documents, ...current]);
              setAssets((current) => [...report.assets, ...current]);
            }}
          />
          <CreateDocumentForm onCreated={(document) => setDocuments((current) => [document, ...current])} />
        </div>
      </section>

      <section className="panel">
        <p className="label">Abenteuer</p>
        <div className="list-grid">
          {grouped.map(({ adventure, documents: adventureDocuments, assets: adventureAssets }) => (
            <article className="panel inset-panel" key={adventure.id}>
              <p className="label">{adventure.name}</p>
              <p>{adventure.description || "Ohne Beschreibung"}</p>
              <p>Sprache: {adventure.language}</p>
              <p>Dokumente: {adventureDocuments.length}</p>
              <p>Assets: {adventureAssets.length}</p>
            </article>
          ))}
          {grouped.length === 0 ? <p>Noch keine Abenteuer vorhanden.</p> : null}
        </div>
      </section>

      <section className="panel">
        <p className="label">Dokumente</p>
        <div className="list-grid">
          {documents.map((document) => (
            <article className="panel inset-panel" key={document.id}>
              <p className="label">{document.name}</p>
              <p>Typ: {document.type}</p>
              <p>Adventure: {document.adventure_id ?? "keins"}</p>
              <p>Chunks: {document.chunk_count}</p>
              <p>Pfad: {document.source_file_path ?? "nur Metadaten"}</p>
            </article>
          ))}
        </div>
      </section>

      <section className="panel">
        <p className="label">Assets</p>
        <div className="list-grid">
          {assets.map((asset) => (
            <article className="panel inset-panel" key={asset.id}>
              <p className="label">{asset.name}</p>
              <p>Typ: {asset.type}</p>
              <p>Quelle: {asset.source_type}</p>
              <p>Adventure: {asset.adventure_id ?? "keins"}</p>
              <p>Entity: {asset.entity_name ?? "-"}</p>
              <p>Location: {asset.location_name ?? "-"}</p>
              <p>Tags: {asset.tags.join(", ") || "-"}</p>
            </article>
          ))}
          {assets.length === 0 ? <p>Noch keine Assets vorhanden.</p> : null}
        </div>
      </section>
    </section>
  );
}
