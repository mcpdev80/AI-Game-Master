# SRD reference snapshots

This folder stores stable upstream reference snapshots used to verify parser output, regenerate structured data, and debug future source changes without depending on a live upstream layout.

Included files:

- `SRD-OGL_V5.1.pdf` — original official SRD 5.1 PDF snapshot downloaded on July 21, 2026
- `SRD-OGL_V5.1.layout.txt` — `pdftotext -layout` extraction of that PDF
- `SRD-v5.2.1-2026-07-21.html` — HTML snapshot of the official current SRD landing page as fetched on July 21, 2026

Upstream sources:

- SRD 5.1 PDF: https://media.wizards.com/2016/downloads/DND/SRD-OGL_V5.1.pdf
- Current SRD page: https://dnd.wizards.com/resources/systems-reference-document

Usage:

- Use `SRD-OGL_V5.1.layout.txt` as the stable baseline for parser and regex validation.
- Use `SRD-OGL_V5.1.pdf` when the exact upstream original is needed for manual verification.
- Use the dated SRD v5.2.1 HTML snapshot to compare the current official landing page against future upstream changes.

These snapshots are reference material. The active 5E 2014 runtime rules profile should continue to use SRD 5.1 content unless intentionally migrated.
