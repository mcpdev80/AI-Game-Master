# OpenAI Build Week Plan – AI Game Master

Stand: 18. Juli 2026
Deadline: 21. Juli 2026, 17:00 PDT / 22. Juli 2026, 02:00 CEST
Empfohlener Track: **Apps for Your Life**

## 1. Entscheidung und Zielbild

Wir reichen nicht die gesamte vorhandene Plattform als neue Entwicklung ein. Wir reichen eine klar abgegrenzte, während der Build Week mit Codex und GPT-5.6 entwickelte Erweiterung ein:

> **GPT-5.6 Live Encounter Director** – A browser-based AI game master that turns player speech, campaign context, character state, and physical dice rolls into a consistent narrated scene shared across an operator view and player screens.

Der Golden Path für Bewertung und Video ist:

1. Eine eigene, lizenzfreie Demo-Kampagne ist bereits geladen.
2. Ein Spieler tritt per Link oder QR-Code bei und sieht seinen Charakter.
3. Der Spieler spricht oder schreibt eine Aktion.
4. GPT-5.6 entscheidet strukturiert, ob ein Wurf nötig ist.
5. Ein physischer Würfel wird per Kamera erkannt oder als sicherer Demo-Fallback manuell bestätigt.
6. GPT-5.6 löst das Ergebnis konsistent auf, aktualisiert den Spielzustand und erzeugt die nächste Erzählung.
7. Operator View, Player Screen und Player Portal zeigen jeweils nur die für sie passenden Informationen.
8. Die Erzählung wird optional vorgelesen.

### Nicht-Ziele bis zur Deadline

- Kein vollständiger Ersatz für alle Rollenspielsysteme.
- Kein Marketplace und keine Monetarisierung.
- Kein komplexes Multi-User-Rechtesystem.
- Keine neue Medienbibliothek, wenn sie nicht für den Golden Path erforderlich ist.
- Keine Optimierung aller vorhandenen Verwaltungsseiten.
- Kein Training oder Fine-Tuning eines eigenen Modells.
- Kein Einsatz urheberrechtlich geschützter D&D-Bücher oder Abenteuer in Repository, öffentlicher Demo oder Video.

## 2. Ergebnis der Bestandsanalyse

### Was bereits stark ist

- Next.js-Frontend mit Operator-, Session-, Player-Screen- und Player-Portal-Ansichten.
- Go-API mit PostgreSQL-Persistenz.
- Kampagnen, Abenteuer, Charaktere, Sessions und individuelle Spielerlinks.
- Dokument-Ingestion und Retrieval für Abenteuer- und Regelkontext.
- Getrennte LLM-Sessions für Erzählung, Regelabfragen und Zusammenfassungen.
- Session-Memory mit History-Kompaktierung, Fakten und strukturiertem Zustand.
- LLM-Gateway mit Parallelitätslimit, Timeouts, Session-Locks und Circuit Breaker.
- Vorhandenes strukturiertes Antwortmodell mit Narration, Würfelanforderung, State Updates und Scene Events.
- Kamera-/Vision-Flow mit lokalem CV-Fallback und Konsens mehrerer Frames.
- STT-/TTS-Anbindung und Player-spezifische Freigabe von Handouts und Medien.
- Der Next.js-Produktionsbuild ist erfolgreich.
- Die Docker-Compose-Konfiguration ist gültig.
- Die Docker-Images für Web, API und Vision bauen erfolgreich.
- Der vollständige Compose-Stack startet und meldet alle Services als healthy.

### Aktueller Stand nach der Umsetzung

Bereits abgeschlossen und verifiziert:

- Git-Baseline `dffac74` mit Tag `pre-build-week-baseline` und nachvollziehbare Build-Week-Commits.
- GPT-5.6 über die OpenAI Responses API mit strict Structured Outputs und `store: false`.
- OpenAI STT mit `gpt-4o-transcribe` und TTS mit `gpt-4o-mini-tts`.
- Rechtssichere, zweisprachige Demo **The Fungal Caverns** inklusive Attribution und Spielerkarte.
- Deterministischer API- und Browser-Golden-Path von der Charaktererstellung bis zur Würfelauflösung.
- Vollständige, persistente Deutsch-/Englisch-Umschaltung für Oberfläche, AI-Antworten, STT und TTS.
- MIT-Lizenz, Third-Party Notices, Content Policy und bereinigtes Repository.
- Produktiver Compose-Stack läuft lokal; API-Health und Web-Build sind grün.

### Tatsächlich verbleibende Einreichungsblocker

| Priorität | Blocker | Aktueller Befund | Erforderliches Ergebnis |
|---|---|---|---|
| P0 | Security-Hardening noch nicht vollständig abgeschlossen | Zentrale Modelloutput-Validierung ist vorhanden und der Golden Path ist wieder grün, aber zusätzliche Negativtests für Prompt-Injection und weitergehende Semantikgrenzen fehlen noch. | Hardening-Regressionen sind mit gezielten Negativtests abgesichert und verbleibende Grenzfälle sind dokumentiert. |
| P0 | Öffentliche API noch nicht vollständig produktionsreif abgesichert | Operator-Secret, CORS-Allowlist, Trusted Proxies, Rate Limits sowie Upload-/ZIP-Härtung sind umgesetzt und verifiziert. Offen bleiben Log-Redaction-Abnahme, Demo-Reset, Budget-Warnungen und das spätere HTTPS-/Deployment-Setup. | Öffentliche Demo ist gegen Manipulation und unkontrollierte API-Kosten geschützt und in der öffentlichen Umgebung vollständig konfiguriert. |
| P0 | Verwundbare Web-Abhängigkeiten | `npm audit --omit=dev` meldet am 18. Juli 2026 drei High- und einen Moderate-Fund; Next.js 16.2.2 und Playwright 1.54.2 sind betroffen. | Sichere Versionen installieren und alle Build-/E2E-Gates erneut grün ausführen. |
| P0 | Kein Remote-Repository | `git remote -v` ist leer. | Vollständigen Verlauf und Baseline-Tag zu einem Judge-zugänglichen Repository pushen. |
| P0 | Kein HTTPS-Zugang für reale Gerätetests | Die Demo ist lokal/LAN erreichbar, aber bislang ohne abgesicherten HTTPS-Zugriff für Mikrofon-/Kamera-Tests auf echten Geräten. | Interner HTTPS-Zugang mit Self-Signed-Zertifikat ist dokumentiert und auf echten Geräten getestet. |
| P0 | Submission-Material fehlt | Judge Guide, Architektur, Security-Hinweise, Evaluation, Video und Devpost-Texte fehlen. | Vollständiges, englisches Submission-Paket mit nachvollziehbaren Testschritten. |
| P1 | Vision-Image baut unnötig langsam | Python 3.14 kompiliert NumPy statt ein verfügbares Wheel zu verwenden. | Unterstützte Python-Basis mit reproduzierbarem, deutlich schnellerem Build. |

