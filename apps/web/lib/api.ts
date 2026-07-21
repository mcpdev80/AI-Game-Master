const serverApiBaseUrl = process.env.API_INTERNAL_BASE_URL ?? process.env.NEXT_PUBLIC_API_BASE_URL;
const browserApiBaseUrl = "";

export const apiBaseUrl =
  typeof window === "undefined"
    ? serverApiBaseUrl ?? "http://api:8080"
    : browserApiBaseUrl;

async function parseJson<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const text = await response.text();
    throw new Error(`${response.status} ${response.statusText}: ${text}`);
  }

  return response.json() as Promise<T>;
}

export async function apiGet<T>(path: string): Promise<T> {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    cache: "no-store",
  });
  return parseJson<T>(response);
}

export async function apiPost<T>(path: string, body: unknown): Promise<T> {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  });

  return parseJson<T>(response);
}

export async function apiPut<T>(path: string, body: unknown): Promise<T> {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  });

  return parseJson<T>(response);
}

export async function apiDelete<T>(path: string): Promise<T> {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    method: "DELETE",
  });
  return parseJson<T>(response);
}

export async function apiUpload<T>(path: string, formData: FormData): Promise<T> {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    method: "POST",
    body: formData,
  });

  return parseJson<T>(response);
}

export async function apiUploadRaw<T>(path: string, body: FormData): Promise<T> {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    method: "POST",
    body,
  });
  return parseJson<T>(response);
}

export type LLMGatewayStatus = {
  status: string;
  in_flight: number;
  max_concurrent_requests: number;
  queue_length: number;
  circuit_breaker_open: boolean;
  circuit_breaker_until?: string | null;
  consecutive_failures: number;
  last_error?: string;
  rejected_requests: number;
  timeout_count: number;
  active_gateway_sessions: number;
  archived_gateway_sessions: number;
  profiles: {
    name: string;
    max_input_tokens: number;
    max_output_tokens: number;
    timeout_seconds: number;
    live_turn_window: number;
  }[];
};

export async function fetchLLMGatewayStatus(): Promise<LLMGatewayStatus> {
  return apiGet<LLMGatewayStatus>("/api/system/llm-gateway/status");
}

export function splitMetadataList(value: unknown): string[] {
  if (!value) return [];
  if (Array.isArray(value)) {
    return value.map((item) => String(item)).filter(Boolean);
  }
  if (typeof value === "string") {
    return value
      .split(/[,\n;]/)
      .map((item) => item.trim())
      .filter(Boolean);
  }
  return [];
}

export type Campaign = {
  id: string;
  name: string;
  description: string;
  created_at: string;
};

export type Adventure = {
  id: string;
  campaign_id: string | null;
  name: string;
  description: string;
  language: string;
  metadata: Record<string, unknown>;
  created_at: string;
};

export type SessionState = {
  scene_summary: string;
  active_npcs: string[];
  open_quests: string[];
  combat?: {
    active: boolean;
    round: number;
    active_turn_index: number;
    initiative_order: {
      id: string;
      character_id?: string;
      name: string;
      side: string;
      participant_type?: string;
      control_mode?: string;
      initiative: number;
      status?: string;
      armor_class?: number;
      hit_point_max?: number;
      current_hit_points?: number;
      temporary_hit_points?: number;
      death_save_successes?: number;
      death_save_failures?: number;
      stable?: boolean;
    }[];
    log: {
      timestamp: string;
      actor_id: string;
      actor_name: string;
      side: string;
      kind: string;
      summary: string;
      details?: Record<string, unknown>;
      public_text?: string;
    }[];
  };
  last_narration: string;
  last_dm_notes: string[];
  active_media_cue: string;
  visual_mode?: string;
  visual_payload?: Record<string, unknown>;
  audio_mode?: string;
  audio_payload?: Record<string, unknown>;
  voice_mode?: string;
  active_voice_profile_id?: string;
  active_speaker_role?: string;
  active_speaker_name?: string;
  tts_status?: string;
  ambient_cue_id?: string;
  play_llm_session_id?: string;
  rules_llm_session_id?: string;
  summary_llm_session_id?: string;
  session_recap?: string;
  selected_rulebook_ids?: string[];
  prompt_config?: {
    gm_style?: string;
    intro_style?: string;
    adventure_focus?: string;
    rules_strictness?: string;
    player_agency_style?: string;
    prompt_override?: string;
  };
  group_inventory?: {
    gold: number;
    items: string[];
    notes: string;
  };
  last_confirmed_roll?: null | {
    dice: { type: string; value: number }[];
    confidence: number;
    timestamp: string;
  };
  last_dice_roll: null | {
    dice: { type: string; value: number }[];
    confidence: number;
    timestamp: string;
  };
};

