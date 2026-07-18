import Link from "next/link";
import { BookOpen, FileText, ImageIcon, Layers, Users } from "lucide-react";
import { PageIntro, Panel, StatCard, StatusPill } from "../studio-primitives";
import { apiBaseUrl } from "../../lib/api";
import type { Adventure, Asset, Campaign, Document } from "../../lib/api";

type LibraryScreenProps = {
  campaigns: Campaign[];
  adventures: Adventure[];
  documents: Document[];
  assets: Asset[];
};

export function LibraryScreen({ campaigns, adventures, documents, assets }: LibraryScreenProps) {
  const activeCampaign = campaigns[0] ?? null;
  const globalRules = documents.filter((document) => document.type === "rules" && !(document.metadata?.campaign_id as string | undefined));
  const characterSheets = documents.filter((document) => document.type === "character_sheet");

  return (
    <div className="page-stack">
      <PageIntro
        eyebrow="Library"
        title="Campaign knowledge, adventures, and display assets"
        description="The library is the canonical content vault: adventures, rulebooks, character sheets, extracted assets, and manually uploaded media all stay grouped here."
        actions={<button className="studio-button">Import New Adventure</button>}
      />

      <section className="dashboard-grid">
        <Panel
          title={activeCampaign ? activeCampaign.name : "No campaign selected"}
          description={activeCampaign?.description || "Campaign-level grouping for adventures, rules, characters, and assets."}
          className="hero-panel"
          action={<StatusPill tone="info">{activeCampaign ? "Active Campaign" : "Global View"}</StatusPill>}
        >
          <div className="stat-grid">
            <StatCard label="Adventures" value={adventures.length} />
            <StatCard label="Rulebooks" value={globalRules.length} />
            <StatCard label="Assets" value={assets.length} />
            <StatCard label="Character Sheets" value={characterSheets.length} />
          </div>
        </Panel>
      </section>

      <section className="dashboard-grid">
        <Panel title="Adventure Shelf" description="Imported adventures and attached context packages.">
          <div className="card-grid card-grid--three">
            {adventures.slice(0, 6).map((adventure) => (
              <article className="media-card" key={adventure.id}>
                <div className="media-card__cover">
                  <BookOpen size={34} />
                </div>
                <div className="media-card__body">
                  <h3>{adventure.name}</h3>
                  <p>{adventure.description || "Adventure package with book, maps, portraits, and handouts."}</p>
                  <div className="meta-chip-row">
                    <StatusPill tone="default">{adventure.language.toUpperCase()}</StatusPill>
                    <StatusPill tone="info">Adventure</StatusPill>
                  </div>
                </div>
              </article>
            ))}
          </div>
        </Panel>

        <Panel title="Rules Vault" description="Global and campaign-scoped rules used by the AI DM.">
          <div className="list-stack">
            {globalRules.slice(0, 8).map((document) => (
              <article className="list-row" key={document.id}>
                <div className="list-row__icon">
                  <FileText size={18} />
                </div>
                <div className="list-row__body">
                  <strong>{document.name}</strong>
                  <p>{document.chunk_count} extracted chunks</p>
                </div>
                <Link className="studio-button studio-button--ghost studio-button--inline" href={`${apiBaseUrl}/api/documents/${document.id}/file`} target="_blank">
                  Open PDF
                </Link>
                <StatusPill tone="info">Global</StatusPill>
              </article>
            ))}
          </div>
        </Panel>

        <Panel title="Asset Gallery" description="Portraits, battlemaps, tokens, handouts, and other player-facing media.">
          <div className="stat-grid">
            <StatCard label="Portraits" value={assets.filter((asset) => asset.type === "portrait" || asset.type === "image").length} />
            <StatCard label="Battlemaps" value={assets.filter((asset) => asset.type === "battlemap" || asset.type === "map").length} />
            <StatCard label="Tokens" value={assets.filter((asset) => asset.type === "token").length} />
            <StatCard label="Handouts" value={assets.filter((asset) => asset.type === "handout").length} />
          </div>
          <div className="card-grid">
            {assets.slice(0, 6).map((asset) => (
              <article className="asset-chip" key={asset.id}>
                <ImageIcon size={18} />
                <div>
                  <strong>{asset.name}</strong>
                  <p>{asset.type}</p>
                </div>
                <Link className="studio-button studio-button--ghost studio-button--inline" href={`${apiBaseUrl}/api/assets/${asset.id}/file`} target="_blank">
                  Open
                </Link>
              </article>
            ))}
          </div>
        </Panel>

        <Panel title="Content Scope" description="Keep the mental model explicit for the operator.">
          <div className="scope-grid">
            <article className="scope-card">
              <Layers size={18} />
              <strong>Global</strong>
              <p>Rulebooks and references available across campaigns.</p>
            </article>
            <article className="scope-card">
              <BookOpen size={18} />
              <strong>Campaign</strong>
              <p>Shared adventure context and long-term campaign material.</p>
            </article>
            <article className="scope-card">
              <FileText size={18} />
              <strong>Adventure</strong>
              <p>PDF book, extracted images, battlemaps, and add-on ZIP assets.</p>
            </article>
            <article className="scope-card">
              <Users size={18} />
              <strong>Characters</strong>
              <p>Imported or generated sheets that can be assigned to players and sessions.</p>
            </article>
          </div>
        </Panel>
      </section>
    </div>
  );
}
