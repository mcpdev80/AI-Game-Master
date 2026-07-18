"use client";

import { useRouter } from "next/navigation";
import { useMemo, useState, useTransition } from "react";
import { BookOpen, FileText, FolderPlus, ImageIcon, Layers, Plus, Trash2, Upload } from "lucide-react";
import { PageIntro, Panel, StatCard, StatusPill } from "../studio-primitives";
import { useNotifications } from "../notifications-provider";
import { apiDelete, apiUpload, splitMetadataList, type Adventure, type Asset, type Campaign, type Document, type ZipImportReport } from "../../lib/api";
import { useI18n } from "../../lib/i18n";

type Props = {
  campaigns: Campaign[];
  adventures: Adventure[];
  documents: Document[];
  assets: Asset[];
};

type TabKey = "overview" | "rulebooks" | "adventures" | "assets";
type ModalKind = "rules" | "adventure" | "asset" | null;
type AssetScopeFilter = "all" | "global" | "linked";

type AssetView = Asset & {
  rulesets: string[];
  linkedAdventureIds: string[];
  linkedAdventureNames: string[];
  previewUrl: string;
  isImage: boolean;
};

function deriveRuleset(document: Document) {
  const work = String(document.metadata?.ruleset_work ?? "");
  const version = String(document.metadata?.ruleset_version ?? "");
  if (work || version) {
    return {
      work: work || "Unassigned",
      version: version || "default",
    };
  }

  const fallback = splitMetadataList(document.metadata?.ruleset_keys)[0];
  if (fallback?.includes(":")) {
    const [derivedWork, derivedVersion] = fallback.split(":");
    return {
      work: derivedWork || "Unassigned",
      version: derivedVersion || "default",
    };
  }

  return {
    work: "Unassigned",
    version: "default",
  };
}

function documentKindLabel(document: Document, locale: "en" | "de") {
  const kind = String(document.metadata?.kind ?? "");
  if (kind === "character_builder_guide") return locale === "de" ? "Builder-Leitfaden" : "Builder Guide";
  if (kind === "level_up_guide") return locale === "de" ? "Stufenaufstiegs-Leitfaden" : "Level-Up Guide";
  if (kind === "short_rules_guide") return locale === "de" ? "Kurzregeln" : "Short Rules";
  return locale === "de" ? "Regelbuch" : "Rulebook";
}

