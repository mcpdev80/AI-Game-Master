## Projekt-Handoff fuer Codex

## Projektname
AI Game Master Platform

## Ziel
Baue in diesem Repository ein plattformübergreifendes MVP für Pen-and-Paper-Rollenspiele, das als Game Master unterstützt oder teilweise autonom leitet.

Das System soll:
1. Abenteuer, Regelwerke und Charakterboegen aus PDFs einlesen und als Wissensbasis verwenden.
2. In mehreren Sprachen erzaehlen koennen und innerhalb einer Sprache unterschiedliche Stimmen bzw. Voice-Profile fuer Erzähler, NPCs und Kreaturen nutzen koennen.
3. Wuerfelergebnisse ueber eine am Laptop angeschlossene Kamera erkennen.
4. Audio-Ambience und Soundeffekte abspielen.
5. Video-/Ambient-Inhalte ueber Beamer oder Leinwand abspielen.
6. Eine Weboberflaeche fuer Setup, Tests, Betrieb und DM-Uebersicht bereitstellen.
7. Auf Linux, Windows und macOS laufen.
8. In einer Minimalvariante komplett auf einem einzelnen Rechner lauffaehig sein.
9. Spaeter optional verteilt laufen koennen, z. B. LLM auf externer GPU-Maschine und andere Dienste lokal oder auf Proxmox.

---

## Leitentscheidung fuer dieses Repo

Dieses Projekt wird nicht nach der urspruenglichen Python/FastAPI-Vorgabe umgesetzt, sondern mit dem tatsaechlichen Zielstack dieses Repos:

- Frontend: Next.js
- Hauptbackend/API: Go
- Realtime: WebSocket
- Datenbank: PostgreSQL fuer produktiv, SQLite fuer lokalen MVP falls sinnvoll
- Deployment: Docker Compose zuerst
- AI-/LLM-/Embedding-Anbindung: providerbasiert und austauschbar
- Dokument-Ingestion, Vision und Mediensteuerung: als getrennte Dienste oder Module, nicht als monolithische Kernlogik

Wichtig:
- Go ist das primaere Backend fuer API, Session-State, Eventing, Orchestrierung und Persistenz.
- AI-spezifische Teilprobleme wie Embeddings, OCR, Vision oder spaetere komplexe CV-Pipelines werden so entkoppelt, dass sie bei Bedarf auch als eigener Dienst umgesetzt werden koennen.
- Die Architektur wird also Go-first, aber nicht Go-only um jeden Preis.

---

## Produktentscheidung
Die Anwendung wird in 2 Hauptteile getrennt:

### 1. Core Backend
Verantwortlich fuer:
- API
- WebSocket Events
- LLM-Orchestrierung
- Session Logic
- Game State
- Persistenz
- Dokumentverwaltung
- Eventvalidierung
- Medien- und Vision-Integration

### 2. Client-/Device-Layer
Verantwortlich fuer:
- Kamerazugriff auf dem Laptop
- Mikrofon-/Audiotests
- Geraetestatus
- lokale browserbasierte UI
- optional spaeter Desktop-Wrapping

Wichtig:
- Die Kamera haengt am Laptop.
- Deshalb darf der Kamerazugriff nicht fest an einen zentralen Host gekoppelt sein.
- Die Architektur muss so sein, dass Kamera-Frames oder Erkennungsergebnisse vom Laptop an das Backend uebertragen werden.

---

## Rollenmodell

Dieses Produkt geht nicht von einem menschlichen Game Master aus.

Stattdessen gibt es 3 klar getrennte Rollen:

### 1. Admin
Der Admin ist die menschliche Operator-Rolle.

Verantwortlich fuer:
- Session vorbereiten
- Kampagne, Abenteuer und Regelwerke auswaehlen
- Charaktere zuordnen
- Geraete pruefen
- Session starten / pausieren / stoppen
- Invite-Links fuer Spieler verteilen
- im Fehlerfall manuell eingreifen

Wichtig:
- Der Admin ist nicht der eigentliche Spielleiter.
- Die UI darf daher nicht davon ausgehen, dass ein Mensch die Narration laufend manuell uebernimmt.

### 2. KI-Dungeon-Master
Die KI ist der eigentliche DM.

Verantwortlich fuer:
- Erzaehlung
- Reaktion auf Spieleraktionen
- Regelauslegung
- Monster- und Abenteuerwissen
- automatisches Triggern von Medien
- automatische Steuerung des Player-Screens
- Beruecksichtigung von Charakter-, Session- und Weltzustand

Wichtig:
- Die aktive Session ist daher keine klassische DM-Konsole fuer einen Menschen, sondern eine Admin-/Operator-Oberflaeche fuer die laufende KI-geleitete Session.

### 3. Spieler
Spieler sind menschliche Teilnehmer am Tisch oder im lokalen Netzwerk.

Sie sollen:
- ueber einen lokalen Link beitreten koennen
- ihren Charakter auf dem Handy oder Laptop aufrufen koennen
- freigegebene Handouts und Informationen sehen koennen
- spaeter optional eigene Interaktionen ausloesen koennen

---

## Player Portal / lokale Einladungslinks

Das System soll im lokalen Netzwerk pro Spieler oder pro Charakter einen eigenen Zugangslink bereitstellen koennen.

Beispiel:
- `https://example.test/join/abc123`

Diese Links sollen z. B. per WhatsApp oder Messenger im lokalen Netzwerk verteilt werden koennen.

