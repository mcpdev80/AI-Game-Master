import type { Campaign, Document, Session } from "../lib/api";

type Props = {
  campaigns: Campaign[];
  sessions: Session[];
  documents: Document[];
  summary: Record<string, unknown>;
};

export function DashboardOverview({ campaigns, sessions, documents, summary }: Props) {
  const counts = (summary.counts ?? {}) as Record<string, number>;
  const services = (summary.services ?? []) as { name: string; status: string }[];

  return (
    <section className="dashboard-grid">
      <article className="panel">
        <p className="label">Bestand</p>
        <p>Kampagnen: {counts.campaigns ?? campaigns.length}</p>
        <p>Sessions: {counts.sessions ?? sessions.length}</p>
        <p>Dokumente: {counts.documents ?? documents.length}</p>
      </article>
      <article className="panel">
        <p className="label">Services</p>
        <div className="list-grid">
          {services.map((service) => (
            <div className="panel inset-panel" key={service.name}>
              <p className="label">{service.name}</p>
              <p>{service.status}</p>
            </div>
          ))}
        </div>
      </article>
      <article className="panel">
        <p className="label">Letzte Session</p>
        {sessions[0] ? (
          <>
            <p>{sessions[0].current_scene || "Ohne Szene"}</p>
            <p>{sessions[0].state.last_narration || "Noch keine Narration gespeichert."}</p>
          </>
        ) : (
          <p>Noch keine Session vorhanden.</p>
        )}
      </article>
    </section>
  );
}