export type Session = {
  id: string;
  campaign_id: string;
  name: string;
  adventure_id: string | null;
  ruleset_work: string;
  ruleset_version: string;
  target_player_count: number;
  join_token: string;
  status: string;
  current_scene: string;
  current_location: string;
  language: string;
  default_voice_profile_id: string | null;
  state: SessionState;
  companions?: SessionCompanion[];
  created_at: string;
  updated_at: string;
};

export type SessionCompanion = {
  id: string;
  session_id: string;
  character_id: string;
  display_name: string;
  control_mode: string;
  status: string;
  tactics_note: string;
  visibility: string;
  current_hit_points?: number | null;
  temporary_hit_points?: number | null;
  conditions: string[];
  resource_overrides: Record<string, unknown>;
  character?: Character | null;
  created_at: string;
  updated_at: string;
};

export type FungalCavernsDemoResponse = {
  campaign: Campaign;
  adventure: Adventure;
  session: Session;
  map_asset: Asset;
  gm_url: string;
  player_screen_url: string;
  reused: boolean;
};

export async function createFungalCavernsDemo(language: "en" | "de" = "en"): Promise<FungalCavernsDemoResponse> {
  return apiPost<FungalCavernsDemoResponse>("/api/demo/fungal-caverns", { language });
}

export type Character = {
  id: string;
  campaign_id: string | null;
  document_id: string | null;
  name: string;
  player_name: string;
  class_and_level: string;
  background: string;
  race: string;
  alignment: string;
  armor_class: number | null;
  speed: string;
  hit_point_max: number | null;
  proficiency_bonus: string;
  abilities: Record<string, number>;
  languages: string[];
  features: string[];
  metadata: Record<string, unknown>;
  created_at: string;
};

export type CharacterBuilderMessage = {
  role: string;
  content: string;
  created_at: string;
};

export type CharacterBuilderPatch = {
  name?: string;
  player_name?: string;
  class_and_level?: string;
  background?: string;
  race?: string;
  alignment?: string;
  armor_class?: number | null;
  speed?: string;
  hit_point_max?: number | null;
  proficiency_bonus?: string;
  abilities?: Record<string, number>;
  languages?: string[];
  features?: string[];
  metadata?: Record<string, unknown>;
};

export type PlayerAccessLink = {
  id: string;
  player_slot_id: string;
  token: string;
  revoked_at: string | null;
  created_at: string;
};

export type PlayerSlot = {
  id: string;
  session_id: string;
  character_id: string | null;
  display_name: string;
  status: string;
  joined_at: string | null;
  created_at: string;
};

export type PlayerLinkSlot = {
  player_slot: PlayerSlot;
  link: PlayerAccessLink | null;
  join_url: string | null;
};

export type Document = {
  id: string;
  adventure_id: string | null;
  type: string;
  name: string;
  source_file_path: string | null;
  metadata: Record<string, unknown>;
  chunk_count: number;
  created_at: string;
};

export type SessionEvent = {
  id: string;
  session_id: string;
  type: string;
  payload: Record<string, unknown>;
  created_at: string;
};

export type Asset = {
  id: string;
  adventure_id: string | null;
  document_id: string | null;
  type: string;
  source_type: string;
  name: string;
  file_path: string;
  mime_type: string;
  entity_name: string | null;
  location_name: string | null;
  tags: string[];
  metadata: Record<string, unknown>;
  created_at: string;
};

