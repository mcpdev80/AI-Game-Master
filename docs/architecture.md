# Architecture

AI Game Master is a local/LAN-hosted tabletop session system with a browser frontend, a Go orchestration API, OpenAI-backed language/speech services, and small supporting state services.

## System overview

```text
Operator browser / Player browser / Player phone
                |
                v
        Next.js web application
     (operator UI, player UI, /api proxy)
                |
                v
            Go API backend
   session orchestration, prompts, validation,
   player-safe filtering, persistence, uploads
        |               |               |
        |               |               |
        v               v               v
 OpenAI Responses   OpenAI Audio     Vision service
 GPT-5.6            STT/TTS          dice analysis
        |                               |
        +---------------+---------------+
                        |
                        v
             PostgreSQL + Redis
        durable state    transient/session coordination
```

## Main components

| Component | Role | Notes |
| --- | --- | --- |
| `apps/web` | Next.js application for operator, player join, player portal, and player screen | Also proxies browser calls through same-origin `/api/*` routes. |
| `apps/api` | Core game backend in Go | Owns session state, prompt construction, structured output validation, and API credential access. |
| `apps/vision` | Python vision service | Used for camera-assisted dice recognition; manual dice entry remains available as fallback. |
| `PostgreSQL` | Durable storage | Campaigns, adventures, characters, sessions, documents, assets, visible state. |
| `Redis` | Coordination/cache | Used for runtime coordination and transient operational state. |
| `OpenAI Responses API` | Main turn engine | `gpt-5.6` produces structured encounter turns and scene narration. |
| `OpenAI audio endpoints` | Speech input/output | `gpt-4o-transcribe` for STT and `gpt-4o-mini-tts` for AI narration. |

## Core gameplay flow

### 1. Session setup

1. The operator creates or opens a campaign/adventure/session.
2. A player joins through a random player link.
3. The player creates or selects a character.
4. The API stores session state in PostgreSQL and exposes only player-safe state to player endpoints.

### 2. Player turn

1. The player speaks or types an action in the Player Screen.
2. The web app sends the request through the Go API.
3. The API builds a structured prompt from:
   - current session state
   - selected rules/adventure context
   - player input
   - language and output constraints
4. GPT-5.6 returns a structured encounter-turn result.
5. The API validates the result before applying any state update.
6. The frontend shows narration, pending roll requests, and updated scene state.

### 3. Dice resolution

1. If the model asks for a roll, the API moves the session into dice-capture mode.
2. The player can:
   - use camera-assisted dice recognition, or
   - enter the die result manually
3. The resolved dice payload is sent back to the Go API.
4. GPT-5.6 receives the turn plus the confirmed roll result and returns the resolved outcome.

### 4. Player-safe release

The operator can release only selected handouts/media to players. Player endpoints never receive GM-only notes, hidden DCs, or internal orchestration metadata.

## Trust and safety boundaries

### Browser vs. backend

- OpenAI credentials stay on the Go API.
- Browsers never call OpenAI directly.
- The web layer uses same-origin `/api/*` calls.

### Structured output boundary

Model outputs do not directly mutate state. The API applies an allowlist:

- only approved session/character fields can change
- invalid entity targets are rejected
- malformed roll requests are rejected
- untrusted retrieved content is separated from system rules

### Player-safe data boundary

There are two views of the same session:

- operator/full state
- player-safe filtered state

The player portal intentionally excludes GM-only fields and internal notes.

## Data flow for a GPT-5.6 turn

```text
Player input
   |
   v
Next.js UI
   |
   v
Go API builds prompt
   |
   +--> session state
   +--> rules/adventure excerpts
   +--> language + schema constraints
   |
   v
OpenAI Responses API (gpt-5.6)
   |
   v
Structured encounter-turn payload
   |
   v
Go validation layer
   |
   +--> accept safe roll request / narration / state updates
   |
   v
Persist to PostgreSQL + return filtered UI state
```

## Speech and media flow

### Speech-to-text

```text
Browser microphone -> Go API -> OpenAI transcription -> text transcript -> UI
```

### Text-to-speech

```text
Narration text -> Go API -> OpenAI TTS -> audio stream/file -> browser playback
```

### Dice vision

```text
Browser camera frame -> Go API / vision path -> vision service -> dice result candidate
```

If vision is unavailable or permission is denied, the player can still enter the result manually.

## Local deployment shape

Current judged deployment model:

- Docker Compose stack on one operator machine
- optional self-signed HTTPS reverse proxy for LAN device testing
- desktop operator browser plus second browser or phone for the player side

The internal HTTPS setup is documented in [LOCAL_HTTPS.md](LOCAL_HTTPS.md).

## Why this architecture fits the demo

- It keeps credentials server-side.
- It gives a clear boundary between model output and game-state mutation.
- It supports multimodal play without requiring every feature to succeed.
- It preserves a player-safe view separate from operator control.
- It stays reproducible for judging through deterministic isolated tests.
