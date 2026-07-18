"use client";

import { useEffect, useMemo, useRef, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { Bot, Check, Copy, MessageSquare, Mic, MicOff, PenSquare, Plus, Trash2, User, Volume2 } from "lucide-react";
import { PageIntro, Panel, StatCard, StatusPill } from "../studio-primitives";
import { useNotifications } from "../notifications-provider";
import {
  apiDelete,
  apiUploadRaw,
  applyCharacterBuilderPatch,
  createCharacter,
  detectDiceFromImage,
  finishCharacterBuilder,
  resolveAbilityScores,
  sendCharacterBuilderMessage,
  splitMetadataList,
  startCharacterBuilder,
  updatePlayerSlotCharacter,
  validateAbilityAssignment,
  apiBaseUrl,
  type Adventure,
  type Campaign,
  type Character,
  type CharacterBuilderMessage,
  type CharacterBuilderPatch,
  type Document,
} from "../../lib/api";

type CharactersScreenProps = {
  characters: Character[];
  campaigns: Campaign[];
  adventures: Adventure[];
  documents: Document[];
  initialBuilderSeed?: {
    portalToken: string;
    returnPath: string;
    playerSlotId: string;
    playerName: string;
    campaignId: string;
    rulesetWork: string;
    rulesetVersion: string;
  } | null;
};

type AbilityKey = "strength" | "dexterity" | "constitution" | "intelligence" | "wisdom" | "charisma";
type BuilderStep = "start" | "chat";
type AbilityMethod = "standard" | "point_buy" | "rolled";
type SheetTab = "overview" | "abilities" | "combat" | "magic" | "personality" | "gear";
const STT_TARGET_SAMPLE_RATE = 32000;
const BUILDER_MAX_RECORDING_MS = 10000;

type SheetFormState = {
  name: string;
  player_name: string;
  class_and_level: string;
  race: string;
  background: string;
  alignment: string;
  languages: string;
  armor_class: string;
  hit_point_max: string;
  current_hit_points: string;
  temporary_hit_points: string;
  speed: string;
  proficiency_bonus: string;
  inspiration: string;
  experience_points: string;
  hit_dice: string;
  concept: string;
  builder_stage: string;
  personality_traits: string;
  ideals: string;
  bonds: string;
  flaws: string;
  backstory: string;
  age: string;
  size: string;
  weight: string;
  eyes: string;
  skin: string;
  hair: string;
  allies: string;
  senses: string;
  tools_and_proficiencies: string;
  weapon_notes: string;
  starting_money: string;
  current_money: string;
  current_inventory: string;
  level_up_available: string;
  combat_overview: string;
  combat_attacks: string;
  skill_proficiencies: string;
  saving_throw_proficiencies: string;
  starting_equipment: string;
  spells: string;
  spell_save_dc: string;
  spell_attack_bonus: string;
  spell_attacks: string;
  spell_notes: string;
};
const guidedRollCount = 7;
const keptRollCount = 6;

type RulesetGroup = {
  key: string;
  work: string;
  version: string;
  documents: Document[];
};

type RosterCharacter = Character & {
  rulesetLabel: string;
  statusLabel: string;
  concept: string;
  selectedDocumentNames: string[];
};

type ParsedTableRow = {
  columns: string[];
  description: string;
};

const abilityOrder: AbilityKey[] = ["strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"];
const skillDefinitions: Array<{ key: string; label: string; ability: AbilityKey }> = [
  { key: "acrobatics", label: "Akrobatik", ability: "dexterity" },
  { key: "arcana", label: "Arkane Kunde", ability: "intelligence" },
  { key: "athletics", label: "Athletik", ability: "strength" },
  { key: "performance", label: "Auftreten", ability: "charisma" },
  { key: "intimidation", label: "Einschüchtern", ability: "charisma" },
  { key: "sleight_of_hand", label: "Fingerfertigkeit", ability: "dexterity" },
  { key: "history", label: "Geschichte", ability: "intelligence" },
  { key: "medicine", label: "Heilkunde", ability: "wisdom" },
  { key: "stealth", label: "Heimlichkeit", ability: "dexterity" },
  { key: "animal_handling", label: "Mit Tieren umgehen", ability: "wisdom" },
  { key: "insight", label: "Motiv erkennen", ability: "wisdom" },
  { key: "investigation", label: "Nachforschungen", ability: "intelligence" },
  { key: "nature", label: "Naturkunde", ability: "intelligence" },
  { key: "religion", label: "Religion", ability: "intelligence" },
  { key: "deception", label: "Täuschen", ability: "charisma" },
  { key: "survival", label: "Überlebenskunst", ability: "wisdom" },
  { key: "persuasion", label: "Überzeugen", ability: "charisma" },
  { key: "perception", label: "Wahrnehmung", ability: "wisdom" },
];
const skillAliases: Record<string, string> = {
  akrobatik: "acrobatics",
  "arkane kunde": "arcana",
  athletik: "athletics",
  auftreten: "performance",
  "einschüchtern": "intimidation",
  einschuchtern: "intimidation",
  fingerfertigkeit: "sleight_of_hand",
  geschichte: "history",
  heilkunde: "medicine",
  heimlichkeit: "stealth",
  "mit tieren umgehen": "animal_handling",
  "motiv erkennen": "insight",
  nachforschungen: "investigation",
  naturkunde: "nature",
  religion: "religion",
  täuschen: "deception",
  taeuschen: "deception",
  "überlebenskunst": "survival",
  ueberlebenskunst: "survival",
  überleben: "survival",
  ueberleben: "survival",
  überzeugen: "persuasion",
  ueberzeugen: "persuasion",
  wahrnehmung: "perception",
};
const abilityLabels: Record<AbilityKey, string> = {
  strength: "STÄRKE",
  dexterity: "GESCHICKLICHKEIT",
  constitution: "KONSTITUTION",
  intelligence: "INTELLIGENZ",
  wisdom: "WEISHEIT",
  charisma: "CHARISMA",
};

function deriveRuleset(metadata: Record<string, unknown>) {
  const work = String(metadata.ruleset_work ?? "");
  const version = String(metadata.ruleset_version ?? "");
  if (work || version) {
    return { work: work || "Unassigned", version: version || "default" };
  }
  const fallback = splitMetadataList(metadata.ruleset_keys)[0] ?? "";
  if (fallback.includes(":")) {
    const [fallbackWork, fallbackVersion] = fallback.split(":");
    return { work: fallbackWork || "Unassigned", version: fallbackVersion || "default" };
  }
  return { work: "Unassigned", version: "default" };
}

function normalizeSkillToken(value: string) {
  const token = value.trim().toLowerCase();
  return skillAliases[token] ?? token;
}

function createEmptyAssignment() {
  return {
    strength: 0,
    dexterity: 0,
    constitution: 0,
    intelligence: 0,
    wisdom: 0,
    charisma: 0,
  };
}

function normalizeAssignment(values: Record<string, number>): Record<AbilityKey, number> {
  return {
    strength: values.strength ?? 0,
    dexterity: values.dexterity ?? 0,
    constitution: values.constitution ?? 0,
    intelligence: values.intelligence ?? 0,
    wisdom: values.wisdom ?? 0,
    charisma: values.charisma ?? 0,
  };
}

function normalizeSuggestedAssignment(value: unknown): Record<AbilityKey, number> {
  if (!value || typeof value !== "object") {
    return createEmptyAssignment();
  }
  const raw = value as Record<string, unknown>;
  return {
    strength: Number(raw.strength) || 0,
    dexterity: Number(raw.dexterity) || 0,
    constitution: Number(raw.constitution) || 0,
    intelligence: Number(raw.intelligence) || 0,
    wisdom: Number(raw.wisdom) || 0,
    charisma: Number(raw.charisma) || 0,
  };
}

function parseBuilderMessages(metadata: Record<string, unknown>): CharacterBuilderMessage[] {
  const raw = metadata.builder_messages;
  if (!Array.isArray(raw)) {
    return [];
  }
  return raw
    .map((entry) => {
      if (!entry || typeof entry !== "object") {
        return null;
      }
      const item = entry as Record<string, unknown>;
      const role = String(item.role ?? "").trim();
      const content = String(item.content ?? "").trim();
      const createdAt = String(item.created_at ?? new Date().toISOString());
      if (!role || !content) {
        return null;
      }
      return { role, content, created_at: createdAt };
    })
    .filter((entry): entry is CharacterBuilderMessage => Boolean(entry));
}

function parseRollSets(text: string): number[][] {
  return text
    .split("\n")
    .map((line) =>
      line
        .split(/[,\s]+/)
        .map((chunk) => Number(chunk.trim()))
        .filter((value) => Number.isFinite(value))
    )
    .filter((line) => line.length > 0);
}

function buildBuilderSpeechText(value: string) {
  return value.replace(/\s+/g, " ").trim();
}

type AudioContextConstructor = typeof AudioContext;

function getAudioContextConstructor(): AudioContextConstructor | null {
  return (
    window.AudioContext ||
    (window as typeof window & { webkitAudioContext?: AudioContextConstructor }).webkitAudioContext ||
    null
  );
}

function pickRecordingMimeType(): string {
  const candidates = ["audio/mp4", "audio/ogg;codecs=opus", "audio/webm;codecs=opus"];
  for (const candidate of candidates) {
    if (typeof MediaRecorder !== "undefined" && MediaRecorder.isTypeSupported(candidate)) {
      return candidate;
    }
  }
  return "";
}

function writeAscii(view: DataView, offset: number, value: string) {
  for (let index = 0; index < value.length; index += 1) {
    view.setUint8(offset + index, value.charCodeAt(index));
  }
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

function speakWithBrowserVoice(text: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (typeof window === "undefined" || !("speechSynthesis" in window) || typeof SpeechSynthesisUtterance === "undefined") {
      reject(new Error("Sprachausgabe konnte nicht abgespielt werden."));
      return;
    }
    const utterance = new SpeechSynthesisUtterance(text);
    utterance.lang = "de-DE";
    utterance.rate = 1;
    utterance.pitch = 1;
    utterance.onend = () => resolve();
    utterance.onerror = () => reject(new Error("Sprachausgabe konnte nicht abgespielt werden."));
    window.speechSynthesis.cancel();
    window.speechSynthesis.speak(utterance);
  });
}

function safeString(value: unknown) {
  return typeof value === "string" ? value : "";
}

function metadataListToText(value: unknown) {
  return splitMetadataList(value).join(", ");
}

function abilityModifier(score: number) {
  return Math.floor((score - 10) / 2);
}

function formatModifier(value: number) {
  return value >= 0 ? `+${value}` : `${value}`;
}

function parseNumericText(value: string) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function hasResolvedRollData(metadata: Record<string, unknown>) {
  const resolvedValues = metadata.resolved_values;
  const rolledSets = metadata.rolled_sets;
  const hasResolvedValues = Array.isArray(resolvedValues) && resolvedValues.length >= keptRollCount;
  const hasRolledSets = Array.isArray(rolledSets) && rolledSets.length >= guidedRollCount;
  return hasResolvedValues || hasRolledSets;
}

function ensureRolledSets(text: string): number[][] {
  const parsed = parseRollSets(text)
    .slice(0, guidedRollCount)
    .map((set) => {
      const next = [...set].slice(0, 4);
      while (next.length < 4) {
        next.push(0);
      }
      return next;
    });
  while (parsed.length < guidedRollCount) {
    parsed.push([0, 0, 0, 0]);
  }
  return parsed;
}

function parseCharacterLevel(classAndLevel: string) {
  const match = classAndLevel.match(/level\s*(\d+)|stufe\s*(\d+)|\b(\d+)\b/i);
  const raw = match?.[1] ?? match?.[2] ?? match?.[3];
  const parsed = Number(raw);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 1;
}

function normalizeClassName(classAndLevel: string) {
  const value = classAndLevel.toLowerCase();
  if (value.includes("barbar")) return "barbarian";
  if (value.includes("bard")) return "bard";
  if (value.includes("cleric") || value.includes("kler")) return "cleric";
  if (value.includes("druid") || value.includes("druide")) return "druid";
  if (value.includes("fighter") || value.includes("kämpfer") || value.includes("kaempfer")) return "fighter";
  if (value.includes("monk") || value.includes("mönch") || value.includes("moench")) return "monk";
  if (value.includes("paladin")) return "paladin";
  if (value.includes("ranger") || value.includes("waldläufer") || value.includes("waldlaeufer")) return "ranger";
  if (value.includes("rogue") || value.includes("schurke")) return "rogue";
  if (value.includes("sorcerer") || value.includes("zauberer")) return "sorcerer";
  if (value.includes("warlock") || value.includes("hexenmeister")) return "warlock";
  if (value.includes("wizard") || value.includes("magier")) return "wizard";
  return "";
}