### Ziel
- Spieler koennen ohne separaten App-Installationsaufwand beitreten
- jeder Spieler kann den eigenen Charakter digital aufrufen
- die KI und der Session-State bleiben die zentrale Quelle der Wahrheit

### Anforderungen
- Link-basiertes Beitreten im lokalen Netz
- jeder Link ist einem `player_slot` oder `character_slot` zugeordnet
- Zugriff soll nur freigegebene, player-safe Inhalte zeigen
- der Admin erzeugt oder verwaltet diese Links vor Sessionstart
- die KI kann waehrend der Session automatisch relevante Inhalte fuer Spieler freigeben

### Inhalte im Player Portal
- Charakteransicht
- Basiswerte
- Portrait / Sheet-Zusammenfassung
- freigegebene Handouts
- freigegebene Bilder oder Battlemaps
- spaeter optional Wuerfel- oder Aktions-Inputs

### Wichtig
- Das Player Portal ist eine eigene Rolle und eigene UI.
- Es darf keine DM-internen Daten oder Notizen offenlegen.
- Player-Screen und Player-Portal sind verwandt, aber nicht identisch:
  - Player-Screen = gemeinsame Ausgabeflaeche fuer Beamer / Zweitmonitor
  - Player-Portal = individueller Link pro Spieler / Charakter

---

## Technische Leitplanken

- Primaere UI: Next.js-Webanwendung
- Hauptbackend: Go
- API-Stil: REST + WebSocket
- Validierung: saubere Request-/Event-Schemas
- Datenbank: PostgreSQL zuerst; SQLite nur fuer lokalen, vereinfachten Betrieb wenn praktikabel
- Queue/Eventbus: optional, fuer MVP nicht verpflichtend
- PDF-Extraktion: als eigener Ingestion-Baustein mit austauschbarer Implementierung
- Vektorindex: lokal nutzbare, austauschbare Loesung
- Vision: separater Dienst oder separates Modul mit klarer API
- Mediensteuerung: separater Dienst oder separates Modul mit klarer API
- Deployment: Docker Compose zuerst
- Spaeter: optional Kubernetes/Proxmox-Verteilung
- OpenClaw oder MCP-Tools: optional als Orchestrierungsschicht, nicht als Kernlogik und nicht als Ersatz fuer das Backend

---

## Zielplattformen

Muss funktionieren auf:
- Linux
- Windows
- macOS

Wichtig:
- Kernfunktionalitaet muss browserbasiert laufen.
- Es darf keine OS-spezifische UI-Abhaengigkeit im MVP geben.
- Wenn native Features spaeter benoetigt werden, kann ein optionaler Desktop-Wrapper ergaenzt werden.

---

## Hauptmodule

### A. Web UI
Baue eine Weboberflaeche mit folgenden Bereichen:

#### 1. Dashboard
- Status aller Dienste
- Modellstatus
- Geraeteverfuegbarkeit
- aktive Session
- aktuell laufende Audio-/Video-Szene
- letzte Wuerfelerkennung
- letzte KI-Aktion

#### 2. Setup / Tests
- Kamera testen
- Mikrofon testen
- Audioausgabe testen
- Videoausgabe / Beamer testen
- PDF-Import pruefen
- LLM-Verbindung pruefen
- Embedding-/RAG-Status pruefen

#### 3. Bibliothek
- Regelwerke hochladen
- Abenteuer hochladen
- Charakterboegen hochladen
- Assets hochladen:
  - Audio
  - Soundeffekte
  - Videos
  - Bilder

Zielstruktur der Library:
- `Uebersicht`
- `Rulebooks`
- `Adventures`
- `Assets`

Wichtig:
- `Campaigns` gehoeren nicht in die Library.
- `Rulebooks` werden nach `Werk + Version` organisiert, z. B.:
  - `5E -> 2014`
  - weitere rechtmäßig bereitgestellte Regelprofile
  - `DSA -> 4`
  - `How to be a Hero -> default`
- `Adventures` koennen einem oder mehreren Regelwerken kompatibel zugeordnet sein oder ungebunden bleiben.
- `Assets` werden als Galerie dargestellt, nach Typ gruppiert und optional Regelwerken oder Adventures zugeordnet.

Upload-Flows:
- Jeder Haupttab hat einen eigenen `+ Add`-Flow.
- Uploads laufen in Modalen.
- Uploads muessen sichtbare States haben:
  - `idle`
  - `uploading`
  - `success`
  - `error`

Benachrichtigungen:
- app-weites Notify-System oben rechts
- neue Meldungen 5 Sekunden als Toast
- History dauerhaft in der Bell/Inbox sichtbar
- relevante Meldungen:
  - Upload erfolgreich
  - Upload fehlgeschlagen
  - Import mit Warnungen
  - Systemfehler

#### 4. Kampagnen-/Session-Verwaltung
- Kampagne anlegen
- Abenteuer auswaehlen
- Charaktere zuordnen
- Regelwerk zuordnen
- Session starten / stoppen

Zielstruktur von `Sessions`:

Die Session-Verwaltung ist die Orchestrierungsschicht zwischen `Library` und `Control Center`.

`Library` sagt:
- welche Inhalte verfuegbar sind

`Control Center` sagt:
- welche Technik bereit ist

`Sessions` verbindet beides:
- fuer den konkreten Spielabend

Die Seite `Sessions` wird in 3 Haupttabs gegliedert:

##### a) `Uebersicht`
- Liste aller Sessions
- Filter:
  - `draft`
  - `ready`
  - `live`
  - `paused`
  - `finished`
- pro Session sichtbar:
  - Session-Name
  - Kampagne
  - Adventure
  - Regelwerk + Version
  - Anzahl Player-Slots
  - Join-Status
  - letzte Aenderung
- Aktionen:
  - `Open`
  - `Duplicate`
  - `Delete`
  - `Start`

##### b) `Setup`
- eigentlicher Session-Builder
- verbindet Inhalte aus der Library mit dem Betriebsstatus aus dem Control Center

Sektionen:
- `Core`
  - Session-Name
  - Sprache
  - Kampagne optional
- `Rules & Adventure`
  - Regelwerk/Werk + Version
  - passende Rulebooks
  - Adventure
  - zusaetzliche Kontexte / Dokumente
- `Characters & Players`
  - Player-Slots
  - Character-Zuordnung
  - Join-Links
- `Devices & AI`
  - Kamera-Status
  - Audio-/Mic-Status
  - LLM-/TTS-Status
  - Player Screen / Portal Status
- `Readiness`
  - klare Checkliste
  - Session erst dann `ready`

##### c) `Runtime`
- Status der laufenden Session
- verbundene Spieler
- letzte Freigaben
- aktives Device-/LLM-Setup
- Aktionen:
  - `Open Active Session`
  - `Pause`
  - `Stop`
  - `Return to Setup`

Session-Datenmodell fuer die spaetere Ausbaustufe:
- `campaign_id` optional
- `adventure_id` optional
- `ruleset_work`
- `ruleset_version`
- `document_ids` fuer relevante Rulebooks/Zusatzdokumente
- `character_ids` bzw. Slot-Zuordnungen
- `device_profile`
- `llm_profile`

Session-Memory und Kontextverwaltung:

Damit die Context-Laenge nicht unkontrolliert waechst und eine Kampagne ueber mehrere Abende fortgesetzt werden kann, braucht jede Session eine feste Summary-/Memory-Architektur.

Die Session darf nicht nur aus einem linearen Chatverlauf bestehen.

Stattdessen werden 5 Ebenen unterschieden:

1. `Turn Memory`
- die letzten relevanten Interaktionen
- kurzlebig
- dient nur dem direkten Antwortkontext

2. `Rolling Session Summary`
- fortlaufende, verdichtete Zusammenfassung dessen, was in der aktuellen Session passiert ist
- wird nach wichtigen Beats aktualisiert:
  - Szenenwechsel
  - Kampfende
  - Questfortschritt
  - wichtige NPC-Interaktion
  - relevante Zustandsaenderung

3. `Structured Session State`
- strukturierte Fakten, die nicht verloren gehen duerfen
- Beispiele:
  - `active_npcs`
  - `quest_state`
  - `location_state`
  - `loot_changes`
  - `party_flags`
  - `revealed_secrets`
  - `player_visible_changes`

4. `Campaign Memory`
- zusammengefasster Langzeitkontext ueber mehrere Sessions
- damit neue Sessions die Story fortsetzen koennen
- dient als persistente Kontinuitaet zwischen Spielabenden

5. `Session Recap`
- kurze, lesbare Zusammenfassung fuer den Start der naechsten Sitzung
- verwendbar fuer:
  - Admin-Ansicht
  - Player-Screen
  - Player-Portal
  - KI-Einstieg in die naechste Session

Anforderungen:
- Zusammenfassungen muessen regelmaessig waehrend der Session erstellt werden, nicht nur am Ende.
- Summaries duerfen nicht nur Freitext sein; es braucht immer auch strukturierte Zustandsupdates.
- Die KI muss auf Basis von:
  - aktuellem Turn-Kontext
  - Rolling Summary
  - Structured State
  - Campaign Memory
  Antworten generieren koennen.
- Beim Start einer neuen Session fuer dieselbe Kampagne muss der aktuelle Session-Kontext aus:
  - Campaign Memory
  - letztem Session Recap
  - relevanten Structured Facts
  rekonstruiert werden koennen.

Ziel:
- lange Kampagnen bleiben steuerbar
- Context-Limits werden nicht sofort ausgeschopft
- neue Sessions koennen sauber an alte Sessions anknuepfen
- die KI behaelt Kontinuitaet, ohne den kompletten Rohverlauf immer wieder zu laden

LLM-Session-Handling:

Zusätzlich zur inhaltlichen Memory-Architektur braucht das System eine klare Trennung der eigentlichen LLM-Kontexte.

Es darf nicht einen einzigen endlosen Modell-Chat fuer alle Aufgaben geben.

Stattdessen werden pro fachlichem Vorgang eigene LLM-Sessions gefuehrt.

Benötigte Session-Typen:

1. `character_builder_session`
- nur fuer die Character-Erstellung
- eigener Chatverlauf
- eigener Builder-Guide
- eigene Summary
- darf nicht mit laufender Spielsession vermischt werden

2. `level_up_session`
- nur fuer Aufstieg und spaetere Character-Weiterentwicklung
- eigener Leitfaden
- getrennt vom Character-Builder

3. `campaign_play_session`
- eigentliche KI-geleitete Spielsession
- Narration
- NPC-Reaktionen
- Regelauslegung im Spielkontext
- Wuerfelinterpretation

4. `rules_lookup_session`
- nur fuer kurze, gezielte Regelfragen
- moeglichst kleiner Kontext
- keine volle Story-Historie

