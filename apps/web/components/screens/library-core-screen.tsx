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

function documentKindLabel(document: Document) {
  const kind = String(document.metadata?.kind ?? "");
  if (kind === "character_builder_guide") return "Builder Guide";
  if (kind === "level_up_guide") return "Level-Up Guide";
  if (kind === "short_rules_guide") return "Short Rules";
  return "Rulebook";
}

export function LibraryCoreScreen({ campaigns, adventures, documents, assets }: Props) {
  const router = useRouter();
  const { locale, tr } = useI18n();
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
        <Panel title="Rulebooks by ruleset" description="Rulebooks werden nach Werk und Version gruppiert.">
          <div className="page-stack">
            {rulesetGroups.length === 0 ? <p className="empty-copy">Noch keine Rulebooks vorhanden.</p> : null}
            {rulesetGroups.map((group) => (
              <section className="page-stack" key={group.key}>
                <div className="library-group-head">
                  <div>
                    <h3 className="studio-panel__title">{group.work}</h3>
                    <p className="studio-panel__description">Version {group.version}</p>
                  </div>
                  <StatusPill tone="info">{group.items.length} Rulebooks</StatusPill>
                </div>
                <div className="list-stack">
                  {group.items.map((document) => (
                    <article className="list-row" key={document.id}>
                      <div className="list-row__icon"><FileText size={18} /></div>
                      <div className="list-row__body">
                        <strong>{document.name}</strong>
                        <p>{document.chunk_count} Chunks</p>
                      </div>
                      <div className="meta-chip-row">
                        <StatusPill tone={String(document.metadata?.kind ?? "") ? "ready" : "info"}>{documentKindLabel(document)}</StatusPill>
                        {document.metadata?.system_document ? <StatusPill tone="default">System</StatusPill> : null}
                      </div>
                      <div className="button-row">
                        {document.source_file_path || document.metadata?.system_document ? (
                          <a className="studio-button studio-button--ghost studio-button--inline" href={fileUrl("documents", document.id)} rel="noreferrer" target="_blank">
                            {String(document.metadata?.kind ?? "") ? "Open Guide" : "Open PDF"}
                          </a>
                        ) : null}
                        <button
                          className="studio-button studio-button--danger studio-button--inline"
                          onClick={() =>
                            handleDelete(
                              `/api/documents/${document.id}`,
                              "Delete rulebook",
                              `${document.name} wurde gelöscht.`,
                              document.metadata?.system_document
                                ? `Systemdokument wirklich ausblenden: ${document.name}? Danach kannst du eine neue short_rules.md hochladen.`
                                : `Rulebook wirklich löschen: ${document.name}?`
                            )
                          }
                          type="button"
                        >
                          <Trash2 size={14} />
                          Delete
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
        <Panel title="Adventures" description="Adventures können mehreren Regelwerken zugeordnet oder zunächst ungebunden sein.">
          <div className="card-grid card-grid--three">
            {adventureCards.length === 0 ? <p className="empty-copy">Noch keine Adventures vorhanden.</p> : null}
            {adventureCards.map((adventure) => (
              <article className="media-card" key={adventure.id}>
                <div className="media-card__cover"><BookOpen size={34} /></div>
                <div className="media-card__body">
                  <h3>{adventure.name}</h3>
                  <p>{adventure.description || "Kein Beschreibungstext hinterlegt."}</p>
                  <div className="meta-chip-row">
                    {adventure.compatibleRulesets.length > 0 ? adventure.compatibleRulesets.map((item) => (
                      <StatusPill key={item} tone="info">{item}</StatusPill>
                    )) : <StatusPill tone="warning">unassigned</StatusPill>}
                  </div>
                  <div className="button-row">
                    <button
                      className="studio-button studio-button--danger studio-button--inline"
                      onClick={() => handleDelete(`/api/adventures/${adventure.id}`, "Delete adventure", `${adventure.name} wurde gelöscht.`, `Adventure wirklich löschen: ${adventure.name}? Zugehörige Adventure-Dokumente und Assets werden mit entfernt.`)}
                      type="button"
                    >
                      <Trash2 size={14} />
                      Delete
                    </button>
                  </div>
                </div>
              </article>
            ))}
          </div>
        </Panel>
      ) : null}

      {activeTab === "assets" ? (
        <Panel title="Assets Gallery" description="Alle Assets in einer Galerie, primär nach Typ geordnet und mit Zuordnungs-Badges.">
          <div className="page-stack">
            <div className="library-toolbar">
              <div className="library-toolbar__group">
                <span className="library-toolbar__label">Typ</span>
                <div className="meta-chip-row">
                  {assetTypeOptions.map((option) => (
                    <button
                      className={`filter-chip${assetTypeFilter === option ? " is-active" : ""}`}
                      key={option}
                      onClick={() => setAssetTypeFilter(option)}
                      type="button"
                    >
                      {option === "all" ? "Alle" : option}
                    </button>
                  ))}
                </div>
              </div>
              <div className="library-toolbar__group">
                <span className="library-toolbar__label">Zuordnung</span>
                <div className="meta-chip-row">
                  {[
                    { key: "all", label: "Alle" },
                    { key: "global", label: "Global" },
                    { key: "linked", label: "Adventure-linked" },
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

            {filteredAssets.length === 0 ? <p className="empty-copy">Keine Assets für die aktuelle Filterkombination vorhanden.</p> : null}

            {assetGroups.map((group) => (
              <section className="page-stack" key={group.type}>
                <div className="library-group-head">
                  <div>
                    <h3 className="studio-panel__title">{group.type}</h3>
                    <p className="studio-panel__description">{group.items.length} Assets in dieser Kategorie.</p>
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
                            <span>{asset.type}</span>
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
                          <StatusPill tone="default">{asset.type}</StatusPill>
                          {asset.linkedAdventureIds.length === 0 ? <StatusPill tone="warning">global</StatusPill> : null}
                          {asset.rulesets.slice(0, 2).map((item) => <StatusPill key={`${asset.id}-${item}`} tone="info">{item}</StatusPill>)}
                          {asset.rulesets.length > 2 ? <StatusPill tone="info">+{asset.rulesets.length - 2}</StatusPill> : null}
                        </div>
                        {asset.linkedAdventureNames.length > 0 ? (
                          <p className="asset-gallery-card__meta" title={asset.linkedAdventureNames.join(", ")}>
                            Adventures: {asset.linkedAdventureNames.join(", ")}
                          </p>
                        ) : null}
                        <div className="button-row">
                          <a className="studio-button studio-button--ghost studio-button--inline" href={asset.previewUrl} rel="noreferrer" target="_blank">
                            Open
                          </a>
                          <button
                            className="studio-button studio-button--danger studio-button--inline"
                            onClick={() => handleDelete(`/api/assets/${asset.id}`, "Delete asset", `${asset.name} wurde gelöscht.`, `Asset wirklich löschen: ${asset.name}?`)}
                            type="button"
                          >
                            <Trash2 size={14} />
                            Delete
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
                <p className="eyebrow">Add</p>
                <h2 className="studio-panel__title">
                  {activeModal === "rules" ? "Rulebook hinzufügen" : activeModal === "adventure" ? "Adventure hinzufügen" : "Asset hinzufügen"}
                </h2>
              </div>
            </div>

            {activeModal === "rules" ? (
              <div className="form-grid">
                <input onChange={(event) => setRulesName(event.target.value)} placeholder="Display name" value={rulesName} />
                <div className="dual-field-grid">
                  <input onChange={(event) => setRulesetWork(event.target.value)} placeholder="Werk" value={rulesetWork} />
                  <input onChange={(event) => setRulesetVersion(event.target.value)} placeholder="Version" value={rulesetVersion} />
                </div>
                <input onChange={(event) => setLanguage(event.target.value)} placeholder="Language" value={language} />
                <p className="muted-copy">
                  Standard für kompakte Tischreferenzen: Datei als <code>short_rules.md</code> hochladen. Sie wird dann automatisch als
                  <strong> Short Rules</strong> markiert.
                </p>
                <label className="file-field">
                  <span>Rules file</span>
                  <input accept=".pdf,application/pdf,.md,text/markdown,text/plain" onChange={(event) => setRulesFile(event.target.files?.[0] ?? null)} type="file" />
                </label>
              </div>
            ) : null}

            {activeModal === "adventure" ? (
              <div className="form-grid">
                <input onChange={(event) => setAdventureName(event.target.value)} placeholder="Adventure name" value={adventureName} />
                <textarea onChange={(event) => setAdventureDescription(event.target.value)} placeholder="Description" value={adventureDescription} />
                <input onChange={(event) => setCompatibleRulesets(event.target.value)} placeholder="Compatible rulesets, e.g. 5E:2014" value={compatibleRulesets} />
                <input onChange={(event) => setLanguage(event.target.value)} placeholder="Language" value={language} />
                <label className="file-field">
                  <span>Adventure PDF</span>
                  <input accept=".pdf,application/pdf" onChange={(event) => setAdventurePdf(event.target.files?.[0] ?? null)} type="file" />
                </label>
                <label className="file-field">
                  <span>Resources ZIP</span>
                  <input accept=".zip,application/zip" onChange={(event) => setAdventureZip(event.target.files?.[0] ?? null)} type="file" />
                </label>
              </div>
            ) : null}

            {activeModal === "asset" ? (
              <div className="form-grid">
                <input onChange={(event) => setAssetName(event.target.value)} placeholder="Asset name" value={assetName} />
                <div className="dual-field-grid">
                  <select onChange={(event) => setAssetType(event.target.value)} value={assetType}>
                    <option value="image">image</option>
                    <option value="portrait">portrait</option>
                    <option value="map">map</option>
                    <option value="battlemap">battlemap</option>
                    <option value="token">token</option>
                    <option value="handout">handout</option>
                    <option value="asset">asset</option>
                  </select>
                  <input onChange={(event) => setAssetTags(event.target.value)} placeholder="Tags, kommagetrennt" value={assetTags} />
                </div>
                <input onChange={(event) => setAssetRulesets(event.target.value)} placeholder="Rulesets, e.g. 5E:2014" value={assetRulesets} />
                <input onChange={(event) => setAssetAdventureIds(event.target.value)} placeholder="Adventure IDs, optional" value={assetAdventureIds} />
                <label className="file-field">
                  <span>Asset file</span>
                  <input onChange={(event) => setAssetFile(event.target.files?.[0] ?? null)} type="file" />
                </label>
              </div>
            ) : null}

            {error ? <p className="error-copy">{error}</p> : null}

            <div className="modal-card__footer">
              <span className="modal-card__spacer">{isPending ? "Uploading..." : ""}</span>
              <div className="button-row modal-card__actions">
                <button className="studio-button studio-button--ghost" onClick={closeModal} type="button">
                  Cancel
                </button>
                <button
                  className="studio-button"
                  disabled={isPending}
                  onClick={activeModal === "rules" ? handleRulesUpload : activeModal === "adventure" ? handleAdventureUpload : handleAssetUpload}
                  type="button"
                >
                  <Upload size={16} />
                  {isPending ? "Uploading..." : "Upload"}
                </button>
              </div>
            </div>
          </section>
        </div>
      ) : null}
    </div>
  );
}
