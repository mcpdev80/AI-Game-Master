"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Camera, FileText, Maximize, Mic, MicOff, Minimize, Pause, RotateCcw, Send, SkipForward, Square, Volume2, VolumeX, Wifi } from "lucide-react";
import { apiBaseUrl, apiPost, apiUploadRaw, detectDiceFromImage, splitMetadataList, type Adventure, type Asset, type Character, type Document, type GMResponse, type PlayerLinkSlot, type Session } from "../../lib/api";

function normalizeAmbientUrl(session: Session | null): string {
  const payload = session?.state.audio_payload;
  if (!payload || typeof payload !== "object") {
    return "";
  }
  const url = payload.url;
  return typeof url === "string" ? url : "";
}

type EnableState = {
  board: boolean;
  audio: boolean;
  mic: boolean;
  cam: boolean;
  fullscreen: boolean;
};

type UIStepRollRequest = {
  type: string;
  label: string;
  dice: string[];
  ability: string;
  skill: string;
  dc: number | null;
  reason: string;
  instructions: string;
  hideDC: boolean;
  followUpOnSuccess: UIStepRollRequest | null;
};
const STT_TARGET_SAMPLE_RATE = 32000;

function parseUIRollRequest(value: unknown): UIStepRollRequest | null {
  if (!value || typeof value !== "object") {
    return null;
  }
  const source = value as Record<string, unknown>;
  const dice = Array.isArray(source.dice) ? source.dice.map((item) => String(item).trim()).filter(Boolean) : [];
  const dc =
    typeof source.dc === "number"
      ? source.dc
      : typeof source.dc === "string" && source.dc.trim()
      ? Number(source.dc)
      : null;
  const followUp = parseUIRollRequest(source.follow_up_on_success);
  return {
    type: typeof source.type === "string" ? source.type : "",
    label: typeof source.label === "string" ? source.label : "",
    dice,
    ability: typeof source.ability === "string" ? source.ability : "",
    skill: typeof source.skill === "string" ? source.skill : "",
    dc: Number.isFinite(dc) ? dc : null,
    reason: typeof source.reason === "string" ? source.reason : "",
    instructions: typeof source.instructions === "string" ? source.instructions : "",
    hideDC: source.hide_dc === true,
    followUpOnSuccess: followUp,
  };
}

function parseNumericModifier(value: unknown): number | null {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value !== "string") {
    return null;
  }
  const match = value.match(/[-+]?\d+/);
  if (!match) {
    return null;
  }
  const parsed = Number(match[0]);
  return Number.isFinite(parsed) ? parsed : null;
}

function abilityModifier(score: number | null | undefined): number | null {
  if (typeof score !== "number" || !Number.isFinite(score)) {
    return null;
  }
  return Math.floor((score - 10) / 2);
}

function formatSigned(value: number): string {
  return value >= 0 ? `+${value}` : `${value}`;
}

function normalizeAbilityKey(value: string): string {
  const normalized = value.trim().toLowerCase();
  switch (normalized) {
    case "str":
    case "stärke":
      return "strength";
    case "dex":
    case "geschicklichkeit":
      return "dexterity";
    case "con":
    case "konstitution":
      return "constitution";
    case "int":
    case "intelligenz":
      return "intelligence";
    case "wis":
    case "weisheit":
      return "wisdom";
    case "cha":
    case "charisma":
      return "charisma";
    default:
      return normalized;
  }
}

function pickActiveRollCharacter(playerLinks: PlayerLinkSlot[], characters: Character[]): Character | null {
  for (const slot of playerLinks) {
    if (slot.player_slot.status !== "ready" && slot.player_slot.status !== "joined") {
      continue;
    }
    if (!slot.player_slot.character_id) {
      continue;
    }
    const character = characters.find((item) => item.id === slot.player_slot.character_id);
    if (character) {
      return character;
    }
  }
  return null;
}

function extractAttackBonusFromCharacter(character: Character | null, requestLabel: string, playerInput: string): { value: number; source: string } | null {
  if (!character) {
    return null;
  }
  const combatAttacks = String(character.metadata?.combat_attacks || "").split("\n").map((line) => line.trim()).filter(Boolean);
  const lookup = `${requestLabel} ${playerInput}`.toLowerCase();
  for (const line of combatAttacks) {
    const columns = line.split("|").map((part) => part.trim());
    if (columns.length < 2) {
      continue;
    }
    const attackName = columns[0].toLowerCase();
    if (attackName && !lookup.includes(attackName)) {
      continue;
    }
    const bonus = parseNumericModifier(columns[1]);
    if (bonus !== null) {
      return { value: bonus, source: columns[0] || "Angriff" };
    }
  }
  return null;
}

function summarizeStageText(input: string): string {
  const text = input.replace(/\s+/g, " ").trim();
  if (!text) {
    return "";
  }
  const parts = text.match(/[^.!?]+[.!?]?/g) ?? [text];
  return parts.slice(0, 2).join(" ").trim();
}

function isIgnorableSTTErrorMessage(message: string): boolean {
  const normalized = message.toLowerCase();
  return normalized.includes("422") || normalized.includes("no speech detected") || normalized.includes("502") || normalized.includes("503") || normalized.includes("504") || normalized.includes("bad gateway");
}

function isIgnorableTTSErrorMessage(message: string): boolean {
  const normalized = message.toLowerCase();
  return normalized.includes("502") || normalized.includes("503") || normalized.includes("504") || normalized.includes("bad gateway");
}

function looksUsablePlayerTurn(text: string): boolean {
  const normalized = text.replace(/\s+/g, " ").trim();
  if (normalized.length < 12) {
    return false;
  }
  const words = normalized.split(" ").filter(Boolean);
  if (words.length < 3) {
    return false;
  }
  const letterCount = (normalized.match(/[a-zA-ZäöüÄÖÜß]/g) || []).length;
  return letterCount >= Math.max(8, Math.floor(normalized.length * 0.45));
}

const WAKE_PHRASE = "Unsere Antwort";
const FINISH_PHRASE = "Ende unserer Antwort";

type AudioContextConstructor = typeof AudioContext;

function getAudioContextConstructor(): AudioContextConstructor | null {
  return (
    window.AudioContext ||
    (window as typeof window & { webkitAudioContext?: AudioContextConstructor }).webkitAudioContext ||
    null
  );
}

function pickRecordingMimeType(): string {
  const candidates = [
    "audio/mp4",
    "audio/ogg;codecs=opus",
    "audio/webm;codecs=opus",
  ];
  for (const candidate of candidates) {
    if (typeof MediaRecorder !== "undefined" && MediaRecorder.isTypeSupported(candidate)) {
      return candidate;
    }
  }
  return "";
}

async function blobToWav(blob: Blob): Promise<Blob> {
  const AudioContextCtor = getAudioContextConstructor();
  if (!AudioContextCtor) {
    throw new Error("AudioContext wird von diesem Browser nicht unterstützt.");
  }

  const arrayBuffer = await blob.arrayBuffer();
  const audioContext = new AudioContextCtor();
  try {
    const decoded = await audioContext.decodeAudioData(arrayBuffer.slice(0));
    return audioBufferToWavBlob(decoded);
  } catch (error) {
    throw new Error(error instanceof Error ? `Audio konnte nicht nach WAV konvertiert werden: ${error.message}` : "Audio konnte nicht nach WAV konvertiert werden.");
  } finally {
    await audioContext.close().catch(() => undefined);
  }
}

async function uploadSTTBlob(blob: Blob, fallbackFilename: string, language?: string): Promise<string> {
  const formData = new FormData();
  try {
    const wavBlob = await blobToWav(blob);
    formData.append("file", wavBlob, fallbackFilename.replace(/\.[^.]+$/, ".wav"));
  } catch {
    formData.append("file", blob, fallbackFilename);
  }
  if (language) {
    formData.append("language", language);
  }
  const result = await apiUploadRaw<{ text: string }>("/api/stt/transcriptions", formData);
  return String(result.text || "").trim();
}

function audioBufferToWavBlob(audioBuffer: AudioBuffer): Blob {
  const sampleRate = STT_TARGET_SAMPLE_RATE;
  const channelCount = 1;
  const channelData = resampleMonoBuffer(mixToMono(audioBuffer), audioBuffer.sampleRate, sampleRate);
  const samples = channelData.length;
  const bytesPerSample = 2;
  const blockAlign = channelCount * bytesPerSample;
  const byteRate = sampleRate * blockAlign;
  const dataSize = samples * blockAlign;
  const buffer = new ArrayBuffer(44 + dataSize);
  const view = new DataView(buffer);

  writeAscii(view, 0, "RIFF");
  view.setUint32(4, 36 + dataSize, true);
  writeAscii(view, 8, "WAVE");
  writeAscii(view, 12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, channelCount, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, byteRate, true);
  view.setUint16(32, blockAlign, true);
  view.setUint16(34, 16, true);
  writeAscii(view, 36, "data");
  view.setUint32(40, dataSize, true);

  let offset = 44;
  for (let index = 0; index < channelData.length; index += 1) {
    const sample = Math.max(-1, Math.min(1, channelData[index] ?? 0));
    const pcmValue = sample < 0 ? sample * 0x8000 : sample * 0x7fff;
    view.setInt16(offset, pcmValue, true);
    offset += 2;
  }

  return new Blob([buffer], { type: "audio/wav" });
}

