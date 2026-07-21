# DM-controlled companions plan

## Goal

Allow solo players or small groups to add one or more existing characters to a session as DM-controlled companions. These companions should travel with the party, appear in combat and initiative, be visible on player-facing surfaces, and be controlled by the AI DM rather than by a human player.

## Guiding decision

Use existing `Character` records as the base sheet for companions.

Do not build a second full NPC character system unless this later proves necessary.

This keeps:

- builder output reusable
- spell/attack/feature data reusable
- UI rendering mostly shared
- scope much smaller than a standalone NPC framework

## Target model

Split responsibilities cleanly:

- `Character`
  - canonical sheet data
  - reusable for players, companions, templates, and demos
- `PlayerSlot`
  - human player access to a session
  - join flow, ready state, portal token
- `SessionCompanion`
  - session-local DM-controlled party member using an existing `Character`

## Recommended data model

Add a new session-scoped table, for example `session_companions`.

Suggested fields:

- `id`
- `session_id`
- `character_id`
- `display_name`
- `control_mode`
  - initially always `dm`
- `status`
  - `active`, `inactive`, `down`, `dead`, `dismissed`
- `marching_order`
  - optional
- `tactics_note`
  - short DM instruction such as "protect the wizard" or "prefer healing over offense"
- `current_hit_points`
- `temporary_hit_points`
- `active_conditions_json`
- `resource_overrides_json`
  - optional spell slots, lay on hands, per-rest resources
- `visibility`
  - whether shown to players/board
- `created_at`
- `updated_at`

Keep session-runtime values here instead of mutating the base `Character` every turn.

## Why session-local runtime state matters

If HP, temp HP, initiative, conditions, and per-session notes are written directly into the base `Character`, one session will permanently alter a reusable companion sheet.

Safer split:

- base `Character`
  - static or long-lived sheet data
- `SessionCompanion`
  - runtime combat/session state

This also makes future reuse easier across multiple campaigns or test sessions.

## Core rules

1. A character may be added to a session as a companion only once.
2. A character cannot be both:
   - assigned to a `PlayerSlot`
   - and assigned as a `SessionCompanion`
   in the same session.
3. Companions do not get:
   - join links
   - portal tokens
   - player-ready state
4. Companions do get:
   - combat presence
   - board visibility
   - DM narration/control
   - optional session notes and tactics

## Backend work

### 1. Schema and store

Add:

- `session_companions` table
- indexes on `session_id` and `character_id`

Store methods:

- `ListSessionCompanions(sessionID)`
- `CreateSessionCompanion(sessionID, characterID, payload)`
- `UpdateSessionCompanion(id, payload)`
- `DeleteSessionCompanion(id)`
- `GetSessionCompanion(id)`

Validation:

- session exists
- character exists
- character not already claimed by player slot in same session
- character not already companion in same session

### 2. Session API

Recommended endpoints:

- `GET /api/sessions/:sessionId/companions`
- `POST /api/sessions/:sessionId/companions`
- `PUT /api/session-companions/:id`
- `DELETE /api/session-companions/:id`

Suggested request payload for create:

```json
{
  "character_id": "uuid",
  "display_name": "optional override",
  "tactics_note": "heal allies first",
  "visibility": "player_visible"
}
```

Suggested update fields:

- `display_name`
- `status`
- `tactics_note`
- `current_hit_points`
- `temporary_hit_points`
- `active_conditions`
- `visibility`

### 3. Session aggregate loading

Every place that loads a live session should also load companions:

- GM session screen
- player-facing session summary
- board view
- GM prompt context

Recommended shape:

- extend session response with `companions`
- do not overload `active_npcs` string arrays for this

### 4. Prompt context for the AI DM

The AI DM must receive a structured companion block in prompts:

- companion name
- role/concept
- AC / HP / speed
- attacks
- spell attacks
- key features
- tactics note
- current status/conditions

Prompt rules to add:

1. DM-controlled companions belong to the party.
2. The AI DM decides their actions.
3. Companions support the players but do not overshadow them.
4. Player characters remain the protagonists.
5. In combat, companions act in initiative order like other participants.
6. Companion actions and rolls must appear in the GM/session log.
7. Player-facing narration should describe companion actions naturally.

### 5. State update allowlist

Current `state_updates` validation is player/session oriented.

Add companion-safe fields, for example:

- `current_hit_points`
- `temporary_hit_points`
- `status`
- `conditions`
- `resource_state`
- `session_notes`

Recommended approach:

- allow `entity_id` values for companion IDs
- validate fields against a companion allowlist
- reject arbitrary mutation of unrelated character sheet fields

## Combat integration

This is the most important technical slice.

### Required changes

Companions must be recognized by combat logic as valid actors.

They need:

- initiative entry
- turn participation
- action resolution
- HP/resource changes
- public narration
- GM-visible combat logging

### Recommended combat identity model

Treat combat actors as:

- player characters
- companions
- enemies
- optionally neutral NPCs later

Each combat actor should have:

- `id`
- `name`
- `side`
  - `players`, `companions`, `enemies`
- `source_type`
  - `character`, `companion`, `monster`
- `initiative`
- `status`

### Initiative behavior

Companions should:

- roll or receive initiative like any other party member
- appear in initiative order on session screen and board
- be distinguishable from players and monsters

### Roll logging

All companion rolls should be shown in the GM/session combat log:

- attack roll
- damage roll
- save/check if relevant

Public board output should remain cinematic rather than purely numeric.

## UI work

### GM session screen

Add a new section:

- party companions

Capabilities:

- add existing character as companion
- remove companion
- set active/inactive
- adjust current HP
- edit tactics note
- inspect summary sheet

### Player portal / player-facing session views

Players should be able to see companions that travel with the party:

- name
- role / short concept
- visible HP state indicator
- maybe simplified stat summary

They should not control them directly.

### Board

Board should show companions as part of the group:

- current active companions
- combat order if combat is active
- health/state indicator

### Session setup

Prefer adding companions in the GM session context, not in global system config.

Reason:

- companions are session-specific, not app-global
- a solo session may want them, another session may not

## Reuse strategy for current Character UI

To keep scope down:

- reuse existing character summary rendering
- do not create a separate companion sheet UI
- add a lighter "companion card" view for session screens

Good MVP:

- name
- class and level
- AC
- current/max HP
- main attacks
- main spells
- tactics note

## Visibility model

Recommended states:

- `hidden`
  - only GM sees it
- `player_visible`
  - player-facing surfaces may show it
- `board_visible`
  - board may show it visually

For MVP, `player_visible` and `board_visible` can be treated as the same if needed.

## Builder interaction

No separate NPC builder is required.

Use the normal character builder to create:

- hirelings
- retainers
- animal handlers
- sidekicks
- helper priests
- allied adventurers

Later, if needed, add:

- companion templates
- quick-create presets
- sidekick-specific sheets

## Suggested rollout phases

### Phase 1 — companion foundation

Scope:

- schema + store
- session companion CRUD
- GM session UI to add/remove companions
- session responses include companions

Done when:

- GM can attach an existing character to a session as DM-controlled companion

### Phase 2 — prompt and runtime awareness

Scope:

- prompt context includes companions
- AI DM knows companions are party-controlled by DM
- companions appear in player-safe session outputs

Done when:

- AI references companions consistently during scene play

### Phase 3 — combat support

Scope:

- companions in initiative
- companion attacks/logging
- companion HP/conditions updated correctly
- board/session combat display includes companions

Done when:

- a solo player plus one companion can complete a full combat correctly

### Phase 4 — polish

Scope:

- tactics presets
- better companion cards
- visibility controls
- richer resource tracking
- demo/session testing scenarios

## Tests required

### Backend

- create companion success
- reject duplicate companion in same session
- reject character already assigned to player slot in same session
- update runtime HP/status
- list companions in session response
- delete companion

### Prompt / state validation

- companion context included in GM prompt
- invalid companion state update rejected
- allowed companion runtime updates accepted

### Combat

- companion appears in initiative
- companion turn advances correctly
- companion rolls are logged
- companion HP change persists
- player-safe text excludes internal-only GM details

### UI / e2e

- GM can add companion from existing character
- player-facing screens show companion summary
- solo combat with companion runs end-to-end

## Risks

### 1. Companion overshadowing players

Mitigation:

- prompt rule: companions support, players decide major direction

### 2. Reusing global character state directly

Mitigation:

- store runtime values in `session_companions`

### 3. Combat complexity

Mitigation:

- integrate companions as first-class combat actors rather than special text-only exceptions

### 4. UI bloat

Mitigation:

- keep companion management in GM session screen
- reuse existing character summary data

## Estimated effort

If implemented with existing characters as the base:

- overall: medium

Breakdown:

- schema/store/API: medium
- prompt/runtime integration: medium
- combat integration: medium to high
- UI: medium
- tests: medium

This is much smaller than building a separate NPC sheet system from scratch.

## Recommended MVP boundary

For the first shipping version, do only this:

- add existing character as DM companion
- make companion visible in session + board
- include companion in combat and logs
- keep runtime HP/status session-local
- no join link
- no player control

Do not include yet:

- full long-rest resource automation
- level scaling
- autonomous recruitment systems
- sidekick-specific builder
- cross-session persistence of runtime injuries

## Recommended next implementation order

1. schema and store
2. session companion CRUD endpoints
3. session payload extension
4. GM UI to manage companions
5. prompt context and DM rules
6. combat integration
7. board/player display
8. tests and golden-path scenario

## Practical example

Solo player session:

- player controls Rowan
- GM adds Brother Alden as a companion
- Alden is marked DM-controlled
- AI DM knows Alden travels with Rowan
- in combat, Alden rolls initiative and acts after/before Rowan as appropriate
- Alden's attacks and healing are logged in session combat log
- board shows Alden as part of the party

This gives the desired solo-with-companion experience without forcing NPCs into the player-slot model.