export type PlayerVisibleState = {
  id: string;
  player_slot_id: string;
  visible_character: Record<string, unknown>;
  visible_handouts: Array<Record<string, unknown>>;
  visible_media: Array<Record<string, unknown>>;
  updated_at: string;
};

export type PlayerPortalSession = {
  token: string;
  session: Session;
  player_slot: PlayerSlot;
  character: Character | null;
  visible_state: PlayerVisibleState;
  available_characters: Character[];
};

export type PrivateChatMessage = {
  id: string;
  session_id: string;
  player_slot_id: string;
  character_id?: string | null;
  role: string;
  content: string;
  language: string;
  created_at: string;
};

export type SessionJoinCandidate = {
  player_slot: PlayerSlot;
  character: Character | null;
};

export type SessionJoinPreview = {
  session_id: string;
  session_name: string;
  session_status: string;
  has_progress: boolean;
  existing_players: SessionJoinCandidate[];
};

export type SystemConfig = {
  llm_provider?: "openai" | "local";
  llm_base_url: string;
  llm_model: string;
};

export type VoiceProfile = {
  id: string;
  name: string;
  language: string;
  style: string;
  role: string;
  provider: string;
  provider_model: string;
  is_default: boolean;
  description: string;
};

export type AbilityResolveResponse = {
  method: string;
  values: number[];
  assignment: Record<string, number>;
  rule_summary: string;
  recommended_reason: string;
  rolled_breakdown?: number[];
  needs_confirmation: boolean;
};

export type AbilityValidateResponse = {
  valid: boolean;
  values: number[];
  assignment: Record<string, number>;
  missing_abilities: string[];
  unexpected_keys: string[];
  duplicate_values: number[];
  recommended_reason: string;
};

export type DiceDetection = {
  type: string;
  value: number;
};

export type DiceBox = {
  x: number;
  y: number;
  w: number;
  h: number;
};

export type DiceDetectionFrame = {
  frame_id: string;
  dice: DiceDetection[];
  confidence: number;
  timestamp: string;
};

export type DetectDiceResponse = {
  dice: DiceDetection[];
  dice_count: number;
  boxes: DiceBox[];
  confidence: number;
  notes: string;
  raw_model: string;
};

export type StabilizeDiceFramesResponse = {
  stable: boolean;
  required_matches: number;
  matching_frames: number;
  stable_dice: DiceDetection[];
  confidence: number;
  signature: string;
  recent_frames: DiceDetectionFrame[];
};

export type ZipImportReport = {
  adventure: Adventure;
  documents: Document[];
  assets: Asset[];
  summary: {
    imported_documents: number;
    imported_assets: number;
    imported_battlemaps: number;
    imported_portraits: number;
    imported_tokens: number;
    imported_handouts: number;
  };
};

export async function fetchSessionEvents(sessionId: string): Promise<SessionEvent[]> {
  const response = await apiGet<{ items: SessionEvent[] }>(`/api/sessions/${sessionId}/events`);
  return response.items;
}

export type GMResponse = {
  session_id: string;
  narration: string;
  language: string;
  rules_used: string[];
  roll_request?: {
    type: string;
    label: string;
    dice: string[];
    ability?: string;
    skill?: string;
    dc?: number;
    reason?: string;
    instructions?: string;
  };
  state_updates: { entity_id: string; field: string; delta?: number; value?: string }[];
  scene_events: { type: string; name: string }[];
  dm_notes: string[];
  context_chunks: { document_id: string; document_name: string; chunk_text: string }[];
  prompt_source: string;
  raw_model: string;
  created_at: string;
};

export type DiceRollEvent = {
  dice: { type: string; value: number }[];
  total?: number;
  summary?: string;
  confidence: number;
  timestamp: string;
};

export async function fetchCampaigns(): Promise<Campaign[]> {
  const response = await apiGet<{ items: Campaign[] }>("/api/campaigns");
  return response.items;
}

export async function fetchAdventures(): Promise<Adventure[]> {
  const response = await apiGet<{ items: Adventure[] }>("/api/adventures");
  return response.items;
}

export async function fetchSessions(): Promise<Session[]> {
  const response = await apiGet<{ items: Session[] }>("/api/sessions");
  return response.items;
}

