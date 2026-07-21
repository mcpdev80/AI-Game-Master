#!/usr/bin/env bash

set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://localhost:18085}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "Missing required command: $1" >&2; exit 1; }
}

require_cmd curl
require_cmd jq

api_request() {
  local method="$1"
  local path="$2"
  local payload="${3:-}"
  local response_file status
  response_file="$(mktemp)"
  local args=(--silent --show-error --max-time 120 --output "${response_file}" --write-out '%{http_code}' --request "${method}" "${API_BASE_URL}${path}")
  if [[ -n "${payload}" ]]; then
    args+=(--header 'Content-Type: application/json' --data "${payload}")
  fi
  status="$(curl "${args[@]}")" || { cat "${response_file}" >&2; rm -f "${response_file}"; return 1; }
  if [[ ! "${status}" =~ ^2[0-9][0-9]$ ]]; then
    echo "${method} ${path} returned HTTP ${status}" >&2
    jq . "${response_file}" >&2 2>/dev/null || cat "${response_file}" >&2
    rm -f "${response_file}"
    return 1
  fi
  cat "${response_file}"
  rm -f "${response_file}"
}

echo "==> Health"
health_ready=false
for _ in $(seq 1 40); do
  if health_json="$(api_request GET /api/health 2>/dev/null)" && jq -e '.status == "ok" and .database == "ok"' <<<"${health_json}" >/dev/null; then
    health_ready=true
    break
  fi
  sleep 0.5
done
[[ "${health_ready}" == "true" ]] || { echo "API health check did not become ready" >&2; exit 1; }

echo "==> Seed Fungal Caverns"
demo_json="$(api_request POST /api/demo/fungal-caverns '{"language":"en"}')"
campaign_id="$(jq -r '.campaign.id' <<<"${demo_json}")"
adventure_id="$(jq -r '.adventure.id' <<<"${demo_json}")"
map_asset_id="$(jq -r '.map_asset.id' <<<"${demo_json}")"

echo "==> Start AI character builder with embedded SRD reference"
builder_json="$(api_request POST /api/characters/builder/start "$(jq -cn \
  --arg campaign_id "${campaign_id}" \
  '{campaign_id:$campaign_id,ruleset_work:"5E",ruleset_version:"2014",selected_document_ids:["embedded-short-rules-dnd-5e"],player_name:"Golden Path Player"}')")"
character_id="$(jq -r '.character.id' <<<"${builder_json}")"
jq -e '.character.metadata.builder_status == "draft" and (.messages | length) == 1' <<<"${builder_json}" >/dev/null

echo "==> Complete one conversational builder turn"
builder_message_json="$(api_request POST "/api/characters/${character_id}/builder/message" \
  '{"message":"I want to play a careful cave scout who protects the group and maps hidden routes."}')"
jq -e '.reply | contains("cave scout")' <<<"${builder_message_json}" >/dev/null
jq -e '.character.metadata.concept | contains("cave scout")' <<<"${builder_message_json}" >/dev/null

echo "==> Apply completed sheet and finish builder"
apply_json="$(api_request POST "/api/characters/${character_id}/builder/apply" '{
  "patch": {
    "name": "Eira Flint",
    "player_name": "Golden Path Player",
    "class_and_level": "Ranger 1",
    "background": "Cave Cartographer",
    "race": "Human",
    "alignment": "Neutral Good",
    "armor_class": 14,
    "speed": "30 ft",
    "hit_point_max": 11,
    "proficiency_bonus": "+2",
    "abilities": {"strength":10,"dexterity":16,"constitution":13,"intelligence":12,"wisdom":15,"charisma":8},
    "languages": ["Common"],
    "features": ["Natural Explorer", "Favored Enemy"],
    "metadata": {
      "builder_stage":"review",
      "builder_status":"draft",
      "creation_method":"standard_array",
      "skill_proficiencies":["Perception","Survival"],
      "saving_throw_proficiencies":["Strength","Dexterity"],
      "hit_dice":"1d10",
      "age":"27",
      "size":"Medium",
      "weight":"145 lb",
      "eyes":"Gray",
      "skin":"Fair",
      "hair":"Brown",
      "personality_traits":"Careful, watchful, and quietly protective.",
      "ideals":"Responsibility. If I know the safe path, I guide others through it.",
      "bonds":"I keep maps and notes so no companion is lost in the dark.",
      "flaws":"I can hesitate too long when every route looks dangerous.",
      "starting_equipment":["Scale mail","Longbow","20 arrows","2 shortswords","Explorer pack"],
      "current_inventory":["Scale mail","Longbow","20 arrows","2 shortswords","Explorer pack","Cartographer notes"],
      "current_money":"10 gp",
      "senses":"Passive Perception 14",
      "combat_attacks":"Longbow | +2 | DEX | 150/600 ft | +5 | 1d8+3 | Piercing\nDescription: Main ranged attack for scouting and opening combat.\nShortsword | +2 | DEX | Melee | +5 | 1d6+3 | Piercing\nDescription: Close-range finesse backup weapon."
    }
  }
}')"
jq -e '.name == "Eira Flint" and .abilities.dexterity == 16' <<<"${apply_json}" >/dev/null
finished_json="$(api_request POST "/api/characters/${character_id}/builder/finish" '{}')"
jq -e '.metadata.builder_status == "ready" and .metadata.builder_stage == "complete"' <<<"${finished_json}" >/dev/null