function deriveProficiencyBonus(classAndLevel: string) {
  const level = parseCharacterLevel(classAndLevel);
  return Math.floor((level - 1) / 4) + 2;
}

function deriveHitDie(classAndLevel: string) {
  switch (normalizeClassName(classAndLevel)) {
    case "barbarian":
      return "1d12";
    case "fighter":
    case "paladin":
    case "ranger":
      return "1d10";
    case "bard":
    case "cleric":
    case "druid":
    case "monk":
    case "rogue":
    case "warlock":
      return "1d8";
    case "sorcerer":
    case "wizard":
      return "1d6";
    default:
      return "";
  }
}

function deriveSavingThrowProficiencies(classAndLevel: string) {
  switch (normalizeClassName(classAndLevel)) {
    case "barbarian":
      return "Stärke, Konstitution";
    case "bard":
      return "Geschicklichkeit, Charisma";
    case "cleric":
    case "druid":
    case "warlock":
    case "wizard":
      return "Weisheit, Charisma";
    case "fighter":
      return "Stärke, Konstitution";
    case "monk":
      return "Stärke, Geschicklichkeit";
    case "paladin":
      return "Weisheit, Charisma";
    case "ranger":
      return "Stärke, Geschicklichkeit";
    case "rogue":
      return "Geschicklichkeit, Intelligenz";
    case "sorcerer":
      return "Konstitution, Charisma";
    default:
      return "";
  }
}

function deriveBaseSpeed(race: string) {
  const value = race.toLowerCase();
  if (value.includes("zwerg") || value.includes("dwarf") || value.includes("gnom") || value.includes("gnome") || value.includes("halbling") || value.includes("halfling")) {
    return "25 ft";
  }
  if (
    value.includes("halbelf") ||
    value.includes("half-elf") ||
    value.includes("elf") ||
    value.includes("mensch") ||
    value.includes("human") ||
    value.includes("tiefling") ||
    value.includes("half-orc") ||
    value.includes("halbork") ||
    value.includes("dragonborn")
  ) {
    return "30 ft";
  }
  return "";
}

function deriveArmorClassValue(sheetForm: CharacterSheetCanvasProps["sheetForm"], assignment: Record<AbilityKey, number>) {
  if (sheetForm.armor_class) {
    return sheetForm.armor_class;
  }
  const dexMod = abilityModifier(assignment.dexterity || 10);
  const equipment = `${sheetForm.starting_equipment} ${sheetForm.weapon_notes}`.toLowerCase();
  const spells = sheetForm.spells.toLowerCase();

  let armorBase = 10;
  let dexContribution = dexMod;
  if (equipment.includes("platten") || equipment.includes("plate")) {
    armorBase = 18;
    dexContribution = 0;
  } else if (equipment.includes("kettenpanzer") || equipment.includes("chain mail")) {
    armorBase = 16;
    dexContribution = 0;
  } else if (equipment.includes("schienen") || equipment.includes("splint")) {
    armorBase = 17;
    dexContribution = 0;
  } else if (equipment.includes("halbplatte") || equipment.includes("half plate")) {
    armorBase = 15;
    dexContribution = Math.min(2, dexMod);
  } else if (equipment.includes("brustplatte") || equipment.includes("breastplate")) {
    armorBase = 14;
    dexContribution = Math.min(2, dexMod);
  } else if (equipment.includes("schuppenpanzer") || equipment.includes("scale")) {
    armorBase = 14;
    dexContribution = Math.min(2, dexMod);
  } else if (equipment.includes("kettenhemd") || equipment.includes("chain shirt")) {
    armorBase = 13;
    dexContribution = Math.min(2, dexMod);
  } else if (equipment.includes("beschlagene leder") || equipment.includes("studded leather")) {
    armorBase = 12;
  } else if (equipment.includes("leder") || equipment.includes("lederrüstung") || equipment.includes("lederruestung") || equipment.includes("leather armor")) {
    armorBase = 11;
  }
  if ((spells.includes("mage armor") || spells.includes("magische rüstung") || spells.includes("magische ruestung")) && armorBase < 13) {
    armorBase = 13;
    dexContribution = dexMod;
  }
  let total = armorBase + dexContribution;
  if (equipment.includes("schild") || equipment.includes("shield")) {
    total += 2;
  }
  return String(total);
}

function deriveSpellcastingAbility(classAndLevel: string): AbilityKey | null {
  switch (normalizeClassName(classAndLevel)) {
    case "bard":
    case "paladin":
    case "sorcerer":
    case "warlock":
      return "charisma";
    case "cleric":
    case "druid":
    case "ranger":
      return "wisdom";
    case "wizard":
      return "intelligence";
    default:
      return null;
  }
}

function parseStructuredRows(value: string, expectedColumns: number) {
  const lines = value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);

  const rows: ParsedTableRow[] = [];
  let pendingDescription = "";

  for (const line of lines) {
    const normalized = line.replace(/^beschreibung\s*:\s*/i, "").trim();
    if (/^beschreibung\s*:/i.test(line)) {
      if (rows.length > 0) {
        rows[rows.length - 1].description = normalized;
      } else {
        pendingDescription = normalized;
      }
      continue;
    }

    const columns = line.split("|").map((part) => part.trim());
    if (columns.length >= expectedColumns) {
      rows.push({
        columns: columns.slice(0, expectedColumns),
        description: pendingDescription,
      });
      pendingDescription = "";
      continue;
    }

    if (rows.length > 0) {
      rows[rows.length - 1].description = rows[rows.length - 1].description
        ? `${rows[rows.length - 1].description} ${line}`
        : line;
    } else {
      pendingDescription = pendingDescription ? `${pendingDescription} ${line}` : line;
    }
  }

  return rows;
}

type CharacterSheetCanvasProps = {
  sheetForm: SheetFormState;
  assignment: Record<AbilityKey, number>;
  builderSkillKeys: Set<string>;
  activeTab: SheetTab;
  onTabChange: (tab: SheetTab) => void;
  isEditMode: boolean;
  onFieldChange: (field: keyof SheetFormState, value: string) => void;
  onSave: () => void;
  saveDisabled: boolean;
  onToggleEditMode: () => void;
};

