"use client";

import { useEffect, useState, useTransition } from "react";
import {
  apiPost,
  fetchSessionEvents,
  type GMResponse,
  type Session,
  type SessionEvent,
} from "../lib/api";
import { MediaPanel } from "./media-panel";
import { useI18n } from "../lib/i18n";

type Props = {
  sessions: Session[];
  initialEvents: SessionEvent[];
};

export function GMConsole({ sessions, initialEvents }: Props) {
  const { locale } = useI18n();
  const [selectedSessionId, setSelectedSessionId] = useState(sessions[0]?.id ?? "");
  const [playerInput, setPlayerInput] = useState("");
  const [diceType, setDiceType] = useState("d20");
  const [diceValue, setDiceValue] = useState("");
  const [result, setResult] = useState<GMResponse | null>(null);
  const [events, setEvents] = useState(initialEvents);
  const [error, setError] = useState("");
  const [isPending, startTransition] = useTransition();

  const session = sessions.find((item) => item.id === selectedSessionId) ?? null;

  useEffect(() => {
    if (!selectedSessionId) {
      setEvents([]);
      return;
    }

    let cancelled = false;
    void fetchSessionEvents(selectedSessionId)
      .then((items) => {
        if (!cancelled) {
          setEvents(items);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setEvents([]);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [selectedSessionId]);

  if (!session) {
    return (
      <section className="panel">
        <p className="label">GM Live</p>
        <p>Lege zuerst eine Session an, bevor du den GM-Stub testest.</p>
      </section>
    );
  }

  return (
    <section className="live-grid">
      <form
        className="stack-form panel"
        onSubmit={(event) => {
          event.preventDefault();
          setError("");
          startTransition(async () => {
            try {
              const diceRoll =
                diceValue.trim() === ""
                  ? undefined
                  : {
                      dice: [{ type: diceType, value: Number(diceValue) }],
                      confidence: 1,
                      timestamp: new Date().toISOString(),
                    };

              const response = await apiPost<GMResponse>("/api/gm/respond", {
                session_id: session.id,
                player_input: playerInput,
                language: locale,
                dice_roll: diceRoll,
              });
              setResult(response);
              setPlayerInput("");
              setDiceValue("");
              setEvents(await fetchSessionEvents(session.id));
            } catch (err) {
              setError(err instanceof Error ? err.message : "GM request failed");
            }
          });
        }}
      >
        <p className="label">GM Prompt</p>
        <select value={selectedSessionId} onChange={(event) => setSelectedSessionId(event.target.value)}>
          {sessions.map((item) => (
            <option key={item.id} value={item.id}>
              {item.current_scene || "Ohne Szene"} | {item.current_location || "Ohne Ort"}
            </option>
          ))}
        </select>
        <p>
          Session: {session.current_scene || "ohne Szene"} in {session.current_location || "ohne Ort"}
        </p>
        <textarea
          value={playerInput}
          onChange={(event) => setPlayerInput(event.target.value)}
          placeholder="Spieleraktion oder DM-Input"
          rows={7}
          required
        />
        <div className="two-col">
          <select value={diceType} onChange={(event) => setDiceType(event.target.value)}>
            <option value="d20">d20</option>
            <option value="d12">d12</option>
            <option value="d10">d10</option>
            <option value="d8">d8</option>
            <option value="d6">d6</option>
            <option value="d4">d4</option>
            <option value="d100">d100</option>
          </select>
          <input value={diceValue} onChange={(event) => setDiceValue(event.target.value)} placeholder="Wurf" />
        </div>
        <button type="submit" disabled={isPending}>
          {isPending ? "LLM antwortet..." : "GM-Antwort erzeugen"}
        </button>
        {error ? <p className="error-text">{error}</p> : null}
      </form>

      <section className="panel">
        <p className="label">Letzte Antwort</p>
        {result ? (
          <>
            <p className="response-copy">{result.narration}</p>
            <p className="meta-copy">
              Quelle: {result.prompt_source} | Modell: {result.raw_model}
            </p>
            <p className="meta-copy">Regeln: {result.rules_used.join(", ") || "keine"}</p>
            <p className="meta-copy">Notizen: {result.dm_notes.join(" | ") || "keine"}</p>
            <p className="meta-copy">
              Medien: {result.scene_events.map((event) => `${event.type}:${event.name}`).join(", ") || "keine"}
            </p>
            <div className="list-grid">
              {result.context_chunks.map((chunk, index) => (
                <article className="panel inset-panel" key={`${chunk.document_id}-${index}`}>
                  <p className="label">{chunk.document_name}</p>
                  <p>{chunk.chunk_text}</p>
                </article>
              ))}
            </div>
          </>
        ) : (
          <p>Noch keine Antwort erzeugt.</p>
        )}
      </section>

      <MediaPanel session={session} result={result} />

      <section className="panel">
        <p className="label">Session State</p>
        <p>Scene Summary: {session.state.scene_summary || "leer"}</p>
        <p>Last Narration: {session.state.last_narration || "leer"}</p>
        <p>Active Media Cue: {session.state.active_media_cue || "keine"}</p>
        <p>Open Quests: {session.state.open_quests.join(", ") || "keine"}</p>
      </section>

      <section className="panel">
        <p className="label">Session Events</p>
        <div className="list-grid">
          {events.length === 0 ? <p>Keine Events vorhanden.</p> : null}
          {events.map((event) => (
            <article className="panel inset-panel" key={event.id}>
              <p className="label">{event.type}</p>
              <p className="meta-copy">{new Date(event.created_at).toLocaleString("de-DE")}</p>
              <pre className="json-block">{JSON.stringify(event.payload, null, 2)}</pre>
            </article>
          ))}
        </div>
      </section>
    </section>
  );
}