export async function fetchSession(sessionId: string): Promise<Session> {
  return apiGet<Session>(`/api/sessions/${sessionId}`);
}

export async function fetchSessionCompanions(sessionId: string): Promise<SessionCompanion[]> {
  const response = await apiGet<{ items: SessionCompanion[] }>(`/api/sessions/${sessionId}/companions`);
  return response.items;
}

export async function createSessionCompanion(
  sessionId: string,
  payload: {
    character_id: string;
    display_name?: string;
    tactics_note?: string;
    visibility?: string;
  },
): Promise<SessionCompanion> {
  return apiPost<SessionCompanion>(`/api/sessions/${sessionId}/companions`, payload);
}

export async function updateSessionCompanion(
  companionId: string,
  payload: {
    display_name?: string;
    status?: string;
    tactics_note?: string;
    visibility?: string;
    current_hit_points?: number | null;
    temporary_hit_points?: number | null;
    conditions?: string[];
    resource_overrides?: Record<string, unknown>;
  },
): Promise<SessionCompanion> {
  return apiPut<SessionCompanion>(`/api/session-companions/${companionId}`, payload);
}

export async function deleteSessionCompanion(companionId: string): Promise<{ deleted: boolean }> {
  return apiDelete<{ deleted: boolean }>(`/api/session-companions/${companionId}`);
}

export async function updateSession(
  sessionId: string,
  payload: {
    campaign_id: string;
    name: string;
    adventure_id?: string | null;
    ruleset_work: string;
    ruleset_version: string;
    target_player_count: number;
    current_scene?: string;
    current_location?: string;
    language?: string;
    default_voice_profile_id?: string | null;
    selected_rulebook_ids?: string[];
    prompt_config?: {
      gm_style?: string;
      intro_style?: string;
      adventure_focus?: string;
      rules_strictness?: string;
      player_agency_style?: string;
      prompt_override?: string;
    };
    group_inventory?: {
      gold: number;
      items: string[];
      notes: string;
    };
  }
): Promise<Session> {
  return apiPut<Session>(`/api/sessions/${sessionId}`, payload);
}

export async function deleteSession(sessionId: string): Promise<{ deleted: boolean }> {
  return apiDelete<{ deleted: boolean }>(`/api/sessions/${sessionId}`);
}

export async function startSession(sessionId: string): Promise<Session> {
  return apiPost<Session>(`/api/sessions/${sessionId}/start`, {});
}

export async function pauseSession(sessionId: string): Promise<Session> {
  return apiPost<Session>(`/api/sessions/${sessionId}/pause`, {});
}

export async function stopSession(sessionId: string): Promise<Session> {
  return apiPost<Session>(`/api/sessions/${sessionId}/stop`, {});
}

export async function fetchDocuments(): Promise<Document[]> {
  const response = await apiGet<{ items: Document[] }>("/api/documents");
  return response.items;
}

export async function fetchAssets(): Promise<Asset[]> {
  const response = await apiGet<{ items: Asset[] }>("/api/assets");
  return response.items;
}

export async function fetchCharacters(): Promise<Character[]> {
  const response = await apiGet<{ items: Character[] }>("/api/characters");
  return response.items;
}

export async function fetchVoiceProfiles(): Promise<VoiceProfile[]> {
  const response = await apiGet<{ items: VoiceProfile[] }>("/api/voice-profiles");
  return response.items;
}

export async function createCharacter(payload: {
  campaign_id?: string | null;
  name: string;
  player_name?: string;
  class_and_level?: string;
  background?: string;
  race?: string;
  alignment?: string;
  armor_class?: number | null;
  speed?: string;
  hit_point_max?: number | null;
  proficiency_bonus?: string;
  abilities: Record<string, number>;
  languages?: string[];
  features?: string[];
  metadata?: Record<string, unknown>;
}): Promise<Character> {
  return apiPost<Character>("/api/characters", payload);
}

