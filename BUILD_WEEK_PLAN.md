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

### Aktuelle Blocker für eine Einreichung

| Blocker | Beobachtung | Konsequenz |
|---|---|---|
| Kein Git-Verlauf | Die Kopie ist noch kein Git-Repository. | Build-Week-Arbeit kann nicht sauber vom Altbestand getrennt werden. |
| Kein funktionsfähiger Golden-Path-Test | `scripts/mvp_smoke_test.sh` sendet einen veralteten Session-Payload und bricht mit HTTP 400 ab. | README-Versprechen und tatsächliche API sind nicht synchron. |
| Keine automatisierten Tests | Es wurden keine Go-, Python-, TypeScript- oder E2E-Testdateien gefunden. | Hohes Regressionsrisiko und schwacher Nachweis technischer Qualität. |
| GPT-5.6 nicht integriert | Der aktive Client verwendet `/chat/completions`, lokale Modell-Defaults und provider-spezifische Felder. | Die Challenge-Kernanforderung ist noch nicht erfüllt. |
| Unsicher für öffentlichen Betrieb | Keine API-Authentifizierung, `Access-Control-Allow-Origin: *`, frei änderbare Systemkonfiguration. | Ein öffentliches Deployment wäre manipulierbar. |
| Urheberrecht/IP | Unter `docs/` liegen D&D-Regelbücher, Abenteuer-ZIPs/PDFs und persönliche Sprachaufnahmen. | Diese Dateien dürfen nicht in das Submission-Repository oder Video gelangen. |
| Repository-Hygiene | Binäres Go-Artefakt, `__pycache__`, `.pnpm-store`, Build-Artefakte und mehrere Lockfiles liegen lokal vor. | Sehr großes oder unsauberes Repository; potenziell falsche Dateien beim ersten Commit. |
| Keine Lizenz | Keine Projektlizenz vorhanden. | Öffentliches Repository erfüllt die Submission-Anforderung nicht sauber. |
| README veraltet | Interne Pfade, LAN-IP-Adressen und lokale Qwen-/Mistral-Defaults dominieren die Anleitung. | Juroren können das Projekt nicht reproduzierbar testen. |
| Dependency-Risiken | Der Web-Container meldet bei `npm install` eine moderate und eine hohe Schwachstelle. | Vor Veröffentlichung prüfen und gezielt beheben oder dokumentieren. |
| Langsamer Vision-Build | Python 3.14 kompiliert NumPy lokal; der frische Vision-Build dauert mehrere Minuten. | Erhöht Ausfall- und Zeitrisiko für Juroren und CI. |

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
- [ ] GitHub-/GitLab-Repository anlegen und den Baseline-Commit vor der Implementierung pushen.

**Abnahme:** `git status` ist sauber, `git ls-files` enthält keine privaten PDFs, ZIPs, WAVs, Binaries, Caches oder Secrets und der Baseline-Tag ist remote sichtbar.

### P0.2 Rechtssichere Demo-Daten

- [ ] Eine kleine eigene Kampagne schreiben, z. B. **The Clockwork Observatory**.
- [ ] Umfang bewusst klein halten: eine Startszene, ein NPC, ein Konflikt, ein Geheimnis, ein möglicher Würfelwurf.
- [ ] Eigene minimalistische Regeldatei erstellen, die nur die für die Demo notwendigen Checks beschreibt.
- [ ] Eigene Beispielcharaktere und Handouts erstellen.
- [ ] Nur eigene oder eindeutig frei lizenzierte Bilder und Sounds einsetzen; Quelle und Lizenz in `THIRD_PARTY_NOTICES.md` dokumentieren.
- [ ] Produktbeschreibung generisch als Tabletop-/Fantasy-RPG formulieren; keine Zugehörigkeit zu Dungeons & Dragons behaupten.

**Abnahme:** Eine frische Installation kann den vollständigen Demo-Flow ohne externe Bücher, private Dateien oder manuelles DB-Editing ausführen.

### P0.3 Golden Path reparieren

- [x] `scripts/mvp_smoke_test.sh` an das aktuelle `CreateSessionRequest` anpassen:
  - `name`
  - `ruleset_work`
  - `ruleset_version`
  - `target_player_count`
- [x] Den realen Start-Workflow korrekt abbilden: Spielerlink erstellen, Player beitreten/ready setzen, danach Session starten.
- [x] Fehlerausgaben mit Response-Body anzeigen, damit HTTP-400-Ursachen sofort sichtbar sind.
- [ ] Demo-Seed oder Adventure-Import in den Test aufnehmen.
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