## 3. Challenge-Anforderungen und Nachweis

| Anforderung | Geplanter Nachweis |
|---|---|
| Projekt mit Codex und GPT-5.6 | Neue GPT-5.6-Responses-Integration, Build-Week-Commits, `BUILD_WEEK_CHANGELOG.md`, Codex-Session-ID. |
| Funktionsfähiges Projekt | Öffentliche Demo plus Docker-Compose-Installationsweg und grüner Golden-Path-Test. |
| Passender Track | Apps for Your Life: gemeinsames Freizeit-, Kreativitäts- und Tabletop-Erlebnis. |
| Projektbeschreibung | Englischer Problem-/Lösungs-/Impact-Text in README und Devpost. |
| Demo unter drei Minuten | Festes Skript aus Abschnitt 11, öffentliches YouTube-Video mit Audio. |
| Codex- und GPT-5.6-Nutzung erklären | Architekturdiagramm, Build-Week-Changelog, README-Abschnitt und Video-Segment. |
| Repository zugänglich | Öffentliches Repository mit Lizenz oder privates Repository mit den geforderten Judge-Einladungen. |
| Setup und Sample Data | Ein Befehl zum Start, `.env.example`, lizenzfreie Demo-Kampagne und Testanleitung. |
| Testzugang | Öffentliche HTTPS-Demo oder klarer Judge-Testzugang bis zum Ende der Bewertungsphase. |
| Bestehendes Projekt sinnvoll erweitert | Baseline-Commit vor der neuen Implementierung, danach kleine nachvollziehbare Build-Week-Commits. |
| Originalität und Rechte | Nur selbst erstellte oder nachweislich frei lizenzierte Texte, Bilder, Audio- und Demo-Assets. |

## 4. P0 – muss vor jeder Feature-Arbeit erledigt werden

### P0.1 Baseline und Git-Nachweis herstellen

- [x] `.gitignore` vor dem ersten Commit erweitern:
  - `.pnpm-store/`
  - `**/__pycache__/`
  - `*.pyc`
  - `apps/api/server`
  - `docs/private/`
  - `docs/**/*.pdf`, `docs/**/*.zip`, `docs/**/*.wav` oder die sensiblen Verzeichnisse vollständig
  - lokale Uploads, Datenbanken, Logs, Coverage und Testartefakte
- [x] Urheberrechtlich problematische und persönliche Dateien aus der Submission-Kopie entfernen oder außerhalb des Repositories ablegen.
- [x] Prüfen, dass kein API-Key, Passwort, Token, privater Hostname oder internes Zertifikat enthalten ist.
- [x] Git initialisieren.
- [x] Ersten Commit eindeutig als importierten Altbestand kennzeichnen, z. B. `chore: import pre-build-week baseline`.
- [x] Tag `pre-build-week-baseline` setzen.
- [x] `BUILD_WEEK_CHANGELOG.md` anlegen und ab dann jeden relevanten Commit mit Codex-/GPT-5.6-Bezug dokumentieren.
- [ ] GitHub-/GitLab-Repository anlegen, den vollständigen Verlauf pushen und sicherstellen, dass der Tag `pre-build-week-baseline` remote sichtbar ist.

**Abnahme:** `git status` ist sauber, `git ls-files` enthält keine privaten PDFs, ZIPs, WAVs, Binaries, Caches oder Secrets und der Baseline-Tag ist remote sichtbar.

### P0.2 Rechtssichere Demo-Daten

- [x] Das systemneutrale One-Page-Abenteuer **The Fungal Caverns** von Logen Nein als klar abgegrenzte Demo auswählen (CC BY 3.0 US).
- [x] Original-GM-PDF und unnummerierte Spielerkarte unverändert einbetten und per SHA-256 gegen die Quelldateien prüfen.
- [x] Englischen strukturierten AI-GM-Leitfaden und deutsche Übersetzung mit Bild-Cues erstellen.
- [x] Einen systemneutralen Beispielcharakter und eine sofort spielbare Startszene anlegen.
- [x] Quelle, Urheber, Lizenz, Änderungen und Lizenz-URL sowohl im Demo-Paket als auch in `THIRD_PARTY_NOTICES.md` dokumentieren.
- [x] Produktbeschreibung generisch als Tabletop-/Fantasy-RPG formulieren; keine Zugehörigkeit zu Dungeons & Dragons behaupten.

**Abnahme:** Eine frische Installation kann den vollständigen Demo-Flow ohne externe Bücher, private Dateien oder manuelles DB-Editing ausführen.

### P0.3 Golden Path reparieren

- [x] `scripts/mvp_smoke_test.sh` an das aktuelle `CreateSessionRequest` anpassen:
  - `name`
  - `ruleset_work`
  - `ruleset_version`
  - `target_player_count`
