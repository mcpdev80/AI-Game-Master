# AI Game Master

A browser-based, 5E-compatible AI game master for shared tabletop sessions. The operator controls the adventure and releases information, while players join from their phones, speak or type actions, roll physical dice, and receive synchronized narration.

> The Build Week implementation uses GPT-5.6 through the OpenAI Responses API, `gpt-4o-transcribe` for player speech, and `gpt-4o-mini-tts` for narrated output. Encounter turns and camera-assisted dice results use strict, versioned Structured Outputs; local providers remain available as optional fallbacks.

## Features

- Campaign, adventure, character, and session management
- Player join links and player-safe views
- Structured narration, roll requests, state updates, and scene events
- Document ingestion and retrieval
- Speech-to-text, text-to-speech, and camera-assisted dice recognition
- PostgreSQL session state, Redis coordination, and a Next.js operator/player interface
- English/German product direction; English will be the public-demo default

## Repository structure

- `apps/web` — Next.js frontend
- `apps/api` — Go API and game-state orchestration
- `apps/vision` — Python dice-recognition service
- `apps/speech-stt` — optional local STT adapter
- `apps/speech-tts` — optional local TTS adapter; requires user-supplied, properly licensed reference audio
- `docs` — architecture, licensing, and implementation notes
- `scripts` — local validation utilities

## Local setup

Requirements: Docker with Compose, `curl`, and `jq` for the smoke test.

```bash
cp .env.example .env
# Add OPENAI_API_KEY to .env
docker compose up -d --build --wait
bash scripts/mvp_smoke_test.sh
```

Default endpoints:

- Web: `http://localhost:3005`
- API health: `http://localhost:8085/api/health`
- Vision health: `http://localhost:8090/health`
- PostgreSQL: `localhost:5435`

The default text provider is OpenAI with `OPENAI_MODEL=gpt-5.6`, `OPENAI_STORE=false`, and `POST /v1/responses`. Speech defaults to `gpt-4o-transcribe` through `POST /v1/audio/transcriptions` and `gpt-4o-mini-tts` through `POST /v1/audio/speech`, using the `cedar` narrator voice. The key is read only by the Go API and is never returned by system endpoints. Set the corresponding provider variable to `local` to use an optional local fallback.

The UI identifies generated narration as an AI-generated voice, as required by OpenAI's usage policy.

Do not commit `.env`, uploads, private campaign material, commercial rulebooks, personal voice recordings, or credentials.

## Languages

English is the target default for the public demo and judging flow. German remains supported. The migration plan is documented in [`docs/I18N_PLAN.md`](docs/I18N_PLAN.md).

## Licensing

Original source code is licensed under the MIT License. Small embedded rule references adapted from SRD 5.1 remain under CC BY 4.0. See [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md) and [`docs/CONTENT_POLICY.md`](docs/CONTENT_POLICY.md).

This project is an independent, 5E-compatible tool. It is not affiliated with or endorsed by Wizards of the Coast.

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

Kommerzielle Regelbücher, Abenteuer, private Kampagnendaten, persönliche Sprachaufnahmen und Zugangsdaten dürfen nicht in das Repository gelangen. Zulässige SRD-5.1-Inhalte sind gemäß CC BY 4.0 gekennzeichnet.