- [ ] Systemprompt in klare Abschnitte teilen: Rolle, Wahrheitshierarchie, Regeln, Player Agency, Output-Schema, Safety.
- [ ] Adventure- und Dokumenttext ausdrücklich als nicht vertrauenswürdigen Kontext kennzeichnen; darin enthaltene Anweisungen dürfen Systemregeln nicht überschreiben.
- [ ] Keine Modellantwort direkt als Systembefehl, Dateipfad, URL oder Datenbank-Query ausführen.
- [ ] `state_updates` serverseitig über Allowlist validieren.
- [ ] Unbekannte Entities und Felder ablehnen statt stillschweigend anwenden.
- [ ] Würfelanforderungen validieren: erlaubte Würfel, Wertebereiche, DC-Bereich und maximale Anzahl.
- [ ] Interne `dm_notes` niemals an Player Portal oder Player Screen ausliefern.

**Abnahme:** Ein manipulierter Abenteuertext kann weder Systemprompt noch Serveraktionen überschreiben; unerlaubte State Updates werden verworfen und geloggt.

## 6. P0 – Test- und Qualitätsnachweis

### Backend-Tests

- [x] Go-Unit-Tests für Parsing des Responses-API-Formats.
- [ ] Tests für gültige, verweigerte, unvollständige und fehlerhafte Structured Outputs.
- [ ] Tests für `state_updates`-Allowlist und Würfelvalidierung.
- [ ] Tests für Session-Memory/Kompaktierung und Trennung von Erzählungs- und Regelkontext.
- [ ] Tests für Player-Safe-Serialisierung: keine DM Notes, versteckten DCs oder privaten Dokumente.
- [x] `httptest.Server` als deterministischer OpenAI-Mock.
- [ ] Mindestens ein Integrationstest: Spieleraktion → Roll Request → bestätigter Wurf → Zustandsänderung.

### Frontend/E2E

- [ ] Playwright einrichten.
- [ ] Golden Path testen: Demo öffnen, Player beitreten, Turn senden, Wurf bestätigen, Narration und State Update sehen.
- [ ] Kamera-Verweigerung und manuellen Würfel-Fallback testen.
- [ ] Lade-, Fehler-, Rate-Limit- und LLM-Fallback-Zustände sichtbar und verständlich darstellen.
- [ ] Responsive Test für Operator Desktop und Player Smartphone.

### Build-Gates

```bash
npm ci
npm run build:web
docker compose config --quiet
docker compose build
docker compose up -d --wait
bash scripts/mvp_smoke_test.sh
```

- [ ] `npm audit` prüfen; Findings gezielt aktualisieren oder begründet dokumentieren, kein blindes `--force`.
- [ ] Vision-Basis auf eine Python-Version mit fertigen NumPy-Wheels umstellen oder Build-Cache sauber dokumentieren.
- [ ] CI-Workflow hinzufügen, der Build, Tests und Secret Scan ausführt.

**Abnahme:** Alle Gates laufen in einem frischen Checkout grün; die Resultate werden in README oder Submission dokumentiert.

## 7. P0 – öffentlicher Demo-Betrieb und Sicherheit

Die aktuelle API darf nicht unverändert öffentlich erreichbar sein.

- [ ] Einen bewusst kleinen Demo-Sicherheitsmechanismus wählen:
  - Operator-Routen durch Login/Testkonto oder Reverse-Proxy-Auth schützen.
  - Player-Zugriff ausschließlich über zufällige, widerrufbare Tokens.
- [ ] `Access-Control-Allow-Origin: *` durch konfigurierbare Allowlist ersetzen.
- [ ] Trusted Proxies explizit konfigurieren.
- [ ] Rate Limits für GPT-, Upload-, STT- und Vision-Endpunkte setzen.
- [ ] Request- und Upload-Größen begrenzen.
- [ ] MIME-Type, Dateierweiterung, Archivinhalt und Pfade gegen Zip-Slip/Path-Traversal validieren.
- [ ] `PUT /api/system/config` nur für Operatoren freigeben oder im Demo-Deployment deaktivieren.
- [ ] Demo-Daten regelmäßig zurücksetzen; keine Nutzerinhalte langfristig speichern.
- [ ] Logs dürfen keine API-Keys, vollständigen Prompts mit privaten Inhalten oder Player-Tokens enthalten.
- [ ] HTTPS und stabile öffentliche URL bereitstellen.
- [ ] Demo bis mindestens zum Ende der Judging Period verfügbar halten.
- [ ] Budget- und Rate-Limit-Warnungen für den OpenAI-Key konfigurieren.