- [x] Den realen Start-Workflow korrekt abbilden: Spielerlink erstellen, Player beitreten/ready setzen, danach Session starten.
- [x] Fehlerausgaben mit Response-Body anzeigen, damit HTTP-400-Ursachen sofort sichtbar sind.
- [x] Idempotenten Demo-Seed mit Kampagne, Abenteuer, EN/DE-Dokumenten, Asset, Charakter und Live-Session bereitstellen.
- [x] OpenAI-Aufruf im Test entweder über echten opt-in API-Test oder einen deterministischen Mock ausführen.
- [x] Test am Ende mit klarer Zusammenfassung und nützlichen URLs beenden.

**Abnahme:** `docker compose up -d --wait && bash scripts/mvp_smoke_test.sh` läuft auf leerer Datenbank ohne manuellen Eingriff grün durch.

## 5. P0 – echte GPT-5.6-Integration

### P0.4 Provider sauber trennen

Neue Konfiguration:

```text
LLM_PROVIDER=openai
OPENAI_API_KEY=
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_MODEL=gpt-5.6
OPENAI_STORE=false
```

- [x] API-Key nur serverseitig aus der Umgebung lesen; nie über `GET /api/system/config` oder das Frontend ausgeben.
- [x] Lokalen OpenAI-kompatiblen Provider optional erhalten, aber OpenAI für Submission und Demo als klaren Standard dokumentieren.
- [x] Provider-spezifische Felder wie `chat_template_kwargs` nicht an OpenAI senden.
- [x] Health-/Summary-Antwort um Provider, Modell und Verbindungstest ergänzen, aber Secrets maskieren.
- [x] UI sichtbar anzeigen lassen: `Powered by GPT-5.6` und welcher Turn tatsächlich von welchem Modell erzeugt wurde.

**Betroffene Dateien:**

- `apps/api/internal/httpapi/config.go`
- `apps/api/internal/httpapi/llm.go`
- `apps/api/internal/httpapi/models.go`
- `apps/api/internal/httpapi/handlers_core.go`
- `apps/web/components/screens/control-center-screen.tsx`
- `.env.example`
- `docker-compose.yml`

### P0.5 Von JSON Mode zu echten Structured Outputs wechseln

Die bestehende Struktur ist eine sehr gute Basis, aber aktuell wird nur `json_object` angefordert und anschließend repariert. Für OpenAI soll ein striktes JSON-Schema verwendet werden.

- [x] OpenAI-Aufrufe über `POST /v1/responses` implementieren.
- [x] Für den Encounter-Turn ein versioniertes Schema `encounter_turn_v1` definieren.
- [x] Schema mindestens für folgende Felder festlegen:
  - `narration`
  - `language`
  - `rules_used`
  - `roll_request`
  - `state_updates`
  - `scene_events`
  - `dm_notes`
  - optional `speaker` und `media_cue`
- [x] Für alle Objekte `additionalProperties: false` verwenden und erforderliche Felder explizit markieren.
- [x] Refusals, unvollständige Responses, Timeouts, Rate Limits und ungültige Schemas als eigene Fehlerarten behandeln.
- [x] Den bestehenden Prosa-Fallback behalten, aber im UI und in Logs klar markieren.
- [x] JSON-Reparatur höchstens als letzter Fallback für lokale Provider behalten; der OpenAI-Hauptpfad darf im Normalfall keine zweite Modellrunde benötigen.
- [x] Für Endnutzer einen stabilen, datensparsamen `safety_identifier` mitsenden, z. B. einen serverseitig gehashten Player-/Session-Identifier.

**Architekturentscheidung:** Der eigene PostgreSQL-Session-State bleibt Source of Truth. OpenAI-Responses werden standardmäßig mit `store: false` verwendet, damit die vorhandene Memory- und Audit-Architektur nachvollziehbar bleibt.

### P0.5b OpenAI Speech

- [x] Spieleraufnahmen serverseitig über `POST /v1/audio/transcriptions` mit `gpt-4o-transcribe` verarbeiten.
- [x] Erzähltext serverseitig über `POST /v1/audio/speech` mit `gpt-4o-mini-tts` als WAV erzeugen.
- [x] Aktive Sprache und kurzen Tabletop-Kontext an die Transkription übergeben.
- [x] Bestehende Rollenprofile auf OpenAI-Stimmen abbilden; `cedar` als Standard verwenden.
- [x] OpenAI-Schlüssel ausschließlich im Backend halten und lokale Audio-Provider optional erhalten.
- [x] KI-generierte Stimmen im Player Screen und Character Builder sichtbar kennzeichnen.
- [x] Deterministische Provider-Mocks sowie einen echten TTS-zu-STT-Rundtrip ausführen.

### P0.6 Prompt und Zustandsänderungen absichern

- [x] Systemprompt in klare Abschnitte teilen: Rolle, Wahrheitshierarchie, Regeln, Player Agency, Output-Schema, Safety.
- [x] Adventure- und Dokumenttext ausdrücklich als nicht vertrauenswürdigen Kontext kennzeichnen; darin enthaltene Anweisungen dürfen Systemregeln nicht überschreiben.
- [x] Keine Modellantwort direkt als Systembefehl, Dateipfad, URL oder Datenbank-Query ausführen.
- [x] `state_updates` serverseitig über Allowlist validieren.
- [x] Unbekannte Entities und Felder ablehnen statt stillschweigend anwenden.
- [x] Würfelanforderungen validieren: erlaubte Würfel, Wertebereiche, DC-Bereich und maximale Anzahl.
- [x] Interne `dm_notes` niemals an Player Portal oder Player Screen ausliefern.

**Abnahme:** Ein manipulierter Abenteuertext kann weder Systemprompt noch Serveraktionen überschreiben; unerlaubte State Updates werden verworfen und geloggt.