5. `summary_session`
- nur fuer Verdichtung und Zusammenfassungen
- schreibt Rolling Summary, Recap und Kampagnenzusammenfassungen
- antwortet nicht direkt als DM

6. `prep_or_planning_session`
- fuer Session-Vorbereitung, Abenteuerstruktur, Encounter-Planung oder Medienplanung
- getrennt vom Live-Spiel

Jede LLM-Session braucht:
- `session_type`
- `session_scope_id`
  - z. B. `character_id`, `campaign_id`, `play_session_id`
- `message_history`
- `working_summary`
- `structured_state`
- `last_active_at`
- `request_profile`
- `token_budget`

`request_profile` steuert pro Typ:
- `temperature`
- `max_tokens`
- `response_format`
- `timeout`
- `system_prompt_variant`

Beispiele:
- `builder`
- `level_up`
- `narration`
- `rules_lookup`
- `summary`
- `planning`

Zentrale Regeln:
- Character-Erstellung bekommt immer eine eigene Builder-Session.
- Die laufende Spielsession bekommt eine eigene Play-Session.
- Summary-/Recap-Arbeit laeuft nie im selben Modellkontext wie die Narration.
- Regelfragen sollen nach Moeglichkeit in einen eigenen kleinen Lookup-Kontext gehen.
- Ein Character-Builder darf nicht den Story-Kontext der laufenden Session aufblasen.
- Eine laufende Session darf nicht mit dem kompletten Character-Build-Verlauf belastet werden.

Produktziel:
- sauber getrennte Modellkontexte
- bessere Stabilitaet
- weniger unnötiger Tokenverbrauch
- gezielte Prompt- und Timeout-Profile pro Aufgabe
- bessere Wiederaufnahme von Charakterbau, Level-Up und Kampagnenfortsetzung

#### 4b. Player-Links / Beitritt
- pro Session lokale Invite-Links erzeugen
- Spieler oder Charaktere einem Link zuordnen
- Status sehen:
  - offen
  - beigetreten
  - getrennt
- Link erneuern / sperren
- Sichtbarkeit freigegebener Inhalte kontrollieren

#### 4c. Character-System

Vor einer sauberen Session-Orchestrierung muss das Character-System fachlich richtig modelliert werden.

Der aktuelle reine Attribut-/Dice-Fokus reicht nicht aus.

Benötigt wird ein eigener Character-Bereich mit 4 klaren Modi:

##### a) `Character-Liste`
- alle erstellten Charaktere als Liste oder Galerie
- Filter:
  - Regelwerk
  - Version
  - Kampagne
  - Spielerzuordnung
- pro Eintrag sichtbar:
  - Name
  - Portrait optional
  - Volk / Klasse / Stufe
  - Regelwerk + Version
  - Status:
    - `draft`
    - `ready`
    - `assigned`
- Aktionen:
  - `Open`
  - `Edit`
  - `Duplicate`
  - `Delete`
  - `Assign to Session`

##### b) `Character-Detail`
- komplette Anzeige eines einzelnen Charakters
- unterteilt in:
  - Basisprofil
  - Attribute
  - Skills
  - Ausruestung
  - Zauber / Faehigkeiten
  - Hintergrund / Notizen
  - Metadaten / Quelle

##### c) `Character-Editor`
- bestehende Charaktere bearbeitbar
- nicht nur Attribute, sondern vollstaendige strukturierte Character-Daten
- mit Speichern als `draft` oder `ready`

##### d) `KI-gefuehrter Character-Builder`

Das ist der Zielmodus fuer V1/V2.

Der Character-Builder muss regelwerkgebunden arbeiten:
- zuerst `Werk + Version` waehlen
- dann erzeugt die KI nur Optionen, die zu diesem Regelwerk passen

Beispiele:
- `5E-compatible 2014`
- weitere rechtmäßig bereitgestellte Regelprofile
- `DSA 4`
- `How to be a Hero`

Der Flow wird KI-gefuehrt und schrittweise aufgebaut:

Zusaetzlich braucht der Builder pro Regelwerk einen **Builder-Leitfaden**.

Dieser Leitfaden soll nicht hart in einem einzelnen Prompt versteckt sein, sondern als regelwerkgebundene Konfigurationsdatei in der Library haengen.

Empfohlene Form:
- YAML-Dateien pro `ruleset_work + ruleset_version`
- Beispiel:
  - `docs/builder-guides/dnd-5e.character-builder.yaml`

Der Leitfaden definiert:
- Reihenfolge der Character-Erstellung
- welche Felder die KI erfragen muss
- welche Werte automatisch berechnet werden
- welche Sheet-Felder in welchem Schritt geschrieben werden
- welche Guardrails gelten

Wichtig:
- Der Leitfaden ist **Ablauf- und Feldlogik**
- Die eigentlichen Regeln kommen weiter aus den ausgewaehlten Rulebooks
- Damit kann dasselbe Prinzip spaeter auch fuer andere Regelwerke genutzt werden:
  - weitere rechtmäßig bereitgestellte Regelprofile
  - `DSA 4`
  - `How to be a Hero`

Schritt 1: Regelwerk waehlen
- Werk
- Version
- Sprache falls relevant

Schritt 2: Grundkonzept
- die KI fragt nach:
  - Charakteridee
  - Rolle
  - Stimmung
  - Setting-Fit
- alternativ:
  - mehrere KI-Vorschlaege zur Auswahl