**Abnahme:** Ein anonymer Besucher kann die Demo ausprobieren, aber weder Systemkonfiguration ändern noch fremde Sessions/Uploads verwalten oder unbegrenzt API-Kosten erzeugen.

## 8. P1 – Produkt- und Demo-Polish

### Ein fokussierter Demo-Modus

- [ ] Startseite mit genau einem primären CTA: `Start the demo adventure`.
- [ ] Demo automatisch mit Seed-Kampagne, Charakter und Session vorbereiten.
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
- [ ] Fehler in klare Benutzeraktionen übersetzen, z. B. `Retry turn`, `Use manual roll`, `Continue without voice`.

### Design

- [ ] Alle sichtbaren Demo-Texte auf Englisch vereinheitlichen.
- [ ] Deutsche UI-Fragmente im Golden Path entfernen oder vollständige Sprachumschaltung anbieten.
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

- [ ] `LICENSE` – passende Lizenz bewusst auswählen.
- [ ] `BUILD_WEEK_CHANGELOG.md` – Baseline versus neue Arbeit.
- [ ] `THIRD_PARTY_NOTICES.md` – verwendete Assets und Lizenzen.
- [ ] `SECURITY.md` – verantwortliche Meldung und Demo-Grenzen.
- [ ] `docs/architecture.md` – kompakte System- und Datenflussgrafik.
- [ ] `docs/judge-testing.md` – fünfminütiger Testablauf und Zugangsdaten.
- [ ] `docs/demo-script.md` – finales Video-Drehbuch.
- [ ] `docs/evals.md` – Testfälle und Ergebnisse für GPT-5.6-Turns.

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

### 18. Juli – Fundament und Nachweis

- P0.1 Repository bereinigen, Git-Baseline, Tag und Remote.
- P0.2 lizenzfreie Demo-Daten definieren.
- P0.3 Smoke-Test reparieren.
- GPT-5.6-Provider-/Responses-Architektur implementieren beginnen.

### 19. Juli – GPT-5.6 Encounter Director

- Responses API vollständig anbinden.
- Structured Output Schema und Fehlerbehandlung.
- State-Update-Validierung und Player-Safe-Ausgabe.
- Deterministische OpenAI-Mocktests.
- Ersten echten GPT-5.6-Golden-Path dreimal ausführen.

### 20. Juli – Demo und Stabilität

- Demo-Modus und Seed-Daten.
- Kamera-, Sprache- und manuelle Fallbacks.
- Playwright Golden Path.
- Public-Deployment-Schutz, HTTPS und Rate Limits.
- README, Changelog, Architektur und Judge-Testanleitung.

### 21. Juli – Submission Day

- Morgens: vollständiger Test auf frischem Checkout und separatem Gerät.
- Demo-Video aufnehmen, schneiden, Untertitel prüfen und hochladen.
- Devpost-Texte, Screenshots, Repository- und Demo-Links eintragen.
- Codex `/feedback`-Session-ID sichern.
- Submission spätestens mehrere Stunden vor 17:00 PDT abschicken.
- Danach Demo-Verfügbarkeit und Logs kontrollieren, keine riskanten Featureänderungen mehr.

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

## 14. Reihenfolge der Umsetzung

Wenn Zeit knapp wird, gilt strikt:

1. IP-/Secret-Bereinigung und Git-Baseline.
2. Funktionsfähiger Smoke-Test.
3. Echte GPT-5.6-Responses-Integration.
4. Structured Outputs und State-Safety.
5. Lizenzfreie Demo-Daten.
6. Stabiler Golden Path mit manuellen Fallbacks.
7. Kritische Tests.
8. Öffentlich abgesichertes Deployment.
9. README, Judge Guide und Video.
10. Erst danach zusätzlicher visueller Polish.

Ein schmaler, dreimal reproduzierbarer Demo-Flow ist wertvoller als weitere halbfertige Features.

## 15. Offizielle Quellen

- [OpenAI Build Week Official Rules](https://openai.devpost.com/rules)
- [OpenAI model guidance for GPT-5.6](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-5.6)
- [Migrate to the Responses API](https://developers.openai.com/api/docs/guides/migrate-to-responses)
- [Structured model outputs](https://developers.openai.com/api/docs/guides/structured-outputs)

Hinweis: Der OpenAI Developer Docs MCP wurde während dieser Analyse eingerichtet. Da neu eingerichtete MCP-Server in der laufenden Sitzung noch nicht verfügbar waren, wurden für diesen Plan ausschließlich die offiziellen OpenAI-Webseiten als Dokumentations-Fallback verwendet.
