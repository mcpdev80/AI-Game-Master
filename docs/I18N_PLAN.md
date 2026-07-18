# English/German product plan

## Goal

The same deployment supports English and German. English is the default for judges; German is available from a visible language switcher. Game state and identifiers remain language-neutral.

Dieselbe Installation unterstützt Englisch und Deutsch. Für die Jury ist Englisch voreingestellt; Deutsch ist über einen sichtbaren Sprachschalter verfügbar. Spielzustand und Bezeichner bleiben sprachneutral.

## Implementation sequence

1. Add `en` and `de` locale dictionaries to the web application and a typed translation helper.
2. Store the selected UI locale in a cookie and allow session language to override narration language.
3. Replace hard-coded user-facing strings screen by screen, starting with the demo Golden Path.
4. Give API errors stable language-neutral codes; translate their display in the web app.
5. Keep separate English and German prompt assets where wording materially affects behavior.
6. Provide original demo campaign content, sample characters, and handouts in both languages.
7. Test both locales in Playwright, including language persistence and player/operator consistency.

## Fallback rules

- Unsupported or missing locale: English.
- Missing German key: English, with a development warning.
- Player input may use either supported language; generated narration follows the session language unless the operator changes it.
- STT and TTS receive the active session language explicitly when their APIs support it.
