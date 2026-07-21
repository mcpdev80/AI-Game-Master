# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog and this project uses Semantic Versioning.

## [0.1.2] - 2026-07-21

### Added
- Embedded SRD 5.1 reference snapshots, generated rule catalogs, and a refresh script for stable source regeneration
- Deterministic SRD-backed character-builder and level-up helpers for spells, classes, monsters, and localized rule references
- Dedicated Playwright coverage for the bilingual spell-builder UI in both English and German

### Changed
- Expanded the character builder to use deterministic bilingual guidance for spell selection, level-up planning, and staged multiclass normalization
- Localized spell rows, short spell descriptions, and measurement displays consistently across builder chat and character-sheet views
- Updated third-party notices and Build Week planning notes for the embedded SRD reference baseline

### Fixed
- Mixed-language leaks in the English spell-builder flow and spell-attack table rendering
- Late builder review corrections for hit points, hit dice, attacks, and derived values
- Character-sheet spell descriptions and related localized UI labels for German and English

## [0.1.1] - 2026-07-21

### Added
- Persistent combat state with initiative order, structured combat log, and session-level reward tracking
- Encounter balancing for spider, goblin, and wolf fights based on active party size and character level
- Player-facing initiative overlay and combat condition indicators for the visual board
- Session-side combat tracker with full fight history and live combat state polling

### Changed
- Updated GM turn handling to keep combat flow, enemy turns, and initiative narration aligned with authoritative session state
- Improved session defaults and API typings for combat, reward, and level-up progression data
- Refined combat UI to hide raw HP details from initiative order while keeping color-based condition feedback visible

### Fixed
- Fallback and embedded document context retrieval for non-database rule/adventure content
- Interactive narration endings so non-roll GM responses consistently hand the turn back to the players
- Localized enemy combat narration to avoid mixed-language output during active encounters

## [0.1.0] - 2026-07-20

### Added
- Build Week release baseline for the public repository
- GPT-5.6 default GM turn flow via the OpenAI Responses API
- OpenAI speech support with `gpt-4o-transcribe` and `gpt-4o-mini-tts`
- Local HTTPS support with provided certificates or self-signed fallback
- Deterministic golden-path coverage for the core demo flow
- Licensed demo adventure package and English-first public demo documentation

### Changed
- Cleaned the repository for redistributable Build Week submission use
- Standardized the public-facing docs and submission materials in English
- Added visible app build metadata for local demo verification
