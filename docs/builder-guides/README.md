# Builder Guides

These files define ruleset-specific character-builder guides.

Purpose:

- give the AI a clear step-by-step flow for character creation
- avoid hiding the entire process inside one oversized prompt
- make the builder swappable per ruleset and version later through the library

Core idea:

- each `ruleset_work + ruleset_version` can have one or more builder guides
- the character builder loads the matching guide from the library configuration
- the AI follows that guide instead of relying only on a generic prompt

Examples:

- `dnd-5e.character-builder.yaml`
- later, for example:
  - `dnd-3.5.character-builder.yaml`
  - `dsa-4.character-builder.yaml`
  - `how-to-be-a-hero.character-builder.yaml`

## Planned library integration

The YAML files should be treated as rulebook-adjacent configuration objects:

- linked to `ruleset_work`
- linked to `ruleset_version`
- optionally linked to preferred `document_ids`

That makes it possible to manage, per ruleset:

- rulebooks
- adventures
- assets
- character builder guides

## Builder design principle

A guide describes:

- builder purpose and tone
- fixed character-creation order
- what is derived automatically
- what the AI must explicitly ask for
- what must not be hallucinated
- which sheet fields are filled at which step

## Important architecture note

The guide does not replace rule text or retrieval.

It is only:

- flow control
- field mapping
- guardrails

Actual rules content still comes from:

- the selected rulebooks
- later retrieval and contextual evidence
