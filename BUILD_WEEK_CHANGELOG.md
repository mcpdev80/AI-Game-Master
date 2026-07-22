# Build Week changelog

## Release 0.2.0

- Added DM-controlled companion support so solo players or small groups can bring cloned companion characters into a session without assigning them to separate human player slots.
- Extended session models, API endpoints, storage, and GM prompt context so companions participate as first-class session actors.
- Improved player/session surfaces to expose richer companion-aware summaries and a stronger local-table play flow.
- Hardened builder and session browser coverage, including more stable narrative opening and player-screen flows.
- Finalized the Build Week merge of the companion and player-surface work onto `main` for the public submission release.

## Release 0.1.2

- Added a stable SRD 5.1 reference baseline to the repository: original local PDF snapshot, extracted text snapshot, current landing-page snapshot, and a refresh helper script.
- Added deterministic SRD-derived catalogs and rule helpers for builder/level-up behavior so spell, class, monster, and localization flows are less dependent on ad-hoc prompt reasoning.
- Fixed bilingual builder consistency for spell advice and spell-sheet rows; English and German now have dedicated browser regression coverage.
- Improved builder stability for multiclass normalization and late review corrections before finalizing a draft.

## Baseline

- Commit: `dffac74`
- Tag: `pre-build-week-baseline`
- Imported project sanitized before Build Week feature implementation.
- Commercial rulebooks, third-party adventures, private voice recordings, binaries, internal deployment data, and non-SRD embedded options were removed before the baseline commit.
- SRD 5.1-derived material is identified in `THIRD_PARTY_NOTICES.md`.

## Build Week work

### English/German product foundation

- English selected as the public-demo default.
- German retained as a first-class selectable locale.
- Shared locale provider, persistent language selection, and a visible shell language switcher added.
- Golden-Path screen translation remains the next i18n step.

### GPT-5.6 Responses integration

- OpenAI is now the default LLM provider; the existing local OpenAI-compatible route remains an optional fallback.
- All OpenAI text calls use `POST /v1/responses` with `gpt-5.6`, configurable reasoning effort, and `store: false` by default.
- Encounter turns use the strict versioned Structured Output schema `encounter_turn_v1`.
- OpenAI dice-image analysis uses Responses vision input and the strict schema `dice_vision_v1`.
- Refusals, incomplete responses, timeouts, rate limits, invalid schemas, API failures, and malformed response bodies are surfaced as typed errors.
- Session identifiers are hashed before being sent as privacy-preserving `safety_identifier` values.
- OpenAI requests never receive the local provider's `chat_template_kwargs`.
- Control Center model discovery and connection tests now run through the API backend, keeping provider credentials out of the browser.
- Deterministic HTTP mock tests cover request shape, strict schema selection, privacy identifiers, refusals, and incomplete responses.

### Golden-path smoke test

- Updated the smoke test to the current session contract and English public-demo defaults.
- The test now creates a character and player link, joins and marks the player ready, then starts the session in the same order as the product UI.
- API failures include the HTTP status and response body for fast diagnosis.
- The live AI assertion requires an OpenAI-generated response whose resolved model starts with `gpt-5.6`.
- Final portal verification checks that the player remains ready and the session is live.
- Encounter gateway budgets now match the existing 12k campaign-play budget instead of rejecting the full game-master prompt before it reaches GPT-5.6.
- The smoke test waits for the asynchronous GPT session opening before sending the first player turn and releases only persisted UUID-backed handouts or media.
- Roll-request narration now respects the selected English/German locale and avoids repeating model-provided dice instructions.
- Live verification passed end-to-end on July 18, 2026: OpenAI resolved the configured `gpt-5.6` alias to `gpt-5.6-sol` for both the generated session opening and the first structured player turn.
- Fixed the production Next.js rewrite fallback from the removed `dungeon-master-api` hostname to the Compose service `api`; embedded rules and session lifecycle actions now reach the backend through the browser proxy.
- Added an isolated deterministic Golden Path stack with an OpenAI-compatible mock; it requires no API key and leaves the normal development database untouched.
- Added a complete API flow covering the embedded SRD reference, conversational character creation, finished character sheet, player join/readiness, session opening, roll request, confirmed physical-die value, state update, map release, and player-safe portal.
- Added Playwright coverage for the same judge-facing browser path from one-click demo seeding through character creation and dice resolution.
- Fixed embedded Character Builder document IDs being sent to PostgreSQL as UUIDs, an undefined DM-notes render crash in live sessions, and leakage of private DM notes, hidden DCs, and internal LLM session IDs through the Player Portal.
- Fixed LAN clients crashing after successful actions when the insecure HTTP context does not expose `crypto.randomUUID`; client IDs now have a deterministic compatibility fallback, and STT reports the HTTPS requirement without crashing.
- Corrected the SRD 5.1 background step: Acolyte remains the only bundled named sample, while the builder now explains and accepts the official custom-background rule instead of forcing Acolyte or copying non-SRD Player's Handbook options.

### OpenAI speech input and output

