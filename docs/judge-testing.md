# Judge Testing Guide

This guide is for a fast local/demo evaluation of the current Build Week version of AI Game Master.

This repository was prepared with substantial help from Codex, and the judged Build Week implementation path primarily uses GPT-5.6 plus OpenAI speech models.

## What the judge should see

In under five minutes, the judge should be able to verify:

- GPT-5.6-powered game-master turns
- AI-assisted character creation
- player-safe session sharing
- speech/camera-ready player experience with manual fallbacks
- deterministic automated test coverage for the critical gameplay path
- clear evidence of where Codex and GPT-5.6 were used

## Environment

Current supported demo setup:

- operator machine on the local network
- optional player phone on the same network
- internal HTTPS with either a provided certificate pair or a self-signed certificate

For Build Week judging, there are two valid verification modes in this repository:

- live OpenAI-backed local demo with an `OPENAI_API_KEY`
- deterministic no-key golden path using the bundled local OpenAI-compatible mock

Reference local URL:

```text
https://dungeon-master.local:3443
```

If `dungeon-master.local` is not resolvable on the judge device, add a hosts entry pointing to the operator machine's LAN IP as described in [LOCAL_HTTPS.md](LOCAL_HTTPS.md).

## Quick start for the operator

From the repository root:

```bash
cp .env.example .env
# add OPENAI_API_KEY to .env
docker compose --profile https up -d --build --wait
```

Health checks:

```bash
curl -k https://localhost:3443/api/health
curl -k -I https://localhost:3443
```

Expected result:

- `/api/health` returns `{"status":"ok",...}`
- the site responds over HTTPS and redirects into the app

## Five-minute judge flow

### Path A: fastest live product walkthrough

1. Open `https://dungeon-master.local:3443`.
2. Go to the character flow and create a new character.
3. Start or open the seeded Fungal Caverns demo.
4. Join the player side from a second browser or phone.
5. Send one player action.
6. Trigger one dice request and resolve it.
7. Confirm that narration updates and the player sees only player-safe state.

What to point out during the walkthrough:

- the GM turn is produced by GPT-5.6 through the OpenAI Responses API
- speech transcription uses `gpt-4o-transcribe`
- voice playback uses `gpt-4o-mini-tts`
- dice capture has a manual fallback if camera permission is unavailable
- visible player state is deliberately narrower than operator state

### Path B: deterministic automated proof

Run the full golden path:

```bash
npm ci
npm run test:golden-path
```

What this validates:

- isolated Compose stack startup
- seeded licensed demo content
- AI character builder start, message turn, patch apply, and finish
- session creation and player join flow
- deterministic roll request and roll resolution
- player-safe portal output
- browser flow via Playwright

This path is intentionally the fastest repo-level proof for judges because it does not require the judge to supply their own OpenAI API key.

## External import compatibility check

If you want one additional manual proof that the importer can handle a real third-party adventure package shape, use `The Abbey` by Internal Rock as an external test source:

- source page: [The Abbey](https://internalrock.itch.io/the-abbey-knave)
- published package shape includes an adventure PDF, tokens, portraits, paperminis, and battlemaps
- this repository does not redistribute those files

What was verified locally on July 21, 2026:

- the live `POST /api/adventures/create-package` endpoint accepted an Abbey-shaped package
- the importer correctly classified:
  - battlemaps
  - tokens
  - portraits
  - printables / paperminis
  - handout PDFs
- the verified runtime stores the imported adventure once in its original language and answers in the active session language at play time, instead of creating duplicated full-language copies
- the verified package limits for this flow are 500 MB upload size and 1 GB extracted size

How to use it during a local check:

1. Download the original files from the source page.
2. Keep the adventure PDF as the required `pdf` upload.
3. Put the extra files into a ZIP using descriptive names that contain keywords such as `battlemap`, `token`, `portrait`, `paperminis`, `map`, `letter`, or `brief`.
4. Import through the Library adventure-creation flow.
5. Confirm the created adventure shows the expected document and asset counts.

## Manual fallback checks

Useful quick checks during judging:

- deny microphone access and confirm text input still works
- deny camera access and confirm manual roll entry still works
- verify AI-generated voice is labeled as AI-generated
- verify the player cannot see GM-only notes in the portal

## Known demo constraints

- HTTPS is local/internal only; in local testing it may use either a provided certificate pair or a self-signed certificate
- the current judged environment is not a public internet deployment
- some tests are deterministic against a local OpenAI-compatible mock rather than the live API
- device testing is strongest when run on the same LAN as the operator machine

## Submission-side reminders

These items are required in the Devpost submission but are not automatically enforced by this repository:

- final category selection
- public YouTube demo link
- `/feedback` Codex Session ID from the main Codex build thread
- final project description written in your own voice

## Troubleshooting

### The page does not open on a phone

- confirm the phone is on the same LAN
- confirm the hosts entry maps `dungeon-master.local` to the operator machine IP
- confirm the certificate used by the local HTTPS proxy is trusted on the device

### Camera or microphone features are blocked

- use HTTPS, not plain HTTP
- trust the local certificate
- refresh the page after changing browser permissions

### Judge needs a faster proof than the live walkthrough

Run:

```bash
bash scripts/mvp_smoke_test.sh
npm run test:golden-path
```

The smoke test hits the live local stack. The golden-path test runs a deterministic isolated stack.

## Files worth opening during review

- [README.md](../README.md)
- [SECURITY.md](../SECURITY.md)
- [docs/LOCAL_HTTPS.md](LOCAL_HTTPS.md)
- [BUILD_WEEK_PLAN.md](../BUILD_WEEK_PLAN.md)
