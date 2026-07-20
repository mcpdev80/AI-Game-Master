# AI Game Master

A browser-based, 5E-compatible AI game master for shared tabletop sessions. The operator controls the adventure and releases information, while players join from their phones, speak or type actions, roll physical dice, and receive synchronized narration.

> The Build Week implementation uses GPT-5.6 through the OpenAI Responses API, `gpt-4o-transcribe` for player speech, and `gpt-4o-mini-tts` for narrated output. Encounter turns and camera-assisted dice results use strict, versioned Structured Outputs; local providers remain available as optional fallbacks.

## Why this project exists

Running a tabletop session with an AI game master usually breaks down at the same places: too much hidden operator context leaks to players, model output is not constrained enough to trust, and multimodal features fail hard when camera or microphone access is missing.

This project is an attempt to make that loop actually usable:

- one operator controls the session
- players join from their own device
- GPT-5.6 handles live encounter turns
- structured validation sits between the model and game state
- camera, speech, and text all have practical fallbacks

## Build Week scope

This repository documents the Build Week conversion of an existing local project into a judgeable OpenAI-based demo. The Build Week work focused on:

- moving the main GM turn flow to GPT-5.6 via the Responses API
- switching speech input/output to OpenAI speech endpoints
- removing non-redistributable content and keeping the repo license-safe
- making English the default public-demo language while retaining German
- adding deterministic tests for the full gameplay path and critical failure cases
- preparing local HTTPS/LAN hosting for real device testing

## What is new in the Build Week version

- GPT-5.6 is the default turn engine for structured encounter resolution
- `gpt-4o-transcribe` powers player speech input
- `gpt-4o-mini-tts` powers narrated output
- the bundled demo adventure is a redistributable, licensed package
- the player view is explicitly filtered to remain player-safe
- critical negative paths are tested, not only the happy path

## Features

- Campaign, adventure, character, and session management
- Player join links and player-safe views
- Structured narration, roll requests, state updates, and scene events
- Document ingestion and retrieval
- Speech-to-text, text-to-speech, and camera-assisted dice recognition
- PostgreSQL session state, Redis coordination, and a Next.js operator/player interface
- English/German product direction; English will be the public-demo default

## Architecture at a glance

- Next.js frontend for operator, player join, player portal, and player screen
- Go backend for orchestration, prompting, validation, persistence, and secret handling
- OpenAI Responses API for structured game-master turns
- OpenAI speech APIs for STT/TTS
- Python vision service for dice recognition
- PostgreSQL for durable state and Redis for transient coordination

For the full system and data-flow view, see [docs/architecture.md](docs/architecture.md).

## Repository structure

- `apps/web` — Next.js frontend
- `apps/api` — Go API and game-state orchestration
- `apps/vision` — Python dice-recognition service
- `apps/speech-stt` — optional local STT adapter
- `apps/speech-tts` — optional local TTS adapter; requires user-supplied, properly licensed reference audio
- `docs` — architecture, licensing, and implementation notes
- `scripts` — local validation utilities

Submission-facing docs:

- [SECURITY.md](SECURITY.md)
- [docs/architecture.md](docs/architecture.md)
- [docs/judge-testing.md](docs/judge-testing.md)
- [docs/demo-script.md](docs/demo-script.md)
- [docs/evals.md](docs/evals.md)

## Local setup

Requirements: Docker with Compose, Node.js/npm, `curl`, `jq`, and Google Chrome (or
`PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH`) for the complete browser test.

```bash
cp .env.example .env
# Add OPENAI_API_KEY to .env
docker compose up -d --build --wait
bash scripts/mvp_smoke_test.sh
```

For internal HTTPS on your LAN with a self-signed certificate, use the local proxy profile described in [`docs/LOCAL_HTTPS.md`](docs/LOCAL_HTTPS.md). That path is intended for local device testing only:

```bash
./scripts/generate_local_https_cert.sh dungeon-master.local 192.168.178.50 30
docker compose --profile https up -d --build --wait
```

The deterministic Golden Path needs no real OpenAI key. It starts an isolated
Compose stack with a local OpenAI-compatible mock, creates a character through
the AI builder, plays a complete roll-request/resolution turn, verifies the
player-safe portal, and drives the corresponding browser flow with Playwright:

```bash
npm ci
npm run test:golden-path
```

The test stack uses separate ports and is removed automatically when the test
finishes. Set `KEEP_TEST_STACK=true` only when debugging it locally.

Default endpoints:

