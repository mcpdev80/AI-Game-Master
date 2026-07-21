# OpenAI Build Week Plan – AI Game Master

Status date: July 20, 2026  
Submission deadline: July 21, 2026, 5:00 PM PDT / July 22, 2026, 02:00 CEST  
Recommended track: **Apps for Your Life**

## 1. Submission framing

We are not submitting the entire historical platform as if it were built from scratch during Build Week. We are submitting a clearly bounded Build Week upgrade built around Codex and GPT-5.6:

> **GPT-5.6 Live Encounter Director** — a browser-based AI game master that turns player speech, campaign context, character state, and physical dice rolls into a consistent narrated scene shared across an operator view and player-safe screens.

## 2. Golden path

The judged path is:

1. Load the included redistributable demo adventure.
2. Create or open a ready player character.
3. Create a session and assign the character.
4. Let a player join through a player link.
5. Start the session on the main display.
6. The player speaks or types an action.
7. GPT-5.6 decides whether a roll is needed.
8. A physical die is read by camera or confirmed through a safe manual fallback.
9. GPT-5.6 resolves the result, updates the state, and produces the next narration.
10. Operator view, player screen, and player portal each show only role-appropriate information.

## 3. Current project state

Already completed and verified:

- GPT-5.6 integrated through the OpenAI Responses API with strict structured output handling.
- OpenAI STT integrated through `gpt-4o-transcribe`.
- OpenAI TTS integrated through `gpt-4o-mini-tts`.
- Redistributable bilingual demo adventure included with attribution.
- Character builder supports German and English.
- Character builder keeps late review corrections for HP, Hit Dice, attacks, and derived values.
- Multiclass builder requests are normalized to a level-1 start draft with planned staged level-ups.
- Builder step completions now include a short explicit confirmation checkpoint.
- Session flow works locally through Docker Compose.
- Deterministic backend and browser golden-path tests exist.
- Local HTTPS/LAN hosting path exists for real-device testing.
- README, architecture notes, content policy, and third-party notices exist.
- Final demo video is included in the repository.

## 4. Remaining blockers before final submission

### P0

- Final public submission text on Devpost
- Final verification that public-facing repo files are in English where required
- Final live-device check over HTTPS
- Final audit of demo instructions, README, and links

### P1

- Extra negative-path coverage beyond the main judged path
- Further dependency cleanup where useful but not submission-blocking

## 5. What must be true before submission

- The repository contains no secrets, private certificates, internal-only files, or commercial source material.
- The README explains setup, quick start, and demo video access clearly.
- The demo can be exercised locally without private content.
- The video matches the actual shipped flow.
- The project description, README, and judged flow all tell the same story.

## 6. Challenge requirement mapping

| Requirement | Evidence |
|---|---|
| Built with Codex and GPT-5.6 | Commit history, Build Week changelog, GPT-5.6 Responses integration |
| Working project | Docker Compose setup, local demo, deterministic golden path |
| Demo video | `docs/media/demo-final-hd.mp4` |
| Repository access | Public GitHub repository |
| Clear setup instructions | `README.md` |
| Explain model usage | README, architecture notes, code, and demo flow |
| Original / rights-safe content | `THIRD_PARTY_NOTICES.md`, `docs/CONTENT_POLICY.md`, bundled licensed demo assets |

## 7. Security and content gate

Before final submission:

- run a final secret scan
- confirm `.env` and local certificates are not tracked
- confirm `test-results/` is not tracked
- confirm only licensed or original assets are shipped
- confirm no DM-only notes leak into player-facing views

## 8. Demo assets in repository

- Final demo video: `docs/media/demo-final-hd.mp4`
- Demo script: `docs/demo-script.md`
- Judge testing notes: `docs/judge-testing.md`
- Architecture notes: `docs/architecture.md`

## 9. Quick submission checklist

- [x] Final demo flow works locally
- [x] Final demo video exists in repo
- [x] Public remote exists
- [x] README includes quick start and demo video
- [x] Core Build Week implementation is committed
- [ ] Final Devpost form text completed
- [ ] Final HTTPS real-device smoke test completed
- [ ] Final repo scan completed immediately before submission

## 10. Non-goals

These are intentionally out of scope for the Build Week submission:

- full support for every RPG system
- a marketplace or monetization layer
- full enterprise auth and permissions
- public production SaaS hosting
- full replacement of a human GM for every play style
- training or fine-tuning a custom model

## 11. Final positioning

The strongest honest framing is:

- a local/LAN-first AI tabletop experience
- a working multimodal GM loop
- a rights-safe demo package
- a real tested path from character creation to narrated encounter resolution

That is specific enough for judging and strong enough to defend technically.
