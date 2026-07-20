# Judge Testing Guide

This guide is for a fast local/demo evaluation of the current Build Week version of AI Game Master.

## What the judge should see

In under five minutes, the judge should be able to verify:

- GPT-5.6-powered game-master turns
- AI-assisted character creation
- player-safe session sharing
- speech/camera-ready player experience with manual fallbacks
- deterministic automated test coverage for the critical gameplay path

## Environment

Current supported demo setup:

- operator machine on the local network
- optional player phone on the same network
- internal HTTPS with a self-signed certificate

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

## Manual fallback checks

Useful quick checks during judging:

- deny microphone access and confirm text input still works
- deny camera access and confirm manual roll entry still works
- verify AI-generated voice is labeled as AI-generated
- verify the player cannot see GM-only notes in the portal

## Known demo constraints

- HTTPS is local/internal only and uses a self-signed certificate
- the current judged environment is not a public internet deployment
- some tests are deterministic against a local OpenAI-compatible mock rather than the live API
- device testing is strongest when run on the same LAN as the operator machine

## Troubleshooting

### The page does not open on a phone

- confirm the phone is on the same LAN
- confirm the hosts entry maps `dungeon-master.local` to the operator machine IP
- confirm the self-signed certificate was trusted on the device

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
