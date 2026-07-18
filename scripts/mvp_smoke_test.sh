#!/usr/bin/env bash

set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://localhost:8085}"
WEB_BASE_URL="${WEB_BASE_URL:-http://localhost:3005}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

require_cmd curl
require_cmd jq

api_request() {
  local method="$1"
  local path="$2"
  local payload="${3:-}"
  local response_file
  local status
  response_file="$(mktemp)"

  local curl_args=(--silent --show-error --max-time 240 --output "${response_file}" --write-out '%{http_code}' --request "${method}" "${API_BASE_URL}${path}")
  if [[ -n "${payload}" ]]; then
    curl_args+=(--header 'Content-Type: application/json' --data "${payload}")
  fi

  if ! status="$(curl "${curl_args[@]}")"; then
    echo "Request failed: ${method} ${path}" >&2
    [[ -s "${response_file}" ]] && cat "${response_file}" >&2
    unlink "${response_file}"
    return 1
  fi
  if [[ ! "${status}" =~ ^2[0-9][0-9]$ ]]; then
    echo "Request failed: ${method} ${path} returned HTTP ${status}" >&2
    jq . "${response_file}" >&2 2>/dev/null || cat "${response_file}" >&2
    unlink "${response_file}"
    return 1
  fi

  cat "${response_file}"
  unlink "${response_file}"
}

echo "==> Health check"
health_json="$(api_request GET /api/health)"
jq -e '.status == "ok" and .database == "ok"' <<<"${health_json}" >/dev/null
jq '{status,database}' <<<"${health_json}"

echo
echo "==> Seed licensed bilingual demo"
demo_json="$(api_request POST /api/demo/fungal-caverns '{"language":"en"}')"
demo_session_id="$(jq -r '.session.id' <<<"${demo_json}")"
demo_asset_id="$(jq -r '.map_asset.id' <<<"${demo_json}")"
jq -e '
  .campaign.id and
  .adventure.id and
  .session.status == "live" and
  .session.state.visual_mode == "scene" and
  .session.state.visual_payload.image_asset_id == .map_asset.id and
  .map_asset.metadata.license == "CC BY 3.0 US"
' <<<"${demo_json}" >/dev/null
demo_documents_json="$(api_request GET /api/documents)"
jq -e '[.items[] | select(.metadata.demo_id == "fungal-caverns-v1")] | length >= 4' <<<"${demo_documents_json}" >/dev/null
demo_repeat_json="$(api_request POST /api/demo/fungal-caverns '{"language":"en"}')"
jq -e --arg session_id "${demo_session_id}" --arg asset_id "${demo_asset_id}" '.reused == true and .session.id == $session_id and .map_asset.id == $asset_id' <<<"${demo_repeat_json}" >/dev/null
jq '{campaign:.campaign.name,adventure:.adventure.name,session:.session.id,map_asset:.map_asset.id,reused}' <<<"${demo_repeat_json}"

suffix="$(date +%s)"
campaign_name="Build Week Demo ${suffix}"
session_name="Clockwork Observatory Test ${suffix}"
scene_name="The Sealed Observatory"
location_name="Moonlit Antechamber"
character_name="Avery Vale ${suffix}"
player_name="Smoke Player"

echo
echo "==> Create campaign"
campaign_json="$(api_request POST /api/campaigns \
  "$(jq -cn --arg name "${campaign_name}" '{name:$name, description:"Original Build Week golden-path test campaign"}')")"
campaign_id="$(jq -r '.id' <<<"${campaign_json}")"
echo "${campaign_json}" | jq '{id,name}'

echo
echo "==> Create session"
session_json="$(api_request POST /api/sessions \
  "$(jq -cn \
    --arg campaign_id "${campaign_id}" \
    --arg name "${session_name}" \
    --arg current_scene "${scene_name}" \
    --arg current_location "${location_name}" \
    '{campaign_id:$campaign_id,name:$name,ruleset_work:"SRD 5.1",ruleset_version:"5.1",target_player_count:1,current_scene:$current_scene,current_location:$current_location,language:"en"}')")"
session_id="$(jq -r '.id' <<<"${session_json}")"
jq -e '.id and .status and .target_player_count == 1' <<<"${session_json}" >/dev/null
echo "${session_json}" | jq '{id,name,status,ruleset_work,ruleset_version,target_player_count,current_scene,current_location,language}'

echo
echo "==> Create character"
character_json="$(api_request POST /api/characters \
  "$(jq -cn --arg name "${character_name}" --arg player_name "${player_name}" '{name:$name, player_name:$player_name, class_and_level:"wizard 1", race:"human", background:"acolyte", abilities:{strength:8,dexterity:14,constitution:13,intelligence:15,wisdom:12,charisma:10}, languages:["Common"], metadata:{creation_method:"build_week_smoke_test"}}')")"
character_id="$(jq -r '.id' <<<"${character_json}")"
jq -e '.id and .name' <<<"${character_json}" >/dev/null
echo "${character_json}" | jq '{id,name,class_and_level,race}'