Schritt 3: Regelwerkskonforme Ableitungen
- je nach Regelwerk:
  - Volk / Spezies
  - Klasse / Profession
  - Hintergrund
  - Startfaehigkeiten
  - Startwerte

Die KI darf hier nicht frei halluzinieren, sondern muss an importierte Rulebooks und Character-Regeln gebunden sein.

Schritt 4: Werte erzeugen
- unterstuetzte Modi:
  - `standard array`
  - `point buy`
  - `rolled`
- wenn `rolled`:
  - kamera-/dice-unterstuetzter Flow
  - gefuehrte Teilwuerfe statt freier Mischwuerfe
  - Werte danach korrigierbar und bestaetigbar

Schritt 5: Review / Confirm
- KI zeigt den fertigen Character zusammengefasst
- Nutzer kann jeden Bereich korrigieren
- erst danach:
  - `Save Draft`
  - `Mark Ready`

Wichtige Produktentscheidung:
- Character-Erstellung ist nicht nur ein Formular
- sie ist ein gefuehrter KI-Prozess mit regelwerkgebundenem Ergebnis

Character-Leitprinzipien:
- ein Character gehoert fachlich zu einem `Werk + Version`
- Sessions koennen spaeter nur Characters desselben oder kompatiblen Regelwerks sinnvoll zuordnen
- Character-Daten muessen fuer die KI wiederverwendbar und strukturiert sein
- Import aus PDFs bleibt wichtig, aber ist nur einer von mehreren Einstiegswegen:
  - `Import existing sheet`
  - `Create with AI`
  - `Create manually`

#### 5. DM-Uebersicht
- aktuelle Szene
- NPC-Liste
- Initiative
- HP / Zustaende
- erkannte Wuerfelwuerfe
- Session-Log
- KI-Vorschlag / erzaehlte Antwort
- manuelle Korrekturen
- manuelles Ausloesen von Sound/Video

Wichtig:
- Dieser Bereich ist eine Admin-/Operator-Uebersicht fuer die laufende KI-Session, nicht die primaere manuelle DM-Steuerung eines Menschen.

#### 6. Settings
- Sprache pro Session
- Sprache pro Spielerprofil
- Voice-Profile pro Session, Rolle, NPC oder Kreaturentyp
- Erzaehlstil
- Automatisierungsgrad:
  - nur assistierend
  - halbautomatisch
  - staerker autonom
- Vision-Empfindlichkeit
- Medienregeln
- LLM-Provider / Modellwahl

#### 7. Player Portal
- individueller Link pro Spieler / Charakter
- mobilfreundliche Darstellung
- Charakterdaten anzeigen
- freigegebene Handouts anzeigen
- freigegebene Bilder / Karten anzeigen
- spaeter optional Spieleraktionen oder Wuerfe uebermitteln

---

### B. Game Master Engine
Verantwortlich fuer:
- Erzaehlung
- Reaktion auf Spieleraktionen
- Einweben von Wuerfelergebnissen
- Zugriff auf Regelwissen
- Zugriff auf Abenteuerwissen
- Zugriff auf Charakterwissen
- Erzeugen strukturierter Aktionen

Wichtig:
Das LLM soll nicht nur Freitext liefern, sondern strukturierte Antworten.

Beispiel-Ausgabeformat:

```json
{
  "narration": "Der Pfeil trifft den Ork in die Schulter.",
  "language": "de",
  "rules_used": ["basic_attack_resolution"],
  "state_updates": [
    {"entity_id": "orc_1", "field": "hp", "delta": -8}
  ],
  "scene_events": [
    {"type": "sfx", "name": "arrow_hit"},
    {"type": "music", "name": "battle_low"}
  ],
  "dm_notes": [
    "Ork wirkt angeschlagen"
  ]
}
```

Regeln:
- Jede Antwort des GM-Systems soll maschinenlesbar sein.
- Die UI rendert daraus Text, Aenderungen und Medienaktionen.
- Kernlogik fuer State-Updates darf nicht nur in Prompts versteckt werden.
- Das Backend validiert strukturierte LLM-Antworten serverseitig.

---

### C. PDF / RAG Modul

Die Anwendung soll PDFs aus 3 Klassen verarbeiten:
1. Regelwerk
2. Abenteuer
3. Charakterbogen

#### Anforderungen
- Upload ueber Web UI
- Speicherung des Originaldokuments
- Extraktion von Text
- Zerlegung in sinnvolle Chunks
- Speicherung mit Metadaten
- Embeddings erzeugen
- retrieval-faehig machen

#### Metadaten pro Chunk
- `source_type`: `rules | adventure | character_sheet`
- `document_name`
- `page_number`
- `section_title`
- `character_name` optional
- `campaign_id` optional

#### Nutzungslogik
- Regelwerk nur fuer Regelfragen und Regelauswertung
- Abenteuer als Story-Grundlage
- Charakterboegen fuer Werte, Inventar, Faehigkeiten, Zauber etc.

#### Wichtig
- Abenteuertext ist nur Grundlage
- Laufender Session State hat Vorrang
- Spielerentscheidungen duerfen Abenteuerverlauf veraendern
- Das System darf nicht starr nur den PDF-Text nacherzaehlen

---

### D. Session / Game State

Baue ein persistentes Zustandsmodell.

#### Benoetigte Entitaeten
- Campaign
- Session
- Player
- Character
- NPC
- Location
- Quest
- Combat
- InitiativeEntry
- Condition
- InventoryItem
- SessionEvent
- DiceRollEvent
- MediaState