function CharacterSheetCanvas({
  sheetForm,
  assignment,
  builderSkillKeys,
  activeTab,
  onTabChange,
  isEditMode,
  onFieldChange,
  onSave,
  saveDisabled,
  onToggleEditMode,
}: CharacterSheetCanvasProps) {
  const proficiency = parseNumericText(sheetForm.proficiency_bonus) ?? deriveProficiencyBonus(sheetForm.class_and_level);
  const passivePerception = 10 + abilityModifier(assignment.wisdom || 10) + (builderSkillKeys.has("perception") ? proficiency : 0);
  const derivedArmorClass = deriveArmorClassValue(sheetForm, assignment);
  const derivedSpeed = sheetForm.speed || deriveBaseSpeed(sheetForm.race) || "—";
  const derivedHitDie = sheetForm.hit_dice || deriveHitDie(sheetForm.class_and_level) || "—";
  const derivedSavingThrows = sheetForm.saving_throw_proficiencies || deriveSavingThrowProficiencies(sheetForm.class_and_level) || "—";
  const spellcastingAbility = deriveSpellcastingAbility(sheetForm.class_and_level);
  const spellcastingModifier = spellcastingAbility ? abilityModifier(assignment[spellcastingAbility] || 10) : null;
  const derivedSpellSaveDC =
    spellcastingModifier === null ? "—" : String(8 + proficiency + spellcastingModifier);
  const derivedSpellAttackBonus =
    spellcastingModifier === null ? "—" : formatModifier(proficiency + spellcastingModifier);
  const combatRows = parseStructuredRows(sheetForm.combat_attacks, 7);
  const spellAttackRows = parseStructuredRows(sheetForm.spell_attacks, 7);
  const renderInlineField = (
    field: keyof SheetFormState,
    options?: { multiline?: boolean; displayFallback?: string; inputPlaceholder?: string }
  ) => {
    if (!isEditMode) {
      return sheetForm[field] || options?.displayFallback || "—";
    }
    if (options?.multiline) {
      return (
        <textarea
          className="sheet-inline-input sheet-inline-input--multiline"
          onChange={(event) => onFieldChange(field, event.target.value)}
          placeholder={options?.inputPlaceholder}
          rows={4}
          value={sheetForm[field]}
        />
      );
    }
    return (
      <input
        className="sheet-inline-input"
        onChange={(event) => onFieldChange(field, event.target.value)}
        placeholder={options?.inputPlaceholder}
        value={sheetForm[field]}
      />
    );
  };

  return (
    <section className="sheet-canvas">
      <div className="sheet-tabs">
        {([
          ["overview", "Überblick"],
          ["abilities", "Attribute & Fertigkeiten"],
          ["combat", "Kampf"],
          ["magic", "Magie"],
          ["personality", "Persönlichkeit"],
          ["gear", "Ausrüstung & Magie"],
        ] as Array<[SheetTab, string]>).map(([tab, label]) => (
          <button
            className={`sheet-tab${activeTab === tab ? " is-active" : ""}`}
            key={tab}
            onClick={() => onTabChange(tab)}
            type="button"
          >
            {label}
          </button>
        ))}
      </div>
      <div className="sheet-canvas__toolbar">
        <button className={`studio-button ${isEditMode ? "studio-button--primary" : "studio-button--ghost"}`} onClick={onToggleEditMode} type="button">
          <PenSquare size={16} />
          {isEditMode ? "Bearbeitung beenden" : "Bearbeiten"}
        </button>
        {isEditMode ? (
          <button className="studio-button studio-button--ghost" disabled={saveDisabled} onClick={onSave} type="button">
            <Check size={16} />
            Speichern
          </button>
        ) : null}
      </div>
      <header className="sheet-canvas__header">
        <div className="sheet-canvas__name">
          <span>CHARAKTERNAME</span>
          <strong>{renderInlineField("name", { displayFallback: "Neuer Charakter" })}</strong>
        </div>
        <div className="sheet-canvas__identity">
          <article>
            <span>Klasse & Stufe</span>
            <strong>{renderInlineField("class_and_level")}</strong>
          </article>
          <article>
            <span>Volk</span>
            <strong>{renderInlineField("race")}</strong>
          </article>
          <article>
            <span>Hintergrund</span>
            <strong>{renderInlineField("background")}</strong>
          </article>
          <article>
            <span>Gesinnung</span>
            <strong>{renderInlineField("alignment")}</strong>
          </article>
          <article>
            <span>Spieler</span>
            <strong>{renderInlineField("player_name")}</strong>
          </article>
        </div>
      </header>

      {activeTab === "overview" ? (
        <div className="sheet-tab-panel">
          <div className="sheet-tab-grid sheet-tab-grid--overview">
            <section className="sheet-box sheet-box--story">
              <div className="sheet-box__title-row">
                <strong>Konzept & Geschichte</strong>
                <span>der aktuelle Stand aus dem Builder</span>
              </div>
              <p>{renderInlineField("concept", { multiline: true, displayFallback: "Noch kein klares Konzept eingetragen." })}</p>
              <p>{renderInlineField("backstory", { multiline: true, displayFallback: "Die Hintergrundgeschichte wächst mit dem Builder-Dialog." })}</p>
            </section>
            <section className="sheet-box">
              <strong>Grunddaten</strong>
              <dl className="sheet-detail-list">
                <div><dt>Spieler</dt><dd>{renderInlineField("player_name")}</dd></div>
                <div><dt>Alter</dt><dd>{renderInlineField("age")}</dd></div>
                <div><dt>Größe</dt><dd>{renderInlineField("size")}</dd></div>
                <div><dt>Gewicht</dt><dd>{renderInlineField("weight")}</dd></div>
                <div><dt>Augen</dt><dd>{renderInlineField("eyes")}</dd></div>
                <div><dt>Haut</dt><dd>{renderInlineField("skin")}</dd></div>
                <div><dt>Haare</dt><dd>{renderInlineField("hair")}</dd></div>
              </dl>
            </section>
          </div>
        </div>
      ) : null}

      {activeTab === "abilities" ? (
        <div className="sheet-tab-panel">
          <div className="sheet-tab-grid sheet-tab-grid--main-abilities">
            {abilityOrder.map((ability) => {
              const score = assignment[ability] || 0;
              return (
                <article className="sheet-ability" key={`canvas-${ability}`}>
                  <span>{abilityLabels[ability]}</span>
                  <strong>{score || "—"}</strong>
                  <em>{score ? formatModifier(abilityModifier(score)) : "—"}</em>
                </article>
              );
            })}
          </div>

          <div className="sheet-tab-grid sheet-tab-grid--ability-status">
            <section className="sheet-box">
              <strong>Attribut-Status</strong>
              <dl className="sheet-detail-list">
                <div><dt>Übungsbonus</dt><dd>{formatModifier(proficiency)}</dd></div>
                <div><dt>Inspiration</dt><dd>{renderInlineField("inspiration")}</dd></div>
                <div><dt>Passive Wahrnehmung</dt><dd>{passivePerception}</dd></div>
                <div><dt>Rettungswurf-Profizienzen</dt><dd>{derivedSavingThrows}</dd></div>
              </dl>
            </section>
            <section className="sheet-box">
              <strong>Abgeleitete Werte</strong>
              <dl className="sheet-detail-list">
                <div><dt>Rüstungsklasse</dt><dd>{derivedArmorClass}</dd></div>
                <div><dt>Initiative</dt><dd>{assignment.dexterity ? formatModifier(abilityModifier(assignment.dexterity)) : "—"}</dd></div>
                <div><dt>Bewegungsrate</dt><dd>{derivedSpeed}</dd></div>
                <div><dt>Trefferwürfel</dt><dd>{derivedHitDie}</dd></div>
              </dl>
            </section>
          </div>

          <section className="sheet-box">
            <div className="sheet-box__title-row">
              <strong>Fertigkeiten</strong>
              <span>darunter die Skills mit berechneten Modifikatoren</span>
            </div>
            <div className="sheet-skills">
              {skillDefinitions.map((skill) => {
                const score = assignment[skill.ability] || 10;
                const isProficient = builderSkillKeys.has(skill.key);
                const total = abilityModifier(score) + (isProficient ? proficiency : 0);
                return (
                  <div className={`sheet-skill${isProficient ? " is-proficient" : ""}`} key={`canvas-skill-${skill.key}`}>
                    <div className="sheet-skill__label">
                      <span>{skill.label}</span>
                    </div>
                    <em>{skill.ability.slice(0, 3).toUpperCase()}</em>
                    <strong>{formatModifier(total)}</strong>
                  </div>
                );
              })}
            </div>
          </section>
        </div>
      ) : null}

      {activeTab === "combat" ? (
        <div className="sheet-tab-panel">
          <section className="sheet-box sheet-box--combat">
            <article>
              <span>Rüstungsklasse</span>
              <strong>{renderInlineField("armor_class")}</strong>
            </article>
            <article>
              <span>Initiative</span>
              <strong>{assignment.dexterity ? formatModifier(abilityModifier(assignment.dexterity)) : "—"}</strong>
            </article>
            <article>
              <span>Bewegung</span>
              <strong>{renderInlineField("speed", { displayFallback: derivedSpeed })}</strong>
            </article>
            <article>
              <span>TP max</span>
              <strong>{renderInlineField("hit_point_max")}</strong>
            </article>
            <article>
              <span>Aktuelle TP</span>
              <strong>{renderInlineField("current_hit_points", { displayFallback: sheetForm.hit_point_max || "—" })}</strong>
            </article>
            <article>
              <span>Temp. TP</span>
              <strong>{renderInlineField("temporary_hit_points", { displayFallback: "0" })}</strong>
            </article>
            <article>
              <span>Trefferwürfel</span>
              <strong>{renderInlineField("hit_dice", { displayFallback: derivedHitDie })}</strong>
            </article>
            <article>
              <span>Übungsbonus</span>
              <strong>{formatModifier(proficiency)}</strong>
            </article>
            <article>
              <span>Inspiration</span>
              <strong>{sheetForm.inspiration || "—"}</strong>
            </article>
            <article>
              <span>Passive Wahrnehmung</span>
              <strong>{passivePerception}</strong>
            </article>
          </section>
          <section className="sheet-box">
            <div className="sheet-box__title-row">
              <strong>Kampfausrüstung</strong>
              <span>was im Kampf direkt relevant ist</span>
            </div>
            {renderInlineField("combat_overview", {
              multiline: true,
              inputPlaceholder: "Rüstung, Schild, Waffen, relevante Gegenstände und Kampfstil.",
            })}
          </section>
          <section className="sheet-box">
            <div className="sheet-box__title-row">
              <strong>Angriffe</strong>
              <span>ANGRIFF · ÜB · ATTR. · REICHWEITE · BONUS · SCHADEN · SCHADENTYP</span>
            </div>
            <div className="sheet-table-card">
              {isEditMode ? (
                <div className="sheet-table__body-copy">
                  {renderInlineField("combat_attacks", {
                    multiline: true,
                    inputPlaceholder:
                      "Beispiel:\nLangbogen | +2 | GES | 150/600 ft | +5 | 1d8+3 | Stich\nBeschreibung: Standard-Fernkampfwaffe.",
                  })}
                </div>
              ) : combatRows.length > 0 ? (
                <div className="sheet-table-list">
                  <div className="sheet-table sheet-table--combat">
                    <div className="sheet-table__head">Angriff</div>
                    <div className="sheet-table__head">ÜB</div>
                    <div className="sheet-table__head">Attr.</div>
                    <div className="sheet-table__head">Reichweite</div>
                    <div className="sheet-table__head">Bonus</div>
                    <div className="sheet-table__head">Schaden</div>
                    <div className="sheet-table__head">Schadentyp</div>
                  </div>
                  {combatRows.map((row, index) => (
                    <div className="sheet-table-entry" key={`combat-row-${index}`}>
                      <div className="sheet-table sheet-table--combat sheet-table--values">
                        {row.columns.map((column, columnIndex) => (
                          <div className="sheet-table__cell" key={`combat-${index}-${columnIndex}`}>{column || "—"}</div>
                        ))}
                      </div>
                      <div className="sheet-table-entry__description">
                        <span>Beschreibung</span>
                        <p>{row.description || "—"}</p>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="sheet-table__body-copy">
                  <p className="muted-copy">Noch keine Angriffe eingetragen.</p>
                </div>
              )}
            </div>
          </section>
        </div>
      ) : null}

      {activeTab === "magic" ? (
        <div className="sheet-tab-panel">
          <section className="sheet-box">
            <div className="sheet-box__title-row">
              <strong>Magie-Status</strong>
              <span>Sp, SG und Zauberangriffsbonus</span>
            </div>
            <div className="sheet-tab-grid sheet-tab-grid--ability-status">
              <dl className="sheet-detail-list">
                <div><dt>Sp</dt><dd>{renderInlineField("spells", { multiline: true, inputPlaceholder: "Zauberplätze, Slots oder verfügbare Magie." })}</dd></div>
              </dl>
              <dl className="sheet-detail-list">
                <div><dt>Zauberrettungswurf-SG</dt><dd>{renderInlineField("spell_save_dc", { displayFallback: derivedSpellSaveDC })}</dd></div>
                <div><dt>Zauberangriffsbonus</dt><dd>{renderInlineField("spell_attack_bonus", { displayFallback: derivedSpellAttackBonus })}</dd></div>
              </dl>
            </div>
          </section>
          <section className="sheet-box">
            <div className="sheet-box__title-row">
              <strong>Zauberangriffe</strong>
              <span>Stufe · Angriff · Attr. · Reichweite · Bonus · Schaden · Schadentyp</span>
            </div>
            <div className="sheet-table-card">
              {isEditMode ? (
                <div className="sheet-table__body-copy">
                  {renderInlineField("spell_attacks", {
                    multiline: true,
                    inputPlaceholder:
                      "Beispiel:\nZaubertrick | Feuerstrahl | CHA | 120 ft | +5 | 1d10 | Feuer\nBeschreibung: Fernzauberangriff.",
                  })}
                </div>
              ) : spellAttackRows.length > 0 ? (
                <div className="sheet-table-list">
                  <div className="sheet-table sheet-table--combat">
                    <div className="sheet-table__head">Stufe</div>
                    <div className="sheet-table__head">Angriff</div>
                    <div className="sheet-table__head">Attr.</div>
                    <div className="sheet-table__head">Reichweite</div>
                    <div className="sheet-table__head">Bonus</div>
                    <div className="sheet-table__head">Schaden</div>
                    <div className="sheet-table__head">Schadentyp</div>
                  </div>
                  {spellAttackRows.map((row, index) => (
                    <div className="sheet-table-entry" key={`spell-row-${index}`}>
                      <div className="sheet-table sheet-table--combat sheet-table--values">
                        {row.columns.map((column, columnIndex) => (
                          <div className="sheet-table__cell" key={`spell-${index}-${columnIndex}`}>{column || "—"}</div>
                        ))}
                      </div>
                      <div className="sheet-table-entry__description">
                        <span>Beschreibung</span>
                        <p>{row.description || "—"}</p>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="sheet-table__body-copy">
                  <p className="muted-copy">Noch keine Zauberangriffe eingetragen.</p>
                </div>
              )}
            </div>
          </section>
          <section className="sheet-box">
            <div className="sheet-box__title-row">
              <strong>Weitere Zauber</strong>
              <span>kleine Beschreibung für andere Zauber</span>
            </div>
            {renderInlineField("spell_notes", {
              multiline: true,
              inputPlaceholder: "Wirkung, Konzentration, Dauer, taktische Hinweise oder Kurzbeschreibung.",
            })}
          </section>
        </div>
      ) : null}

      {activeTab === "personality" ? (
        <div className="sheet-tab-panel">
          <div className="sheet-tab-grid sheet-tab-grid--overview">
            <section className="sheet-box">
              <strong>Persönlichkeit</strong>
              <dl className="sheet-detail-list">
                <div><dt>Merkmale</dt><dd>{renderInlineField("personality_traits", { multiline: true })}</dd></div>
                <div><dt>Ideale</dt><dd>{renderInlineField("ideals", { multiline: true })}</dd></div>
                <div><dt>Bindungen</dt><dd>{renderInlineField("bonds", { multiline: true })}</dd></div>
                <div><dt>Makel</dt><dd>{renderInlineField("flaws", { multiline: true })}</dd></div>
              </dl>
            </section>
            <section className="sheet-box">
              <strong>Körper & Auftreten</strong>
              <dl className="sheet-detail-list">
                <div><dt>Alter</dt><dd>{renderInlineField("age")}</dd></div>
                <div><dt>Größe</dt><dd>{renderInlineField("size")}</dd></div>
                <div><dt>Gewicht</dt><dd>{renderInlineField("weight")}</dd></div>
                <div><dt>Augen</dt><dd>{renderInlineField("eyes")}</dd></div>
                <div><dt>Haut</dt><dd>{renderInlineField("skin")}</dd></div>
                <div><dt>Haare</dt><dd>{renderInlineField("hair")}</dd></div>
                <div><dt>Sprachen</dt><dd>{renderInlineField("languages", { multiline: true, displayFallback: "—" })}</dd></div>
                <div><dt>Sinne</dt><dd>{renderInlineField("senses")}</dd></div>
              </dl>
            </section>
          </div>
        </div>
      ) : null}

      {activeTab === "gear" ? (
        <div className="sheet-tab-panel">
          <section className="sheet-box">
            <strong>Ausrüstung</strong>
            <dl className="sheet-detail-list">
              <div><dt>Startausrüstung</dt><dd>{renderInlineField("starting_equipment", { multiline: true })}</dd></div>
              <div><dt>Geld</dt><dd>{renderInlineField("starting_money", { inputPlaceholder: "z. B. 15 gp, 7 sp" })}</dd></div>
              <div><dt>Aktuelles Geld</dt><dd>{renderInlineField("current_money", { inputPlaceholder: "z. B. 22 gp, 4 sp" })}</dd></div>
              <div><dt>Aktuelles Inventar</dt><dd>{renderInlineField("current_inventory", { multiline: true, inputPlaceholder: "Gefundene, gekaufte oder gehandelte Gegenstände." })}</dd></div>
              <div><dt>Level-Up bereit</dt><dd>{renderInlineField("level_up_available")}</dd></div>
              <div><dt>Werkzeuge</dt><dd>{renderInlineField("tools_and_proficiencies", { multiline: true })}</dd></div>
              <div><dt>Waffen-Notizen</dt><dd>{renderInlineField("weapon_notes", { multiline: true })}</dd></div>
              <div><dt>Verbündete</dt><dd>{renderInlineField("allies", { multiline: true })}</dd></div>
              <div><dt>EP</dt><dd>{renderInlineField("experience_points")}</dd></div>
            </dl>
          </section>
        </div>
      ) : null}
    </section>
  );
}

export function CharactersScreen({ characters, campaigns, documents, initialBuilderSeed }: CharactersScreenProps) {
  const router = useRouter();
  const { notify } = useNotifications();
  const [rulesetFilter, setRulesetFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");
  const [isBuilderOpen, setIsBuilderOpen] = useState(false);
  const [builderStep, setBuilderStep] = useState<BuilderStep>("start");
  const [builderCharacter, setBuilderCharacter] = useState<Character | null>(null);
  const [builderMessages, setBuilderMessages] = useState<CharacterBuilderMessage[]>([]);
  const [builderInput, setBuilderInput] = useState("");
  const [isBuilderResponding, setIsBuilderResponding] = useState(false);
  const [builderSpeechLoadingKey, setBuilderSpeechLoadingKey] = useState("");
  const [builderSpeechActiveKey, setBuilderSpeechActiveKey] = useState("");
  const [builderSpeechUrl, setBuilderSpeechUrl] = useState("");
  const [builderSpeechError, setBuilderSpeechError] = useState<string | null>(null);
  const [builderReferencePopup, setBuilderReferencePopup] = useState<null | {
    documentId: string;
    documentName: string;
    page: number | null;
  }>(null);
  const [builderSTTError, setBuilderSTTError] = useState<string | null>(null);
  const [builderSTTStatus, setBuilderSTTStatus] = useState("");
  const [isBuilderRecording, setIsBuilderRecording] = useState(false);
  const [isBuilderTranscribing, setIsBuilderTranscribing] = useState(false);
  const [builderPlayerName, setBuilderPlayerName] = useState("");
  const [selectedRulesetKey, setSelectedRulesetKey] = useState("");
  const [selectedDocumentIds, setSelectedDocumentIds] = useState<string[]>([]);
  const [selectedCampaignId, setSelectedCampaignId] = useState("");
  const [isRollModalOpen, setIsRollModalOpen] = useState(false);
  const [rollCameraStatus, setRollCameraStatus] = useState<"idle" | "ready" | "error" | "unsupported">("idle");
  const [rollCameraMessage, setRollCameraMessage] = useState("Kamera noch nicht gestartet.");
  const [isRollCapturing, setIsRollCapturing] = useState(false);
  const [currentRollIndex, setCurrentRollIndex] = useState(0);
  const [confirmedRolls, setConfirmedRolls] = useState<boolean[]>(() => Array.from({ length: guidedRollCount }, () => false));
  const [activeSheetTab, setActiveSheetTab] = useState<SheetTab>("overview");
  const [isSheetEditMode, setIsSheetEditMode] = useState(false);
  const [sheetForm, setSheetForm] = useState<SheetFormState>({
    name: "",
    player_name: "",
    class_and_level: "",
    race: "",
    background: "",
    alignment: "",
    languages: "",
    armor_class: "",
    hit_point_max: "",
    current_hit_points: "",
    temporary_hit_points: "",
    speed: "",
    proficiency_bonus: "",
    inspiration: "",
    experience_points: "",
    hit_dice: "",
    concept: "",
    builder_stage: "",
    personality_traits: "",
    ideals: "",
    bonds: "",
    flaws: "",
    backstory: "",
    age: "",
    size: "",
    weight: "",
    eyes: "",
    skin: "",
    hair: "",
    allies: "",
    senses: "",
    tools_and_proficiencies: "",
    weapon_notes: "",
    starting_money: "",
    current_money: "",
    current_inventory: "",
    level_up_available: "",
    combat_overview: "",
    combat_attacks: "",
    skill_proficiencies: "",
    saving_throw_proficiencies: "",
    starting_equipment: "",
    spells: "",
    spell_save_dc: "",
    spell_attack_bonus: "",
    spell_attacks: "",
    spell_notes: "",
  });
  const [abilityMethod, setAbilityMethod] = useState<AbilityMethod>("standard");
  const [rolledSetsText, setRolledSetsText] = useState("6, 6, 5, 2\n6, 5, 4, 3\n6, 5, 5, 1\n6, 4, 4, 3\n5, 5, 4, 2\n6, 4, 3, 2\n5, 4, 3, 1");
  const [resolvedValues, setResolvedValues] = useState<number[]>([]);
  const [assignment, setAssignment] = useState<Record<AbilityKey, number>>(createEmptyAssignment());
  const [ruleSummary, setRuleSummary] = useState("");
  const [validationMessage, setValidationMessage] = useState("");
  const [assignmentConfirmed, setAssignmentConfirmed] = useState(false);
  const [isPending, startTransition] = useTransition();
  const transcriptEndRef = useRef<HTMLDivElement | null>(null);
  const builderSpeechObjectUrlRef = useRef<string | null>(null);
  const builderSpeechAudioRef = useRef<HTMLAudioElement | null>(null);
  const builderSpeechRequestKeyRef = useRef("");
  const lastAutoPlayedBuilderSpeechKeyRef = useRef("");
  const builderRecorderRef = useRef<MediaRecorder | null>(null);
  const builderStopTimerRef = useRef<number | null>(null);
  const builderAudioStreamRef = useRef<MediaStream | null>(null);
  const builderRecordedChunksRef = useRef<Blob[]>([]);
  const rollVideoRef = useRef<HTMLVideoElement | null>(null);
  const rollCanvasRef = useRef<HTMLCanvasElement | null>(null);
  const rollStreamRef = useRef<MediaStream | null>(null);
  const seedAppliedRef = useRef(false);

  const rulesDocuments = useMemo(
    () =>
      documents.filter(
        (document) => document.type === "rules" && !String(document.metadata?.kind ?? "").endsWith("_guide")
      ),
    [documents]
  );
  const rulesetGroups = useMemo<RulesetGroup[]>(() => {
    const groups = new Map<string, RulesetGroup>();
    for (const document of rulesDocuments) {
      const ruleset = deriveRuleset(document.metadata);
      const key = `${ruleset.work}::${ruleset.version}`;
      const current = groups.get(key) ?? { key, work: ruleset.work, version: ruleset.version, documents: [] };
      current.documents.push(document);
      groups.set(key, current);
    }
    return [...groups.values()].sort((left, right) => left.key.localeCompare(right.key));
  }, [rulesDocuments]);

  const roster = useMemo<RosterCharacter[]>(
    () =>
      characters.map((character) => {
        const ruleset = deriveRuleset(character.metadata);
        return {
          ...character,
          rulesetLabel: `${ruleset.work} ${ruleset.version}`,
          statusLabel: safeString(character.metadata.builder_status) || "ready",
          concept: safeString(character.metadata.concept),
          selectedDocumentNames: splitMetadataList(character.metadata.selected_document_names),
        };
      }),
    [characters]
  );

  const availableRulesetFilters = useMemo(
    () => ["all", ...new Set(roster.map((character) => character.rulesetLabel).sort((left, right) => left.localeCompare(right)))],
    [roster]
  );

  const filteredCharacters = useMemo(
    () =>
      roster.filter((character) => {
        if (rulesetFilter !== "all" && character.rulesetLabel !== rulesetFilter) {
          return false;
        }
        if (statusFilter !== "all" && character.statusLabel !== statusFilter) {
          return false;
        }
        return true;
      }),
    [roster, rulesetFilter, statusFilter]
  );

  const selectedRulesetGroup = useMemo(
    () => rulesetGroups.find((group) => group.key === selectedRulesetKey) ?? rulesetGroups[0] ?? null,
    [rulesetGroups, selectedRulesetKey]
  );
  const builderReferenceDocument = useMemo(() => {
    if (!builderReferencePopup) {
      return null;
    }
    return documents.find((document) => document.id === builderReferencePopup.documentId) ?? null;
  }, [builderReferencePopup, documents]);
  const builderReferenceFileUrl = builderReferenceDocument
    ? `${apiBaseUrl}/api/documents/${builderReferenceDocument.id}/file${
        builderReferencePopup?.page ? `#page=${builderReferencePopup.page}` : ""
      }`
    : "";

  const builderSkillKeys = useMemo(
    () => new Set(splitMetadataList(sheetForm.skill_proficiencies).map(normalizeSkillToken)),
    [sheetForm.skill_proficiencies]
  );

  useEffect(() => {
    if (!selectedRulesetKey && rulesetGroups[0]) {
      setSelectedRulesetKey(rulesetGroups[0].key);
    }
  }, [rulesetGroups, selectedRulesetKey]);

  useEffect(() => {
    if (!initialBuilderSeed || seedAppliedRef.current || rulesetGroups.length === 0) {
      return;
    }
    seedAppliedRef.current = true;
    openNewBuilder();
    setBuilderPlayerName(initialBuilderSeed.playerName);
    setSelectedCampaignId(initialBuilderSeed.campaignId);
    const matchingRuleset = rulesetGroups.find(
      (group) => group.work === initialBuilderSeed.rulesetWork && group.version === initialBuilderSeed.rulesetVersion
    );
    if (matchingRuleset) {
      setSelectedRulesetKey(matchingRuleset.key);
    }
    notify({
      title: "Character Builder",
      message: "Builder wurde aus dem Player Portal vorbefuellt geoeffnet.",
      tone: "info",
    });
  }, [initialBuilderSeed, notify, rulesetGroups]);

  useEffect(() => {
    if (!selectedRulesetGroup) {
      setSelectedDocumentIds([]);
      return;
    }
    setSelectedDocumentIds((current) => {
      const validIds = current.filter((id) => selectedRulesetGroup.documents.some((document) => document.id === id));
      if (validIds.length > 0) {
        return validIds;
      }
      return selectedRulesetGroup.documents.map((document) => document.id);
    });
  }, [selectedRulesetGroup]);

  useEffect(() => {
    transcriptEndRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [builderMessages, isBuilderResponding]);

  useEffect(() => {
    if (builderStep !== "chat" || isBuilderResponding || builderSpeechLoadingKey) {
      return;
    }
    const latestAssistantMessage = [...builderMessages]
      .reverse()
      .find((message) => message.role === "assistant" && message.content.trim());
    if (!latestAssistantMessage) {
      return;
    }
    const speechKey = `${latestAssistantMessage.created_at}:${latestAssistantMessage.content}`;
    if (lastAutoPlayedBuilderSpeechKeyRef.current === speechKey) {
      return;
    }
    lastAutoPlayedBuilderSpeechKeyRef.current = speechKey;
    void handlePlayBuilderSpeech(latestAssistantMessage);
  }, [builderMessages, builderSpeechLoadingKey, builderStep, isBuilderResponding]);

  useEffect(() => {
    return () => {
      builderRecorderRef.current?.stop();
      builderAudioStreamRef.current?.getTracks().forEach((track) => track.stop());
      if (builderSpeechObjectUrlRef.current) {
        URL.revokeObjectURL(builderSpeechObjectUrlRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (
      isBuilderOpen &&
      builderStep === "chat" &&
      safeString(builderCharacter?.metadata.creation_method) === "rolled" &&
      safeString(builderCharacter?.metadata.builder_stage) === "ability_scores" &&
      builderCharacter?.metadata &&
      !hasResolvedRollData(builderCharacter.metadata)
    ) {
      setIsRollModalOpen(true);
    }
  }, [builderCharacter, builderStep, isBuilderOpen]);

  useEffect(() => {
    if (!isRollModalOpen) {
      if (rollStreamRef.current) {
        rollStreamRef.current.getTracks().forEach((track) => track.stop());
        rollStreamRef.current = null;
      }
      if (rollVideoRef.current) {
        rollVideoRef.current.srcObject = null;
      }
      setRollCameraStatus("idle");
      setRollCameraMessage("Kamera noch nicht gestartet.");
    }
  }, [isRollModalOpen]);

  function syncSheetForm(character: Character) {
    setSheetForm({
      name: character.name,
      player_name: character.player_name,
      class_and_level: character.class_and_level,
      race: character.race,
      background: character.background,
      alignment: character.alignment,
      languages: character.languages.join(", "),
      armor_class: character.armor_class != null ? String(character.armor_class) : "",
      hit_point_max: character.hit_point_max != null ? String(character.hit_point_max) : "",
      current_hit_points: safeString(character.metadata.current_hit_points),
      temporary_hit_points: safeString(character.metadata.temporary_hit_points),
      speed: character.speed,
      proficiency_bonus: character.proficiency_bonus,
      inspiration: safeString(character.metadata.inspiration),
      experience_points: safeString(character.metadata.experience_points),
      hit_dice: safeString(character.metadata.hit_dice),
      concept: safeString(character.metadata.concept),
      builder_stage: safeString(character.metadata.builder_stage),
      personality_traits: safeString(character.metadata.personality_traits),
      ideals: safeString(character.metadata.ideals),
      bonds: safeString(character.metadata.bonds),
      flaws: safeString(character.metadata.flaws),
      backstory: safeString(character.metadata.backstory),
      age: safeString(character.metadata.age),
      size: safeString(character.metadata.size),
      weight: safeString(character.metadata.weight),
      eyes: safeString(character.metadata.eyes),
      skin: safeString(character.metadata.skin),
      hair: safeString(character.metadata.hair),
      allies: safeString(character.metadata.allies),
      senses: safeString(character.metadata.senses),
      tools_and_proficiencies: metadataListToText(character.metadata.tools_and_proficiencies),
      weapon_notes: metadataListToText(character.metadata.weapon_notes),
      starting_money: safeString(character.metadata.starting_money),
      current_money: safeString(character.metadata.current_money),
      current_inventory: metadataListToText(character.metadata.current_inventory),
      level_up_available: safeString(character.metadata.level_up_available),
      combat_overview: safeString(character.metadata.combat_overview),
      combat_attacks: safeString(character.metadata.combat_attacks),
      skill_proficiencies: metadataListToText(character.metadata.skill_proficiencies),
      saving_throw_proficiencies: metadataListToText(character.metadata.saving_throw_proficiencies),
      starting_equipment: metadataListToText(character.metadata.starting_equipment),
      spells: metadataListToText(character.metadata.spells),
      spell_save_dc: safeString(character.metadata.spell_save_dc),
      spell_attack_bonus: safeString(character.metadata.spell_attack_bonus),
      spell_attacks: safeString(character.metadata.spell_attacks),
      spell_notes: safeString(character.metadata.spell_notes),
    });
    setAbilityMethod((safeString(character.metadata.creation_method) as AbilityMethod) || "standard");
    const metadataRolledSets = character.metadata.rolled_sets;
    if (Array.isArray(metadataRolledSets) && metadataRolledSets.length > 0) {
      const normalized = metadataRolledSets
        .map((set) => (Array.isArray(set) ? set.map((value) => Number(value) || 0) : []))
        .filter((set) => set.length > 0)
        .map((set) => {
          const next = [...set].slice(0, 4);
          while (next.length < 4) next.push(0);
          return next;
        });
      if (normalized.length > 0) {
        setRolledSetsText(normalized.map((set) => set.join(", ")).join("\n"));
        setConfirmedRolls(
          Array.from({ length: guidedRollCount }, (_, index) => {
            const set = normalized[index];
            return Boolean(set) && set.every((value) => value >= 1 && value <= 6);
          })
        );
      }
    }
    const storedAssignment = normalizeAssignment(character.abilities);
    const hasStoredAbilities = Object.values(storedAssignment).some((value) => value > 0);
    setAssignment(hasStoredAbilities ? storedAssignment : normalizeSuggestedAssignment(character.metadata.suggested_assignment));
  }

  function openNewBuilder() {
    setBuilderCharacter(null);
    setBuilderMessages([]);
    setBuilderInput("");
    setIsBuilderResponding(false);
    setBuilderPlayerName("");
    setSelectedCampaignId("");
    setIsRollModalOpen(false);
    setCurrentRollIndex(0);
    setConfirmedRolls(Array.from({ length: guidedRollCount }, () => false));
    setBuilderStep("start");
    setActiveSheetTab("overview");
    setResolvedValues([]);
    setAssignment(createEmptyAssignment());
    setAssignmentConfirmed(false);
    setValidationMessage("");
    setRuleSummary("");
    setIsBuilderOpen(true);
  }

  function openBuilderForCharacter(character: RosterCharacter) {
    setBuilderCharacter(character);
    setBuilderMessages(parseBuilderMessages(character.metadata));
    setBuilderInput("");
    setIsBuilderResponding(false);
    setBuilderReferencePopup(null);
    setBuilderPlayerName(character.player_name);
    setIsRollModalOpen(false);
    setCurrentRollIndex(0);
    setConfirmedRolls(Array.from({ length: guidedRollCount }, () => false));
    syncSheetForm(character);
    setBuilderStep("chat");
    setActiveSheetTab("overview");
    setIsBuilderOpen(true);
  }

  function closeBuilder() {
    setIsBuilderOpen(false);
    setBuilderStep("start");
    setBuilderInput("");
    setIsBuilderResponding(false);
    setBuilderReferencePopup(null);
    setIsRollModalOpen(false);
    setCurrentRollIndex(0);
    setConfirmedRolls(Array.from({ length: guidedRollCount }, () => false));
    setValidationMessage("");
    setRuleSummary("");
  }

  function currentRollSets() {
    return ensureRolledSets(rolledSetsText);
  }

  function updateRolledSet(rowIndex: number, dieIndex: number, value: number) {
    const sets = currentRollSets();
    sets[rowIndex][dieIndex] = value;
    setRolledSetsText(sets.map((set) => set.join(", ")).join("\n"));
  }

  async function handleStartRollCamera() {
    if (typeof navigator === "undefined" || !navigator.mediaDevices?.getUserMedia) {
      setRollCameraStatus("unsupported");
      setRollCameraMessage("Dieser Browser unterstützt keinen Kamerazugriff.");
      return;
    }
    try {
      if (rollStreamRef.current) {
        rollStreamRef.current.getTracks().forEach((track) => track.stop());
      }
      const stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: false });
      rollStreamRef.current = stream;
      if (rollVideoRef.current) {
        rollVideoRef.current.srcObject = stream;
      }
      setRollCameraStatus("ready");
      setRollCameraMessage("Kamera aktiv. Jetzt genau 4d6 für den aktuellen Wurf ins Bild halten.");
    } catch (error) {
      setRollCameraStatus("error");
      setRollCameraMessage(error instanceof Error ? error.message : "Kamera konnte nicht gestartet werden.");
    }
  }

  function captureRollFrame() {
    const video = rollVideoRef.current;
    const canvas = rollCanvasRef.current;
    if (!video || !canvas || video.videoWidth === 0 || video.videoHeight === 0) {
      return null;
    }
    const width = video.videoWidth;
    const height = video.videoHeight;
    const cropWidth = Math.round(width * 0.86);
    const cropHeight = Math.round(height * 0.86);
    const cropX = Math.max(0, Math.round((width - cropWidth) / 2));
    const cropY = Math.max(0, Math.round((height - cropHeight) / 2));
    const targetWidth = 1280;
    const targetHeight = Math.round((cropHeight / cropWidth) * targetWidth);
    canvas.width = targetWidth;
    canvas.height = targetHeight;
    const context = canvas.getContext("2d");
    if (!context) {
      return null;
    }
    context.imageSmoothingEnabled = true;
    context.imageSmoothingQuality = "high";
    context.drawImage(video, cropX, cropY, cropWidth, cropHeight, 0, 0, canvas.width, canvas.height);
    return canvas.toDataURL("image/jpeg", 0.92);
  }

  async function handleDetectCurrentRoll() {
    if (rollCameraStatus !== "ready") {
      setRollCameraMessage("Bitte zuerst die Kamera für den Würfelschritt starten.");
      return;
    }
    const imageDataUrl = captureRollFrame();
    if (!imageDataUrl) {
      setRollCameraMessage("Es konnte gerade kein Kamerabild gelesen werden.");
      return;
    }
    setIsRollCapturing(true);
    setRollCameraMessage(`Werte für Wurf ${currentRollIndex + 1} werden ausgewertet...`);
    try {
      const response = await detectDiceFromImage({ image_data_url: imageDataUrl, language: "de" });
      const detected = response.dice
        .filter((die) => die.type.toLowerCase() === "d6")
        .map((die) => die.value)
        .filter((value) => value >= 1 && value <= 6)
        .slice(0, 4);
      if (detected.length === 0) {
        setRollCameraMessage(response.notes || "Keine klaren d6 erkannt. Bitte neu legen und nochmal auswerten.");
        return;
      }
      while (detected.length < 4) {
        detected.push(1);
      }
      const sets = currentRollSets();
      sets[currentRollIndex] = detected;
      setRolledSetsText(sets.map((set) => set.join(", ")).join("\n"));
      setConfirmedRolls((current) => current.map((value, index) => (index === currentRollIndex ? false : value)));
      setRollCameraMessage(
        `Wurf ${currentRollIndex + 1} erkannt: ${detected.join(", ")}. Bitte kurz prüfen und bei Bedarf korrigieren.`
      );
    } catch (error) {
      setRollCameraMessage(error instanceof Error ? error.message : "Kameraauswertung fehlgeschlagen.");
    } finally {
      setIsRollCapturing(false);
    }
  }

  function handleRollSetChange(rowIndex: number, dieIndex: number, value: number) {
    updateRolledSet(rowIndex, dieIndex, value);
    setConfirmedRolls((current) => current.map((entry, index) => (index === rowIndex ? false : entry)));
  }

  function finalizeRollSequence(sets: number[][]) {
    if (!builderCharacter) {
      return;
    }
    setIsRollModalOpen(false);
    setIsBuilderResponding(true);
    setRollCameraMessage("Alle sechs Würfe sind bestätigt. Die KI übernimmt jetzt die Werte und macht weiter.");
    startTransition(async () => {
      try {
        const resolved = await resolveAbilityScores({
          method: "rolled",
          class: sheetForm.class_and_level,
          rolled_sets: sets,
        });
        setResolvedValues(resolved.values);
        setAssignment(normalizeAssignment(resolved.assignment));
        setRuleSummary(resolved.rule_summary);
        setAssignmentConfirmed(false);
        setValidationMessage("Die sechs Würfe wurden übernommen. Die KI geht jetzt mit dir den nächsten Schritt durch.");

        const updatedDraft = await applyCharacterBuilderPatch(builderCharacter.id, {
          patch: {
            metadata: {
              creation_method: "rolled",
              builder_stage: "ability_scores_review",
              rolled_sets: sets,
              resolved_values: resolved.values,
              suggested_assignment: resolved.assignment,
            },
          },
        });
        const followUp = await sendCharacterBuilderMessage(builderCharacter.id, {
          message: `Die sieben 4d6-Würfe sind jetzt abgeschlossen. Die Einzelwürfe waren ${sets
            .map((set, index) => `Wurf ${index + 1}: ${set.join(", ")}`)
            .join(" | ")}. Der schwächste Gesamtwurf wurde automatisch gestrichen, die sechs verwendeten Werte sind ${resolved.values.join(
            ", "
          )}. Bitte gib diese Werte jetzt im Chat sauber aus, erkläre kurz dass der schlechteste Wurf gestrichen wurde, frage ob alles stimmt, und führe dann direkt durch die Verteilung auf Stärke, Geschicklichkeit, Konstitution, Intelligenz, Weisheit und Charisma weiter.`,
        });
        setBuilderCharacter(followUp.character ?? updatedDraft);
        setBuilderMessages(followUp.messages);
        syncSheetForm(followUp.character);
        setAssignment(normalizeAssignment(resolved.assignment));
        setResolvedValues(resolved.values);
        setCurrentRollIndex(0);
        setConfirmedRolls(Array.from({ length: guidedRollCount }, () => false));
        notify({ title: "Dice Step", message: "Alle sieben Würfe wurden verarbeitet und die besten sechs an den Builder übergeben.", tone: "success" });
        router.refresh();
      } catch (error) {
        notify({
          title: "Dice Step",
          message: error instanceof Error ? error.message : "Die gewürfelten Werte konnten nicht an den Builder übergeben werden.",
          tone: "error",
        });
      } finally {
        setIsBuilderResponding(false);
      }
    });
  }

  function handleConfirmCurrentRoll() {
    const sets = currentRollSets();
    const current = sets[currentRollIndex];
    if (current.some((value) => value < 1 || value > 6)) {
      setRollCameraMessage("Bitte zuerst vier gültige d6-Werte für den aktuellen Wurf eintragen oder erkennen lassen.");
      return;
    }
    const nextConfirmed = confirmedRolls.map((entry, index) => (index === currentRollIndex ? true : entry));
    setConfirmedRolls(nextConfirmed);
    const nextIndex = nextConfirmed.findIndex((entry) => !entry);
    if (nextIndex === -1) {
      finalizeRollSequence(sets);
      return;
    }
    setCurrentRollIndex(nextIndex);
    setRollCameraMessage(`Wurf ${currentRollIndex + 1} bestätigt. Jetzt bitte Wurf ${nextIndex + 1} mit 4d6 machen.`);
  }

  function handleStartBuilder() {
    if (!selectedRulesetGroup || selectedDocumentIds.length === 0) {
      notify({ title: "Builder", message: "Bitte mindestens ein Regelbuch auswaehlen.", tone: "warning" });
      return;
    }
    setBuilderReferencePopup(null);
    startTransition(async () => {
      try {
        const response = await startCharacterBuilder({
          campaign_id: selectedCampaignId || undefined,
          ruleset_work: selectedRulesetGroup.work,
          ruleset_version: selectedRulesetGroup.version,
          selected_document_ids: selectedDocumentIds,
          player_name: builderPlayerName || undefined,
        });
        setBuilderCharacter(response.character);
        setBuilderMessages(response.messages);
        syncSheetForm(response.character);
        setBuilderStep("chat");
        notify({ title: "Character Draft", message: "Neuer Draft wurde gestartet.", tone: "success" });
      } catch (error) {
        notify({
          title: "Character Builder",
          message: error instanceof Error ? error.message : "Builder konnte nicht gestartet werden.",
          tone: "error",
        });
      }
    });
  }

  function handleSendBuilderMessage() {
    if (!builderCharacter || !builderInput.trim() || isBuilderResponding) {
      return;
    }
    const outgoingMessage = builderInput.trim();
    const optimisticUserMessage: CharacterBuilderMessage = {
      role: "user",
      content: outgoingMessage,
      created_at: new Date().toISOString(),
    };
    setBuilderMessages((current) => [...current, optimisticUserMessage]);
    setBuilderInput("");
    setIsBuilderResponding(true);

    startTransition(async () => {
      try {
        const response = await sendCharacterBuilderMessage(builderCharacter.id, { message: outgoingMessage });
        setBuilderCharacter(response.character);
        setBuilderMessages(response.messages);
        syncSheetForm(response.character);
        if (response.ui_action === "open_document" && response.ui_payload) {
          const documentId = String(response.ui_payload.document_id ?? "").trim();
          const documentName = String(response.ui_payload.document_name ?? "").trim();
          const pageValue = Number(response.ui_payload.document_page);
          if (documentId && documentName) {
            setBuilderReferencePopup({
              documentId,
              documentName,
              page: Number.isFinite(pageValue) && pageValue > 0 ? pageValue : null,
            });
          } else {
            setBuilderReferencePopup(null);
          }
        } else {
          setBuilderReferencePopup(null);
        }
        if (safeString(response.character.metadata.creation_method) === "rolled") {
          setAbilityMethod("rolled");
        } else if (safeString(response.character.metadata.creation_method) === "point_buy") {
          setAbilityMethod("point_buy");
        } else if (safeString(response.character.metadata.creation_method) === "standard") {
          setAbilityMethod("standard");
        }
        notify({ title: "Builder", message: "Draft wurde mit der KI aktualisiert.", tone: "success" });
      } catch (error) {
        setBuilderMessages((current) => current.filter((message) => message !== optimisticUserMessage));
        setBuilderInput(outgoingMessage);
        notify({
          title: "Builder",
          message: error instanceof Error ? error.message : "Nachricht konnte nicht verarbeitet werden.",
          tone: "error",
        });
      } finally {
        setIsBuilderResponding(false);
      }
    });
  }

  async function handlePlayBuilderSpeech(message: CharacterBuilderMessage) {
    const speechKey = `${message.created_at}:${message.content}`;
    builderSpeechRequestKeyRef.current = speechKey;
    setBuilderSpeechLoadingKey(speechKey);
    setBuilderSpeechError(null);
    try {
      const response = await fetch(
        `${apiBaseUrl}/api/tts-audio?voice=${encodeURIComponent("narrator-default")}&language=de&text=${encodeURIComponent(
          buildBuilderSpeechText(message.content)
        )}`,
        { cache: "no-store" }
      );
      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || `${response.status}`);
      }
      const blob = await response.blob();
      if (builderSpeechObjectUrlRef.current) {
        const currentAudio = builderSpeechAudioRef.current;
        if (currentAudio) {
          currentAudio.pause();
          currentAudio.removeAttribute("src");
          currentAudio.load();
        }
        URL.revokeObjectURL(builderSpeechObjectUrlRef.current);
      }
      const objectUrl = URL.createObjectURL(blob);
      if (builderSpeechRequestKeyRef.current !== speechKey) {
        URL.revokeObjectURL(objectUrl);
        return;
      }
      builderSpeechObjectUrlRef.current = objectUrl;
      setBuilderSpeechUrl(objectUrl);
      setBuilderSpeechActiveKey(speechKey);
      const audio = builderSpeechAudioRef.current;
      if (!audio) {
        throw new Error("Audioelement nicht bereit.");
      }
      audio.src = objectUrl;
      audio.currentTime = 0;
      audio.load();
      try {
        await audio.play();
      } catch (error) {
        if (error instanceof DOMException && error.name === "AbortError") {
          return;
        }
        throw error;
      }
    } catch (error) {
      setBuilderSpeechActiveKey(speechKey);
      setBuilderSpeechUrl("");
      setBuilderSpeechError(error instanceof Error && error.message ? error.message : "Sprachausgabe konnte nicht abgespielt werden.");
    } finally {
      setBuilderSpeechLoadingKey("");
    }
  }

  async function handleToggleBuilderRecording() {
    if (isBuilderRecording) {
      builderRecorderRef.current?.stop();
      return;
    }
    setBuilderSTTError(null);
    setBuilderSTTStatus("Mikrofon wird gestartet...");
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      builderAudioStreamRef.current?.getTracks().forEach((track) => track.stop());
      builderAudioStreamRef.current = stream;
      const mimeType = pickRecordingMimeType();
      const recorder = mimeType ? new MediaRecorder(stream, { mimeType }) : new MediaRecorder(stream);
      builderRecordedChunksRef.current = [];
      recorder.onstart = () => {
        setIsBuilderRecording(true);
        setBuilderSTTStatus("Sprich jetzt. Die Aufnahme endet automatisch nach 12 Sekunden.");
        if (builderStopTimerRef.current) {
          window.clearTimeout(builderStopTimerRef.current);
        }
        builderStopTimerRef.current = window.setTimeout(() => {
          builderStopTimerRef.current = null;
          if (recorder.state !== "inactive") {
            recorder.stop();
          }
        }, BUILDER_MAX_RECORDING_MS);
      };
      recorder.ondataavailable = (event) => {
        if (event.data && event.data.size > 0) {
          builderRecordedChunksRef.current.push(event.data);
        }
      };
      recorder.onerror = () => {
        if (builderStopTimerRef.current) {
          window.clearTimeout(builderStopTimerRef.current);
          builderStopTimerRef.current = null;
        }
        setBuilderSTTError("Sprachaufnahme konnte nicht gestartet werden.");
        setBuilderSTTStatus("");
        setIsBuilderRecording(false);
      };
      recorder.onstop = async () => {
        if (builderStopTimerRef.current) {
          window.clearTimeout(builderStopTimerRef.current);
          builderStopTimerRef.current = null;
        }
        setIsBuilderRecording(false);
        setBuilderSTTStatus("");
        const recordedType = recorder.mimeType || builderRecordedChunksRef.current[0]?.type || "audio/webm";
        const extension = recordedType.includes("ogg") ? "ogg" : recordedType.includes("mp4") ? "mp4" : "webm";
        const blob = new Blob(builderRecordedChunksRef.current, { type: recordedType });
        builderRecordedChunksRef.current = [];
        stream.getTracks().forEach((track) => track.stop());
        builderAudioStreamRef.current = null;
        if (blob.size === 0) {
          setBuilderSTTError("Keine Sprachaufnahme erkannt.");
          return;
        }
        setIsBuilderTranscribing(true);
        setBuilderSTTStatus("Spracherkennung läuft...");
        try {
          const transcript = await uploadSTTBlob(blob, `builder.${extension}`, "de");
          if (!transcript) {
            throw new Error("Keine Sprache erkannt.");
          }
          setBuilderInput((current) => (current.trim() ? `${current.trim()} ${transcript}` : transcript));
          setBuilderSTTStatus("Transkript übernommen.");
        } catch (error) {
          setBuilderSTTError(error instanceof Error && error.message ? error.message : "Spracherkennung fehlgeschlagen.");
          setBuilderSTTStatus("");
        } finally {
          setIsBuilderTranscribing(false);
        }
      };
      builderRecorderRef.current = recorder;
      recorder.start();
    } catch (error) {
      setBuilderSTTError(error instanceof Error && error.message ? error.message : "Mikrofon konnte nicht geöffnet werden.");
      setBuilderSTTStatus("");
    }
  }

  useEffect(() => {
    return () => {
      if (builderStopTimerRef.current) {
        window.clearTimeout(builderStopTimerRef.current);
      }
    };
  }, []);

  function handleApplySheetPatch() {
    if (!builderCharacter) {
      return;
    }
    const patch: CharacterBuilderPatch = {
      name: sheetForm.name,
      player_name: sheetForm.player_name,
      class_and_level: sheetForm.class_and_level,
      race: sheetForm.race,
      background: sheetForm.background,
      alignment: sheetForm.alignment,
      languages: splitMetadataList(sheetForm.languages),
      armor_class: parseNumericText(sheetForm.armor_class),
      hit_point_max: parseNumericText(sheetForm.hit_point_max),
      speed: sheetForm.speed,
      proficiency_bonus: sheetForm.proficiency_bonus,
      metadata: {
        inspiration: sheetForm.inspiration,
        experience_points: sheetForm.experience_points,
        hit_dice: sheetForm.hit_dice,
        concept: sheetForm.concept,
        builder_stage: sheetForm.builder_stage,
        personality_traits: sheetForm.personality_traits,
        ideals: sheetForm.ideals,
        bonds: sheetForm.bonds,
        flaws: sheetForm.flaws,
        backstory: sheetForm.backstory,
        age: sheetForm.age,
        size: sheetForm.size,
        weight: sheetForm.weight,
        eyes: sheetForm.eyes,
        skin: sheetForm.skin,
        hair: sheetForm.hair,
        allies: sheetForm.allies,
        senses: sheetForm.senses,
        current_hit_points: sheetForm.current_hit_points,
        temporary_hit_points: sheetForm.temporary_hit_points,
        tools_and_proficiencies: splitMetadataList(sheetForm.tools_and_proficiencies),
        weapon_notes: splitMetadataList(sheetForm.weapon_notes),
        starting_money: sheetForm.starting_money,
        current_money: sheetForm.current_money,
        current_inventory: splitMetadataList(sheetForm.current_inventory),
        level_up_available: sheetForm.level_up_available,
        combat_overview: sheetForm.combat_overview,
        combat_attacks: sheetForm.combat_attacks,
        skill_proficiencies: splitMetadataList(sheetForm.skill_proficiencies),
        saving_throw_proficiencies: splitMetadataList(sheetForm.saving_throw_proficiencies),
        starting_equipment: splitMetadataList(sheetForm.starting_equipment),
        spells: splitMetadataList(sheetForm.spells),
        spell_save_dc: sheetForm.spell_save_dc,
        spell_attack_bonus: sheetForm.spell_attack_bonus,
        spell_attacks: sheetForm.spell_attacks,
        spell_notes: sheetForm.spell_notes,
      },
    };
    startTransition(async () => {
      try {
        const updated = await applyCharacterBuilderPatch(builderCharacter.id, { patch });
        setBuilderCharacter(updated);
        syncSheetForm(updated);
        setIsSheetEditMode(false);
        notify({ title: "Character Sheet", message: "Sheet-Aenderungen wurden gespeichert.", tone: "success" });
        router.refresh();
      } catch (error) {
        notify({
          title: "Character Sheet",
          message: error instanceof Error ? error.message : "Sheet konnte nicht gespeichert werden.",
          tone: "error",
        });
      }
    });
  }

  function handleResolveAbilities() {
    const payload =
      abilityMethod === "rolled"
        ? {
            method: "rolled" as const,
            class: sheetForm.class_and_level,
            rolled_sets: parseRollSets(rolledSetsText),
          }
        : abilityMethod === "point_buy"
          ? {
              method: "point_buy" as const,
              class: sheetForm.class_and_level,
              point_buy: [15, 14, 13, 12, 10, 8],
            }
          : {
              method: "standard" as const,
              class: sheetForm.class_and_level,
            };

    startTransition(async () => {
      try {
        const response = await resolveAbilityScores(payload);
        setResolvedValues(response.values);
        setAssignment(normalizeAssignment(response.assignment));
        setRuleSummary(response.rule_summary);
        setAssignmentConfirmed(false);
        setValidationMessage("");
      } catch (error) {
        notify({
          title: "Ability Scores",
          message: error instanceof Error ? error.message : "Werte konnten nicht aufgeloest werden.",
          tone: "error",
        });
      }
    });
  }

  function handleValidateAbilities() {
    startTransition(async () => {
      try {
        const response = await validateAbilityAssignment({
          class: sheetForm.class_and_level,
          values: resolvedValues,
          assignment,
        });
        setAssignmentConfirmed(response.valid);
        setValidationMessage(
          response.valid
            ? "Zuordnung bestaetigt. Die Werte koennen jetzt in den Draft uebernommen werden."
            : `Zuordnung muss korrigiert werden. Missing: ${response.missing_abilities.join(", ") || "none"}`
        );
      } catch (error) {
        notify({
          title: "Ability Validation",
          message: error instanceof Error ? error.message : "Zuordnung konnte nicht geprueft werden.",
          tone: "error",
        });
      }
    });
  }

  function handleApplyAbilities() {
    if (!builderCharacter || !assignmentConfirmed) {
      notify({ title: "Ability Scores", message: "Bitte zuerst die Zuordnung bestaetigen.", tone: "warning" });
      return;
    }
    startTransition(async () => {
      try {
        const updated = await applyCharacterBuilderPatch(builderCharacter.id, {
          patch: {
            abilities: assignment,
            metadata: {
              creation_method: abilityMethod,
              resolved_values: resolvedValues,
            },
          },
        });
        const followUp = await sendCharacterBuilderMessage(builderCharacter.id, {
          message: `Die bestätigten Attributwürfe sind ${resolvedValues.join(", ")}. Die Zuordnung lautet STR ${assignment.strength}, DEX ${assignment.dexterity}, CON ${assignment.constitution}, INT ${assignment.intelligence}, WIS ${assignment.wisdom}, CHA ${assignment.charisma}. Bitte prüfe die Werte, frage kurz nach ob alles stimmt und fahre dann mit dem nächsten Schritt der Charaktererstellung fort.`,
        });
        setBuilderCharacter(followUp.character);
        setBuilderMessages(followUp.messages);
        syncSheetForm(followUp.character);
        setIsRollModalOpen(false);
        notify({ title: "Ability Scores", message: "Werte wurden in den Draft uebernommen.", tone: "success" });
        router.refresh();
      } catch (error) {
        notify({
          title: "Ability Scores",
          message: error instanceof Error ? error.message : "Werte konnten nicht gespeichert werden.",
          tone: "error",
        });
      }
    });
  }

  function handleFinishBuilder() {
    if (!builderCharacter) {
      return;
    }
    startTransition(async () => {
      try {
        const updated = await finishCharacterBuilder(builderCharacter.id);
        setBuilderCharacter(updated);
        if (initialBuilderSeed?.playerSlotId) {
          await updatePlayerSlotCharacter(initialBuilderSeed.playerSlotId, { character_id: updated.id });
        }
        notify({ title: "Character Ready", message: "Der Charakter ist jetzt als ready markiert.", tone: "success" });
        if (initialBuilderSeed?.returnPath) {
          router.push(initialBuilderSeed.returnPath);
          return;
        }
        router.refresh();
        closeBuilder();
      } catch (error) {
        notify({
          title: "Character Ready",
          message: error instanceof Error ? error.message : "Builder konnte nicht abgeschlossen werden.",
          tone: "error",
        });
      }
    });
  }

  function handleDuplicateCharacter(character: RosterCharacter) {
    startTransition(async () => {
      try {
        await createCharacter({
          campaign_id: character.campaign_id,
          name: `${character.name} Copy`,
          player_name: character.player_name,
          class_and_level: character.class_and_level,
          background: character.background,
          race: character.race,
          alignment: character.alignment,
          armor_class: character.armor_class,
          speed: character.speed,
          hit_point_max: character.hit_point_max,
          proficiency_bonus: character.proficiency_bonus,
          abilities: character.abilities,
          languages: character.languages,
          features: character.features,
          metadata: {
            ...character.metadata,
            builder_status: "draft",
          },
        });
        notify({ title: "Character Copy", message: "Kopie wurde erstellt.", tone: "success" });
        router.refresh();
      } catch (error) {
        notify({
          title: "Character Copy",
          message: error instanceof Error ? error.message : "Kopie konnte nicht erstellt werden.",
          tone: "error",
        });
      }
    });
  }

  function handleDeleteCharacter(character: RosterCharacter) {
    if (!window.confirm(`Character "${character.name}" wirklich loeschen?`)) {
      return;
    }
    startTransition(async () => {
      try {
        await apiDelete<{ deleted: boolean }>(`/api/characters/${character.id}`);
        notify({ title: "Character Deleted", message: `${character.name} wurde geloescht.`, tone: "success" });
        router.refresh();
      } catch (error) {
        notify({
          title: "Character Deleted",
          message: error instanceof Error ? error.message : "Character konnte nicht geloescht werden.",
          tone: "error",
        });
      }
    });
  }

  return (
    <div className="page-stack">
      <PageIntro
        eyebrow="Characters"
        title="Character roster with AI-guided draft building"
        description="Neue Charaktere starten aus der Übersicht, binden sich an vorhandene Regelwerke und wachsen dann in einem KI-Dialog zu einem live gespeicherten Draft heran."
      />

      <div className="dashboard-grid">
        <Panel title="Roster Overview" description="Der Characters-Bereich startet jetzt als Liste, nicht mehr als Inline-Builder." className="hero-panel">
          <div className="stat-grid">
            <StatCard label="Characters" value={roster.length} />
            <StatCard label="Drafts" value={roster.filter((character) => character.statusLabel === "draft").length} />
            <StatCard label="Ready" value={roster.filter((character) => character.statusLabel === "ready").length} />
            <StatCard label="Rulesets" value={rulesetGroups.length} />
          </div>
          <div className="button-row">
            <button className="studio-button studio-button--primary" onClick={openNewBuilder} type="button">
              <Plus size={16} />
              Neuen Charakter erstellen
            </button>
          </div>
        </Panel>
      </div>

      <Panel title="Character Roster" description="Liste, Filter, Open/Edit/Delete. Der KI-Builder startet nur noch als grosses Modal.">
        <div className="library-toolbar">
          <div className="library-toolbar__group">
            <span className="library-toolbar__label">Regelwerk</span>
            <div className="meta-chip-row">
              {availableRulesetFilters.map((option) => (
                <button
                  className={`filter-chip${rulesetFilter === option ? " is-active" : ""}`}
                  key={option}
                  onClick={() => setRulesetFilter(option)}
                  type="button"
                >
                  {option === "all" ? "Alle" : option}
                </button>
              ))}
            </div>
          </div>
          <div className="library-toolbar__group">
            <span className="library-toolbar__label">Status</span>
            <div className="meta-chip-row">
              {["all", "draft", "ready", "assigned"].map((option) => (
                <button
                  className={`filter-chip${statusFilter === option ? " is-active" : ""}`}
                  key={option}
                  onClick={() => setStatusFilter(option)}
                  type="button"
                >
                  {option === "all" ? "Alle" : option}
                  </button>
              ))}
            </div>
          </div>
        </div>

        <div className="card-grid card-grid--three">
          {filteredCharacters.length === 0 ? <p className="empty-copy">Noch keine Characters fuer den aktuellen Filter.</p> : null}
          {filteredCharacters.map((character) => (
            <article className="media-card media-card--roster" key={character.id}>
              <div className="media-card__cover media-card__cover--character">
                <User size={34} />
              </div>
              <div className="media-card__body">
                <div className="media-card__head">
                  <h3>{character.name}</h3>
                  <div className="meta-chip-row">
                    <StatusPill tone="info">{character.rulesetLabel}</StatusPill>
                    <StatusPill tone={character.statusLabel === "draft" ? "warning" : character.statusLabel === "assigned" ? "ready" : "default"}>
                      {character.statusLabel}
                    </StatusPill>
                  </div>
                </div>
                <p>{character.race || "Unknown ancestry"} · {character.class_and_level || "Role not chosen yet"}</p>
                <div className="character-roster-abilities">
                  {abilityOrder.map((ability) => (
                    <div className="character-roster-ability" key={`${character.id}-${ability}`}>
                      <span>{abilityLabels[ability].slice(0, 3)}</span>
                      <strong>{character.abilities[ability] || "—"}</strong>
                    </div>
                  ))}
                </div>
                {character.concept ? <p className="muted-copy">{character.concept}</p> : <p className="muted-copy">Noch kein Konzept gespeichert.</p>}
                {character.selectedDocumentNames.length > 0 ? (
                  <div className="meta-chip-row">
                    {character.selectedDocumentNames.slice(0, 3).map((name) => (
                      <StatusPill key={name} tone="default">{name}</StatusPill>
                    ))}
                  </div>
                ) : null}
                <div className="button-row">
                  <button
                    className="studio-button studio-button--ghost studio-button--inline"
                    onClick={() => openBuilderForCharacter(character)}
                    type="button"
                  >
                    <PenSquare size={14} />
                    Edit
                  </button>
                  <button
                    className="studio-button studio-button--ghost studio-button--inline"
                    onClick={() => handleDuplicateCharacter(character)}
                    type="button"
                  >
                    <Copy size={14} />
                    Duplicate
                  </button>
                  <button
                    className="studio-button studio-button--danger studio-button--inline"
                    onClick={() => handleDeleteCharacter(character)}
                    type="button"
                  >
                    <Trash2 size={14} />
                    Delete
                  </button>
                </div>
              </div>
            </article>
          ))}
        </div>
      </Panel>

      {isBuilderOpen ? (
        <div className="modal-overlay" onClick={closeBuilder} role="presentation">
          <section
            aria-modal="true"
            className="modal-card modal-card--builder"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div className="modal-card__header">
              <div>
                <p className="eyebrow">Character Builder</p>
                <h2>{builderStep === "start" ? "Neuen Charakter vorbereiten" : "KI-geführter Character Draft"}</h2>
                <p className="muted-copy">
                  {builderStep === "start"
                    ? "Erst Regelwerk und Buchbasis wählen. Danach startet links die KI-Konversation und rechts wächst das Character Sheet mit."
                    : "Links der Builder-Dialog, rechts das live gespeicherte Sheet. Jede bestätigte Entscheidung geht direkt in den Draft."}
                </p>
              </div>
              <button className="studio-button studio-button--ghost studio-button--inline" onClick={closeBuilder} type="button">
                Close
              </button>
            </div>

            {builderStep === "start" ? (
              <div className="page-stack">
                {initialBuilderSeed ? (
                  <div className="story-box story-box--hero">
                    <User size={16} />
                    <div>
                      <strong>Portal-Vorbelegung aktiv</strong>
                      <p>
                        {initialBuilderSeed.playerName} · {initialBuilderSeed.rulesetWork} {initialBuilderSeed.rulesetVersion}
                      </p>
                    </div>
                  </div>
                ) : null}
                <div className="dual-field-grid">
                  <label className="field-stack">
                    <span>Regelwerk</span>
                    <select onChange={(event) => setSelectedRulesetKey(event.target.value)} value={selectedRulesetKey}>
                      {rulesetGroups.map((group) => (
                        <option key={group.key} value={group.key}>
                          {group.work} {group.version}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="field-stack">
                    <span>Kampagne</span>
                    <select onChange={(event) => setSelectedCampaignId(event.target.value)} value={selectedCampaignId}>
                      <option value="">Noch keiner Kampagne zuordnen</option>
                      {campaigns.map((campaign) => (
                        <option key={campaign.id} value={campaign.id}>
                          {campaign.name}
                        </option>
                      ))}
                    </select>
                  </label>
                </div>

                <label className="field-stack">
                  <span>Spielername</span>
                  <input onChange={(event) => setBuilderPlayerName(event.target.value)} placeholder="Optional: Spielername" value={builderPlayerName} />
                </label>

                <Panel title="Buchbasis" description="Die KI priorisiert diese Buecher. Nicht ausgewaehlte Buecher werden nicht Standardbasis.">
                  <div className="meta-chip-row">
                    {selectedRulesetGroup?.work === "5E" && selectedRulesetGroup?.version === "2014" ? (
                      <>
                        <StatusPill tone="ready">Beispiel fest verdrahtet</StatusPill>
                        <StatusPill tone="info">Character Builder Guide</StatusPill>
                        <StatusPill tone="default">Level-Up Guide</StatusPill>
                      </>
                    ) : (
                      <StatusPill tone="default">Noch kein fest verdrahteter Beispiel-Guide für dieses Regelwerk</StatusPill>
                    )}
                  </div>
                  <div className="builder-documents-grid">
                    {selectedRulesetGroup?.documents.map((document) => {
                      const checked = selectedDocumentIds.includes(document.id);
                      return (
                        <label className={`builder-document-card${checked ? " is-selected" : ""}`} key={document.id}>
                          <input
                            checked={checked}
                            onChange={() => {
                              setSelectedDocumentIds((current) =>
                                checked ? current.filter((id) => id !== document.id) : [...current, document.id]
                              );
                            }}
                            type="checkbox"
                          />
                          <div>
                            <strong>{document.name}</strong>
                            <p>{document.chunk_count} chunks indexed</p>
                          </div>
                        </label>
                      );
                    })}
                  </div>
                </Panel>
              </div>
            ) : (
              <div className="character-builder-layout">
                <section className="character-builder-chat">
                  <div className="character-builder-chat__head">
                    <div>
                      <strong>Builder Chat</strong>
                      <p>
                        Text und Sprache im Builder.
						{safeString(builderCharacter?.metadata.ruleset_work) === "5E" &&
						  safeString(builderCharacter?.metadata.ruleset_version) === "2014"
						  ? " The bundled 5E-compatible SRD 5.1 example guide is active."
                          : ""}
                      </p>
                    </div>
                    <Bot size={18} />
                  </div>
                  <div className="character-builder-transcript">
                    {builderMessages.map((message, index) => {
                      const speechKey = `${message.created_at}:${message.content}`;
                      return (
                        <article className={`builder-message builder-message--${message.role}`} key={`${message.created_at}-${index}`}>
                          <strong>{message.role === "assistant" ? "AI DM" : "You"}</strong>
                          <p>{message.content}</p>
                          {message.role === "assistant" ? (
                            <div className="builder-message__voice">
                              <div className="builder-message__voice-head">
                                <span className="builder-message__voice-label">
                                  <Volume2 size={14} />
                                  Freundliche Erzählerstimme
                                </span>
                                <button
                                  className="studio-button studio-button--ghost studio-button--inline"
                                  disabled={builderSpeechLoadingKey === speechKey}
                                  onClick={() => handlePlayBuilderSpeech(message)}
                                  type="button"
                                >
                                  {builderSpeechLoadingKey === speechKey ? "Lädt..." : "Abspielen"}
                                </button>
                              </div>
                              <audio
                                controls={builderSpeechActiveKey === speechKey && Boolean(builderSpeechUrl)}
                                ref={builderSpeechAudioRef}
                              >
                                Dein Browser kann die Character-Builder-Stimme nicht direkt abspielen.
                              </audio>
							  <p className="player-audio-note">KI-generierte Stimme / AI-generated voice</p>
                              {builderSpeechActiveKey === speechKey && builderSpeechUrl ? (
                                null
                              ) : null}
                              {builderSpeechActiveKey === speechKey && builderSpeechError ? (
                                <p className="player-audio-note">{builderSpeechError}</p>
                              ) : null}
                            </div>
                          ) : null}
                        </article>
                      );
                    })}
                    {isBuilderResponding ? (
                      <article className="builder-message builder-message--assistant builder-message--working">
                        <strong>AI DM</strong>
                        <div className="builder-thinking">
                          <span className="builder-thinking__dot" />
                          <span className="builder-thinking__dot" />
                          <span className="builder-thinking__dot" />
                          <p>Die KI arbeitet gerade an deinem Character Draft.</p>
                        </div>
                      </article>
                    ) : null}
                    <div ref={transcriptEndRef} />
                  </div>
                  <div className="character-builder-composer">
                    <textarea
                      onChange={(event) => setBuilderInput(event.target.value)}
                      placeholder="Beschreibe Konzept, Rolle, Herkunft oder beantworte die letzte Frage der KI."
                      value={builderInput}
                    />
                    {builderSTTStatus ? <p className="player-audio-note">{builderSTTStatus}</p> : null}
                    {builderSTTError ? <p className="player-audio-note">{builderSTTError}</p> : null}
                    <div className="button-row">
                      <button
                        className="studio-button studio-button--ghost"
                        disabled={isPending || isBuilderResponding || isBuilderTranscribing}
                        onClick={handleToggleBuilderRecording}
                        type="button"
                      >
                        {isBuilderRecording ? <MicOff size={16} /> : <Mic size={16} />}
                        {isBuilderRecording ? "Aufnahme beenden" : isBuilderTranscribing ? "Transkribiert..." : "Spracheingabe"}
                      </button>
                      <button
                        className="studio-button studio-button--primary"
                        disabled={isPending || isBuilderResponding || !builderInput.trim()}
                        onClick={handleSendBuilderMessage}
                        type="button"
                      >
                        <MessageSquare size={16} />
                        {isBuilderResponding ? "KI antwortet..." : "Nachricht senden"}
                      </button>
                    </div>
                  </div>
                </section>

                <section className="character-builder-sheet">
                  <div className="character-builder-sheet__section">
                    <div className="character-builder-sheet__head">
                      <div>
                        <strong>Live Character Sheet</strong>
                        <p>Rechts steht jetzt der eigentliche Bogen. Die KI füllt ihn Schritt für Schritt mit dir gemeinsam.</p>
                      </div>
                      {builderCharacter ? (
                        <StatusPill tone={safeString(builderCharacter.metadata.builder_status) === "draft" ? "warning" : "ready"}>
                          {safeString(builderCharacter.metadata.builder_status) || "draft"}
                        </StatusPill>
                      ) : null}
                    </div>
                    <CharacterSheetCanvas
                      activeTab={activeSheetTab}
                      assignment={assignment}
                      builderSkillKeys={builderSkillKeys}
                      isEditMode={isSheetEditMode}
                      onFieldChange={(field, value) => setSheetForm((current) => ({ ...current, [field]: value }))}
                      onSave={handleApplySheetPatch}
                      onToggleEditMode={() => setIsSheetEditMode((current) => !current)}
                      onTabChange={setActiveSheetTab}
                      saveDisabled={isPending || !builderCharacter}
                      sheetForm={sheetForm}
                    />
                  </div>

                  <div className="character-builder-sheet__section">
                    <strong>Buchbasis</strong>
                    <div className="meta-chip-row">
                      {(builderCharacter ? splitMetadataList(builderCharacter.metadata.selected_document_names) : []).map((name) => (
                        <StatusPill key={name} tone="default">{name}</StatusPill>
                      ))}
                    </div>
                  </div>
                </section>
              </div>
            )}

            <div className="modal-card__footer">
              <span className="modal-card__spacer">{isPending ? "Saving..." : ""}</span>
              <div className="button-row modal-card__actions">
                <button className="studio-button studio-button--ghost" onClick={closeBuilder} type="button">
                  Close
                </button>
                {builderStep === "start" ? (
                  <button className="studio-button studio-button--primary" disabled={isPending || selectedDocumentIds.length === 0} onClick={handleStartBuilder} type="button">
                    <Plus size={16} />
                    Builder starten
                  </button>
                ) : (
                  <button className="studio-button studio-button--primary" disabled={isPending || !builderCharacter} onClick={handleFinishBuilder} type="button">
                    <Check size={16} />
                    Als ready markieren
                  </button>
                )}
              </div>
            </div>
          </section>
        </div>
      ) : null}

      {builderReferencePopup && builderReferenceDocument ? (
        <div className="player-popup">
          <div className="player-popup__backdrop" onClick={() => setBuilderReferencePopup(null)} />
          <div className="player-popup__panel player-popup__panel--meta">
            <div className="player-popup__header">
              <strong>{builderReferencePopup.documentName}</strong>
              <button className="studio-button studio-button--ghost" onClick={() => setBuilderReferencePopup(null)} type="button">
                Schließen
              </button>
            </div>
            <div className="player-popup__document">
              <p>
                {builderReferencePopup.page ? `Geöffnet auf Seite ${builderReferencePopup.page}.` : "Das Regelwerk ist geöffnet."}
              </p>
              <div className="player-popup__embed-wrap">
                <iframe className="player-popup__embed" src={builderReferenceFileUrl} title={builderReferencePopup.documentName} />
              </div>
            </div>
          </div>
        </div>
      ) : null}

      {isBuilderOpen && isRollModalOpen ? (
        <div className="modal-overlay" onClick={() => setIsRollModalOpen(false)} role="presentation">
          <section
            aria-modal="true"
            className="modal-card modal-card--roll"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div className="modal-card__header">
              <div>
                <p className="eyebrow">Dice Step</p>
                <h2>Jetzt 7 mal 4d6 würfeln</h2>
                <p className="muted-copy">
                  Die KI hat den Attribut-Wurf aufgerufen. Wir machen jetzt jeden Wurf gemeinsam: <strong>7 mal 4 x d6</strong> werfen, per Kamera
                  auswerten lassen, falls nötig korrigieren und dann den einzelnen Wurf bestätigen. Danach wird der <strong>schwächste Gesamtwurf
                  automatisch gestrichen</strong> und die besten sechs Werte gehen zurück an die KI.
                </p>
              </div>
              <button className="studio-button studio-button--ghost studio-button--inline" onClick={() => setIsRollModalOpen(false)} type="button">
                Close
              </button>
            </div>

            <div className="roll-modal-layout">
              <section className="roll-camera-panel">
                <div className="character-builder-sheet__head">
                  <div>
                    <strong>Aktueller Wurf {currentRollIndex + 1}</strong>
                    <p>Lege nur die 4d6 für diesen Wurf in das Kamerabild.</p>
                  </div>
                  <StatusPill tone={rollCameraStatus === "ready" ? "ready" : rollCameraStatus === "error" ? "warning" : "info"}>
                    {rollCameraStatus}
                  </StatusPill>
                </div>
                <div className="meta-chip-row">
                  <StatusPill tone="info">{confirmedRolls.filter(Boolean).length} / {guidedRollCount} bestätigt</StatusPill>
                  <StatusPill tone="default">Schritt {currentRollIndex + 1} von {guidedRollCount}</StatusPill>
                </div>
                <div className="camera-preview-shell camera-preview-shell--roll">
                  <video autoPlay className="camera-preview" muted playsInline ref={rollVideoRef} />
                  <canvas hidden ref={rollCanvasRef} />
                  {rollCameraStatus !== "ready" ? (
                    <div className="camera-overlay camera-overlay--idle">
                      <span>No live preview yet</span>
                    </div>
                  ) : null}
                </div>
                <p className="muted-copy">{rollCameraMessage}</p>
                <div className="button-row">
                  <button className="studio-button studio-button--ghost" onClick={() => void handleStartRollCamera()} type="button">
                    Kamera starten
                  </button>
                  <button className="studio-button studio-button--primary" disabled={rollCameraStatus !== "ready" || isRollCapturing} onClick={() => void handleDetectCurrentRoll()} type="button">
                    {isRollCapturing ? "Auswertung läuft..." : "Kamera auswerten"}
                  </button>
                </div>
              </section>

              <section className="roll-entry-panel">
                <div className="roll-step-grid">
                  {currentRollSets().map((set, rowIndex) => (
                    <article className={`roll-step-card${rowIndex === currentRollIndex ? " is-active" : ""}`} key={`roll-set-${rowIndex}`}>
                      <div className="character-builder-sheet__head">
                        <strong>Wurf {rowIndex + 1}</strong>
                        {confirmedRolls[rowIndex] ? <StatusPill tone="ready">bestätigt</StatusPill> : null}
                      </div>
                      <div className="roll-dice-row">
                        {set.map((value, dieIndex) => (
                          <label className="roll-die-input" key={`roll-${rowIndex}-${dieIndex}`}>
                            <span>d6 #{dieIndex + 1}</span>
                            <input
                              max={6}
                              min={1}
                              onChange={(event) =>
                                handleRollSetChange(rowIndex, dieIndex, Math.max(1, Math.min(6, Number(event.target.value) || 1)))
                              }
                              type="number"
                              value={value || ""}
                            />
                          </label>
                        ))}
                      </div>
                      {rowIndex === currentRollIndex ? (
                        <div className="button-row">
                          <button className="studio-button studio-button--ghost" onClick={handleConfirmCurrentRoll} type="button">
                            Diesen Wurf bestätigen
                          </button>
                        </div>
                      ) : null}
                    </article>
                  ))}
                </div>
              </section>
            </div>

            <div className="modal-card__footer">
              <span className="modal-card__spacer">
                Nach jedem bestätigten Wurf springt der nächste Schritt automatisch an. Nach Wurf 7 werden die besten 6 Werte übernommen und der schlechteste Wurf gestrichen.
              </span>
              <div className="button-row modal-card__actions">
                <button className="studio-button studio-button--ghost" onClick={() => setIsRollModalOpen(false)} type="button">
                  Zurück zum Builder
                </button>
              </div>
            </div>
          </section>
        </div>
      ) : null}
    </div>
  );
}