## 6. P0 – Test- und Qualitätsnachweis

### Backend-Tests

- [x] Go-Unit-Tests für Parsing des Responses-API-Formats.
- [x] Tests für gültige, verweigerte und unvollständige Structured Outputs.
- [x] Tests für HTTP-Fehler, ungültiges JSON und schemawidrige Structured Outputs ergänzen.
- [x] Tests für `state_updates`-Allowlist und Würfelvalidierung.
- [x] Tests für Session-Memory/Kompaktierung und Trennung von Erzählungs- und Regelkontext.
- [x] Tests für Player-Safe-Serialisierung: keine DM Notes, versteckten DCs oder internen LLM-Session-IDs.
- [x] `httptest.Server` als deterministischer OpenAI-Mock.
- [x] Integrationstest: Spieleraktion → Roll Request → bestätigter Wurf → Zustandsänderung (Playwright E2E).

### Frontend/E2E

- [x] Playwright einrichten.
- [x] Golden Path testen: Demo öffnen, Charakter erstellen, Player beitreten, Turn senden, Wurf bestätigen, Narration und State Update sehen.
- [x] Kamera-Verweigerung und manuellen Würfel-Fallback testen.
- [x] Lade-, Fehler-, Rate-Limit- und LLM-Fallback-Zustände sichtbar und verständlich darstellen.
  - [x] Sichtbarer Rate-Limit-Fehler im Player Screen per Playwright abgesichert.
  - [x] Ladezustand, generischer Fehlerzustand und klarer LLM-Fallback-Text im Player Screen per Playwright abgesichert.
- [ ] Responsive Test für Operator Desktop und Player Smartphone.

### Build-Gates

```bash
npm ci
npm run build:web
docker compose config --quiet
docker compose build
docker compose up -d --wait
bash scripts/mvp_smoke_test.sh
npm run test:golden-path
```

- [x] `npm audit --omit=dev` geprüft: Ausgangszustand am 18. Juli 2026 mit drei High- und einem Moderate-Fund dokumentiert.
- [x] Next.js auf `16.2.10` und Playwright auf `1.55.1` aktualisiert; Audit, Build und Golden Path danach erneut erfolgreich ausgeführt. Verbleibend sind zwei Moderate-Funde aus `postcss` innerhalb `next`, aber keine High/Critical-Funde mehr.
- [ ] Vision-Basis auf eine Python-Version mit fertigen NumPy-Wheels umstellen oder Build-Cache sauber dokumentieren.
- [ ] CI-Workflow hinzufügen, der Build, Tests und Secret Scan ausführt.

**Abnahme:** Keine High/Critical-Funde mehr in den Submission-relevanten Node-Abhängigkeiten; Web-Build und Golden Path bleiben nach dem Upgrade vollständig grün. Für den finalen Ship-Zustand müssen zusätzlich noch Vision-Basisimage, frischer Checkout und Dokumentation abgeschlossen werden.

## 7. P0 – öffentlicher Demo-Betrieb und Sicherheit

Die aktuelle API darf nicht unverändert öffentlich erreichbar sein.

- [x] Einen bewusst kleinen Demo-Sicherheitsmechanismus wählen:
  - Operator-Routen durch Login/Testkonto oder Reverse-Proxy-Auth schützen.
  - Player-Zugriff ausschließlich über zufällige, widerrufbare Tokens.
- [x] `Access-Control-Allow-Origin: *` durch konfigurierbare Allowlist ersetzen.
- [x] Trusted Proxies explizit konfigurieren.
- [x] Rate Limits für GPT-, Upload-, STT- und Vision-Endpunkte setzen.
- [x] Request- und Upload-Größen begrenzen.
- [x] MIME-Type, Dateierweiterung, Archivinhalt und Pfade gegen Zip-Slip/Path-Traversal validieren.
- [x] `PUT /api/system/config` nur für Operatoren freigeben oder im Demo-Deployment deaktivieren.
- [x] Demo-Daten regelmäßig zurücksetzen; keine Nutzerinhalte langfristig speichern.
- [x] Logs dürfen keine API-Keys, vollständigen Prompts mit privaten Inhalten oder Player-Tokens enthalten.
- [x] Internen HTTPS-Zugang mit Self-Signed-Zertifikat bereitstellen.
- [ ] Interne Demo auf den benötigten Geräten während der Testphase stabil verfügbar halten.
- [x] Budget- und Rate-Limit-Warnungen für den OpenAI-Key konfigurieren.

**Abnahme:** Kernschutz für die lokale/interne Demo ist serverseitig implementiert und der Golden Path bleibt grün. Für den finalen P0-Abschluss fehlen jetzt nur noch der interne HTTPS-Zugang für Gerätetests sowie die stabile Verfügbarkeit während der Testphase.

## 8. P1 – Produkt- und Demo-Polish

### Ein fokussierter Demo-Modus

- [ ] Startseite mit genau einem primären CTA: `Start the demo adventure`.
- [x] Demo automatisch mit Seed-Kampagne, Abenteuer, Charakter und Session vorbereiten.
- [ ] Operator View auf die für den Golden Path nötigen Elemente reduzieren:
  - aktueller Spielerinput
  - GPT-5.6-Status
  - erwarteter Würfelwurf
  - erkannter/bestätigter Wurf
  - angewendete State Updates
  - Player-Screen-Vorschau
- [ ] Ein sichtbares „Why this happened“-Panel ergänzen: verwendete Regel-/Abenteuerquellen, keine Chain-of-Thought-Ausgabe.
- [ ] Statuschips für `GPT-5.6`, `camera`, `speech`, `player connected`.
- [ ] Manuelle Fallbacks immer erreichbar halten: Text statt Sprache, Würfelwert statt Kamera, Browser-Audio statt externem TTS-Service.
- [x] Fehler in klare Benutzeraktionen übersetzen, z. B. `Retry turn`, `Use manual roll`, `Continue without voice`.