function mixToMono(audioBuffer: AudioBuffer): Float32Array {
  const channels = Math.max(1, audioBuffer.numberOfChannels);
  if (channels === 1) {
    return audioBuffer.getChannelData(0).slice();
  }
  const output = new Float32Array(audioBuffer.length);
  for (let channelIndex = 0; channelIndex < channels; channelIndex += 1) {
    const data = audioBuffer.getChannelData(channelIndex);
    for (let sampleIndex = 0; sampleIndex < data.length; sampleIndex += 1) {
      output[sampleIndex] += data[sampleIndex] / channels;
    }
  }
  return output;
}

function resampleMonoBuffer(input: Float32Array, sourceSampleRate: number, targetSampleRate: number): Float32Array {
  if (!Number.isFinite(sourceSampleRate) || sourceSampleRate <= 0 || sourceSampleRate === targetSampleRate) {
    return input;
  }
  const targetLength = Math.max(1, Math.round(input.length * targetSampleRate / sourceSampleRate));
  const output = new Float32Array(targetLength);
  const ratio = sourceSampleRate / targetSampleRate;
  for (let index = 0; index < targetLength; index += 1) {
    const sourceIndex = index * ratio;
    const leftIndex = Math.floor(sourceIndex);
    const rightIndex = Math.min(leftIndex + 1, input.length - 1);
    const weight = sourceIndex - leftIndex;
    const left = input[leftIndex] ?? 0;
    const right = input[rightIndex] ?? left;
    output[index] = left + (right - left) * weight;
  }
  return output;
}

function writeAscii(view: DataView, offset: number, value: string) {
  for (let index = 0; index < value.length; index += 1) {
    view.setUint8(offset + index, value.charCodeAt(index));
  }
}

function extractDetectedDieValue(notes: string): number | null {
  const normalized = notes.replace(/\s+/g, " ").trim();
  if (!normalized) {
    return null;
  }
  const explicitPatterns = [
    /oberseite zeigt (?:die )?zahl (\d{1,3})/i,
    /oberseite zeigt(?: [a-zäöüß]+){0,3} die (\d{1,3})/i,
    /oberseite zeigt(?: [a-zäöüß]+){0,3} (\d{1,3})/i,
    /die zahl (\d{1,3}) ist auf der oberseite/i,
    /zahl (\d{1,3}) ist auf der oberseite/i,
    /zahl (\d{1,3}) .*oberseite/i,
    /oberseite .*zahl (\d{1,3})/i,
    /(\d{1,3}) ist auf der oberseite/i,
    /shows? (?:the )?(?:number |value )?(\d{1,3})/i,
    /zeigt (\d{1,3})/i,
    /zahl (\d{1,3})/i,
    /value[: ]+(\d{1,3})/i,
  ];
  for (const pattern of explicitPatterns) {
    const match = normalized.match(pattern);
    if (match) {
      const value = Number(match[1]);
      if (Number.isFinite(value) && value > 0) {
        return value;
      }
    }
  }
  const allNumbers = Array.from(normalized.matchAll(/\b\d{1,3}\b/g))
    .map((match) => Number(match[0]))
    .filter((value) => Number.isFinite(value) && value > 0);
  if (allNumbers.length === 0) {
    return null;
  }
  return allNumbers[allNumbers.length - 1] ?? null;
}

function maxDieValue(dieType: string): number | null {
  const match = dieType.trim().toLowerCase().match(/^d(\d{1,3})$/);
  if (!match) {
    return null;
  }
  const value = Number(match[1]);
  return Number.isFinite(value) && value > 0 ? value : null;
}

function buildDiceDetectionMessage(
  notes: string,
  expectedDice: string[],
  detectedDice: Array<{ type: string; value: number }>,
  finalValues: number[]
): string {
  const trimmedNotes = notes.trim();
  const hasImpossibleDetectedDie = detectedDice.some((die) => {
    const maxValue = maxDieValue(die.type);
    return maxValue !== null && die.value > maxValue;
  });
  if (trimmedNotes && !hasImpossibleDetectedDie) {
    return trimmedNotes;
  }
  const expectedSummary = expectedDice.map((type, index) => `${type} ${finalValues[index] ?? 1}`).join(", ");
  if (hasImpossibleDetectedDie) {
    return `Der erkannte Text war unplausibel. Übernommen wurde: ${expectedSummary}. Bitte kurz prüfen und bei Bedarf korrigieren.`;
  }
  return `Erkannt: ${expectedSummary}. Bitte kurz prüfen und bei Bedarf korrigieren.`;
}