echo
echo "==> Create player link"
player_link_json="$(api_request POST "/api/sessions/${session_id}/player-links" \
  "$(jq -cn --arg display_name "${player_name}" --arg character_id "${character_id}" '{display_name:$display_name, character_id:$character_id}')")"
player_slot_id="$(jq -r '.player_slot.id' <<<"${player_link_json}")"
join_url="$(jq -r '.join_url' <<<"${player_link_json}")"
token="${join_url##*/}"
echo "${player_link_json}" | jq '{player_slot:.player_slot.display_name, join_url}'

echo
echo "==> Join player portal"
portal_json="$(api_request POST "/api/player-portal/join/${token}")"
jq -e '.player_slot.status == "joined"' <<<"${portal_json}" >/dev/null
jq '{player_slot:.player_slot.display_name, status:.player_slot.status}' <<<"${portal_json}"

echo
echo "==> Mark player ready"
ready_json="$(api_request PUT "/api/player-slots/${player_slot_id}/status" '{"status":"ready"}')"
jq -e '.status == "ready"' <<<"${ready_json}" >/dev/null
jq '{id,display_name,status}' <<<"${ready_json}"

echo
echo "==> Start session"
started_json="$(api_request POST "/api/sessions/${session_id}/start" '{}')"
jq -e '.status == "live"' <<<"${started_json}" >/dev/null
jq '{id,name,status}' <<<"${started_json}"

echo
echo "==> Wait for asynchronous GPT-5.6 session opening"
gateway_idle=false
for _ in $(seq 1 100); do
  gateway_json="$(api_request GET /api/system/llm-gateway/status)"
	current_session_json="$(api_request GET "/api/sessions/${session_id}")"
	opening_narration="$(jq -r '.state.last_narration // ""' <<<"${current_session_json}")"
  if jq -e '.in_flight == 0' <<<"${gateway_json}" >/dev/null && [[ -n "${opening_narration}" && "${opening_narration}" != "${scene_name}" ]]; then
    gateway_idle=true
    break
  fi
  sleep 1
done
if [[ "${gateway_idle}" != "true" ]]; then
  echo "LLM gateway did not become idle after session opening" >&2
  jq '{status,in_flight,last_error,circuit_breaker_open}' <<<"${gateway_json}" >&2
  exit 1
fi
jq '{status,in_flight,circuit_breaker_open}' <<<"${gateway_json}"
jq '{opening_narration:.state.last_narration}' <<<"${current_session_json}"

echo
echo "==> Ask GPT-5.6 game master"
gm_json="$(api_request POST /api/gm/respond \
  "$(jq -cn --arg session_id "${session_id}" '{session_id:$session_id, player_input:"I inspect the sealed brass door and listen for movement beyond it.", language:"en"}')")"
jq -e '.session_id and (.narration | length > 0) and (.raw_model | startswith("gpt-5.6")) and .prompt_source == "llm"' <<<"${gm_json}" >/dev/null
echo "${gm_json}" | jq '{session_id,prompt_source,raw_model,language,narration,roll_request,state_updates,scene_events}'

uuid_pattern='^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$'
document_id="$(api_request GET /api/documents | jq -r --arg pattern "${uuid_pattern}" '[.items[] | select(.id | test($pattern))][0].id // empty')"
asset_id="$(api_request GET /api/assets | jq -r --arg pattern "${uuid_pattern}" '[.items[] | select(.id | test($pattern))][0].id // empty')"

if [[ -n "${document_id}" || -n "${asset_id}" ]]; then
  echo
  echo "==> Release visible state to player portal"
  visible_payload="$(jq -cn --arg document_id "${document_id}" --arg asset_id "${asset_id}" '
    {
      handout_document_ids: (if $document_id == "" then [] else [$document_id] end),
      media_asset_ids: (if $asset_id == "" then [] else [$asset_id] end)
    }'
  )"
  api_request PUT "/api/player-slots/${player_slot_id}/visible-state" "${visible_payload}" | jq '{player_slot_id, visible_handouts, visible_media}'
fi

echo
echo "==> Portal verification"
verified_portal_json="$(api_request GET "/api/player-portal/me?token=${token}")"
jq -e '.player_slot.status == "ready" and .session.status == "live"' <<<"${verified_portal_json}" >/dev/null
jq '{player_slot:.player_slot.display_name,status:.player_slot.status,session_status:.session.status,visible_handouts:(.visible_state.visible_handouts|length),visible_media:(.visible_state.visible_media|length)}' <<<"${verified_portal_json}"

echo
echo "==> Golden path passed"
echo "Provider: openai"
echo "Model:    $(jq -r '.raw_model' <<<"${gm_json}")"
echo "Session:  ${session_id}"

echo
echo "==> Useful URLs"
echo "Operator sessions: ${WEB_BASE_URL}/sessions"
echo "Active session:    ${WEB_BASE_URL}/sessions/${session_id}"
echo "Player join:       ${WEB_BASE_URL}/join/${token}"
echo "Player portal:     ${WEB_BASE_URL}/player-portal/${token}"