echo "==> Create playable session and join with built character"
session_json="$(api_request POST /api/sessions "$(jq -cn \
  --arg campaign_id "${campaign_id}" --arg adventure_id "${adventure_id}" \
  '{campaign_id:$campaign_id,adventure_id:$adventure_id,name:"Golden Path Fungal Caverns",ruleset_work:"5E",ruleset_version:"2014",target_player_count:1,current_scene:"Sheltered entrance",current_location:"Cavern entrance",language:"en"}')")"
session_id="$(jq -r '.id' <<<"${session_json}")"
link_json="$(api_request POST "/api/sessions/${session_id}/player-links" "$(jq -cn --arg id "${character_id}" '{display_name:"Golden Path Player",character_id:$id}')")"
slot_id="$(jq -r '.player_slot.id' <<<"${link_json}")"
join_url="$(jq -r '.join_url' <<<"${link_json}")"
join_token="${join_url##*/}"
portal_json="$(api_request POST "/api/player-portal/join/${join_token}" '{}')"
portal_token="$(jq -r '.token' <<<"${portal_json}")"
api_request PUT "/api/player-slots/${slot_id}/status" '{"status":"ready"}' | jq -e '.status == "ready"' >/dev/null

echo "==> Start session and wait for deterministic AI opening"
api_request POST "/api/sessions/${session_id}/start" '{}' | jq -e '.status == "live"' >/dev/null
opening_ready=false
for _ in $(seq 1 100); do
  gateway_json="$(api_request GET /api/system/llm-gateway/status)"
  current_session="$(api_request GET "/api/sessions/${session_id}")"
  opening_narration="$(jq -r '.state.last_narration // ""' <<<"${current_session}")"
  if jq -e '.in_flight == 0' <<<"${gateway_json}" >/dev/null && [[ -n "${opening_narration}" && "${opening_narration}" != "Sheltered entrance" ]]; then
    opening_ready=true
    break
  fi
  sleep 1
done
if [[ "${opening_ready}" != "true" ]]; then
  echo "Opening did not complete" >&2
  jq '{status,in_flight,last_error,circuit_breaker_open}' <<<"${gateway_json}" >&2
  jq '{last_narration:.state.last_narration,current_scene:.current_scene,visual_mode:.state.visual_mode}' <<<"${current_session}" >&2
  exit 1
fi

echo "==> Request roll"
roll_json="$(api_request POST /api/gm/respond "$(jq -cn --arg id "${session_id}" '{session_id:$id,player_input:"I inspect the boulder and search for a safe passage.",language:"en"}')")"
jq -e '.raw_model == "gpt-5.6-golden-path-mock" and .roll_request.type == "check" and .roll_request.dice == ["d20"]' <<<"${roll_json}" >/dev/null
api_request GET "/api/sessions/${session_id}" | jq -e '.state.visual_mode == "dice_capture"' >/dev/null

echo "==> Confirm dice and resolve turn"
resolved_json="$(api_request POST /api/gm/respond "$(jq -cn --arg id "${session_id}" '{
  session_id:$id,
  player_input:"I inspect the boulder and search for a safe passage.",
  language:"en",
  dice_roll:{dice:[{type:"d20",value:17}],total:20,summary:"d20 17 + Perception 3 = 20",confidence:1,timestamp:"2026-07-18T10:00:00Z"}
}')")"
jq -e '.roll_request == null and (.narration | contains("safe passage"))' <<<"${resolved_json}" >/dev/null
final_session="$(api_request GET "/api/sessions/${session_id}")"
jq -e --arg asset_id "${map_asset_id}" '
  .state.visual_mode == "scene" and
  .state.visual_payload.image_asset_id == $asset_id and
  .state.group_inventory.gold == 3
' <<<"${final_session}" >/dev/null

echo "==> Release map and verify player-safe portal"
api_request PUT "/api/player-slots/${slot_id}/visible-state" "$(jq -cn --arg id "${map_asset_id}" '{handout_document_ids:[],media_asset_ids:[$id]}')" >/dev/null
verified_portal="$(api_request GET "/api/player-portal/me?token=${portal_token}")"
jq -e '
  .character.name == "Eira Flint" and
  .player_slot.status == "ready" and
  .session.status == "live" and
  (.visible_state.visible_media | length) == 1 and
  (.session.state | has("last_dm_notes") | not)
' <<<"${verified_portal}" >/dev/null

jq -cn \
  --arg session_id "${session_id}" \
  --arg character_id "${character_id}" \
  --arg portal_token "${portal_token}" \
  --arg map_asset_id "${map_asset_id}" \
  '{status:"passed",session_id:$session_id,character_id:$character_id,portal_token:$portal_token,map_asset_id:$map_asset_id}'
