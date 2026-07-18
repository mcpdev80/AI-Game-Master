# AI Game Master

A browser-based, 5E-compatible AI game master for shared tabletop sessions. The operator controls the adventure and releases information, while players join from their phones, speak or type actions, roll physical dice, and receive synchronized narration.

> This repository is being prepared for OpenAI Build Week. The imported baseline still uses an OpenAI-compatible local text endpoint; the next implementation step migrates the core turn flow to GPT-5.6 and the Responses API.

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
docker compose up -d --build --wait
bash scripts/mvp_smoke_test.sh
```

Default endpoints:

- Web: `http://localhost:3005`
- API health: `http://localhost:8085/api/health`
- Vision health: `http://localhost:8090/health`
- PostgreSQL: `localhost:5435`

The current baseline expects optional OpenAI-compatible text, STT, and TTS services configured through `.env`. Do not commit `.env`, uploads, private campaign material, commercial rulebooks, personal voice recordings, or credentials.

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
docker compose up -d --build --wait
bash scripts/mvp_smoke_test.sh
```

Kommerzielle Regelbücher, Abenteuer, private Kampagnendaten, persönliche Sprachaufnahmen und Zugangsdaten dürfen nicht in das Repository gelangen. Zulässige SRD-5.1-Inhalte sind gemäß CC BY 4.0 gekennzeichnet.