### Design

- [x] Alle sichtbaren Demo-Texte auf Englisch vereinheitlichen.
- [x] Vollständige persistente Sprachumschaltung für Deutsch und Englisch anbieten.
- [ ] Mobile Player-Ansicht mit realem Smartphone testen.
- [ ] Visuelle Hierarchie für Video optimieren: große Narration, klarer Würfelstatus, wenig Verwaltungsrauschen.
- [ ] Leere und Lade-Zustände gestalten.
- [ ] Barrierearme Kontraste, Tastaturbedienung und sichtbare Fokuszustände prüfen.

## 9. P1 – README und Submission-Paket

Die neue englische README soll in dieser Reihenfolge aufgebaut sein:

1. Produktname und Ein-Satz-Pitch.
2. Hero-Screenshot oder kurzes GIF.
3. Problem und Zielgruppe.
4. Was während OpenAI Build Week neu gebaut wurde.
5. Wie GPT-5.6 eingesetzt wird.
6. Wie Codex die Entwicklung beschleunigt hat und welche Entscheidungen der Mensch getroffen hat.
7. Architekturdiagramm.
8. Demo-Link und Demo-Anleitung.
9. Lokales Setup in wenigen Schritten.
10. Konfiguration der OpenAI-Umgebungsvariablen.
11. Tests und reproduzierbare Befehle.
12. Datenschutz, Limits und bekannte Einschränkungen.
13. Lizenz und Third-Party Notices.

Zusätzliche Dateien:

- [x] `LICENSE` – MIT-Lizenz für den eigenen Quellcode; Drittinhalte bleiben unter ihren jeweiligen Lizenzen.
- [x] `BUILD_WEEK_CHANGELOG.md` – Baseline versus neue Arbeit.
- [x] `THIRD_PARTY_NOTICES.md` – verwendete Assets und Lizenzen.
- [x] `SECURITY.md` – verantwortliche Meldung und Demo-Grenzen.
- [x] `docs/architecture.md` – kompakte System- und Datenflussgrafik.
- [x] `docs/judge-testing.md` – fünfminütiger Testablauf und Zugangsdaten.
- [x] `docs/demo-script.md` – finales Video-Drehbuch.
- [x] `docs/evals.md` – Testfälle und Ergebnisse für GPT-5.6-Turns.

## 10. P1 – kleine, aussagekräftige GPT-5.6-Evaluation

Mindestens 12 feste Fälle, davon:

- 3 normale Szenenfortsetzungen.
- 2 Würfelanforderungen mit korrektem Typ und DC.
- 2 Auflösungen nach Würfelergebnis.
- 2 Regel-/Adventure-Grounding-Fälle mit Quellen.
- 1 Prompt-Injection im hochgeladenen Abenteuertext.
- 1 ungültiges State Update.
- 1 Deutsch-/Englisch-Konsistenzfall.

Messgrößen:

- JSON-Schema-Erfolgsquote.
- Anteil der Turns ohne Reparatur/Fallback.
- korrekter Roll-Request.
- korrekte State Updates.
- keine geheimen Informationen im Player Output.
- Quellenbezug vorhanden und passend.
- Latenz p50/p95 und grobe Token-Nutzung.

Zielwerte für die Demo:

- 100 % schema-valide Antworten in den festen Testfällen.
- 0 unerlaubte State Updates.
- 0 Leaks von DM Notes oder Hidden DC.
- Golden Path in drei Wiederholungen ohne manuellen Neustart erfolgreich.

## 11. Video unter drei Minuten

### Drehbuch

| Zeit | Inhalt |
|---|---|
| 0:00–0:15 | Problem: Tabletop-Abende brauchen Vorbereitung und häufig einen menschlichen Game Master. |
| 0:15–0:30 | Lösung und Zielgruppe in einem Satz; Operator- und Player-Screen nebeneinander zeigen. |
| 0:30–0:50 | Spieler tritt per Link/QR bei; Charakter erscheint auf dem Smartphone. |
| 0:50–1:15 | Spieler spricht eine Aktion; UI zeigt Transkript und GPT-5.6 verarbeitet Adventure-, Charakter- und Session-Kontext. |
| 1:15–1:40 | GPT-5.6 fordert strukturiert einen Wurf; physischer Würfel wird erkannt und bestätigt. |
| 1:40–2:05 | GPT-5.6 erzählt die Konsequenz; Player Screen, Inventar/Quest/Scene State aktualisieren sich. |
| 2:05–2:25 | Kurz zeigen: Quellenbezug, getrennte private DM Notes und Player-safe Ausgabe. |
| 2:25–2:45 | Technischer Überblick: Responses API, Structured Outputs, Go, Next.js, Vision, PostgreSQL. |
| 2:45–2:58 | Codex-Beitrag, menschliche Produktentscheidungen und Abschluss. |

### Video-Regeln

- [ ] Gesamtlänge sicher unter 3:00, Ziel 2:50–2:58.
- [ ] Verständliche englische Tonspur; alternativ vollständige englische Übersetzung/Untertitel.
- [ ] Öffentlich auf YouTube.
- [ ] Keine geschützten Bücher, Markenassets, Musik oder persönlichen Daten im Bild.
- [ ] GPT-5.6 und Codex ausdrücklich nennen und ihre Rollen erklären.
- [ ] Nur tatsächlich funktionierende Features zeigen.

## 12. Zeitplan bis zur Deadline

### 18. Juli – Produktkern abgeschlossen