#### Muss speicherbar sein
- aktueller Ort
- aktuelle Szene
- bekannte NPCs
- Queststatus
- HP / Conditions
- Initiative
- letzte Wuerfelwuerfe
- Zeitlinie der Session
- welche Audio-/Video-Szene aktiv ist

---

### E. Vision / Dice Service

Separater Dienst oder separates Modul fuer Wuerfelerkennung.

#### Ziel
- Kamera haengt am Laptop
- Kamera wird lokal angesprochen
- erkannte Wuerfelwerte werden als Events an Backend gesendet

#### MVP
- feste Kamera
- definierter Dice Tray empfohlen
- zunaechst d20
- danach d6, d8, d10, d12, d100

#### API Event

```json
{
  "event_type": "dice_roll",
  "dice": [
    {"type": "d20", "value": 17}
  ],
  "confidence": 0.94,
  "timestamp": "2026-04-01T20:14:00+02:00"
}
```

#### Regeln
- Vision-Service darf unabhaengig vom LLM laufen
- Erkennungsergebnis muss in UI sichtbar sein
- DM muss Wert bestaetigen oder korrigieren koennen

---

### F. Media Director

Separater Dienst oder separates Modul fuer Mediensteuerung.

#### Aufgaben
- Ambience starten/stoppen
- Soundeffekte abspielen
- Hintergrundvideo starten/wechseln
- Lautstaerkeprofile verwalten
- Szenenwechsel umsetzen

#### Input
Maschinenlesbare Events vom GM-System, z. B.

```json
[
  {"type": "ambience", "name": "forest_night"},
  {"type": "sfx", "name": "twig_snap"},
  {"type": "video", "name": "campfire_loop"}
]
```

#### Asset-Mapping
Baue Asset-Tabellen oder Konfigurationen:
- `ambience_id -> Datei`
- `sfx_id -> Datei`
- `video_id -> Datei`

#### Externe Audioquelle fuer den MVP
Fuer Ambient-Sound und einfache Szenen-Audioausgabe soll im MVP bevorzugt `Tabletop Audio` genutzt werden:
- Website: `https://tabletopaudio.com`
- Nutzung ueber direkte Stream-URLs, kein SDK notwendig
- Vorteil: kostenlos, schnell integrierbar, gut passend fuer `dungeon`, `tavern`, `battle`, `horror`, `forest` und aehnliche Szenen

Beispielhafte Stream-URLs:
- `https://tabletopaudio.com/forest_at_night.mp3`
- `https://tabletopaudio.com/dungeon_1.mp3`
- `https://tabletopaudio.com/tavern.mp3`
- `https://tabletopaudio.com/battle.mp3`

Beispiel fuer Mapping im Media Director:

```json
{
  "forest_night": {
    "type": "stream",
    "provider": "tabletopaudio",
    "url": "https://tabletopaudio.com/forest_at_night.mp3"
  },
  "dungeon_dark": {
    "type": "stream",
    "provider": "tabletopaudio",
    "url": "https://tabletopaudio.com/dungeon_1.mp3"
  },
  "battle_low": {
    "type": "stream",
    "provider": "tabletopaudio",
    "url": "https://tabletopaudio.com/battle.mp3"
  }
}
```

#### Playback-Idee im Go-Service
Der Media Director kann Streams direkt ueber externe Player wie `ffplay` starten.

Beispielidee:

```go
package main

import "os/exec"

func playStream(url string) error {
    cmd := exec.Command("ffplay", "-nodisp", "-autoexit", url)
    return cmd.Start()
}
```

Wichtige Architekturentscheidung:
- Tabletop Audio ist fuer den MVP die bevorzugte Ambient-Quelle
- lokale Dateien und spaetere andere Provider sollen aber ueber dasselbe Mapping-Modell austauschbar bleiben
- der Media Director darf also nicht hart auf nur eine Quelle verdrahtet werden

#### UI-Funktionen
- manuelles Testen
- manuelles Triggern
- Vorschau
- Stop/Resume

---

### G. Speech Layer

Optional im MVP, aber Architektur vorbereiten.

#### Eingehend
- Speech-to-Text fuer Spieleraktionen

#### Ausgehend
- Text-to-Speech fuer Erzaehlerstimme
- Text-to-Speech mit unterschiedlichen Voice-Profilen je nach Rolle oder Figur, z. B. tiefe raue Stimme fuer Orks, helle oder weiche Stimme fuer Elfen, neutrale Stimme fuer Narration

#### Voice-Profil-Konzept
- Stimme kann nach Sprache, Rolle, NPC, Fraktion oder Kreaturentyp gemappt werden
- Voice-Mapping muss konfigurierbar sein
- Fuer jede Session soll festgelegt werden koennen, welche Stimme fuer Narration, Monster, NPCs oder einzelne Schluesselfiguren genutzt wird
- Wenn kein passendes Voice-Profil verfuegbar ist, muss ein Fallback auf Standardstimme oder reine Textausgabe moeglich sein

#### Wichtig
- Muss deaktivierbar sein
- Textausgabe darf immer ohne TTS funktionieren

---

## Architekturprinzipien

