# Demo Script

This is the current English demo script for a sub-3-minute Build Week video.

## Goal

Show that AI Game Master is a playable, browser-based tabletop flow where:

- GPT-5.6 runs the encounter turn
- the player can join from a second device
- speech and dice handling are built in
- manual fallbacks keep the session usable
- the player only sees player-safe information

## Runtime target

About 2 minutes 30 seconds.

## Shot list and narration

### 0:00 - 0:12

On screen:

- Title card
- Control Center or seeded demo landing state

Voiceover:

> AI Game Master is a browser-based tabletop session system. One person runs the adventure, players join from their own device, and GPT-5.6 handles the live game-master turn.

### 0:12 - 0:30

On screen:

- Character flow
- Create a new player character
- Show English UI

Voiceover:

> The experience starts with a guided character flow. For Build Week, I moved the core game-master path to OpenAI's Responses API and kept the product bilingual, with English as the default public demo language.

### 0:30 - 0:52

On screen:

- Session setup or seeded Fungal Caverns demo
- Player join link or player portal open on a second device/browser

Voiceover:

> The operator opens a prepared adventure and shares a player link. The player gets a clean, player-safe view instead of the full operator state, so hidden notes and internal logic stay private.

### 0:52 - 1:18

On screen:

- Player Screen
- Player speaks or types an action
- Operator or split-screen showing the response arrive

Voiceover:

> When the player acts, the request goes through a Go backend that builds a structured prompt for GPT-5.6. The model returns narration, roll requests, and allowed state changes in a validated format before anything is applied.

### 1:18 - 1:42

On screen:

- Dice request appears
- Camera-assisted roll flow, or direct manual roll entry

Voiceover:

> If the turn needs a physical die, the session switches into dice capture. The player can use the camera-based flow, but the important part for a real session is the fallback: if camera access fails, the die result can still be entered manually.

### 1:42 - 2:02

On screen:

- Resolved narration
- Scene image or map visible on the player side
- Inventory or state update visible

Voiceover:

> After the roll is confirmed, GPT-5.6 resolves the outcome, the scene updates, and the player sees only the information that was intentionally released.

### 2:02 - 2:18

On screen:

- Brief glimpse of microphone / AI voice labeling
- maybe show the player-side error/fallback area

Voiceover:

> Speech is built in using OpenAI transcription and text-to-speech, and the interface labels AI-generated voice clearly. The system is designed so that text input, manual rolls, and player-safe delivery still work when device features are limited.

### 2:18 - 2:30

On screen:

- Fast cut to tests or repo/docs
- Golden-path test command or plan/docs overview

Voiceover:

> I backed the critical path with deterministic tests, including complete session flow, model-output validation, visible rate-limit errors, and manual roll fallback. The result is a playable local demo that is technically inspectable, not just a video mockup.

## Recording notes

- Use English UI for the full capture.
- Prefer one clean seeded session over multiple resets.
- If live microphone/camera permissions are unreliable, record the manual fallback path rather than forcing a flaky take.
- Keep terminal footage short; show test names or passing output, not a full install log.
- Avoid any commercial rulebook material or branding in the video.