- OpenAI is now the default speech provider: `gpt-4o-transcribe` for STT and `gpt-4o-mini-tts` for TTS.
- Existing local Parakeet/Piper-compatible endpoints remain available through `STT_PROVIDER=local` and `TTS_PROVIDER=local`.
- OpenAI credentials remain server-side; browser audio continues to use the existing application endpoints.
- Transcriptions include the active `en`/`de` language and a compact tabletop-domain prompt for names, fantasy terms, punctuation, and dice notation.
- Existing voice profiles map to OpenAI built-in voices, with `cedar` as the default narrator voice.
- The Player Screen and Character Builder disclose that narration voices are AI-generated.
- Deterministic mock tests verify authentication, multipart transcription fields, OpenAI TTS instructions, WAV responses, and voice mapping.
- A live roundtrip generated a WAV with `gpt-4o-mini-tts` and transcribed it exactly with `gpt-4o-transcribe` on July 18, 2026.

### Licensed bilingual demo adventure and visual direction

- Added **The Fungal Caverns** by Logen Nein as the bundled system-neutral demo under CC BY 3.0 US, with source, creator, license, and modification notices in both the package and repository notices.
- Preserved the original GM PDF and unkeyed player map, and added an English AI-GM adaptation plus a German translation.
- Added an idempotent one-click seed that creates the campaign, adventure, indexed bilingual documents, license record, player-safe map, example character, and live demo session.
- Added runtime scene-asset resolution: structured scene cues are matched against adventure assets, with an adventure map fallback.
- Scene images now render directly on the Player Screen instead of being restricted to combat overlays.
- Live verification passed: duplicate seeding reused the same records, the served map matched the source SHA-256, and a real GPT-5.6 roll-request/resolution turn restored the correct map to scene mode.

### Runtime hardening and validation

- Added server-side validation for structured model state updates before they are applied.
- Restricted accepted state mutations to an explicit allowlist for session and character fields.
- Rejected unknown entity IDs, empty entity IDs, campaign entity mutation, and mismatched delta/value payloads.
- Tightened roll-request validation for type, dice notation, follow-up chains, labels, and DC ranges.
- Wrapped retrieved adventure/rules content as untrusted context so it cannot silently override higher-level prompt rules.
- Added regression tests for the validation layer and for prompt-injection-style untrusted content handling.

### Combat state, encounter flow, and session progression

- Added persistent combat state to the session model with initiative order, active turn index, round counter, and a structured combat log.
- Added session-side reward and progression tracking with `last_reward_summary`, `level_up_queue`, and `awaiting_level_up_rest`.
- Added automatic combat start detection for hostile attack setups so encounter state no longer depends only on explicit initiative wording.
- Added authoritative combat-state narration when a fight starts, preventing the model from contradicting actual initiative order.
- Added enemy-turn resolution against tracked Armor Class with structured hit/miss/damage combat-log entries.
- Added session-side combat-resolution and rest-transition detection to keep fight cleanup and level-up readiness in sync.
- Added XP-threshold handling for 5E-compatible level-up readiness and queue generation from active session characters.
- Added regression tests for character-level parsing, XP-based level-up eligibility, combat auto-start, and encounter-state initialization.

### Encounter balancing

- Replaced fixed spider-only combat assumptions with a generic encounter-definition system.
- Added encounter balancing by active party size, average level, and highest level.
- Added graduated combat variants for spiders, goblins, and wolves instead of always spawning the heaviest named enemy.
- Downgraded solo and low-level encounters to safer variants, while stronger parties can receive higher-tier or plural enemy groups.
- Added tests for low-level spider and goblin balancing as well as higher-level wolf encounter selection.

### Live combat UI and player-safe board updates

- Added a live combat tracker to the session screen with initiative order, active turn indication, and the full combat log for the current or last fight.
- Added polling-based live session refresh for GM session detail screens so combat state, rolls, and events update without a manual reload.
- Added a persistent initiative overlay to the player-facing visual board so turn order remains visible during combat scenes.
- Added color-coded combat condition indicators for players and enemies on both the GM session screen and the player board.
- Removed raw HP details from initiative order displays; only player-safe color/status indicators remain visible.
- Kept the last combat log visible after combat ends so the full encounter history can still be reviewed.

### GM-flow and localization fixes

- Ensured non-roll narration ends with a direct handoff question so the players are always prompted for the next action.
- Localized enemy combat narration based on session language to avoid German/English mixed combat output.
- Improved embedded-document context retrieval so non-database adventure/rules content participates in GM context assembly.
- Added browser-side combat UI helpers and updated API typings for the new combat/session state fields.

### Frontend resilience and negative-path coverage

- Added a visible player-facing error path for `429 Too Many Requests` responses from `/api/gm/respond`.
- Added Playwright coverage for the visible rate-limit error in the Player Screen.
- Added Playwright coverage for camera-denied manual roll fallback in the Player Screen.
- Hardened the dice fallback browser test to tolerate the live popup timing used by the current UI.
- Kept manual fallbacks reachable when browser camera access is unavailable.

### Internal HTTPS and LAN demo hosting

- Added an internal HTTPS reverse-proxy profile for local/LAN device testing.
- Added self-signed certificate generation for a chosen hostname and LAN IP.
- Verified health checks and app reachability through `https://dungeon-master.local:3443` in the local environment.
- Documented the client trust and hosts-file steps required for desktop/phone testing.

### Submission documentation

- Added `SECURITY.md` to document supported demo boundaries, residual risk, and disclosure expectations.
- Added `docs/architecture.md` for the system and data-flow overview.
- Added `docs/judge-testing.md` for the five-minute judge path and deterministic proof path.
- Added `docs/evals.md` to record the current automated and manual evaluation status.
- Added `docs/demo-script.md` with the current sub-3-minute English video script.
- Expanded `README.md` with Build Week scope, technical architecture, testing, challenges, and current limits.
