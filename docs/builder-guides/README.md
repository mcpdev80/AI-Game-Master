# Builder Guides

Diese Dateien definieren regelwerkgebundene Character-Builder-Leitfaeden.

Ziel:
- der KI einen sauberen roten Faden fuer die Charaktererstellung geben
- den Ablauf nicht hart in einem einzigen Prompt verstecken
- spaeter pro Regelwerk / Version austauschbar und in der Library verwaltbar machen

Die Idee:
- pro `ruleset_work + ruleset_version` gibt es genau einen oder mehrere Builder-Guides
- der Character-Builder laedt den passenden Guide aus der Library-Konfiguration
- die KI arbeitet dann gegen diesen Leitfaden statt nur gegen einen generischen Prompt

Beispiele:
- `dnd-5e.character-builder.yaml`
- spaeter z. B.:
  - `dnd-3.5.character-builder.yaml`
  - `dsa-4.character-builder.yaml`
  - `how-to-be-a-hero.character-builder.yaml`

## Geplanter spaeterer Anschluss an die Library

Die YAML-Dateien sollen fachlich wie Rulebook-nahe Konfigurationsobjekte behandelt werden:
- zugeordnet zu `ruleset_work`
- zugeordnet zu `ruleset_version`
- optional mit `document_ids`, wenn bestimmte Buecher bevorzugt werden sollen

Damit kann spaeter in der Library pro Regelwerk gepflegt werden:
- Rulebooks
- Adventures
- Assets
- Character Builder Guides

## Grundprinzip fuer den Builder

Ein Guide beschreibt:
- Builder-Zweck und Ton
- feste Reihenfolge der Character-Erstellung
- was automatisch berechnet wird
- was die KI erfragen muss
- welche Dinge nicht halluziniert werden duerfen
- welche Sheet-Felder bei welchem Schritt gefuellt werden

## Wichtiger Architekturpunkt

Der Guide ersetzt keine Regeltexte und kein Retrieval.
Er ist nur:
- Ablaufsteuerung
- Feldzuordnung
- Guardrail

Die eigentlichen Inhalte kommen weiterhin aus:
- ausgewaehlten Rulebooks
- spaeterem Retrieval / Kontext
