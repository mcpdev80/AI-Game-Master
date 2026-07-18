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