- [x] Repository bereinigt, Git-Baseline und Tag erstellt.
- [x] Lizenzfreie Demo-Daten integriert.
- [x] GPT-5.6 Responses API und strict Structured Outputs integriert.
- [x] OpenAI STT/TTS integriert.
- [x] API- und Browser-Golden-Path erstellt und erfolgreich ausgeführt.
- [x] Deutsch/Englisch vollständig umgesetzt.

### 19. Juli – Hardening, Abhängigkeiten und Regressionstests

- P0.6 auf dem nun verifizierten Stand halten und die offenen Negativtests ergänzen.
- Negative Backend-Tests für Prompt Injection, zusätzliche semantisch ungültige State Updates, ungültige Würfel und fehlerhafte Responses ergänzen.
- Next.js und Playwright sind bereits auf den verifizierten Mindeststand aktualisiert; offen bleibt nur noch die Dokumentation des Rest-Risikos mit zwei Moderate-Funden.
- Vision-Basisimage auf eine Python-Version mit NumPy-Wheels umstellen.
- Danach `npm audit`, Web-Build, Go-Tests und den isolierten Golden Path ausführen.

### 20. Juli – Öffentliche Demo und Submission-Paket

- Operator-Schutz, CORS-Allowlist, Rate Limits und Größenlimits implementieren.
- Operator-Reset für `DELETE /api/demo/fungal-caverns` verifizieren und in den Judge-Runbook aufnehmen.
- HTTPS-Deployment mit stabiler URL bereitstellen.
- Demo auf Desktop und echtem Smartphone mit Kamera, Mikrofon und manuellem Fallback testen.
- README finalisieren sowie `SECURITY.md`, Architektur, Judge Guide, Evaluation und Video-Drehbuch erstellen.
- Remote-Repository veröffentlichen und einen frischen Clone vollständig testen.

### 21. Juli – Submission Day

- Morgens: alle Ship-Gates und den echten GPT-5.6-Golden-Path dreimal ausführen.
- Demo-Video aufnehmen, auf unter drei Minuten schneiden, englische Tonspur/Untertitel prüfen und öffentlich hochladen.
- Devpost-Texte, Kategorie, Screenshots, Repository-, Demo- und Video-Link eintragen.
- Codex `/feedback`-Session-ID dieser Kernimplementierung sichern und eintragen.
- Submission spätestens mehrere Stunden vor 17:00 PDT abschicken.
- Danach nur noch Verfügbarkeit und Logs kontrollieren; keine riskanten Featureänderungen mehr.

## 13. Harte Ship-/No-Ship-Gates

Das Projekt wird nur eingereicht, wenn alle folgenden Punkte erfüllt sind:

- [x] GPT-5.6 wird im echten Golden Path aufgerufen und `raw_model`/Provider ist nachvollziehbar.
- [ ] Die Mehrheit der neuen Kernfunktion wurde in einer dokumentierten Codex-Session gebaut.
- [x] Baseline und Build-Week-Commits sind klar getrennt.
- [x] Keine urheberrechtlich problematischen oder privaten Dateien im Repository oder Video.
- [ ] Frischer Checkout startet nach README-Anleitung.
- [x] Golden-Path-Smoke-Test ist grün.
- [x] Web- und API-Build sind grün.
- [x] Mindestens die kritischen Backend- und E2E-Tests sind grün.
- [ ] Öffentliche Demo ist per HTTPS erreichbar und gegen Missbrauch begrenzt.
- [ ] Judge-Testanleitung funktioniert ohne Rückfrage.
- [ ] Video ist öffentlich, hat Audio und ist kürzer als drei Minuten.
- [ ] README erklärt Codex, GPT-5.6, menschliche Entscheidungen und neue Build-Week-Arbeit.
- [x] Lizenz und Third-Party Notices sind vorhanden.
- [ ] Repository-Link und Demo-Link sind bis zum Ende der Bewertung verfügbar.

## 14. Verbindliche nächste Schritte

Die folgenden Arbeitspakete werden in dieser Reihenfolge umgesetzt. Ein Paket gilt erst als abgeschlossen, wenn seine Abnahme erfüllt ist.

### Schritt 1 – Modelloutput und Prompt-Kontext absichern

**Status:** ✅ Abgeschlossen (18. Juli 2026).

**Ziel:** GPT-5.6 darf nur erlaubte Änderungen am serverseitigen Spielzustand auslösen.

**Umsetzung:**

1. [x] In `apps/api/internal/httpapi` eine zentrale Validierungsfunktion für `GMResponse` ergänzt, die vor jeder Persistierung ausgeführt wird (`validation.go`).
2. [x] Für `state_updates` eine explizite Allowlist der im Golden Path benötigten Felder definiert; unbekannte Felder und Entities werden verworfen.
3. [x] Datentypen und Eingabemodi für die freigegebenen Golden-Path-Felder geprüft: numerische Updates nur per `delta` oder numerischem `value`, Listen-/Notizfelder nur per nichtleerem `value`.
4. [x] `roll_request` validiert: nur unterstützte Typen (`attack|damage|check|save`), höchstens 10 Würfel insgesamt, plausible DC-Grenzen (1–50), gültige Dice-Notation inklusive `d20` und maximal eine direkte Follow-up-Stufe.
5. [x] Fehlerhafte Modelländerungen werden nicht teilweise übernommen. Sie werden verworfen, ohne interne Details an Player auszugeben; Rejects werden in Log und DM Notes protokolliert.
6. [x] Systemprompts in Rolle, Wahrheitshierarchie, Regeln, Player Agency, untrusted context, Output und Safety gegliedert.
7. [x] `untrusted_context_rule` im `gmUserPrompt` eingefügt; Adventure- und Dokumenttext wird im Prompt explizit als unzuverlässige Benutzerdaten behandelt.
8. [x] Sicherstellen, dass Modelltexte niemals als Dateipfad, URL, Shell-Befehl oder SQL ausgeführt werden.

**Tests:**