export async function updateCharacter(
  characterId: string,
  payload: {
    campaign_id?: string | null;
    name: string;
    player_name?: string;
    class_and_level?: string;
    background?: string;
    race?: string;
    alignment?: string;
    armor_class?: number | null;
    speed?: string;
    hit_point_max?: number | null;
    proficiency_bonus?: string;
    abilities: Record<string, number>;
    languages?: string[];
    features?: string[];
    metadata?: Record<string, unknown>;
  }
): Promise<Character> {
  return apiPut<Character>(`/api/characters/${characterId}`, payload);
}

export async function startCharacterBuilder(payload: {
  campaign_id?: string | null;
  ruleset_work: string;
  ruleset_version: string;
  selected_document_ids: string[];
  name?: string;
  player_name?: string;
  language: "en" | "de";
}): Promise<{ character: Character; messages: CharacterBuilderMessage[] }> {
  return apiPost<{ character: Character; messages: CharacterBuilderMessage[] }>("/api/characters/builder/start", payload);
}

export async function sendCharacterBuilderMessage(
  characterId: string,
  payload: { message: string; language: "en" | "de" }
): Promise<{
  character: Character;
  messages: CharacterBuilderMessage[];
  reply: string;
  applied_patch: CharacterBuilderPatch;
  ui_action?: string;
  ui_payload?: Record<string, unknown>;
}> {
  return apiPost<{
    character: Character;
    messages: CharacterBuilderMessage[];
    reply: string;
    applied_patch: CharacterBuilderPatch;
    ui_action?: string;
    ui_payload?: Record<string, unknown>;
  }>(
    `/api/characters/${characterId}/builder/message`,
    payload
  );
}

export async function applyCharacterBuilderPatch(
  characterId: string,
  payload: { patch: CharacterBuilderPatch }
): Promise<Character> {
  return apiPost<Character>(`/api/characters/${characterId}/builder/apply`, payload);
}

export async function finishCharacterBuilder(characterId: string): Promise<Character> {
  return apiPost<Character>(`/api/characters/${characterId}/builder/finish`, {});
}

export async function fetchPlayerLinks(sessionId: string): Promise<PlayerLinkSlot[]> {
  const response = await apiGet<{ items: PlayerLinkSlot[] }>(`/api/sessions/${sessionId}/player-links`);
  return response.items;
}

export async function updatePlayerVisibleState(
  playerSlotId: string,
  payload: {
    handout_document_ids?: string[];
    media_asset_ids?: string[];
  }
): Promise<PlayerVisibleState> {
  return apiPut<PlayerVisibleState>(`/api/player-slots/${playerSlotId}/visible-state`, payload);
}

export async function fetchPlayerPortal(token: string): Promise<PlayerPortalSession> {
  return apiGet<PlayerPortalSession>(`/api/player-portal/me?token=${encodeURIComponent(token)}`);
}

export async function joinPlayerPortal(token: string): Promise<PlayerPortalSession> {
  return apiPost<PlayerPortalSession>(`/api/player-portal/join/${encodeURIComponent(token)}`, {});
}

export async function joinSession(token: string, payload: { display_name?: string; player_slot_id?: string }): Promise<{
  session_token: string;
  portal_token: string;
  join_url: string;
  portal: PlayerPortalSession;
}> {
  return apiPost(`/api/sessions/join/${encodeURIComponent(token)}`, payload);
}

export async function fetchSessionJoinPreview(token: string): Promise<SessionJoinPreview> {
  return apiGet<SessionJoinPreview>(`/api/sessions/join/${encodeURIComponent(token)}`);
}

export async function fetchSystemConfig(): Promise<SystemConfig> {
  return apiGet<SystemConfig>("/api/system/config");
}

export async function updateSystemConfig(payload: SystemConfig): Promise<SystemConfig> {
  return apiPut<SystemConfig>("/api/system/config", payload);
}

export async function testLLMConnection(payload: SystemConfig): Promise<{ content: string; model: string }> {
  return apiPost<{ content: string; model: string }>("/api/system/llm-test", payload);
}

export async function fetchLLMModels(payload: SystemConfig): Promise<{ models: string[] }> {
  return apiPost<{ models: string[] }>("/api/system/llm-models", payload);
}

export async function updatePlayerSlotCharacter(playerSlotId: string, payload: { character_id: string }) {
  return apiPut(`/api/player-slots/${playerSlotId}/character`, payload);
}