1. Strikte Trennung zwischen Narration, Regelermittlung, Zustand, Vision und Medien.
2. Keine Kernlogik nur im Prompt verstecken.
3. Geschaeftslogik in Services, nicht in UI.
4. Jedes LLM-Ergebnis strukturiert zurueckgeben.
5. UI muss Korrekturen erlauben.
6. Alles lokal lauffaehig halten.
7. Remote-/Verteilbetrieb nur als Erweiterung.
8. AI-Provider, Embedding-Provider und Vektorindex austauschbar halten.
9. Gemeinsame Datenmodelle ueber klar versionierte Schemas oder OpenAPI/JSON-Schema definieren.

---

## Deployment-Modi

### Modus 1: Single Machine
Alles auf einem Rechner:
- Web UI
- Backend
- DB
- RAG
- Vision
- Media Director
- optional lokales LLM

### Modus 2: Split Deployment
- Laptop: Browser + Kamera
- GPU-/LLM-Host: LLM + Embeddings + ggf. Vision
- lokaler oder externer Server: API + DB + UI + Media Services

Wichtig:
Der Code muss beide Modi unterstuetzen.

---

## API-Design (erste Version)

### REST Endpoints
- `POST /api/documents/upload`
- `GET /api/documents`
- `POST /api/campaigns`
- `GET /api/campaigns`
- `POST /api/sessions`
- `GET /api/sessions/{id}`
- `POST /api/sessions/{id}/start`
- `POST /api/sessions/{id}/stop`
- `POST /api/sessions/{id}/player-links`
- `GET /api/sessions/{id}/player-links`
- `POST /api/player-portal/join/{token}`
- `GET /api/player-portal/me`
- `GET /api/voice-profiles`
- `POST /api/voice-profiles`
- `PUT /api/voice-assignments/{id}`
- `POST /api/vision/dice-roll`
- `POST /api/media/play`
- `POST /api/media/stop`
- `POST /api/gm/respond`
- `POST /api/gm/confirm-dice`
- `POST /api/gm/manual-event`
- `GET /api/health`

### WebSocket Channels
- `/ws/system-status`
- `/ws/session-events`
- `/ws/vision-events`
- `/ws/media-events`
- `/ws/gm-events`

Hinweis fuer Media-Events:
- `media/play` und `media-events` muessen sowohl lokale Dateien als auch externe Stream-URLs wie `Tabletop Audio` unterstuetzen
- ein Medienevent soll deshalb mindestens `type`, `name`, `provider`, `source_kind` und `url` oder `file_path` tragen

---

## Datenmodell (erste Version)

### Document
- `id`
- `type`
- `name`
- `source_file_path`
- `metadata_json`
- `created_at`

### DocumentChunk
- `id`
- `document_id`
- `chunk_text`
- `page_number`
- `section_title`
- `embedding_ref`
- `metadata_json`

### Campaign
- `id`
- `name`
- `description`
- `ruleset_document_id`
- `adventure_document_id`

### Character
- `id`
- `campaign_id`
- `name`
- `sheet_document_id`
- `stats_json`
- `inventory_json`
- `abilities_json`

### Session
- `id`
- `campaign_id`
- `status`
- `current_scene`
- `current_location`
- `language`
- `default_voice_profile_id`
- `created_at`
- `updated_at`

### PlayerSlot
- `id`
- `session_id`
- `character_id`
- `display_name`
- `joined_at`
- `status`

### PlayerAccessToken
- `id`
- `player_slot_id`
- `token`
- `expires_at` optional
- `revoked_at` optional

### PlayerVisibleState
- `id`
- `player_slot_id`
- `visible_character_json`
- `visible_handouts_json`
- `visible_media_json`
- `updated_at`

### SessionEvent
- `id`
- `session_id`
- `type`
- `payload_json`
- `created_at`

### NPC
- `id`
- `session_id`
- `name`
- `role`
- `species_or_type`
- `state_json`

### CombatState
- `id`
- `session_id`
- `active`
- `round_number`
- `turn_index`

### MediaState
- `id`
- `session_id`
- `active_ambience`
- `active_music`
- `active_video`
- `volume_json`

### VoiceProfile
- `id`
- `name`
- `language`
- `provider`
- `provider_voice_id`
- `style`
- `pitch`
- `speaking_rate`
- `tags_json`
- `preview_text`
- `is_default`
- `created_at`

### VoiceAssignment
- `id`
- `session_id`
- `target_type`
- `target_id`
- `language`
- `voice_profile_id`
- `priority`
- `created_at`

#### Regeln fuer VoiceAssignment
- `target_type` kann z. B. `narrator`, `npc`, `character`, `faction`, `creature_type` oder `global_default` sein
- `target_id` ist optional, wenn die Zuweisung nicht auf eine konkrete Figur geht
- Aufloesung erfolgt von spezifisch nach allgemein:
  1. konkrete Figur
  2. Kreaturentyp oder Fraktion
  3. Session-Default
  4. globaler Fallback

---

## MVP-Umfang

Baue zuerst ein lauffaehiges MVP mit diesen Funktionen:
1. Web UI
2. PDF-Upload
3. PDF-Ingestion
4. Abenteuer + Regelwerk + Charakterbogen als getrennte Quellen
5. Kampagne anlegen
6. Session starten
7. GM-Antwort erzeugen auf Basis von:
   - Benutzerinput
   - Session State
   - RAG-Kontext
8. Strukturierte Rueckgabe
9. Media Events erzeugen
10. einfache Medienwiedergabe
11. Kamera-Testseite
12. d20-Wuerfelerkennung als Event
13. manuelle Bestaetigung/Korrektur von Wuerfeln
14. Session-Log
15. alles lokal per Docker Compose startbar

---

## Nicht-Ziele fuer MVP

