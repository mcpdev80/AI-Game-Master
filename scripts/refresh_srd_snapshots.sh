#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target_dir="$repo_root/docs/reference/srd"
embedded_target="$repo_root/apps/api/internal/httpapi/embedded_rules/dnd-5e.srd51_source.md"
current_date_utc="$(date -u +%F)"
current_snapshot="$target_dir/SRD-v5.2.1-${current_date_utc}.html"

mkdir -p "$target_dir"

curl -L -o "$target_dir/SRD-OGL_V5.1.pdf" https://media.wizards.com/2016/downloads/DND/SRD-OGL_V5.1.pdf
pdftotext -layout "$target_dir/SRD-OGL_V5.1.pdf" "$target_dir/SRD-OGL_V5.1.layout.txt"
curl -L -o "$current_snapshot" https://dnd.wizards.com/resources/systems-reference-document

python3 - <<'PY' "$target_dir/SRD-OGL_V5.1.layout.txt" "$embedded_target" "$current_date_utc"
from pathlib import Path
import sys
source = Path(sys.argv[1])
target = Path(sys.argv[2])
snapshot_date = sys.argv[3]
text = source.read_text(errors='ignore').strip()
header = f"""# SRD 5.1 original source snapshot / SRD-5.1-Originalquelle

Official snapshot source for the 5E 2014 rules profile used by this app.

- Source PDF: `docs/reference/srd/SRD-OGL_V5.1.pdf`
- Extracted text snapshot: `docs/reference/srd/SRD-OGL_V5.1.layout.txt`
- Upstream URL: https://media.wizards.com/2016/downloads/DND/SRD-OGL_V5.1.pdf
- Snapshot date: {snapshot_date}
- License context: see `THIRD_PARTY_NOTICES.md`

This file intentionally preserves an extracted copy of the official SRD 5.1 text so parsing, validation, and future data regeneration can fall back to a stable in-repo reference even if upstream pages, downloads, or layouts change.

## Extracted text snapshot

```text
"""
target.write_text(header + text + "\n```\n")
PY

echo "Refreshed SRD snapshots in $target_dir"