export async function updatePlayerSlotStatus(playerSlotId: string, payload: { status: "invited" | "joined" | "ready" | "locked" }) {
  return apiPut(`/api/player-slots/${playerSlotId}/status`, payload);
}

export async function updatePlayerPortalCharacter(
  token: string,
  payload: {
    current_hit_points?: number;
    temporary_hit_points?: number;
    current_money?: string;
    experience_points?: string;
    inspiration?: string;
    session_notes?: string;
    current_inventory?: string[];
  }
): Promise<Character> {
  return apiPut<Character>(`/api/player-portal/character?token=${encodeURIComponent(token)}`, payload);
}

export async function updatePlayerPortalGroupInventory(
  token: string,
  payload: {
    gold?: number;
    items?: string[];
    notes?: string;
  }
): Promise<SessionState["group_inventory"]> {
  return apiPut<SessionState["group_inventory"]>(`/api/player-portal/group-inventory?token=${encodeURIComponent(token)}`, payload);
}

export async function fetchPlayerPortalPrivateChat(token: string): Promise<PrivateChatMessage[]> {
  const response = await apiGet<{ items: PrivateChatMessage[] }>(`/api/player-portal/private-chat?token=${encodeURIComponent(token)}`);
  return response.items;
}

export async function sendPlayerPortalPrivateChat(
  token: string,
  payload: { message: string; language: string }
): Promise<{ message: PrivateChatMessage; reply: PrivateChatMessage; messages: PrivateChatMessage[] }> {
  return apiPost<{ message: PrivateChatMessage; reply: PrivateChatMessage; messages: PrivateChatMessage[] }>(
    `/api/player-portal/private-chat?token=${encodeURIComponent(token)}`,
    payload
  );
}

export async function updateSessionRuntimeState(
  sessionId: string,
  payload: {
    visual_mode?: string;
    visual_payload?: Record<string, unknown>;
    audio_mode?: string;
    audio_payload?: Record<string, unknown>;
    voice_mode?: string;
    active_voice_profile_id?: string;
    active_speaker_role?: string;
    active_speaker_name?: string;
    tts_status?: string;
    ambient_cue_id?: string;
    session_recap?: string;
  }
) {
  return apiPut<Session>(`/api/sessions/${sessionId}/runtime-state`, payload);
}

export async function resolveAbilityScores(payload: {
  method: "standard" | "point_buy" | "rolled";
  class?: string;
  rolled_sets?: number[][];
  point_buy?: number[];
}): Promise<AbilityResolveResponse> {
  return apiPost<AbilityResolveResponse>("/api/characters/ability-scores/resolve", payload);
}

export async function validateAbilityAssignment(payload: {
  values: number[];
  assignment: Record<string, number>;
  class?: string;
}): Promise<AbilityValidateResponse> {
  return apiPost<AbilityValidateResponse>("/api/characters/ability-scores/validate-assignment", payload);
}

export async function detectDiceFromImage(payload: {
  image_data_url: string;
  language?: string;
}): Promise<DetectDiceResponse> {
  const response = await apiPost<DetectDiceResponse>("/api/vision/dice/detect", payload);
  return {
    ...response,
    dice: Array.isArray(response.dice) ? response.dice : [],
    boxes: Array.isArray(response.boxes) ? response.boxes : [],
    dice_count:
      typeof response.dice_count === "number"
        ? response.dice_count
        : Array.isArray(response.boxes)
          ? response.boxes.length
          : Array.isArray(response.dice)
            ? response.dice.length
            : 0,
  };
}

export async function stabilizeDiceFrames(payload: {
  frames: DiceDetectionFrame[];
  min_consensus?: number;
}): Promise<StabilizeDiceFramesResponse> {
  const response = await apiPost<StabilizeDiceFramesResponse>("/api/vision/dice/stabilize", payload);
  return {
    ...response,
    stable_dice: Array.isArray(response.stable_dice) ? response.stable_dice : [],
    recent_frames: Array.isArray(response.recent_frames) ? response.recent_frames : [],
  };
}