export function LibraryCoreScreen({ campaigns, adventures, documents, assets }: Props) {
  const router = useRouter();
  const { locale, tr } = useI18n();
  const assetTypeLabel = (type: string) => (({
    image: tr("image", "Bild"), portrait: tr("portrait", "Porträt"), map: tr("map", "Karte"), battlemap: tr("battle map", "Kampfkarte"),
    token: tr("token", "Spielfigur"), handout: tr("handout", "Handout"), asset: tr("asset", "Medium"), audio: tr("audio", "Audio"),
  } as Record<string, string>)[type] ?? type);
  const { notify } = useNotifications();
  const [activeTab, setActiveTab] = useState<TabKey>("overview");
  const [activeModal, setActiveModal] = useState<ModalKind>(null);
  const [isPending, startTransition] = useTransition();
  const [error, setError] = useState<string | null>(null);

  const [language, setLanguage] = useState<string>(locale);
  const tabs: { key: TabKey; label: string }[] = [
    { key: "overview", label: tr("Overview", "Übersicht") },
    { key: "rulebooks", label: tr("Rulebooks", "Regelbücher") },
    { key: "adventures", label: tr("Adventures", "Abenteuer") },
    { key: "assets", label: tr("Assets", "Medien") },
  ];
  const [rulesName, setRulesName] = useState("");
  const [rulesFile, setRulesFile] = useState<File | null>(null);
  const [rulesetWork, setRulesetWork] = useState("5E");
  const [rulesetVersion, setRulesetVersion] = useState("5e");

  const [adventureName, setAdventureName] = useState("");
  const [adventureDescription, setAdventureDescription] = useState("");
  const [adventurePdf, setAdventurePdf] = useState<File | null>(null);
  const [adventureZip, setAdventureZip] = useState<File | null>(null);
  const [compatibleRulesets, setCompatibleRulesets] = useState("");

  const [assetName, setAssetName] = useState("");
  const [assetFile, setAssetFile] = useState<File | null>(null);
  const [assetType, setAssetType] = useState("image");
  const [assetRulesets, setAssetRulesets] = useState("");
  const [assetAdventureIds, setAssetAdventureIds] = useState("");
  const [assetTags, setAssetTags] = useState("");
  const [assetTypeFilter, setAssetTypeFilter] = useState("all");
  const [assetScopeFilter, setAssetScopeFilter] = useState<AssetScopeFilter>("all");

  const rulesDocuments = useMemo(() => documents.filter((document) => document.type === "rules"), [documents]);
  const rulesetGroups = useMemo(() => {
    const groups = new Map<string, Document[]>();
    for (const document of rulesDocuments) {
      const { work, version } = deriveRuleset(document);
      const key = `${work}::${version}`;
      const current = groups.get(key) ?? [];
      current.push(document);
      groups.set(key, current);
    }
    return [...groups.entries()].map(([key, items]) => {
      const [work, version] = key.split("::");
      return { key, work, version, items };
    });
  }, [rulesDocuments]);

  const rulebookCount = rulesDocuments.length;
  const rulesetCount = rulesetGroups.length;

  const adventureCards = useMemo(
    () =>
      adventures.map((adventure) => ({
        ...adventure,
        compatibleRulesets: splitMetadataList(adventure.metadata?.compatible_rulesets ?? adventure.metadata?.ruleset_keys),
      })),
    [adventures]
  );

  const assetGallery = useMemo(
    () =>
      assets.map((asset) => ({
        ...asset,
        rulesets: splitMetadataList(asset.metadata?.compatible_rulesets ?? asset.metadata?.ruleset_keys),
        linkedAdventureIds: splitMetadataList(asset.metadata?.adventure_ids),
        linkedAdventureNames: adventures
          .filter((adventure) => splitMetadataList(asset.metadata?.adventure_ids).includes(adventure.id))
          .map((adventure) => adventure.name),
        previewUrl: `/api/assets/${asset.id}/file`,
        isImage: asset.mime_type.startsWith("image/"),
      })),
    [adventures, assets]
  );

  const assetTypeOptions = useMemo(() => {
    return ["all", ...new Set(assetGallery.map((asset) => asset.type).filter(Boolean).sort((left, right) => left.localeCompare(right)))];
  }, [assetGallery]);

  const assetCategorySummary = useMemo(() => {
    return [...new Set(assetGallery.map((asset) => asset.type).filter(Boolean).sort((left, right) => left.localeCompare(right)))].map((type) => ({
      type,
      count: assetGallery.filter((asset) => asset.type === type).length,
    }));
  }, [assetGallery]);

  const filteredAssets = useMemo(() => {
    return assetGallery.filter((asset) => {
      if (assetTypeFilter !== "all" && asset.type !== assetTypeFilter) {
        return false;
      }
      if (assetScopeFilter === "global" && asset.linkedAdventureIds.length > 0) {
        return false;
      }
      if (assetScopeFilter === "linked" && asset.linkedAdventureIds.length === 0) {
        return false;
      }
      return true;
    });
  }, [assetGallery, assetScopeFilter, assetTypeFilter]);

  const assetGroups = useMemo(() => {
    const groups = new Map<string, AssetView[]>();
    for (const asset of filteredAssets) {
      const current = groups.get(asset.type) ?? [];
      current.push(asset);
      groups.set(asset.type, current);
    }
    return [...groups.entries()]
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([type, items]) => ({ type, items }));
  }, [filteredAssets]);

  function closeModal() {
    setActiveModal(null);
    setError(null);
  }

  function resetForms() {
    setRulesName("");
    setRulesFile(null);
    setRulesetWork("5E");
    setRulesetVersion("5e");
    setAdventureName("");
    setAdventureDescription("");
    setAdventurePdf(null);
    setAdventureZip(null);
    setCompatibleRulesets("");
    setAssetName("");
    setAssetFile(null);
    setAssetType("image");
    setAssetRulesets("");
    setAssetAdventureIds("");
    setAssetTags("");
    setLanguage(locale);
  }

  function finishSuccess(title: string, message: string) {
    notify({ title, message, tone: "success" });
    closeModal();
    resetForms();
    router.refresh();
  }

  function finishError(title: string, err: unknown) {
    const message = err instanceof Error ? err.message : tr("Unknown error", "Unbekannter Fehler");
    setError(message);
    notify({ title, message, tone: "error" });
  }

  function tabActionLabel(tab: TabKey) {
    switch (tab) {
      case "rulebooks":
        return tr("Add Rulebook", "Regelbuch hinzufügen");
      case "adventures":
        return tr("Add Adventure", "Abenteuer hinzufügen");
      case "assets":
        return tr("Add Asset", "Medium hinzufügen");
      default:
        return "Add";
    }
  }

  function openAddModalForTab(tab: TabKey) {
    if (tab === "rulebooks") setActiveModal("rules");
    if (tab === "adventures") setActiveModal("adventure");
    if (tab === "assets") setActiveModal("asset");
  }

  function fileUrl(kind: "documents" | "assets", id: string) {
    return `/api/${kind}/${id}/file`;
  }

  function handleRulesUpload() {
    if (!rulesFile) {
      setError(tr("Select a rulebook PDF.", "Bitte ein Regelbuch-PDF auswählen."));
      return;
    }
    setError(null);
    startTransition(async () => {
      try {
        const form = new FormData();
        form.append("file", rulesFile);
        form.append("type", "rules");
        form.append("name", rulesName || rulesFile.name);
        form.append("language", language);
        form.append("ruleset_work", rulesetWork);
        form.append("ruleset_version", rulesetVersion);
        await apiUpload<Document>("/api/documents/upload", form);
        finishSuccess(tr("Rulebook uploaded", "Regelbuch hochgeladen"), tr(`${rulesName || rulesFile.name} was saved.`, `${rulesName || rulesFile.name} wurde gespeichert.`));
      } catch (err) {
        finishError("Rulebook upload failed", err);
      }
    });
  }

  function handleAdventureUpload() {
    if (!adventurePdf || !adventureName.trim()) {
      setError(tr("Adventure name and PDF are required.", "Abenteuername und PDF sind erforderlich."));
      return;
    }
    setError(null);
    startTransition(async () => {
      try {
        const form = new FormData();
        form.append("name", adventureName);
        form.append("description", adventureDescription);
        form.append("language", language);
        form.append("pdf", adventurePdf);
        if (adventureZip) form.append("resources_zip", adventureZip);
        form.append("compatible_rulesets", compatibleRulesets);
        await apiUpload<ZipImportReport>("/api/adventures/create-package", form);
        finishSuccess(tr("Adventure imported", "Abenteuer importiert"), tr(`${adventureName} was imported.`, `${adventureName} wurde importiert.`));
      } catch (err) {
        finishError("Adventure import failed", err);
      }
    });
  }

  function handleAssetUpload() {
    if (!assetFile) {
      setError(tr("Select an asset file.", "Bitte eine Mediendatei auswählen."));
      return;
    }
    setError(null);
    startTransition(async () => {
      try {
        const form = new FormData();
        form.append("file", assetFile);
        form.append("name", assetName || assetFile.name);
        form.append("type", assetType);
        form.append("language", language);
        form.append("compatible_rulesets", assetRulesets);
        form.append("adventure_ids", assetAdventureIds);
        form.append("tags", assetTags);
        await apiUpload<Asset>("/api/assets/upload", form);
        finishSuccess(tr("Asset uploaded", "Medium hochgeladen"), tr(`${assetName || assetFile.name} was saved.`, `${assetName || assetFile.name} wurde gespeichert.`));
      } catch (err) {
        finishError("Asset upload failed", err);
      }
    });
  }

  function handleDelete(path: string, title: string, successMessage: string, confirmMessage: string) {
    if (typeof window !== "undefined" && !window.confirm(confirmMessage)) {
      return;
    }
    startTransition(async () => {
      try {
        await apiDelete<{ deleted: boolean; id: string }>(path);
        notify({ title, message: successMessage, tone: "success" });
        router.refresh();
      } catch (err) {
        finishError(`${title} failed`, err);
      }
    });
  }

  return (
    <div className="page-stack">
      <PageIntro
        eyebrow={tr("Library", "Bibliothek")}
        title={tr("Campaign knowledge, rulebooks, adventures, and assets", "Kampagnenwissen, Regelbücher, Abenteuer und Medien")}
        description={tr("Browse content by type. Rulebooks are versioned, adventures support multiple rulesets, and assets can be global or linked.", "Durchsuche Inhalte nach Typ. Regelbücher sind versioniert, Abenteuer unterstützen mehrere Regelwerke und Medien können global oder verknüpft sein.")}
        actions={
          activeTab !== "overview" ? (
            <button className="studio-button" onClick={() => openAddModalForTab(activeTab)} type="button">
              <Plus size={16} />
              {tabActionLabel(activeTab)}
            </button>
          ) : undefined
        }
      />

      <div className="library-tabs">
        {tabs.map((tab) => (
          <button
            className={`library-tab${activeTab === tab.key ? " is-active" : ""}`}
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            type="button"
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === "overview" ? (
        <section className="dashboard-grid">
          <Panel title={tr("Library Overview", "Bibliotheksübersicht")} description={tr("Summary of the most important available content.", "Zusammenfassung der wichtigsten verfügbaren Inhalte.")} className="hero-panel">
            <div className="stat-grid">
              <StatCard label={tr("Rulebooks", "Regelbücher")} value={rulebookCount} />
              <StatCard label={tr("Rulesets", "Regelwerke")} value={rulesetCount} />
              <StatCard label={tr("Adventures", "Abenteuer")} value={adventures.length} />
              <StatCard label={tr("Assets", "Medien")} value={assets.length} />
            </div>
          </Panel>

          <Panel title={tr("Asset Categories", "Medienkategorien")} description={tr("Categories of all available assets in the library.", "Kategorien aller verfügbaren Medien in der Bibliothek.")}>
            <div className="list-stack">
              {assetCategorySummary.length === 0 ? <p className="empty-copy">{tr("No assets available yet.", "Noch keine Medien vorhanden.")}</p> : null}
              {assetCategorySummary.map((item) => (
                <article className="list-row" key={item.type}>
                  <div className="list-row__icon"><Layers size={18} /></div>
                  <div className="list-row__body">
                    <strong>{item.type}</strong>
                    <p>{tr("Asset category", "Medienkategorie")}</p>
                  </div>
                  <StatusPill tone="info">{item.count}</StatusPill>
                </article>
              ))}
            </div>
          </Panel>
        </section>
      ) : null}

      {activeTab === "rulebooks" ? (
        <Panel title={tr("Rulebooks by Ruleset", "Regelbücher nach Regelwerk")} description={tr("Rulebooks are grouped by work and version.", "Regelbücher werden nach Werk und Version gruppiert.")}>
          <div className="page-stack">
            {rulesetGroups.length === 0 ? <p className="empty-copy">{tr("No rulebooks available yet.", "Noch keine Regelbücher vorhanden.")}</p> : null}
            {rulesetGroups.map((group) => (
              <section className="page-stack" key={group.key}>
                <div className="library-group-head">
                  <div>
                    <h3 className="studio-panel__title">{group.work}</h3>
                    <p className="studio-panel__description">Version {group.version}</p>
                  </div>
                  <StatusPill tone="info">{group.items.length} {tr("rulebooks", "Regelbücher")}</StatusPill>
                </div>
                <div className="list-stack">
                  {group.items.map((document) => (
                    <article className="list-row" key={document.id}>
                      <div className="list-row__icon"><FileText size={18} /></div>
                      <div className="list-row__body">
                        <strong>{document.name}</strong>
                        <p>{document.chunk_count} {tr("chunks", "Abschnitte")}</p>
                      </div>
                      <div className="meta-chip-row">
                        <StatusPill tone={String(document.metadata?.kind ?? "") ? "ready" : "info"}>{documentKindLabel(document, locale)}</StatusPill>
                        {document.metadata?.system_document ? <StatusPill tone="default">System</StatusPill> : null}
                      </div>
                      <div className="button-row">
                        {document.source_file_path || document.metadata?.system_document ? (
                          <a className="studio-button studio-button--ghost studio-button--inline" href={fileUrl("documents", document.id)} rel="noreferrer" target="_blank">
                            {String(document.metadata?.kind ?? "") ? tr("Open Guide", "Guide öffnen") : tr("Open PDF", "PDF öffnen")}
                          </a>
                        ) : null}
                        <button
                          className="studio-button studio-button--danger studio-button--inline"
                          onClick={() =>
                            handleDelete(
                              `/api/documents/${document.id}`,
                              tr("Delete rulebook", "Regelbuch löschen"),
                              tr(`${document.name} was deleted.`, `${document.name} wurde gelöscht.`),
                              document.metadata?.system_document
                                ? tr(`Really hide system document ${document.name}? You can upload a new short_rules.md afterward.`, `Systemdokument ${document.name} wirklich ausblenden? Danach kannst du eine neue short_rules.md hochladen.`)
                                : tr(`Really delete rulebook ${document.name}?`, `Regelbuch ${document.name} wirklich löschen?`)
                            )
                          }
                          type="button"
                        >
                          <Trash2 size={14} />
                          {tr("Delete", "Löschen")}
                        </button>
                      </div>
                    </article>
                  ))}
                </div>
              </section>
            ))}
          </div>
        </Panel>
      ) : null}

      {activeTab === "adventures" ? (
        <Panel title={tr("Adventures", "Abenteuer")} description={tr("Adventures can support multiple rulesets or remain unassigned.", "Abenteuer können mehreren Regelwerken zugeordnet oder zunächst ungebunden sein.")}>
          <div className="card-grid card-grid--three">
            {adventureCards.length === 0 ? <p className="empty-copy">{tr("No adventures available yet.", "Noch keine Abenteuer vorhanden.")}</p> : null}
            {adventureCards.map((adventure) => (
              <article className="media-card" key={adventure.id}>
                <div className="media-card__cover"><BookOpen size={34} /></div>
                <div className="media-card__body">
                  <h3>{adventure.name}</h3>
                  <p>{adventure.description || tr("No description available.", "Keine Beschreibung hinterlegt.")}</p>
                  <div className="meta-chip-row">
                    {adventure.compatibleRulesets.length > 0 ? adventure.compatibleRulesets.map((item) => (
                      <StatusPill key={item} tone="info">{item}</StatusPill>
                    )) : <StatusPill tone="warning">{tr("unassigned", "nicht zugeordnet")}</StatusPill>}
                  </div>
                  <div className="button-row">
                    <button
                      className="studio-button studio-button--danger studio-button--inline"
                      onClick={() => handleDelete(`/api/adventures/${adventure.id}`, tr("Delete adventure", "Abenteuer löschen"), tr(`${adventure.name} was deleted.`, `${adventure.name} wurde gelöscht.`), tr(`Really delete adventure ${adventure.name}? Associated documents and assets will also be removed.`, `Abenteuer ${adventure.name} wirklich löschen? Zugehörige Dokumente und Medien werden ebenfalls entfernt.`))}
                      type="button"
                    >
                      <Trash2 size={14} />
                      {tr("Delete", "Löschen")}
                    </button>
                  </div>
                </div>
              </article>
            ))}
          </div>
        </Panel>
      ) : null}

      {activeTab === "assets" ? (
        <Panel title={tr("Asset Gallery", "Mediengalerie")} description={tr("All assets grouped by type with assignment badges.", "Alle Medien nach Typ gruppiert und mit Zuordnungskennzeichen.")}>
          <div className="page-stack">
            <div className="library-toolbar">
              <div className="library-toolbar__group">
                <span className="library-toolbar__label">{tr("Type", "Typ")}</span>
                <div className="meta-chip-row">
                  {assetTypeOptions.map((option) => (
                    <button
                      className={`filter-chip${assetTypeFilter === option ? " is-active" : ""}`}
                      key={option}
                      onClick={() => setAssetTypeFilter(option)}
                      type="button"
                    >
                      {option === "all" ? tr("All", "Alle") : option}
                    </button>
                  ))}
                </div>
              </div>
              <div className="library-toolbar__group">
                <span className="library-toolbar__label">{tr("Assignment", "Zuordnung")}</span>
                <div className="meta-chip-row">
                  {[
                    { key: "all", label: tr("All", "Alle") },
                    { key: "global", label: tr("Global", "Global") },
                    { key: "linked", label: tr("Adventure-linked", "Mit Abenteuer verknüpft") },
                  ].map((option) => (
                    <button
                      className={`filter-chip${assetScopeFilter === option.key ? " is-active" : ""}`}
                      key={option.key}
                      onClick={() => setAssetScopeFilter(option.key as AssetScopeFilter)}
                      type="button"
                    >
                      {option.label}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            {filteredAssets.length === 0 ? <p className="empty-copy">{tr("No assets match the current filters.", "Keine Medien entsprechen den aktuellen Filtern.")}</p> : null}

            {assetGroups.map((group) => (
              <section className="page-stack" key={group.type}>
                <div className="library-group-head">
                  <div>
                    <h3 className="studio-panel__title">{assetTypeLabel(group.type)}</h3>
                    <p className="studio-panel__description">{group.items.length} {tr("assets in this category.", "Medien in dieser Kategorie.")}</p>
                  </div>
                </div>
                <div className="asset-gallery-grid">
                  {group.items.map((asset) => (
                    <article className="asset-gallery-card" key={asset.id}>
                      <a className="asset-gallery-card__preview" href={asset.previewUrl} rel="noreferrer" target="_blank">
                        {asset.isImage ? (
                          <img alt={asset.name} className="asset-gallery-card__image" loading="lazy" src={asset.previewUrl} />
                        ) : (
                          <div className="asset-gallery-card__fallback">
                            <Layers size={28} />
                            <span>{assetTypeLabel(asset.type)}</span>
                          </div>
                        )}
                      </a>
                      <div className="asset-gallery-card__body">
                        <div className="asset-gallery-card__title-row">
                          <div className="list-row__icon"><Layers size={18} /></div>
                          <div className="asset-gallery-card__title-wrap">
                            <strong title={asset.name}>{asset.name}</strong>
                            <p>{asset.mime_type || asset.type}</p>
                          </div>
                        </div>
                        <div className="meta-chip-row">
                          <StatusPill tone="default">{assetTypeLabel(asset.type)}</StatusPill>
                          {asset.linkedAdventureIds.length === 0 ? <StatusPill tone="warning">global</StatusPill> : null}
                          {asset.rulesets.slice(0, 2).map((item) => <StatusPill key={`${asset.id}-${item}`} tone="info">{item}</StatusPill>)}
                          {asset.rulesets.length > 2 ? <StatusPill tone="info">+{asset.rulesets.length - 2}</StatusPill> : null}
                        </div>
                        {asset.linkedAdventureNames.length > 0 ? (
                          <p className="asset-gallery-card__meta" title={asset.linkedAdventureNames.join(", ")}>
                            {tr("Adventures", "Abenteuer")}: {asset.linkedAdventureNames.join(", ")}
                          </p>
                        ) : null}
                        <div className="button-row">
                          <a className="studio-button studio-button--ghost studio-button--inline" href={asset.previewUrl} rel="noreferrer" target="_blank">
                            {tr("Open", "Öffnen")}
                          </a>
                          <button
                            className="studio-button studio-button--danger studio-button--inline"
                            onClick={() => handleDelete(`/api/assets/${asset.id}`, tr("Delete asset", "Medium löschen"), tr(`${asset.name} was deleted.`, `${asset.name} wurde gelöscht.`), tr(`Really delete asset ${asset.name}?`, `Medium ${asset.name} wirklich löschen?`))}
                            type="button"
                          >
                            <Trash2 size={14} />
                            {tr("Delete", "Löschen")}
                          </button>
                        </div>
                      </div>
                    </article>
                  ))}
                </div>
              </section>
            ))}
          </div>
        </Panel>
      ) : null}

      {activeModal ? (
        <div className="modal-overlay" onClick={closeModal} role="presentation">
          <section className="modal-card" onClick={(event) => event.stopPropagation()} role="dialog">
            <div className="modal-card__header">
              <div>
                <p className="eyebrow">{tr("Add", "Hinzufügen")}</p>
                <h2 className="studio-panel__title">
                  {activeModal === "rules" ? tr("Add Rulebook", "Regelbuch hinzufügen") : activeModal === "adventure" ? tr("Add Adventure", "Abenteuer hinzufügen") : tr("Add Asset", "Medium hinzufügen")}
                </h2>
              </div>
            </div>

            {activeModal === "rules" ? (
              <div className="form-grid">
                <input onChange={(event) => setRulesName(event.target.value)} placeholder={tr("Display name", "Anzeigename")} value={rulesName} />
                <div className="dual-field-grid">
                  <input onChange={(event) => setRulesetWork(event.target.value)} placeholder={tr("Work", "Werk")} value={rulesetWork} />
                  <input onChange={(event) => setRulesetVersion(event.target.value)} placeholder="Version" value={rulesetVersion} />
                </div>
                <input onChange={(event) => setLanguage(event.target.value)} placeholder={tr("Language", "Sprache")} value={language} />
                <p className="muted-copy">
                  {tr("For a compact table reference, upload the file as ", "Für eine kompakte Tischreferenz lade die Datei als ")}<code>short_rules.md</code>{tr(". It will automatically be marked as", ". Sie wird automatisch markiert als")}<strong> {tr("Short Rules", "Kurzregeln")}</strong>.
                </p>
                <label className="file-field">
                  <span>{tr("Rules file", "Regeldatei")}</span>
                  <input accept=".pdf,application/pdf,.md,text/markdown,text/plain" onChange={(event) => setRulesFile(event.target.files?.[0] ?? null)} type="file" />
                </label>
              </div>
            ) : null}

            {activeModal === "adventure" ? (
              <div className="form-grid">
                <input onChange={(event) => setAdventureName(event.target.value)} placeholder={tr("Adventure name", "Abenteuername")} value={adventureName} />
                <textarea onChange={(event) => setAdventureDescription(event.target.value)} placeholder={tr("Description", "Beschreibung")} value={adventureDescription} />
                <input onChange={(event) => setCompatibleRulesets(event.target.value)} placeholder={tr("Compatible rulesets, e.g. 5E:2014", "Kompatible Regelwerke, z. B. 5E:2014")} value={compatibleRulesets} />
                <input onChange={(event) => setLanguage(event.target.value)} placeholder={tr("Language", "Sprache")} value={language} />
                <label className="file-field">
                  <span>{tr("Adventure PDF", "Abenteuer-PDF")}</span>
                  <input accept=".pdf,application/pdf" onChange={(event) => setAdventurePdf(event.target.files?.[0] ?? null)} type="file" />
                </label>
                <label className="file-field">
                  <span>{tr("Resources ZIP", "Ressourcen-ZIP")}</span>
                  <input accept=".zip,application/zip" onChange={(event) => setAdventureZip(event.target.files?.[0] ?? null)} type="file" />
                </label>
              </div>
            ) : null}

            {activeModal === "asset" ? (
              <div className="form-grid">
                <input onChange={(event) => setAssetName(event.target.value)} placeholder={tr("Asset name", "Medienname")} value={assetName} />
                <div className="dual-field-grid">
                  <select onChange={(event) => setAssetType(event.target.value)} value={assetType}>
                    <option value="image">{tr("image", "Bild")}</option>
                    <option value="portrait">{tr("portrait", "Porträt")}</option>
                    <option value="map">{tr("map", "Karte")}</option>
                    <option value="battlemap">{tr("battle map", "Kampfkarte")}</option>
                    <option value="token">{tr("token", "Spielfigur")}</option>
                    <option value="handout">{tr("handout", "Handout")}</option>
                    <option value="asset">{tr("asset", "Medium")}</option>
                  </select>
                  <input onChange={(event) => setAssetTags(event.target.value)} placeholder={tr("Tags, comma-separated", "Tags, kommagetrennt")} value={assetTags} />
                </div>
                <input onChange={(event) => setAssetRulesets(event.target.value)} placeholder={tr("Rulesets, e.g. 5E:2014", "Regelwerke, z. B. 5E:2014")} value={assetRulesets} />
                <input onChange={(event) => setAssetAdventureIds(event.target.value)} placeholder={tr("Adventure IDs, optional", "Abenteuer-IDs, optional")} value={assetAdventureIds} />
                <label className="file-field">
                  <span>{tr("Asset file", "Mediendatei")}</span>
                  <input onChange={(event) => setAssetFile(event.target.files?.[0] ?? null)} type="file" />
                </label>
              </div>
            ) : null}

            {error ? <p className="error-copy">{error}</p> : null}

            <div className="modal-card__footer">
              <span className="modal-card__spacer">{isPending ? tr("Uploading...", "Wird hochgeladen …") : ""}</span>
              <div className="button-row modal-card__actions">
                <button className="studio-button studio-button--ghost" onClick={closeModal} type="button">
                  {tr("Cancel", "Abbrechen")}
                </button>
                <button
                  className="studio-button"
                  disabled={isPending}
                  onClick={activeModal === "rules" ? handleRulesUpload : activeModal === "adventure" ? handleAdventureUpload : handleAssetUpload}
                  type="button"
                >
                  <Upload size={16} />
                  {isPending ? tr("Uploading...", "Wird hochgeladen …") : tr("Upload", "Hochladen")}
                </button>
              </div>
            </div>
          </section>
        </div>
      ) : null}
    </div>
  );
}