function normalizeSpeechCommandPrefix(text: string): string {
  return text
    .trim()
    .toLowerCase()
    .replace(/^[\s"'.:;,_-]+/, "")
    .replace(/\s+/g, " ");
}

export function PlayerScreenView({
  session,
  documents,
  assets,
  adventures,
  characters,
  playerLinks,
}: {
  session: Session | null;
  documents: Document[];
  assets: Asset[];
  adventures: Adventure[];
  characters: Character[];
  playerLinks: PlayerLinkSlot[];
}) {
  const [liveSession, setLiveSession] = useState<Session | null>(session);
  const [enableState, setEnableState] = useState<EnableState>({
    board: false,
    audio: false,
    mic: false,
    cam: false,
    fullscreen: false,
  });
  const [isSpeaking, setIsSpeaking] = useState(false);
  const [ambientError, setAmbientError] = useState<string | null>(null);
  const [ttsError, setTTSError] = useState<string | null>(null);
  const [promptInput, setPromptInput] = useState("");
  const [promptPending, setPromptPending] = useState(false);
  const [promptError, setPromptError] = useState<string | null>(null);
  const [sttPending, setSTTPending] = useState(false);
  const [sttError, setSTTError] = useState<string | null>(null);
  const [sttTranscript, setSTTTranscript] = useState("");
  const [isRecording, setIsRecording] = useState(false);
  const [playbackVolume, setPlaybackVolume] = useState(0.72);
  const [popupVisible, setPopupVisible] = useState(false);
  const [metaPopup, setMetaPopup] = useState<null | "rules" | "adventure">(null);
  const [isAudioMuted, setIsAudioMuted] = useState(false);
  const [isMicMuted, setIsMicMuted] = useState(false);
  const [wakeListening, setWakeListening] = useState(false);
  const [conversationPhase, setConversationPhase] = useState<"idle" | "wake" | "capturing">("idle");
  const [diceDetectPending, setDiceDetectPending] = useState(false);
  const [diceSubmitPending, setDiceSubmitPending] = useState(false);
  const [diceDetectMessage, setDiceDetectMessage] = useState("");
  const [diceDetectError, setDiceDetectError] = useState<string | null>(null);
  const [diceValues, setDiceValues] = useState<number[]>([]);
  const [rollResolutionPending, setRollResolutionPending] = useState(false);
  const [localRollRequest, setLocalRollRequest] = useState<UIStepRollRequest | null>(null);
  const [collectedRollDice, setCollectedRollDice] = useState<Array<{ type: string; value: number }>>([]);

  const ambientRef = useRef<HTMLAudioElement | null>(null);
  const ttsRef = useRef<HTMLAudioElement | null>(null);
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const diceCanvasRef = useRef<HTMLCanvasElement | null>(null);
  const ttsObjectUrlRef = useRef<string | null>(null);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const wakeRecorderRef = useRef<MediaRecorder | null>(null);
  const wakeRestartTimerRef = useRef<number | null>(null);
  const audioStreamRef = useRef<MediaStream | null>(null);
  const videoStreamRef = useRef<MediaStream | null>(null);
  const recordedChunksRef = useRef<Blob[]>([]);
  const wakeChunksRef = useRef<Blob[]>([]);
  const currentTTSKeyRef = useRef<string>("");
  const ttsRequestRef = useRef(0);
  const captureBufferRef = useRef("");
  const captureSilenceChunksRef = useRef(0);
  const conversationPhaseRef = useRef<"idle" | "wake" | "capturing">("idle");
  const promptPendingRef = useRef(false);
  const wakeProcessingRef = useRef(false);

  useEffect(() => {
    conversationPhaseRef.current = conversationPhase;
  }, [conversationPhase]);

  useEffect(() => {
    promptPendingRef.current = promptPending;
  }, [promptPending]);

  useEffect(() => {
    setLiveSession(session);
  }, [session]);

  useEffect(() => {
    let cancelled = false;
    const refreshSession = async () => {
      try {
        const response = await fetch(`${apiBaseUrl}/api/sessions`, { cache: "no-store" });
        if (!response.ok) {
          return;
        }
        const payload = (await response.json()) as { items?: Session[] };
        const items = Array.isArray(payload.items) ? payload.items : [];
        const next = items.find((item) => item.status === "live") ?? items[0] ?? null;
        if (!cancelled) {
          setLiveSession(next);
        }
      } catch {
        // keep last known state
      }
    };
    refreshSession();
    const timer = window.setInterval(refreshSession, 3000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, []);

  useEffect(() => {
    const ambient = ambientRef.current;
    if (!ambient) {
      return;
    }
    ambient.volume = (isAudioMuted ? 0 : (isSpeaking ? 0.16 : 0.52) * playbackVolume);
  }, [isAudioMuted, isSpeaking, playbackVolume]);

  useEffect(() => {
    audioStreamRef.current?.getAudioTracks().forEach((track) => {
      track.enabled = !isMicMuted;
    });
  }, [isMicMuted, enableState.mic]);

  useEffect(() => {
    const video = videoRef.current;
    const stream = videoStreamRef.current;
    if (!video || !stream) {
      return;
    }
    if (video.srcObject !== stream) {
      video.srcObject = stream;
    }
    void video.play().catch(() => undefined);
  }, [enableState.cam, popupVisible]);

  useEffect(() => {
    return () => {
      if (ttsObjectUrlRef.current) {
        URL.revokeObjectURL(ttsObjectUrlRef.current);
      }
      if (wakeRestartTimerRef.current) {
        window.clearTimeout(wakeRestartTimerRef.current);
      }
      wakeRecorderRef.current?.stop();
      audioStreamRef.current?.getTracks().forEach((track) => track.stop());
      videoStreamRef.current?.getTracks().forEach((track) => track.stop());
    };
  }, []);

  const visualMode = liveSession?.state.visual_mode || "pause_or_recap";
  const effectiveVisualMode = rollResolutionPending ? "scene" : visualMode;
  const visualPayload = liveSession?.state.visual_payload || {};
  const serverRollRequest = useMemo(
    () =>
      parseUIRollRequest({
        type: visualPayload.roll_type,
        label: visualPayload.roll_label,
        dice: visualPayload.roll_dice,
        ability: visualPayload.roll_ability,
        skill: visualPayload.roll_skill,
        dc: visualPayload.roll_dc,
        reason: visualPayload.roll_reason,
        instructions: visualPayload.instructions,
        hide_dc: visualPayload.hide_dc,
        follow_up_on_success: visualPayload.follow_up_on_success,
      }),
    [visualPayload]
  );
  const activeRollRequest = localRollRequest || serverRollRequest;
  const rollRequestType = activeRollRequest?.type || "";
  const rollRequestLabel = activeRollRequest?.label || "";
  const rollRequestDice = activeRollRequest?.dice || [];
  const rollRequestAbility = activeRollRequest?.ability || "";
  const rollRequestSkill = activeRollRequest?.skill || "";
  const rollRequestReason = activeRollRequest?.reason || "";
  const rollRequestInstructions = activeRollRequest?.instructions || "";
  const pendingRollPlayerInput = typeof visualPayload.pending_player_input === "string" ? visualPayload.pending_player_input.trim() : "";
  const rollRequestDC = activeRollRequest?.dc && activeRollRequest.dc > 0 ? activeRollRequest.dc : null;
  const rollRequestHideDC = activeRollRequest?.hideDC === true;
  const rollRequestFollowUpOnSuccess = activeRollRequest?.followUpOnSuccess ?? null;
  const hasRollRequest = visualPayload.type === "roll_request";
  const normalizedRollDice = useMemo(
    () => (rollRequestDice.length > 0 ? rollRequestDice : ["d20"]),
    [rollRequestDice]
  );
  const ttsUrl = liveSession
    ? `${apiBaseUrl}/api/sessions/${liveSession.id}/tts-audio?mode=${encodeURIComponent(visualMode)}&updated_at=${encodeURIComponent(
        liveSession.updated_at
      )}`
    : "";
  const ambientUrl = normalizeAmbientUrl(liveSession);
  const ambientActive = Boolean(ambientUrl) && ["ambient_loop", "combat_loop"].includes(liveSession?.state.audio_mode || "");
  const ttsQueued = liveSession?.state.tts_status === "queued";
  const ttsPlaybackKey = liveSession ? `${liveSession.id}:${liveSession.updated_at}:${visualMode}:${liveSession.state.tts_status || "idle"}` : "";
  const ttsWaitingToStart = Boolean(ttsPlaybackKey) && ttsQueued && currentTTSKeyRef.current !== ttsPlaybackKey;
  const narrationText = String(
    visualPayload.narration ||
      visualPayload.scene ||
      liveSession?.state.last_narration ||
      liveSession?.state.session_recap ||
      "Warte auf die nächste Ausgabe des AI DM."
  );
  const referencedDocument =
    typeof visualPayload.document_id === "string" ? documents.find((document) => document.id === visualPayload.document_id) ?? null : null;
  const referencedAsset =
    typeof visualPayload.image_asset_id === "string" ? assets.find((asset) => asset.id === visualPayload.image_asset_id) ?? null : null;
  const assetUrl = referencedAsset ? `${apiBaseUrl}/api/assets/${referencedAsset.id}/file` : "";
  const sessionAdventure = liveSession ? adventures.find((item) => item.id === liveSession.adventure_id) ?? null : null;
  const rulesDocument = useMemo(() => {
    if (!liveSession) {
      return null;
    }
    return (
      documents.find((document) => {
        if (document.type !== "rules") {
          return false;
        }
        const work = String(document.metadata.ruleset_work || "");
        const version = String(document.metadata.ruleset_version || "");
        if (work === liveSession.ruleset_work && version === liveSession.ruleset_version) {
          return true;
        }
        return splitMetadataList(document.metadata.ruleset_keys).includes(`${liveSession.ruleset_work}:${liveSession.ruleset_version}`);
      }) ?? null
    );
  }, [documents, liveSession]);
  const activeRollCharacter = useMemo(() => pickActiveRollCharacter(playerLinks, characters), [playerLinks, characters]);
  const explicitAttackBonus = useMemo(
    () => extractAttackBonusFromCharacter(activeRollCharacter, rollRequestLabel, pendingRollPlayerInput || promptInput || sttTranscript),
    [activeRollCharacter, pendingRollPlayerInput, promptInput, rollRequestLabel, sttTranscript]
  );
  const derivedAbilityModifier = useMemo(() => {
    if (!activeRollCharacter || !rollRequestAbility) {
      return null;
    }
    const key = normalizeAbilityKey(rollRequestAbility);
    return abilityModifier(activeRollCharacter.abilities?.[key]);
  }, [activeRollCharacter, rollRequestAbility]);
  const proficiencyBonus = useMemo(() => parseNumericModifier(activeRollCharacter?.proficiency_bonus), [activeRollCharacter]);
  const rollComputation = useMemo(() => {
    if (!normalizedRollDice.length || diceValues.length < normalizedRollDice.length) {
      return null;
    }
    const normalizedValues = normalizedRollDice.map((_, index) => Math.max(1, Math.round(diceValues[index] ?? 1)));
    const base = normalizedValues[0] ?? 1;
    if (rollRequestType === "attack") {
      if (explicitAttackBonus) {
        return {
          total: base + explicitAttackBonus.value,
          breakdown: `Gewürfelt ${base} ${formatSigned(explicitAttackBonus.value)} Angriffsbonus = ${base + explicitAttackBonus.value}`,
        };
      }
      const parts: string[] = [`Gewürfelt ${base}`];
      let total = base;
      if (derivedAbilityModifier !== null) {
        total += derivedAbilityModifier;
        parts.push(`${formatSigned(derivedAbilityModifier)} ${rollRequestAbility || "Attribut"}`);
      }
      if (proficiencyBonus !== null) {
        total += proficiencyBonus;
        parts.push(`${formatSigned(proficiencyBonus)} Übungsbonus`);
      }
      if (parts.length > 1) {
        return {
          total,
          breakdown: `${parts.join(" ")} = ${total}`,
        };
      }
      return {
        total: base,
        breakdown: `Gewürfelt ${base}`,
      };
    }
    if (rollRequestType === "damage") {
      const rolledDamage = normalizedValues.reduce((sum, value) => sum + value, 0);
      const parts: string[] = [`Gewürfelt ${normalizedValues.join(" + ")}`];
      let total = rolledDamage;
      if (derivedAbilityModifier !== null) {
        total += derivedAbilityModifier;
        parts.push(`${formatSigned(derivedAbilityModifier)} ${rollRequestAbility || "Attribut"}`);
      }
      return {
        total,
        breakdown: parts.length > 1 ? `${parts.join(" ")} = ${total}` : `Gewürfelt ${rolledDamage}`,
      };
    }
    return null;
  }, [derivedAbilityModifier, diceValues, explicitAttackBonus, normalizedRollDice, proficiencyBonus, rollRequestAbility, rollRequestType]);
  const joinedPlayers = playerLinks.filter((slot) => slot.player_slot.status === "joined" || slot.player_slot.status === "ready").length;
  const readyPlayers = playerLinks.filter((slot) => slot.player_slot.status === "ready").length;
  const sessionRulebookCount = rulesDocument ? 1 : 0;
  const referencedDocumentPage =
    typeof visualPayload.document_page === "number"
      ? visualPayload.document_page
      : typeof visualPayload.document_page === "string" && visualPayload.document_page.trim()
        ? Number(visualPayload.document_page)
        : null;
  const rulesFileUrl = rulesDocument
    ? `${apiBaseUrl}/api/documents/${rulesDocument.id}/file${Number.isFinite(referencedDocumentPage) && referencedDocumentPage && referencedDocumentPage > 0 ? `#page=${Math.trunc(referencedDocumentPage)}` : ""}`
    : "";
  const shouldWakeListen =
    enableState.board &&
    enableState.mic &&
    !isMicMuted &&
    !isRecording &&
    !isSpeaking &&
    !ttsWaitingToStart &&
    !promptPending &&
    !sttPending;

  const voiceConsoleState = useMemo(() => {
    if (promptPending) {
      return { mode: "processing", title: "Verarbeitet", theme: "gold" };
    }
    if (sttPending) {
      return { mode: "transcribing", title: "Transkribiert", theme: "cyan" };
    }
    if (conversationPhase === "capturing") {
      return { mode: "listening", title: "Spieler spricht", theme: "blue" };
    }
    if (isRecording) {
      return { mode: "listening", title: "Lauscht", theme: "blue" };
    }
    if (isSpeaking || ttsWaitingToStart) {
      return { mode: "speaking", title: "DM spricht", theme: "red" };
    }
    return { mode: "idle", title: `Warte auf "${WAKE_PHRASE}"`, theme: "blue" };
  }, [conversationPhase, isRecording, isSpeaking, promptPending, sttPending, ttsWaitingToStart]);

  useEffect(() => {
    if (rollResolutionPending) {
      setPopupVisible(false);
      return;
    }
    if (["rules_reference", "combat", "dice_capture"].includes(visualMode)) {
      setPopupVisible(true);
      return;
    }
    setPopupVisible(false);
  }, [liveSession?.updated_at, rollResolutionPending, visualMode]);

  useEffect(() => {
    if (rollResolutionPending && visualMode !== "dice_capture") {
      setRollResolutionPending(false);
    }
  }, [rollResolutionPending, visualMode]);

  useEffect(() => {
    if (!hasRollRequest) {
      setDiceValues([]);
      setDiceDetectMessage("");
      setDiceDetectError(null);
      setDiceDetectPending(false);
      setDiceSubmitPending(false);
      setLocalRollRequest(null);
      setCollectedRollDice([]);
      return;
    }
    if (!localRollRequest) {
      setCollectedRollDice([]);
    }
    setDiceValues((current) => {
      if (current.length === normalizedRollDice.length) {
        return current;
      }
      return normalizedRollDice.map(() => 1);
    });
  }, [hasRollRequest, localRollRequest, normalizedRollDice]);

  useEffect(() => {
    const payload = visualPayload && typeof visualPayload === "object" ? visualPayload : {};
    if (payload.dismiss_popup === true) {
      setPopupVisible(false);
      return;
    }
    if (!popupVisible) {
      return;
    }
    const secondsValue = Number(payload.auto_close_seconds);
    const shouldAutoClose = payload.auto_close === true || Number.isFinite(secondsValue);
    if (!shouldAutoClose) {
      return;
    }
    const timeoutMs = Math.max(1200, (Number.isFinite(secondsValue) ? secondsValue : 6) * 1000);
    const timer = window.setTimeout(() => setPopupVisible(false), timeoutMs);
    return () => window.clearTimeout(timer);
  }, [popupVisible, visualPayload]);

  useEffect(() => {
    const ambient = ambientRef.current;
    if (!ambient || !enableState.audio || !ambientActive || !ambientUrl) {
      return;
    }
    ambient.loop = true;
    ambient.volume = isAudioMuted ? 0 : (isSpeaking ? 0.16 : 0.52) * playbackVolume;
    void ambient.play().catch(() => {
      setAmbientError("Ambient konnte nicht automatisch gestartet werden.");
    });
  }, [ambientActive, ambientUrl, enableState.audio, isAudioMuted, isSpeaking, playbackVolume]);

  async function transcribeWakeBlob(blob: Blob, mimeType: string) {
    const extension = mimeType.includes("ogg") ? "ogg" : mimeType.includes("mp4") ? "mp4" : "webm";
    return uploadSTTBlob(blob, `wake.${extension}`, liveSession?.language);
  }

  async function finalizeWakeCapture(reason: "end_keyword" | "silence") {
    const finalText = captureBufferRef.current.replace(/ende unserer antwort/gi, "").trim();
    captureBufferRef.current = "";
    captureSilenceChunksRef.current = 0;
    setConversationPhase("idle");
    setWakeListening(true);
    if (!finalText || promptPendingRef.current) {
      if (reason === "end_keyword") {
        setSTTTranscript(finalText || "Keine Spielerantwort erkannt.");
      }
      return;
    }
    if (!looksUsablePlayerTurn(finalText)) {
      setSTTTranscript(finalText || "Spielerantwort zu kurz oder unklar.");
      return;
    }
    setSTTTranscript(finalText);
    setPromptInput(finalText);
    await sendPlayerInput(finalText, true);
  }

  async function processWakeTranscriptChunk(transcript: string) {
    const cleanedTranscript = transcript.replace(/\s+/g, " ").trim();
    if (!cleanedTranscript) {
      if (conversationPhaseRef.current === "capturing" && captureBufferRef.current.trim()) {
        captureSilenceChunksRef.current += 1;
        if (captureSilenceChunksRef.current >= 2) {
          await finalizeWakeCapture("silence");
        }
      }
      return;
    }

    const normalized = normalizeSpeechCommandPrefix(cleanedTranscript);
    if (conversationPhaseRef.current !== "capturing") {
      const wakePrefix = WAKE_PHRASE.toLowerCase();
      if (!normalized.startsWith(wakePrefix)) {
        return;
      }
      const remainder = cleanedTranscript
        .slice(cleanedTranscript.toLowerCase().indexOf(wakePrefix) + WAKE_PHRASE.length)
        .trim()
        .replace(/^[:,.-]\s*/, "");
      captureBufferRef.current = remainder;
      captureSilenceChunksRef.current = 0;
      setConversationPhase("capturing");
      setSTTTranscript(remainder || `${WAKE_PHRASE} erkannt.`);
      return;
    }

    captureSilenceChunksRef.current = 0;
    captureBufferRef.current = `${captureBufferRef.current} ${cleanedTranscript}`.trim();
    setSTTTranscript(captureBufferRef.current);
    if (/ende unserer antwort/i.test(captureBufferRef.current)) {
      await finalizeWakeCapture("end_keyword");
    }
  }

  useEffect(() => {
    if (!shouldWakeListen) {
      if (wakeRestartTimerRef.current) {
        window.clearTimeout(wakeRestartTimerRef.current);
        wakeRestartTimerRef.current = null;
      }
      wakeRecorderRef.current?.stop();
      wakeRecorderRef.current = null;
      wakeChunksRef.current = [];
      setWakeListening(false);
      if (conversationPhaseRef.current !== "capturing") {
        setConversationPhase("idle");
      }
      return;
    }

    let cancelled = false;
    const startWakeRecorderSegment = async () => {
      try {
        const baseStream = audioStreamRef.current || (await navigator.mediaDevices.getUserMedia({ audio: true }));
        audioStreamRef.current = baseStream;
        const mimeType = pickRecordingMimeType();
        const recorder = mimeType ? new MediaRecorder(baseStream, { mimeType }) : new MediaRecorder(baseStream);
        wakeChunksRef.current = [];
        recorder.onstart = () => {
          if (cancelled) return;
          setWakeListening(true);
          setConversationPhase((current) => (current === "capturing" ? current : "wake"));
          setSTTError(null);
        };
        recorder.ondataavailable = (event) => {
          if (cancelled || !event.data || event.data.size === 0) {
            return;
          }
          wakeChunksRef.current.push(event.data);
        };
        recorder.onstop = async () => {
          if (!cancelled) {
            setWakeListening(false);
          }
          if (cancelled) {
            wakeChunksRef.current = [];
            return;
          }
          const chunkType = recorder.mimeType || wakeChunksRef.current[0]?.type || "audio/webm";
          const blob = new Blob(wakeChunksRef.current, { type: chunkType });
          wakeChunksRef.current = [];
          if (blob.size > 0 && !wakeProcessingRef.current) {
            wakeProcessingRef.current = true;
            try {
              const transcript = await transcribeWakeBlob(blob, chunkType);
              if (!cancelled) {
                await processWakeTranscriptChunk(transcript);
              }
            } catch (error) {
              if (!cancelled) {
                const message = error instanceof Error ? error.message : "Wake-Word fehlgeschlagen.";
                if (!isIgnorableSTTErrorMessage(message)) {
                  setSTTError(`Wake-Word fehlgeschlagen: ${message}`);
                }
              }
            } finally {
              wakeProcessingRef.current = false;
            }
          }
          if (!cancelled && shouldWakeListen) {
            wakeRestartTimerRef.current = window.setTimeout(() => {
              wakeRestartTimerRef.current = null;
              void startWakeRecorderSegment();
            }, 120);
          }
        };
        wakeRecorderRef.current = recorder;
        recorder.start();
        wakeRestartTimerRef.current = window.setTimeout(() => {
          wakeRestartTimerRef.current = null;
          if (!cancelled && recorder.state !== "inactive") {
            recorder.stop();
          }
        }, 10000);
      } catch (error) {
        if (!cancelled) {
          setWakeListening(false);
          const message = error instanceof Error ? error.message : "Wake-Word fehlgeschlagen.";
          if (!isIgnorableSTTErrorMessage(message)) {
            setSTTError(`Wake-Word fehlgeschlagen: ${message}`);
          }
        }
      }
    };

    if (!wakeRecorderRef.current || wakeRecorderRef.current.state === "inactive") {
      void startWakeRecorderSegment();
    }

    return () => {
      cancelled = true;
      if (wakeRestartTimerRef.current) {
        window.clearTimeout(wakeRestartTimerRef.current);
        wakeRestartTimerRef.current = null;
      }
      if (wakeRecorderRef.current && wakeRecorderRef.current.state !== "inactive") {
        wakeRecorderRef.current.stop();
      }
      wakeRecorderRef.current = null;
      wakeChunksRef.current = [];
      setWakeListening(false);
    };
  }, [shouldWakeListen]);

  useEffect(() => {
    if (!enableState.audio || !ttsQueued || !ttsUrl || !ttsPlaybackKey) {
      return;
    }
    if (currentTTSKeyRef.current === ttsPlaybackKey) {
      return;
    }
    let cancelled = false;
    const requestId = ttsRequestRef.current + 1;
    ttsRequestRef.current = requestId;
    const loadAndPlay = async () => {
      setTTSError(null);
      try {
        const response = await fetch(ttsUrl, { cache: "no-store" });
        if (!response.ok) {
          const text = await response.text();
          throw new Error(text || `${response.status}`);
        }
        const blob = await response.blob();
        if (cancelled || ttsRequestRef.current !== requestId) {
          return;
        }
        if (ttsObjectUrlRef.current) {
          URL.revokeObjectURL(ttsObjectUrlRef.current);
        }
        const objectUrl = URL.createObjectURL(blob);
        ttsObjectUrlRef.current = objectUrl;
        currentTTSKeyRef.current = ttsPlaybackKey;
        if (!cancelled) {
        }
        if (!ttsRef.current) {
          return;
        }
        if (ttsRef.current.src !== objectUrl) {
          ttsRef.current.src = objectUrl;
        }
        await ttsRef.current.play();
      } catch (error) {
        if (!cancelled) {
          const message = error instanceof Error ? error.message : "Sprachausgabe konnte nicht geladen werden.";
          if (!isIgnorableTTSErrorMessage(message)) {
            setTTSError(message);
          }
        }
      }
    };
    void loadAndPlay();
    return () => {
      cancelled = true;
    };
  }, [enableState.audio, ttsPlaybackKey, ttsQueued, ttsUrl]);

  async function handleActivateBoard() {
    let audio = false;
    let mic = false;
    let cam = false;
    let fullscreen = false;

    try {
      const AudioContextCtor = getAudioContextConstructor();
      if (AudioContextCtor) {
        const context = new AudioContextCtor();
        await context.resume();
        audio = true;
      } else {
        audio = true;
      }
    } catch {
      audio = false;
    }

    try {
      audioStreamRef.current = await navigator.mediaDevices.getUserMedia({ audio: true });
      mic = true;
    } catch {
      mic = false;
    }

    try {
      videoStreamRef.current = await navigator.mediaDevices.getUserMedia({ video: true });
      cam = true;
    } catch {
      cam = false;
    }

    try {
      await document.documentElement.requestFullscreen?.();
      fullscreen = Boolean(document.fullscreenElement);
    } catch {
      fullscreen = false;
    }

    setEnableState({
      board: true,
      audio,
      mic,
      cam,
      fullscreen,
    });
  }

  async function sendPlayerInput(text: string, clearInput = true) {
    if (!liveSession || !text.trim()) {
      return;
    }
    setPromptPending(true);
    setPromptError(null);
    try {
      await apiPost<GMResponse>("/api/gm/respond", {
        session_id: liveSession.id,
        player_input: text.trim(),
        language: "de",
      });
      if (clearInput) {
        setPromptInput("");
      }
      const response = await fetch(`${apiBaseUrl}/api/sessions/${liveSession.id}`, { cache: "no-store" });
      if (response.ok) {
        const updated = (await response.json()) as Session;
        setLiveSession(updated);
      }
    } catch (error) {
      setPromptError(error instanceof Error ? error.message : "Nachricht konnte nicht gesendet werden.");
    } finally {
      setPromptPending(false);
    }
  }

  function captureDiceFrame(): string | null {
    const video = videoRef.current;
    const canvas = diceCanvasRef.current;
    if (!video || !canvas || video.videoWidth <= 0 || video.videoHeight <= 0) {
      return null;
    }
    canvas.width = video.videoWidth;
    canvas.height = video.videoHeight;
    const context = canvas.getContext("2d");
    if (!context) {
      return null;
    }
    context.imageSmoothingEnabled = true;
    context.imageSmoothingQuality = "high";
    context.drawImage(video, 0, 0, canvas.width, canvas.height);
    return canvas.toDataURL("image/jpeg", 0.92);
  }

  async function handleDetectDiceRoll() {
    if (!enableState.cam) {
      setDiceDetectError("Die Kamera ist nicht aktiv. Aktiviere zuerst die Kamera im Board.");
      return;
    }
    const imageDataUrl = captureDiceFrame();
    if (!imageDataUrl) {
      setDiceDetectError("Es konnte gerade kein Kamerabild gelesen werden.");
      return;
    }
    setDiceDetectPending(true);
    setDiceDetectError(null);
    setDiceDetectMessage("Würfel werden ausgewertet...");
    try {
      const response = await detectDiceFromImage({ image_data_url: imageDataUrl, language: "de" });
      const noteValue = extractDetectedDieValue(String(response.notes || ""));
      const detectedDice = Array.isArray(response.dice)
        ? response.dice
            .map((die) => ({
              type: String(die.type || "").trim().toLowerCase(),
              value: Number(die.value || 0),
            }))
            .filter((die) => die.value > 0)
        : [];
      const remainingDice = [...detectedDice];
      const nextValues = normalizedRollDice.map((expectedType, index) => {
        const normalizedExpectedType = expectedType.toLowerCase();
        const exactIndex = remainingDice.findIndex((die) => die.type === normalizedExpectedType);
        if (exactIndex >= 0) {
          const [match] = remainingDice.splice(exactIndex, 1);
          if (normalizedRollDice.length === 1 && noteValue && noteValue > 0 && noteValue !== match.value) {
            return noteValue;
          }
          return match.value;
        }
        if (normalizedRollDice.length === 1 && detectedDice.length === 1) {
          if (noteValue && noteValue > 0 && noteValue !== detectedDice[0].value) {
            return noteValue;
          }
          return detectedDice[0].value;
        }
        if (normalizedRollDice.length === 1 && noteValue && noteValue > 0) {
          return noteValue;
        }
        if (index < remainingDice.length) {
          const [fallback] = remainingDice.splice(index, 1);
          return fallback.value;
        }
        return diceValues[index] ?? 1;
      });
      setDiceValues(nextValues);
      setDiceDetectMessage(buildDiceDetectionMessage(String(response.notes || ""), normalizedRollDice, detectedDice, nextValues));
    } catch (error) {
      setDiceDetectError(error instanceof Error ? error.message : "Würfelerkennung fehlgeschlagen.");
      setDiceDetectMessage("");
    } finally {
      setDiceDetectPending(false);
    }
  }

  function handleDiceValueChange(index: number, value: number) {
    setDiceValues((current) =>
      current.map((entry, entryIndex) => (entryIndex === index ? Math.max(1, Math.min(100, value || 1)) : entry))
    );
  }

  async function handleSubmitDiceRoll() {
    if (!liveSession || !hasRollRequest || diceSubmitPending) {
      return;
    }
    const playerInput = pendingRollPlayerInput || promptInput.trim() || sttTranscript.trim() || "Ich würfle jetzt.";
    const dice = normalizedRollDice.map((type, index) => ({
      type,
      value: Math.max(1, Math.round(diceValues[index] ?? 1)),
    }));
    const combinedDice = [...collectedRollDice, ...dice];
    if (!localRollRequest && rollRequestType === "attack" && rollRequestFollowUpOnSuccess && rollRequestDC !== null) {
      const attackTotal = rollComputation?.total ?? dice.reduce((sum, die) => sum + die.value, 0);
      if (attackTotal >= rollRequestDC) {
        setCollectedRollDice(combinedDice);
        setLocalRollRequest(rollRequestFollowUpOnSuccess);
        setDiceValues((rollRequestFollowUpOnSuccess.dice.length > 0 ? rollRequestFollowUpOnSuccess.dice : ["d20"]).map(() => 1));
        setDiceDetectMessage("");
        setDiceDetectError(null);
        setDiceSubmitPending(false);
        return;
      }
    }
    setDiceSubmitPending(true);
    setRollResolutionPending(true);
    setPopupVisible(false);
    setPromptError(null);
    try {
      await apiPost<GMResponse>("/api/gm/respond", {
        session_id: liveSession.id,
        player_input: playerInput,
        language: "de",
        dice_roll: {
          dice: combinedDice,
          total: rollComputation?.total,
          summary: rollComputation?.breakdown,
          confidence: 1,
          timestamp: new Date().toISOString(),
        },
      });
      const response = await fetch(`${apiBaseUrl}/api/sessions/${liveSession.id}`, { cache: "no-store" });
      if (response.ok) {
        const updated = (await response.json()) as Session;
        setLiveSession(updated);
      }
      setDiceDetectMessage("");
      setDiceDetectError(null);
      setLocalRollRequest(null);
      setCollectedRollDice([]);
    } catch (error) {
      setRollResolutionPending(false);
      setPromptError(error instanceof Error ? error.message : "Wurfergebnis konnte nicht gesendet werden.");
    } finally {
      setDiceSubmitPending(false);
    }
  }

  async function handleSendPrompt() {
    await sendPlayerInput(promptInput, true);
  }

  async function handleFinalizePlayerTurn() {
    const text = (sttTranscript || promptInput).trim();
    if (!text || !liveSession || promptPending || sttPending) {
      return;
    }
    captureBufferRef.current = "";
    captureSilenceChunksRef.current = 0;
    setConversationPhase("idle");
    await sendPlayerInput(text, true);
    setSTTTranscript("");
  }

  async function handleQuickContinue() {
    if (!liveSession || promptPending) {
      return;
    }
    await sendPlayerInput("Weiter.", false);
  }

  function handleStopPlayback() {
    ttsRef.current?.pause();
    ambientRef.current?.pause();
    setIsSpeaking(false);
  }

  async function handleRetrySpeech() {
    if (!ttsUrl || !enableState.audio) {
      return;
    }
    setTTSError(null);
    currentTTSKeyRef.current = "";
    try {
      const response = await fetch(`${ttsUrl}&retry=${Date.now()}`, { cache: "no-store" });
      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || `${response.status}`);
      }
      const blob = await response.blob();
      if (ttsObjectUrlRef.current) {
        URL.revokeObjectURL(ttsObjectUrlRef.current);
      }
      const objectUrl = URL.createObjectURL(blob);
      ttsObjectUrlRef.current = objectUrl;
      if (ttsRef.current) {
        ttsRef.current.src = objectUrl;
        await ttsRef.current.play();
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : "Sprachausgabe konnte nicht erneut gestartet werden.";
      if (!isIgnorableTTSErrorMessage(message)) {
        setTTSError(message);
      }
    }
  }

  function handlePausePlayback() {
    ttsRef.current?.pause();
    ambientRef.current?.pause();
    setIsSpeaking(false);
  }

  function adjustVolume(delta: number) {
    setPlaybackVolume((current) => Math.max(0, Math.min(1.2, Number((current + delta).toFixed(2)))));
  }

  function handleToggleAudioMute() {
    setIsAudioMuted((current) => !current);
  }

  function handleToggleMicMute() {
    setIsMicMuted((current) => !current);
  }

  async function handleToggleFullscreen() {
    try {
      if (document.fullscreenElement) {
        await document.exitFullscreen();
        setEnableState((current) => ({ ...current, fullscreen: false }));
        return;
      }
      await document.documentElement.requestFullscreen?.();
      setEnableState((current) => ({ ...current, fullscreen: Boolean(document.fullscreenElement || true) }));
    } catch {
      setEnableState((current) => ({ ...current, fullscreen: Boolean(document.fullscreenElement) }));
    }
  }

  async function handleStartRecording() {
    if (!enableState.mic || isRecording || sttPending) {
      return;
    }
    setSTTError(null);
    setSTTTranscript("");
    try {
      const baseStream =
        audioStreamRef.current || (await navigator.mediaDevices.getUserMedia({ audio: true }));
      audioStreamRef.current = baseStream;
      const mimeType = pickRecordingMimeType();
      const recorder = mimeType ? new MediaRecorder(baseStream, { mimeType }) : new MediaRecorder(baseStream);
      recordedChunksRef.current = [];
      recorder.ondataavailable = (event) => {
        if (event.data && event.data.size > 0) {
          recordedChunksRef.current.push(event.data);
        }
      };
      recorder.onstop = async () => {
        setIsRecording(false);
        const blob = new Blob(recordedChunksRef.current, { type: recorder.mimeType || "audio/webm" });
        if (blob.size === 0) {
          setSTTError("Es wurde keine Sprachaufnahme erkannt.");
          return;
        }
        setSTTPending(true);
        try {
          const transcript = await uploadSTTBlob(blob, recorder.mimeType.includes("ogg") ? "speech.ogg" : recorder.mimeType.includes("mp4") ? "speech.mp4" : "speech.webm", liveSession?.language);
          setSTTTranscript(transcript);
          setPromptInput(transcript);
        } catch (error) {
          setSTTError(error instanceof Error ? error.message : "Sprachaufnahme konnte nicht transkribiert werden.");
        } finally {
          setSTTPending(false);
        }
      };
      mediaRecorderRef.current = recorder;
      recorder.start();
      setIsRecording(true);
    } catch (error) {
      setSTTError(error instanceof Error ? error.message : "Mikrofon konnte nicht gestartet werden.");
    }
  }

  function handleStopRecording() {
    if (mediaRecorderRef.current && mediaRecorderRef.current.state !== "inactive") {
      mediaRecorderRef.current.stop();
    }
  }

  const stageTitle =
    effectiveVisualMode === "rules_reference"
      ? String(visualPayload.document_name || referencedDocument?.name || "Regelwerk")
      : effectiveVisualMode === "dice_capture"
      ? hasRollRequest
        ? rollRequestLabel || "Probe ausführen"
        : "Würfelkamera"
      : rollResolutionPending
      ? "Wurf wird aufgelöst"
      : String(visualPayload.title || liveSession?.name || "AI DM Bühne");

  const stageText =
    effectiveVisualMode === "rules_reference"
      ? String(visualPayload.excerpt || "Kein Auszug geladen.")
      : effectiveVisualMode === "dice_capture" && hasRollRequest
      ? [
          rollRequestInstructions,
          rollRequestReason,
          rollRequestDice.length ? `Würfel: ${rollRequestDice.join(", ")}` : "",
          rollRequestDC !== null && Number.isFinite(rollRequestDC) && !rollRequestHideDC ? `SG ${rollRequestDC}` : "",
        ]
          .filter(Boolean)
          .join(" ")
      : rollResolutionPending
      ? "Der bestätigte Wurf wird gerade an den Spielleiter übergeben."
      : summarizeStageText(String(visualPayload.scene || narrationText));

  if (!enableState.board) {
    return (
      <main className="player-screen player-screen--boot">
        <div className="player-screen__backdrop" />
        <section className="boot-screen">
          <p className="eyebrow">AI DM Visual Board</p>
          <h1>{liveSession?.name || "Session wird vorbereitet"}</h1>
          <p>
            Aktiviere das Board einmal bewusst, damit Browser Audio, Mikrofon, Kamera und optional Fullscreen freigeben können.
          </p>
          <button className="studio-button boot-screen__button" onClick={handleActivateBoard} type="button">
            Board aktivieren
          </button>
          <div className="boot-screen__list">
            <span><Volume2 size={16} /> Audio freigeben</span>
			<span>Die Erzählerstimme ist KI-generiert / AI-generated.</span>
            <span><Mic size={16} /> Mikrofon freigeben</span>
            <span><Camera size={16} /> Kamera freigeben</span>
            <span><Maximize size={16} /> Fullscreen optional</span>
          </div>
        </section>
      </main>
    );
  }

  return (
    <main className="player-screen player-screen--live">
      <div className="player-screen__backdrop" />

      <header className="player-topbar">
        <div className="player-topbar__meta">
          <p className="eyebrow">AI DM Visual Board</p>
          <strong>{liveSession?.name || "Keine Live-Session"}</strong>
        </div>
        <div className="player-topbar__chips">
          <span aria-label={`Verbindung ${liveSession ? "aktiv" : "inaktiv"}`} className={`player-chip player-chip--icon ${liveSession ? "is-active" : ""}`} title={`Verbindung ${liveSession ? "aktiv" : "inaktiv"}`}>
            <Wifi size={15} />
          </span>
          <div className="player-chip-group">
            <button
              aria-label={`Audio ${enableState.audio ? (isAudioMuted ? "stumm" : "aktiv") : "aus"}`}
              className={`player-chip player-chip--icon ${enableState.audio && !isAudioMuted ? "is-active" : ""}`}
              onClick={handleToggleAudioMute}
              title={`Audio ${enableState.audio ? (isAudioMuted ? "stumm" : "aktiv") : "aus"}`}
              type="button"
            >
              {isAudioMuted ? <VolumeX size={15} /> : <Volume2 size={15} />}
            </button>
            <div className="player-chip-group__hover">
              <button aria-label="Leiser" className="player-chip-group__mini" onClick={() => adjustVolume(-0.12)} type="button">
                <VolumeX size={13} />
              </button>
              <span className="player-chip-group__value">{Math.round(playbackVolume * 100)}%</span>
              <button aria-label="Lauter" className="player-chip-group__mini" onClick={() => adjustVolume(0.12)} type="button">
                <Volume2 size={13} />
              </button>
            </div>
          </div>
          <button
            aria-label={`Mikrofon ${enableState.mic ? (isMicMuted ? "stumm" : "aktiv") : "aus"}`}
            className={`player-chip player-chip--icon ${enableState.mic && !isMicMuted ? "is-active" : ""}`}
            onClick={handleToggleMicMute}
            title={`Mikrofon ${enableState.mic ? (isMicMuted ? "stumm" : "aktiv") : "aus"}`}
            type="button"
          >
            {isMicMuted ? <MicOff size={15} /> : <Mic size={15} />}
          </button>
          <span aria-label={`Kamera ${enableState.cam ? "aktiv" : "aus"}`} className={`player-chip player-chip--icon ${enableState.cam ? "is-active" : ""}`} title={`Kamera ${enableState.cam ? "aktiv" : "aus"}`}>
            <Camera size={15} />
          </span>
          <button
            aria-label={`Fullscreen ${enableState.fullscreen ? "an" : "aus"}`}
            className={`player-chip player-chip--icon ${enableState.fullscreen ? "is-active" : ""}`}
            onClick={handleToggleFullscreen}
            title={`Fullscreen ${enableState.fullscreen ? "an" : "aus"}`}
            type="button"
          >
            {enableState.fullscreen ? <Minimize size={15} /> : <Maximize size={15} />}
          </button>
        </div>
      </header>

      <section className="player-stage">
        <div className="player-stage__frame">
          <div className="player-stage__meta">
            <span>{stageTitle}</span>
          </div>
          {effectiveVisualMode === "scene" && referencedAsset ? (
            <img alt={referencedAsset.name} className="player-stage__image player-stage__image--map" src={assetUrl} />
          ) : effectiveVisualMode === "rules_reference" ? (
            <div className="player-stage__status">
              <FileText size={22} />
              <div>
                <strong>Regelreferenz bereit</strong>
                <p>Der AI DM hat eine Regelwerksansicht vorbereitet. Öffne das Overlay für die große Ansicht.</p>
              </div>
              <button className="studio-button" onClick={() => setPopupVisible(true)} type="button">
                Regelwerk öffnen
              </button>
            </div>
          ) : effectiveVisualMode === "combat" && referencedAsset ? (
            <div className="player-stage__status">
              <strong>Visuelles Asset bereit</strong>
              <p>Der AI DM zeigt gerade ein Bild oder Handout. Öffne das Overlay für die große Ansicht.</p>
              <button className="studio-button" onClick={() => setPopupVisible(true)} type="button">
                Bild öffnen
              </button>
            </div>
          ) : effectiveVisualMode === "dice_capture" ? (
            <div className="player-stage__status">
              <Camera size={22} />
              <div>
                <strong>{hasRollRequest ? stageTitle : "Würfelkamera bereit"}</strong>
                <p>
                  {hasRollRequest
                    ? stageText || "Lege die geforderten Würfel in die Kamera und würfle jetzt."
                    : "Bei einem Wurf wird die Kamera als Overlay geöffnet, damit der Wurf im Fokus bleibt."}
                </p>
                {hasRollRequest ? (
                  <div className="player-stage__roll-request">
                    {rollRequestDice.length ? <span>Würfel: {rollRequestDice.join(", ")}</span> : null}
                    {rollRequestAbility ? <span>Attribut: {rollRequestAbility}</span> : null}
                    {rollRequestSkill ? <span>Fertigkeit: {rollRequestSkill}</span> : null}
                    {rollRequestDC !== null && Number.isFinite(rollRequestDC) && !rollRequestHideDC ? <span>SG {rollRequestDC}</span> : null}
                    {rollRequestType ? <span>Typ: {rollRequestType}</span> : null}
                  </div>
                ) : null}
              </div>
              <button className="studio-button" disabled={!enableState.cam} onClick={() => setPopupVisible(true)} type="button">
                Kamera öffnen
              </button>
            </div>
          ) : (
            <div className="player-stage__scene">
              <div className="player-stage__session-box">
                <div className="player-stage__session-meta">
                  <button className="player-chip" onClick={() => setMetaPopup("rules")} type="button">
                    {liveSession ? `${liveSession.ruleset_work} ${liveSession.ruleset_version}` : "Regelwerk"}
                  </button>
                  <span className="player-session-meta__text">
                    Spieler {readyPlayers}/{Math.max(joinedPlayers, liveSession?.target_player_count || 0)}
                  </span>
                  <span className="player-session-meta__text">
                    Regelwerke {sessionRulebookCount}
                  </span>
                </div>
                <div className="player-session-adventure">
                  <button className="player-chip" onClick={() => setMetaPopup("adventure")} type="button">
                    {sessionAdventure ? sessionAdventure.name : "Abenteuer"}
                  </button>
                  <p>{sessionAdventure?.description || "Für diese Session ist noch kein Abenteuer mit Beschreibung hinterlegt."}</p>
                </div>
              </div>
            </div>
          )}
        </div>
      </section>

      {popupVisible ? (
        <div className="player-popup">
          <div className="player-popup__backdrop" onClick={() => setPopupVisible(false)} />
          <div className="player-popup__panel">
            <div className="player-popup__header">
              <strong>{stageTitle}</strong>
              <button className="studio-button studio-button--ghost" onClick={() => setPopupVisible(false)} type="button">
                Schließen
              </button>
            </div>
            <div className="player-popup__body">
              {effectiveVisualMode === "combat" && referencedAsset ? (
                <img alt={referencedAsset.name} className="player-popup__image" src={assetUrl} />
              ) : effectiveVisualMode === "dice_capture" ? (
                <div className="player-popup__dice">
                  {hasRollRequest ? (
                    <div className="player-popup__dice-callout">
                      <strong>{stageTitle}</strong>
                      <p>{stageText || "Würfle jetzt und halte die Würfel klar in die Kamera."}</p>
                      <div className="player-stage__roll-request">
                        {normalizedRollDice.map((die, index) => (
                          <label className="roll-die-input" key={`session-die-${index}`}>
                            <span>{die}</span>
                            <input
                              min={1}
                              onChange={(event) => handleDiceValueChange(index, Number(event.target.value))}
                              type="number"
                              value={diceValues[index] ?? 1}
                            />
                          </label>
                        ))}
                      </div>
                      {rollComputation?.breakdown ? <p>{rollComputation.breakdown}</p> : null}
                      {diceDetectMessage ? <p>{diceDetectMessage}</p> : null}
                      {diceDetectError ? <p className="error-copy">{diceDetectError}</p> : null}
                      <div className="button-row">
                        <button
                          className="studio-button studio-button--ghost"
                          disabled={!enableState.cam || diceDetectPending}
                          onClick={() => void handleDetectDiceRoll()}
                          type="button"
                        >
                          {diceDetectPending ? "Auswertung läuft..." : "Kamera auswerten"}
                        </button>
                        <button
                          className="studio-button"
                          disabled={diceSubmitPending}
                          onClick={() => void handleSubmitDiceRoll()}
                          type="button"
                        >
                          {diceSubmitPending ? "Übermittle Wurf..." : "Wurf bestätigen"}
                        </button>
                      </div>
                    </div>
                  ) : null}
                  {enableState.cam ? (
                    <>
                      <video autoPlay className="player-popup__video" muted playsInline ref={videoRef} />
                      <canvas hidden ref={diceCanvasRef} />
                    </>
                  ) : (
                    <div className="player-popup__document">
                      <Camera size={24} />
                      <div>
                        <strong>Kamera nicht aktiv</strong>
                        <p>Die Würfelkamera braucht eine aktive Browser-Kamera. Aktiviere das Board mit Kamerazugriff und öffne dann den Dialog erneut.</p>
                      </div>
                    </div>
                  )}
                </div>
              ) : effectiveVisualMode === "rules_reference" ? (
                <div className="player-popup__document">
                  <FileText size={24} />
                  <div>
                    <strong>{stageTitle}</strong>
                    <p>{stageText}</p>
                  </div>
                </div>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}

      {metaPopup ? (
        <div className="player-popup">
          <div className="player-popup__backdrop" onClick={() => setMetaPopup(null)} />
          <div className="player-popup__panel player-popup__panel--meta">
            <div className="player-popup__header">
              <strong>{metaPopup === "rules" ? "Regelwerk" : "Abenteuer"}</strong>
              <button className="studio-button studio-button--ghost" onClick={() => setMetaPopup(null)} type="button">
                Schließen
              </button>
            </div>
            <div className="player-popup__document">
              {metaPopup === "rules" ? (
                rulesDocument ? (
                  <>
                    <strong>{rulesDocument.name}</strong>
                    <p>Scrollbare Regelwerksansicht für die aktuell gewählte Session.</p>
                    <div className="player-popup__embed-wrap">
                      <iframe className="player-popup__embed" src={rulesFileUrl} title={rulesDocument.name} />
                    </div>
                  </>
                ) : (
                  <>
                    <strong>{`${liveSession?.ruleset_work || "—"} ${liveSession?.ruleset_version || ""}`}</strong>
                    <p>Für diese Session wurde noch kein passendes Regelwerkdokument gefunden.</p>
                  </>
                )
              ) : (
                <>
                  <strong>{sessionAdventure?.name || "Kein Abenteuer gewählt"}</strong>
                  <p>{sessionAdventure?.description || "Für diese Session ist noch keine Abenteuerbeschreibung hinterlegt."}</p>
                </>
              )}
            </div>
          </div>
        </div>
      ) : null}

      <section className="player-overlay">
        <article className="player-overlay__story">
          <p className="eyebrow">AI DM sagt</p>
          <p className="player-overlay__narration">{narrationText}</p>
          <div className="player-overlay__transcript">
            <strong>Spielerantwort</strong>
            <p>{sttTranscript || promptInput || `Warte auf "${WAKE_PHRASE}".`}</p>
          </div>
        </article>

        <div className="player-overlay__side">
          <article className={`voice-console voice-console--${voiceConsoleState.theme}`}>
            <div className="voice-console__header">
              <div>
                <p className="eyebrow">AI DM Voice Console</p>
                <h2>{voiceConsoleState.title}</h2>
				<p className="player-audio-note">KI-generierte Stimme / AI-generated voice</p>
              </div>
              <div className="voice-console__badge">{voiceConsoleState.mode}</div>
            </div>
            <div className="voice-console__scanner" aria-hidden="true">
              <div className="voice-console__side voice-console__side--left">
                <button className="voice-console__key voice-console__key--amber" disabled={isRecording || sttPending || !enableState.mic} onClick={handleStartRecording} type="button">Listen</button>
                <button className="voice-console__key voice-console__key--amber" onClick={() => setPromptInput(sttTranscript || promptInput)} type="button">Reply</button>
                <button className="voice-console__key voice-console__key--red" onClick={handleStopPlayback} type="button">Stop</button>
              </div>
              <div className="voice-console__center">
              <div className="voice-console__bars">
                <span className="voice-console__bar voice-console__bar--left" />
                <span className="voice-console__bar voice-console__bar--center" />
                <span className="voice-console__bar voice-console__bar--right" />
              </div>
              <div className="voice-console__transport voice-console__transport--column">
                  <button className="voice-console__action voice-console__action--amber" onClick={handlePausePlayback} type="button"><Pause size={14} />Pause</button>
                  <button className="voice-console__action voice-console__action--red" onClick={handleStopPlayback} type="button"><Square size={14} />Stop</button>
                  <button className="voice-console__action voice-console__action--amber" disabled={promptPending || !liveSession} onClick={handleQuickContinue} type="button"><SkipForward size={14} />Weiter</button>
                  <button className="voice-console__action voice-console__action--amber" disabled={promptPending || sttPending || !liveSession || !(sttTranscript || promptInput).trim()} onClick={handleFinalizePlayerTurn} type="button"><Send size={14} />Antwort abschließen</button>
              </div>
              <div className="voice-console__mini">
                <button aria-label="Lauter" className="voice-console__mini-button" onClick={() => adjustVolume(0.12)} type="button"><Volume2 size={14} /></button>
                <button aria-label="Leiser" className="voice-console__mini-button" onClick={() => adjustVolume(-0.12)} type="button"><VolumeX size={14} /></button>
              </div>
            </div>
            <div className="voice-console__side voice-console__side--right">
              <button className="voice-console__key voice-console__key--amber" onClick={handleRetrySpeech} type="button">Retry</button>
              <button className="voice-console__key voice-console__key--amber" disabled={promptPending || !liveSession} onClick={handleQuickContinue} type="button">Weiter</button>
            </div>
          </div>
        </article>

          <article className="player-overlay__composer">
            <p className="eyebrow">Manuelle Eingabe</p>
            <div className="player-composer__row">
              <button className="studio-button studio-button--ghost" disabled={isRecording || sttPending || !enableState.mic} onClick={isRecording ? handleStopRecording : handleStartRecording} type="button">
                <Mic size={16} />
                {isRecording ? "Aufnahme stoppen" : "Sprache aufnehmen"}
              </button>
              <textarea onChange={(event) => setPromptInput(event.target.value)} placeholder={`Beschreibe eine Aktion oder sage später "${WAKE_PHRASE}". Abschließen mit "${FINISH_PHRASE}".`} rows={3} value={promptInput} />
            </div>
            <div className="player-composer__feedback">
              {sttError ? <p className="error-copy">{sttError}</p> : null}
              {promptError ? <p className="error-copy">{promptError}</p> : null}
              {ambientError ? <p className="player-audio-note">{ambientError}</p> : null}
            </div>
            <div className="button-row button-row--end">
              <button className="studio-button studio-button--ghost" disabled={promptPending || sttPending || !liveSession || !(sttTranscript || promptInput).trim()} onClick={handleFinalizePlayerTurn} type="button">
                Antwort abschließen
              </button>
              <button className="studio-button" disabled={promptPending || sttPending || !liveSession || !promptInput.trim()} onClick={handleSendPrompt} type="button">
                Senden
              </button>
            </div>
          </article>
        </div>
      </section>

      {ambientActive ? <audio ref={ambientRef} hidden loop src={ambientUrl} /> : null}
      {enableState.audio ? (
        <audio
          ref={ttsRef}
          hidden
          onPlay={() => {
            setTTSError(null);
            setIsSpeaking(true);
          }}
          onEnded={() => setIsSpeaking(false)}
          onPause={() => setIsSpeaking(false)}
          onError={() => {
            setIsSpeaking(false);
            setTTSError(null);
          }}
        />
      ) : null}
    </main>
  );
}
