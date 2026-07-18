# Build Week changelog

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

### OpenAI speech input and output

- OpenAI is now the default speech provider: `gpt-4o-transcribe` for STT and `gpt-4o-mini-tts` for TTS.
- Existing local Parakeet/Piper-compatible endpoints remain available through `STT_PROVIDER=local` and `TTS_PROVIDER=local`.
- OpenAI credentials remain server-side; browser audio continues to use the existing application endpoints.
- Transcriptions include the active `en`/`de` language and a compact tabletop-domain prompt for names, fantasy terms, punctuation, and dice notation.
- Existing voice profiles map to OpenAI built-in voices, with `cedar` as the default narrator voice.
- The Player Screen and Character Builder disclose that narration voices are AI-generated.
- Deterministic mock tests verify authentication, multipart transcription fields, OpenAI TTS instructions, WAV responses, and voice mapping.
- A live roundtrip generated a WAV with `gpt-4o-mini-tts` and transcribed it exactly with `gpt-4o-transcribe` on July 18, 2026.