- [x] Erlaubtes State Update wird vollständig übernommen.
- [x] Unbekanntes Feld und unbekannte Entity werden vollständig verworfen.
- [x] Zusätzliche semantische Grenzen wie negative Gold-/XP-Werte werden explizit als Negativtests ergänzt oder bewusst dokumentiert.
- [x] Ungültiger Würfel, zu viele Würfel, DC außerhalb des Bereichs und ungültige Dice-Notation werden abgewiesen.
- [x] Ein expliziter Prompt-Injection-Test mit `ignore previous instructions` für Adventure-/Dokumentkontext wird ergänzt.
- [x] Player-Antwort enthält weiterhin keine `dm_notes`, versteckten DCs oder internen IDs.
- [x] Bestehender Golden Path bleibt mit aktivierter Servervalidierung vollständig grün.

**Abnahme:** Die zentrale Servervalidierung ist aktiv, die zusätzlichen Validation-Tests sind grün, und `npm run test:golden-path` bleibt nach dem Hardening vollständig grün.

### Schritt 2 – Verwundbare Abhängigkeiten beseitigen

**Status:** ✅ Kernziel erreicht (18. Juli 2026), ein Restpunkt zum Vision-Image bleibt offen.

**Ziel:** Keine bekannten High- oder Critical-Funde in den für die Submission installierten Node-Abhängigkeiten.

**Umsetzung:**

1. [x] Next.js von `16.2.2` auf `16.2.10` aktualisieren.
2. [x] `@playwright/test` auf `1.55.1` aktualisieren.
3. [x] Lockfile ausschließlich über `npm install` aktualisieren; kein `npm audit fix --force`.
4. [ ] Vision-Dockerfile von Python 3.14 auf eine stabile Python-Version mit passenden NumPy-Wheels umstellen und den Image-Build prüfen.

**Abnahmebefehle:**

```bash
npm audit --omit=dev
npm run build:web
docker compose config --quiet
docker compose build
npm run test:golden-path
```

**Verifiziert am 18. Juli 2026:**

- [x] `npm audit --omit=dev` meldet keine High-/Critical-Funde mehr.
- [x] `npm run build:web` ist grün.
- [x] `npm run test:golden-path` ist nach dem Upgrade vollständig grün.
- [ ] Vision-Basisimage ist noch nicht auf schnellere NumPy-Wheels umgestellt.

**Restzustand:** Zwei Moderate-Funde aus `postcss` innerhalb `next` bleiben sichtbar. Für den aktuell formulierten P0-Abhängigkeits-Exit sind sie nicht blockierend; sie sollten aber im Submission-Paket knapp dokumentiert werden, falls bis zur Deadline kein sauberer Upstream-Fix ohne Seiteneffekt verfügbar ist.

### Schritt 3 – Öffentliche Demo absichern

**Ziel:** Anonyme Juroren können den vorgesehenen Demo-Flow verwenden, aber keine Administration übernehmen oder unbegrenzt Kosten erzeugen.

**Umsetzung:**

1. Operator-Routen und sämtliche schreibenden Verwaltungsendpunkte mit einem serverseitigen Demo-Operator-Secret oder vorgeschalteter Authentifizierung schützen.
2. `PUT /api/system/config`, Modelltest, Modellliste, Uploads, Löschen sowie Kampagnen-/Session-Verwaltung nur für Operatoren erlauben.
3. Player-Zugriffe nur über zufällige, widerrufbare Join-/Portal-Tokens erlauben; keine Sessiondaten allein anhand einer erratbaren ID ausgeben.
4. `Access-Control-Allow-Origin: *` durch `CORS_ALLOWED_ORIGINS` ersetzen und nur die HTTPS-Demo-Domain sowie lokale Entwicklungsadressen erlauben.
5. Trusted Proxies explizit setzen; Forwarded Headers nur von diesen Proxies akzeptieren.
6. Rate Limits getrennt für GPT, STT/TTS, Vision, Demo-Seed, Join und Uploads setzen.
7. Request- und Uploadgrößen begrenzen. MIME-Type, Dateiendung, ZIP-Einträge und Zielpfade gegen Zip-Slip/Path-Traversal prüfen.
8. [x] Logs auf API-Keys, Authorization-Header, Player-Tokens und vollständige private Prompts prüfen und diese Werte redigieren.
9. [x] Demo-Daten automatisch oder per dokumentiertem Admin-Vorgang zurücksetzen.
10. [x] OpenAI-Projektbudget und Warnschwellen außerhalb der App konfigurieren bzw. app-seitig sichtbar machen (`systemSummary` + `.env.example`/Compose-Defaults für Soft-/Hard-Limit und Alert-E-Mail).

**Verifiziert am 18. Juli 2026:**

- [x] Operator-geschützte Admin-Routen, CORS-Allowlist und Trusted Proxies sind aktiv.
- [x] Rate Limits für Demo-Seed, `gm/respond`, STT, Vision und Character Builder sind aktiv.
- [x] Upload-, Audio- und ZIP-Grenzen inklusive Path-Traversal-/Typ-Prüfung sind aktiv.
- [x] `go test ./internal/httpapi/...` ist grün.
- [x] `npm run test:golden-path` bleibt mit aktivierter Security-Schicht vollständig grün.

**Restzustand:** Für den Abschluss von Schritt 3 fehlen jetzt nur noch HTTPS und die tatsächliche öffentliche Laufzeitverfügbarkeit. Die serverseitigen Security- und Kosten-Guardrails sind implementiert.

### Schritt 4 – Internes HTTPS bereitstellen und auf echten Geräten testen

**Ziel:** Kamera, Mikrofon und Audio funktionieren im tatsächlichen Judge-Szenario.

**Umsetzung:**

