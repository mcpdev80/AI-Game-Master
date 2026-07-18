#!/usr/bin/env python3
"""Deterministic OpenAI-compatible test double for the complete golden path."""

from __future__ import annotations

import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


MODEL = "gpt-5.6-golden-path-mock"


def response_payload(text: str) -> bytes:
    return json.dumps(
        {
            "model": MODEL,
            "status": "completed",
            "output": [
                {
                    "type": "message",
                    "content": [{"type": "output_text", "text": text}],
                }
            ],
        }
    ).encode("utf-8")


def encounter_turn(body: dict) -> str:
    prompt = json.dumps(body.get("input", []), ensure_ascii=False).lower()
    compact_prompt = prompt.replace("\\", "").replace(" ", "").replace("\n", "")
    if "__session_start__" in prompt:
        return json.dumps(
            {
                "narration": "Rain whispers outside the sheltered cave while a boulder conceals narrow cracks leading underground. What do you do?",
                "language": "en",
                "rules_used": [],
                "roll_request": None,
                "state_updates": [],
                "scene_events": [{"type": "map", "name": "fungal_caverns_map"}],
                "dm_notes": [],
            }
        )
    if '"dice_roll":{' in compact_prompt:
        return json.dumps(
            {
                "narration": "With the strong result, the boulder shifts and reveals a safe passage into the fungal caverns.",
                "language": "en",
                "rules_used": [],
                "roll_request": None,
                "state_updates": [
                    {"entity_id": "session", "field": "group_gold", "delta": 3, "value": ""}
                ],
                "scene_events": [{"type": "map", "name": "fungal_caverns_map"}],
                "dm_notes": ["Golden-path mock resolved the confirmed roll."],
            }
        )
    return json.dumps(
        {
            "narration": "Finding the safest route requires a Perception check. Roll one d20.",
            "language": "en",
            "rules_used": [],
            "roll_request": {
                "type": "check",
                "label": "Find the safe passage",
                "dice": ["d20"],
                "ability": "Wisdom",
                "skill": "Perception",
                "dc": 12,
                "hide_dc": False,
                "reason": "Loose stone makes the narrow route dangerous.",
                "instructions": "Roll one d20 and add Perception.",
                "follow_up_on_success": None,
            },
            "state_updates": [],
            "scene_events": [{"type": "ambience", "name": "cave_drips"}],
            "dm_notes": ["Golden-path mock requested a deterministic roll."],
        }
    )


def builder_turn() -> str:
    return json.dumps(
        {
            "reply": "A careful cave scout is a strong concept. I saved that direction in the live draft.",
            "updates": {
                "metadata": {
                    "concept": "A careful cave scout who maps dangerous underground routes.",
                    "builder_stage": "race",
                }
            },
        }
    )


class Handler(BaseHTTPRequestHandler):
    server_version = "GoldenPathOpenAIMock/1.0"

    def log_message(self, format: str, *args: object) -> None:
        print(format % args, flush=True)

    def send_bytes(self, status: int, content_type: str, payload: bytes) -> None:
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def do_GET(self) -> None:
        if self.path == "/health":
            self.send_bytes(200, "application/json", b'{"status":"ok"}')
            return
        self.send_bytes(404, "application/json", b'{"error":"not found"}')

    def do_POST(self) -> None:
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length)
        if self.path == "/v1/audio/speech":
            self.send_bytes(200, "audio/wav", b"RIFF-golden-path-test-audio")
            return
        if self.path == "/v1/audio/transcriptions":
            self.send_bytes(200, "application/json", b'{"text":"I inspect the hidden passage."}')
            return
        if self.path != "/v1/responses":
            self.send_bytes(404, "application/json", b'{"error":"not found"}')
            return
        try:
            body = json.loads(raw or b"{}")
        except json.JSONDecodeError:
            self.send_bytes(400, "application/json", b'{"error":{"message":"invalid json"}}')
            return
        format_type = (
            body.get("text", {}).get("format", {}).get("type", "")
            if isinstance(body.get("text"), dict)
            else ""
        )
        if format_type == "json_object":
            text = builder_turn()
        else:
            text = encounter_turn(body)
        self.send_bytes(200, "application/json", response_payload(text))


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 8099), Handler).serve_forever()
