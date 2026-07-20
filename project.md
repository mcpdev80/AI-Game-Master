# Project Handoff for Codex

## Project name

AI Game Master

## Current goal

Maintain and ship a browser-based tabletop RPG assistant centered on an AI game master flow that is usable for local sessions, live demos, and Build Week judging.

The project should:

1. ingest rulebooks, adventures, and character material where licensing allows it
2. support AI narration, player input, and structured encounter resolution
3. support multilingual play, currently German and English
4. support camera-assisted physical dice handling with manual fallback
5. support operator, player-screen, and player-portal views
6. run locally through Docker Compose
7. remain repository-safe for public judging

## Current stack

This repository is implemented with the stack that actually exists here:

- Frontend: Next.js
- Main backend/API: Go
- Database: PostgreSQL
- Coordination/cache: Redis
- Vision service: Python
- Deployment: Docker Compose
- LLM/STT/TTS integration: provider-based, with OpenAI as the main judged path

Important:

- Go is the primary backend for API, orchestration, persistence, structured validation, and session state.
- Specialized tasks such as vision and optional local speech adapters are separate services/modules.
- The repository is Go-first, not monolithic.

## Product model

The product is split into four practical layers:

### 1. Core backend

Responsible for:

- API endpoints
- session logic
- LLM orchestration
- structured validation
- character builder state
- persistence
- player-safe filtering
- media and vision integration

### 2. Operator UI

Responsible for:

- setup
- demo launch
- campaign/session management
- character management
- session start/pause/stop
- assigning players and reviewing live state

### 3. Player-facing surfaces

Responsible for:

- player join flow
- player portal per player slot
- player-safe character and handout access
- visual board / shared session display

### 4. Device integration

Responsible for:

- microphone access
- speaker output
- optional camera capture for dice
- browser-based local interaction

## Role model

The product assumes three distinct roles:

### 1. Admin / Operator

A human prepares and supervises the session:

- choose campaign, ruleset, and adventure
- create or assign characters
- create player links
- test devices
- start, pause, and stop sessions
- intervene if needed

The operator is not the primary storyteller.

### 2. AI Dungeon Master

The AI is the active GM layer:

- narration
- encounter resolution
- state updates
- rule-aware prompting
- roll requests
- player-safe scene progression

### 3. Players

Human participants who:

- join through local links
- view their characters
- receive released handouts and images
- speak or type actions
- confirm physical dice results when required

## Player links and portal

The system supports link-based session access per player slot.

Example:

- `https://example.test/session-join/abc123`

Goals:

- no app installation required
- player-safe data only
- character access per player
- operator-managed links
- AI/session state remains the source of truth

Player portal content may include:

- character view
- summary values
- released handouts
- released images and maps
- later optional player actions and dice input

Important:

- player portal and shared player screen are related, but not identical
- player portal is personal
- player screen is the shared display surface
- neither may expose DM-only notes

## Technical guardrails

- Primary UI is browser-based.
- Main API is REST-based.
- The repository must stay public-demo safe.
- Secrets must stay in environment variables and never enter tracked files.
- Commercial rulebooks, private PDFs, personal voice material, and local uploads must not be committed.
- Structured outputs and server-side validation are required between model output and game state.
- OpenAI is the main Build Week path; local alternatives may remain optional.

## Target environments

Must remain usable on:

- Linux
- Windows
- macOS

Important:

- core functionality must stay browser-first
- no OS-specific desktop UI dependency is required for the MVP
- optional wrappers may be added later, but are not part of the current core

## Main functional areas

### A. Web UI

Expected product areas:

- Dashboard
- Campaigns
- Characters
- Sessions
- Library
- Control Center
- Player join / portal / screen

### B. Library and content

The repository distinguishes between:

- rulebooks
- adventures
- assets
- character-builder guides

Campaigns are not library objects.

### C. Sessions

Sessions should support:

- selected campaign
- selected adventure
- selected ruleset
- target player count
- player link creation
- start / pause / stop
- live AI narration and resolution

### D. Character builder

The builder should:

- work step-by-step
- remain ruleset-aware
- persist messages and draft state
- support German and English
- separate mechanical background from narrative backstory
- fill derived combat and spell data into the draft

## Current Build Week position

The current public-facing Build Week story is:

- GPT-5.6 drives structured encounter turns
- OpenAI speech APIs power STT and TTS
- the demo adventure is redistributable and licensed
- the product is local/LAN-first, not public SaaS
- the judged path is a real working session loop, not just a mock UI

## What is intentionally not promised

Do not overstate the project.

The current project does not claim:

- full support for every tabletop system
- a production multi-tenant public platform
- complete replacement of human GMs in all scenarios
- enterprise permissions or billing
- unrestricted use of commercial RPG content

## Source-of-truth documents

For current public submission and shipping context, prefer:

- `README.md`
- `BUILD_WEEK_PLAN.md`
- `BUILD_WEEK_CHANGELOG.md`
- `docs/architecture.md`
- `docs/CONTENT_POLICY.md`
- `THIRD_PARTY_NOTICES.md`

If older notes in this repository conflict with shipped code or current submission docs, the shipped code and the documents above take precedence.