1. Web unter einem stabilen internen Hostnamen mit Self-Signed-Zertifikat per HTTPS bereitstellen; Datenbank und Redis nicht öffentlich exponieren.
2. Healthchecks, Restart-Policy und persistente Konfiguration prüfen.
3. Operator Desktop, Player Screen und Player Portal gleichzeitig testen.
4. Auf einem echten Smartphone testen: Join-Link/QR, Spracheingabe, TTS-Wiedergabe, Kameraerlaubnis, Würfelerkennung und manueller Würfel-Fallback.
5. Fehlerfälle testen: Mikrofon verweigert, Kamera verweigert, OpenAI-Timeout, Rate Limit und Audio-Autoplay blockiert.
6. Demo während der lokalen Test- und Aufnahmesessions stabil erreichbar halten.

**Abnahme:** Der Golden Path läuft über die interne HTTPS-URL auf Desktop plus Smartphone dreimal hintereinander ohne Serverneustart.

**Status 20. Juli 2026:** Self-Signed-HTTPS unter `https://dungeon-master.local:3443` ist lokal aufgebaut und per Healthcheck verifiziert. Offen sind die realen Gerätetests über Desktop/Smartphone sowie die dreifache störungsfreie End-to-End-Abnahme.

### Schritt 5 – Submission-Dokumentation fertigstellen

**Ziel:** Juroren verstehen Nutzen, technische Tiefe und Testweg ohne Rückfrage.

**Zu erstellen oder zu überarbeiten:**

- `README.md`: Pitch, Zielgruppe, Screenshots, Build-Week-Neuentwicklung, GPT-5.6-Nutzung, Codex-Beitrag, menschliche Entscheidungen, Architektur, Demo, Setup, Tests, Datenschutz und Einschränkungen.
- `SECURITY.md`: unterstützte Demo-Grenzen, verantwortliche Meldung und Umgang mit Zugangsdaten.
- `docs/architecture.md`: Client → Go API → GPT-5.6/Speech/Vision → PostgreSQL/Redis sowie Player-safe Datenfluss.
- `docs/judge-testing.md`: öffentlicher Link, Testzugang und ein höchstens fünfminütiger Schritt-für-Schritt-Test.
- `docs/demo-script.md`: sekundengenaues englisches Drehbuch unter drei Minuten.
- `docs/evals.md`: feste Testfälle, Messwerte, Resultate und bekannte Grenzen.
- `BUILD_WEEK_CHANGELOG.md`: alle weiteren Safety-, Deployment- und Submission-Commits ergänzen.

**Abnahme:** Eine unbeteiligte Person kann nur mit README/Judge Guide die Demo starten, den Golden Path abschließen und GPT-5.6-/Codex-Beitrag erklären.

### Schritt 6 – Repository veröffentlichen und frischen Clone prüfen

**Umsetzung:**

1. Repository auf GitHub oder GitLab erstellen.
2. Vollständigen Branch und alle Tags pushen; `pre-build-week-baseline` muss sichtbar sein.
3. Repository öffentlich schalten oder `testing@devpost.com` und `build-week-event@openai.com` Zugriff geben.
4. Secret-, Dateinamen-, Marken- und Lizenzscan ausführen; jedes binäre Asset manuell gegen `THIRD_PARTY_NOTICES.md` prüfen.
5. In einem neuen Verzeichnis frisch klonen und ausschließlich nach README starten.
6. Dort `npm ci` und `npm run test:golden-path` ausführen.

**Abnahme:** Frischer Clone funktioniert ohne lokale, nicht versionierte Dateien; Repository- und Tag-Links sind von außen erreichbar.

### Schritt 7 – Evaluation und finale Regression

1. Die zwölf Fälle aus Abschnitt 10 ausführen und Ergebnisse dokumentieren.
2. Deterministischen Golden Path einmal und echten GPT-5.6-Golden-Path dreimal ausführen.
3. DE und EN jeweils mindestens einmal vollständig durchspielen.
4. Player-safe Ausgabe, Quellenanzeige, State Updates, STT/TTS und manuelle Fallbacks prüfen.
5. Keine neuen Features mehr beginnen, sobald alle Ship-Gates grün sind.

### Schritt 8 – Video und Devpost-Submission

1. Englisches Video nach Abschnitt 11 aufnehmen; Ziel 2:50 bis 2:58 Minuten.
2. Audio, Untertitel, lesbare UI und fehlende private/geschützte Inhalte prüfen.
3. Video öffentlich auf YouTube hochladen.
4. Devpost-Projektbeschreibung, Kategorie **Apps for Your Life**, Screenshots sowie Repository-, Demo- und Video-Link eintragen.
5. Erklären, welche Kernfunktionen Codex beschleunigt hat, wie GPT-5.6 zur Laufzeit arbeitet und welche Entscheidungen bewusst vom Menschen getroffen wurden.
6. Mit `/feedback` die Codex-Session-ID der Kernimplementierung ermitteln und in das Submission-Formular eintragen.
7. Submission mehrere Stunden vor der Deadline absenden und anschließend alle Links in einem privaten Browserfenster prüfen.

Ein schmaler, dreimal reproduzierbarer Demo-Flow ist wertvoller als weitere halbfertige Features.

## 15. Offizielle Quellen

- [OpenAI Build Week Official Rules](https://openai.devpost.com/rules)
- [OpenAI model guidance for GPT-5.6](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-5.6)
- [Migrate to the Responses API](https://developers.openai.com/api/docs/guides/migrate-to-responses)
- [Structured model outputs](https://developers.openai.com/api/docs/guides/structured-outputs)

Hinweis: Der OpenAI Developer Docs MCP wurde während dieser Analyse eingerichtet. Da neu eingerichtete MCP-Server in der laufenden Sitzung noch nicht verfügbar waren, wurden für diesen Plan ausschließlich die offiziellen OpenAI-Webseiten als Dokumentations-Fallback verwendet.
