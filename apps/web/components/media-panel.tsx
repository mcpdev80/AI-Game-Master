import type { GMResponse, Session } from "../lib/api";

type Props = {
  session: Session;
  result: GMResponse | null;
};

export function MediaPanel({ session, result }: Props) {
  const sceneEvents = result?.scene_events ?? [];

  return (
    <section className="panel">
      <p className="label">Media Panel</p>
      <p>Aktive Cue: {session.state.active_media_cue || "keine"}</p>
      <div className="list-grid">
        {sceneEvents.length === 0 ? <p>Keine aktuellen scene_events.</p> : null}
        {sceneEvents.map((event, index) => (
          <article className="panel inset-panel" key={`${event.type}-${event.name}-${index}`}>
            <p className="label">{event.type}</p>
            <p>{event.name}</p>
          </article>
        ))}
      </div>
    </section>
  );
}