Nicht im ersten Schritt bauen:
- vollstaendige Regelauslegung aller Systeme
- perfekte OCR fuer jedes PDF
- komplexe Mehrkamera-Unterstuetzung
- vollautonomes DMing ohne Kontrolle
- ausgereifte Voice-Agent-Pipeline
- Cloud-Zwang
- Benutzer-/Mandantenverwaltung fuer viele Gruppen

---

## UI-Seiten, die konkret gebaut werden sollen

1. `/dashboard`
2. `/setup`
3. `/library`
4. `/campaigns`
5. `/sessions/:id`
6. `/dm/live`
7. `/settings`

---

## Orchestrierung mit MCP / optionalen Tools

Externe Orchestrierung ist optional.

Wenn MCP-Tools oder spaetere Agent-Layer eingebunden werden:
- nur als Tool-/Workflow-Orchestrierung
- nicht als primaeres State-System
- nicht als Ersatz fuer Backend/API
- nicht als Ersatz fuer Datenmodell

Moegliche Tools:
- `get_campaign_context`
- `query_rules`
- `query_adventure`
- `query_character_sheet`
- `register_dice_roll`
- `trigger_media_event`
- `update_session_state`

Wenn keine externe Orchestrierung eingebunden wird, muss das System trotzdem vollstaendig lauffaehig sein.

---

## Qualitaetsanforderungen

- sauber strukturierter Code
- modular
- testbar
- klare Schnittstellen
- keine harte OS-Kopplung
- keine UI-Logik im Backend und keine Business-Logik im Frontend
- Logging fuer alle Services
- Konfigurierbarkeit ueber env + config files

---

## Sicherheit / Stabilitaet

- Dateiupload validieren
- Groessenlimits definieren
- Pfade absichern
- keine willkuerliche Dateiausfuehrung
- Medien nur aus bekannten Asset-Pfaden laden
- LLM-Ausgaben niemals direkt als Systembefehl ausfuehren
- Vision-/Media-/GM-Events validieren

---

## Projektstruktur-Vorschlag

```text
/dungeon_master
  /apps
    /web
    /api
  /services
    /document-ingestion
    /vision
    /media-director
  /packages
    /shared-types
    /shared-config
    /ui-components
  /infrastructure
    /docker
    /compose
  /docs
```

Hinweise:
- `apps/web` ist eine Next.js-App.
- `apps/api` ist das Go-Hauptbackend.
- `services/*` sind klar getrennte Hilfsdienste oder spaetere Auslagerungspunkte.
- Gemeinsame Typen und Event-Definitionen werden zentral gehalten.

---

## Empfohlene Reihenfolge der Umsetzung

### Phase 1
- Repository-Struktur
- Docker Compose
- Go-API-Grundgeruest
- Next.js-Web-UI-Grundgeruest
- Datenbank-Setup
- Healthchecks

### Phase 2
- Dokument-Upload
- PDF-Parsing
- Chunking
- Metadaten
- Vektorindex-Anbindung

### Phase 3
- Kampagnen-/Session-Modelle
- Session State
- GM Engine Stub
- erste strukturierte GM-Antworten

### Phase 4
- DM Live View
- Session Timeline
- manuelle Eingaben
- manuelle Mediensteuerung

### Phase 5
- Vision Service
- Kamera-Test
- d20-Erkennung
- Dice Events
- Bestaetigen/Korrigieren in UI

### Phase 6
- Media Director
- Audio
- Video
- Event-Mapping
- Tabletop-Audio-Streaming anbinden
- Stream-Mapping fuer Ambience/SFX definieren
- Wiedergabe ueber externen Player oder Audio-Backend testen

### Phase 7
- Mehrsprachigkeit
- optionale STT/TTS
- optionale MCP-/Agent-Integration

---

## Akzeptanzkriterien

Ein Build gilt als erfolgreich, wenn folgendes funktioniert:
1. Anwendung startet lokal per Docker Compose.
2. Web UI ist ueber Browser erreichbar.
3. PDFs koennen hochgeladen und als Regeln/Abenteuer/Charakterboegen klassifiziert werden.
4. Eine Kampagne kann erstellt werden.
5. Eine Session kann gestartet werden.
6. Ein Benutzerinput fuehrt zu einer strukturierten GM-Antwort.
7. Die GM-Antwort kann RAG-Kontext aus PDFs verwenden.
8. Audio- und Video-Events koennen aus der UI und per API ausgeloest werden.
9. Kamera-Test funktioniert auf dem Laptop.
10. Ein d20-Wurf kann erkannt oder manuell bestaetigt werden.
11. Session State bleibt ueber Requests hinweg erhalten.
12. Alles laeuft auf Linux, Windows und macOS mindestens im browserbasierten Modus.

---

## Konkrete Bitte an Codex

Bitte implementiere dieses Projekt als MVP in kleinen, klar getrennten Schritten.

Arbeite in dieser Reihenfolge:
1. Grundgeruest Monorepo
2. Docker Compose
3. Backend API in Go
4. Web UI in Next.js
5. Datenmodell
6. Dokument-Upload + Ingestion
7. GM Engine Stub
8. Session State
9. DM Live UI
10. Vision-Service
11. Media Director
12. Integration und Tests

Wichtig:
- Erst lauffaehiger MVP
- Dann Erweiterungen
- Saubere Schnittstellen
- Austauschbare LLM-/Embedding-Provider
- Keine unnoetige fruehe Komplexitaet