- Web: `http://localhost:3005`
- Local HTTPS proxy: `https://dungeon-master.local:3443` after enabling the `https` profile
- API health: `http://localhost:8085/api/health`
- Vision health: `http://localhost:8090/health`
- PostgreSQL: `localhost:5435`

The default text provider is OpenAI with `OPENAI_MODEL=gpt-5.6`, `OPENAI_STORE=false`, and `POST /v1/responses`. Speech defaults to `gpt-4o-transcribe` through `POST /v1/audio/transcriptions` and `gpt-4o-mini-tts` through `POST /v1/audio/speech`, using the `cedar` narrator voice. The key is read only by the Go API and is never returned by system endpoints. Set the corresponding provider variable to `local` to use an optional local fallback.

The UI identifies generated narration as an AI-generated voice, as required by OpenAI's usage policy.

Do not commit `.env`, uploads, private campaign material, commercial rulebooks, personal voice recordings, or credentials.

## Testing

Fast local validation:

```bash
bash scripts/mvp_smoke_test.sh
```

Deterministic isolated golden path:

```bash
npm run test:golden-path
```

Current highlighted coverage includes:

- backend validation of structured state updates and roll requests
- typed handling for OpenAI rate limits, invalid JSON, and invalid schemas
- full character-builder-to-session-to-roll-resolution path
- browser test for camera-denied manual roll fallback
- browser test for visible rate-limit failure in the player screen

For the current evaluation status, see [docs/evals.md](docs/evals.md).

## Human decisions and Codex contribution

Human decisions in this Build Week version included:

- what content could legally remain in the repository
- which product path should be demo-first instead of feature-complete
- which device flows and fallbacks matter most for judging
- which claims should be documented honestly versus postponed

Codex was used as an implementation collaborator for:

- code changes and refactors
- test creation and debugging
- validation hardening
- local HTTPS/demo setup
- submission documentation and cleanup planning

## Challenges and tradeoffs

- Keeping the repository redistributable meant removing or replacing material that could not be shipped publicly.
- A judgeable AI demo needs stronger validation than a private prototype, so structured output checks and player-safe filtering became mandatory.
- Real browser/device flows are less reliable than API-only demos, which is why manual fallbacks and deterministic tests were prioritized.
- The current hosting model is intentionally local/internal rather than pretending to be production-ready public infrastructure.

## Languages

English is the target default for the public demo and judging flow. German remains supported. The migration plan is documented in [`docs/I18N_PLAN.md`](docs/I18N_PLAN.md).

## Licensing

Original source code is licensed under the MIT License. Small embedded rule references adapted from SRD 5.1 remain under CC BY 4.0. See [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md) and [`docs/CONTENT_POLICY.md`](docs/CONTENT_POLICY.md).

This project is an independent, 5E-compatible tool. It is not affiliated with or endorsed by Wizards of the Coast.

## Current limits

- The judged deployment model is local/LAN hosting, not a public SaaS deployment.
- Internal HTTPS currently uses a self-signed certificate for device testing.
- Some automated tests use a deterministic local OpenAI-compatible mock instead of the live API.
- Real device validation over HTTPS is still a required final ship check.

---

## Deutsch

AI Game Master ist eine browserbasierte, 5E-kompatible Spielleiter-Anwendung für gemeinsame Pen-and-Paper-Runden. Die öffentliche Demo wird standardmäßig Englisch verwenden; Deutsch bleibt als auswählbare Sprache erhalten.

Lokaler Start:

```bash
cp .env.example .env
# OPENAI_API_KEY in .env eintragen
docker compose up -d --build --wait
bash scripts/mvp_smoke_test.sh
```

Lokales HTTPS im internen Netz mit Self-Signed-Zertifikat ist in [`docs/LOCAL_HTTPS.md`](docs/LOCAL_HTTPS.md) dokumentiert:

```bash
./scripts/generate_local_https_cert.sh dungeon-master.local 192.168.178.50 30
docker compose --profile https up -d --build --wait
```

Vollständiger, deterministischer Test inklusive Character Builder, Session,
Würfelauflösung, Player Portal und Playwright-Browserablauf:

```bash
npm ci
npm run test:golden-path
```

Kommerzielle Regelbücher, Abenteuer, private Kampagnendaten, persönliche Sprachaufnahmen und Zugangsdaten dürfen nicht in das Repository gelangen. Zulässige SRD-5.1-Inhalte sind gemäß CC BY 4.0 gekennzeichnet.
